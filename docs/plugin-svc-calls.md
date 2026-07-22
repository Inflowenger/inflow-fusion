# Plugin-originated service calls — `request/svc.<ACTION>`

A plugin written with the **inflow-plugin-sdk** can call back into *your backend*
mid-job:

```go
// inside a plugin's RequestHandler (inflow-plugin-sdk)
resp := job.CmdSvcCall(
    "add.db.record",                     // action — what it asks your backend to do
    map[string]any{"rows": batch},       // data   — the payload
    map[string]any{"table": "events"},   // op     — operation metadata
)
```

The runtime (Fractal) receives this as the job command
`inflow.cpu.<PLUGIN_ID>.<JOB_ID>.request/svc.add.db.record`, cuts the inflowv1
command prefix, and re-issues it as a plain NATS request addressed to the bare
**action** — `add.db.record` — on the **plugin space** (the builtin-plugins
account). Your backend implements the other end: subscribe to the action subject
on that account and respond.

The canonical use is a **feeder plugin**: a plugin that ingests from an external
system and pushes into yours — appending to a DB, feeding a store — without the
flow author wiring an extrinsic node for it.

## Why the action is not an extrinsic subject

This is a deliberate indirection. The plugin names an *action* (`log`,
`add.db.record`, …); it never addresses your registered extrinsic service
subjects (the `inflow.req.*` defaults or anything you registered with
`svcHandler.ImplHandlerOnSubject`) — those live on your backend's default infra
connection, and plugin calls don't arrive there. That keeps this surface safe:

- the action catalog is **yours** — you decide which actions exist and subscribe
  only to those; an action nobody serves simply times out and fails the node;
- a plugin can never reach an internal service the flow author didn't intend to
  expose to plugins;
- every call is **origin-tagged**, so grant policy stays in your handler.

## Wire contract

| | |
|---|---|
| Transport | NATS request/response on the **builtin-plugins account** (plugin space) |
| Subject   | the action, verbatim — `log`, `add.db.record`, … |
| Header    | `origin: plugin:<node title>` |
| Body      | `models.ExtSvcRequestBody` — `{"data": …, "op": {…}, "node": {…}}` |
| Response  | raw bytes; relayed verbatim to the plugin as `CmdSvcCall`'s return value |

Body fields: `Data` is the plugin's payload, `OperationData` (`op`) its operation
metadata, and `Node` the full node the plugin is executing as — the same envelope
an extrinsic node sends, which is exactly why a granted plugin call behaves like
running an extrinsic from inside the plugin.

**Refusing a call:** reply with an error payload (e.g. `{"error":"not granted"}`)
— the plugin receives it as `CmdSvcCall`'s return and can react. If instead
nothing responds (no subscriber, timeout), the engine fails the plugin node with
a bad-request conclusion and the job ends — so always respond, even to refuse.

## Implementing the handler

Get a connection on the builtin-plugins account with
`spaces.GetCredOnBuiltinPluginAcc` (requires `inflow.InitBackend` to have run;
the connection is cached per account, so one connection serves every action you
register), then subscribe to your action subjects on it:

```go
import (
	"strings"

	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	spaces "github.com/Inflowenger/inflow-fusion/spaces"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

infra, err := spaces.GetCredOnBuiltinPluginAcc("my-backend-svc")
if err != nil { /* handle */ }
nat, err := natsHandler.GetNatsByInfraIsolate(*infra)
if err != nil { /* handle */ }
conn := nat.GetConnection()

_, err = conn.Subscribe("add.db.record", func(msg *nats.Msg) {
	var req models.ExtSvcRequestBody
	if err := sonic.Unmarshal(msg.Data, &req); err != nil {
		msg.Respond([]byte(`{"error":"bad request"}`))
		return
	}

	// Grant check — every plugin-originated call carries the origin header.
	origin := msg.Header.Get("origin") // "plugin:<node title>"
	if !strings.HasPrefix(origin, "plugin:") || !granted(origin, "add.db.record") {
		msg.Respond([]byte(`{"error":"not granted"}`))
		return
	}

	// req.Data / req.OperationData / req.Node — do the work.
	msg.Respond([]byte(`{"status":"saved"}`))
})
```

Notes:

- **Wildcards work** — subscribe `add.db.*` and dispatch on `msg.Subject`, the
  same pattern `ImplHandlerOnSubject` uses for extrinsic services.
- **Scale with queue groups** — `conn.QueueSubscribe(action, "backend", handler)`
  lets several backend instances share one action.
- **Publish your action catalog** to plugin authors: the action string is the
  whole contract between a plugin's `CmdSvcCall` and your handler.

## Sequence

```
plugin (go-plugin-sdk)                Fractal (engine)                 your backend
  │                                        │                              │
  │ REQ inflow.cpu.<id>.<job>.request/svc.add.db.record                   │
  │───────────────────────────────────────►│                              │
  │                                        │ cut `request/svc.` prefix    │
  │                                        │ + origin header, + node      │
  │                                        │ REQ add.db.record            │
  │                                        │─────────────────────────────►│  grant check,
  │                                        │                              │  handle
  │                                        │ reply (raw bytes)            │
  │                    reply relayed       │◄─────────────────────────────│
  │◄───────────────────────────────────────│                              │
  │  = CmdSvcCall's return value           │                              │
```

## Next

- [nodes/plugin.md](nodes/plugin.md) — the plugin node itself and its two
  subject registers
- [nodes/extrinsic.md](nodes/extrinsic.md) — the extrinsic node this call
  deliberately does *not* impersonate
- [infra.md](infra.md) — your backend's other NATS surfaces (flow/context
  services, extrinsic services)
- inflow-plugin-sdk `docs/jobs-and-commands.md` — the calling side
  (`job.CmdSvcCall`) and `docs/protocol-inflowv1.md` for the wire subject

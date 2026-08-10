# Extrinsic node — `models.ExtrinsicNodeType`

The **"call my own backend"** primitive. When the engine reaches an extrinsic node
it **publishes to a NATS subject you registered and uses the reply as the node's
output** — one request in, one response out. This is how a flow invokes your domain
logic (write a row, compute something, hit an internal service) without the engine
knowing anything about your storage or code.

```
flow reaches node ──► engine publishes to subject ──► your handler runs ──► reply = node output
```

Unlike a [Plugin](plugin.md) (a rich, long-running external process with its own UI
and job lifecycle), an extrinsic node is deliberately **thin**: a plain
request/reply against a subject your backend owns. Registration lives in this SDK's
`svcHandler`; the wire-level detail is in [../infra.md](../infra.md).

## Rule data

```go
type ExtrinsicRule struct {
	InfraIsolated     InfraIsolated  // optional: run over an isolated account instead of default
	Subject           string         // NATS subject to publish to
	OperationData     map[string]any // `op` — the operation payload; may carry {{ }} variables
	ReqTimeoutSecound uint8          // default 5
}
```

The handler receives `models.ExtSvcRequestBody{Data, OperationData, Node}`, where
`Data` is the **scoped context slice** the node was pointed at and `OperationData`
(`op`) is this rule's payload. They are different things — don't read `op` out of
`Data`.

## Builder

```go
n := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.orders",
	nodes.WithOpData(map[string]any{"table": "orders"}))
```

`WithIsolated(...)` scopes the call to a specific NATS account/space (the same
`spaces` concept plugins use — see the README's provisioning section). If
`InfraIsolated.Account` is empty, it defaults to the shared `inflow` account
connection. `WithOpData(...)` sets the `op` payload.

## `op` is not static — `{{ }}` variables

Before publishing, the engine resolves **runtime variables** in `op` against the
flow context as it stands at that moment. A handler therefore receives values,
never templates. This is what lets one node definition carry data that only
exists mid-run.

```go
nodes.WithOpData(map[string]any{
	"table":   "orders",                  // plain value, passed through
	"limit":   "{{$.cfg.limit}}",         // → 25       (a number, not "25")
	"who":     "user {{$.user.name}}",    // → "user Mehdi"
	"current": "{{$this}}",               // → the slice this run is scoped to
})
```

Four behaviours worth designing around, because they decide the shape your
handler should expect:

| | |
|---|---|
| **Whole-value placeholder keeps its type** | `"{{$.cfg.limit}}"` arrives as the JSON number `25`, `"{{$.cfg.on}}"` as `true`, an object path as an object. Unmarshal `op` into a typed struct accordingly — a `string` field here will fail. |
| **A placeholder inside text is interpolated** | `"user {{$.user.name}}"` → `"user Mehdi"`, always a string. Objects render as their JSON. |
| **Only root-level string values are walked** | `{"a": {"b": "{{$.x}}"}}` is **not** resolved — the placeholder reaches your handler verbatim. Keep templated fields at the top level of `op`. |
| **A path that matches nothing is not an error** | The node does not fail. A whole-value placeholder becomes `{}`; inside text it interpolates as `{}` (`"value:{{$.nope}}"` → `"value:{}"`). Validate in the handler if a field is required. |

### `$this` — the node's current location

A path may start at **`$this`**, a root of inflow's own outside the JSON path
spec, meaning *the location this run was handed* — the slice the node's `scope`
selected. Because a scope matching many locations (`$.orders[*]`) runs the node
once per location, `$this` is how `op` follows the run:

```go
nodes.WithOpData(map[string]any{
	"orderId": "{{$this.id}}",   // this pass's order, not a fixed index
})
```

`{{$.orders[0].id}}` would send the same order on every pass. A path without
`$this` is untouched, and `$thisOne` is an ordinary field name, not the keyword.

The same resolution — including `$this` — applies to the `op` a plugin sends
through `CmdSvcCall`; see [../plugin-svc-calls.md](../plugin-svc-calls.md).

## Registering the service side

An extrinsic node is only half the story — something must answer on the subject.
You register a handler with `svcHandler.ImplHandlerOnSubject(name, topic, handler)`:

```go
svcHandler.ImplHandlerOnSubject("db_handler", svcHandler.SvcTopic("my.internal.svc.persist.*"),
	func(header nats.Header, data []byte) ([]byte, error) {
		table := strings.Split(header.Get("recv_subject"), ".")[4]
		saveOnTable(table, data)
		return []byte(`{"status":"saved"}`), nil // reply becomes the node's output
	})
```

### How the mechanism works

1. **Register.** `ImplHandlerOnSubject` subscribes on `topic.ConvertToSubscribe()` —
   every `{param}` placeholder becomes a NATS `*` wildcard. So
   `svc.add.issue.{TABLE_NAME}` subscribes as `svc.add.issue.*`, letting **one
   handler serve many logical topics** (one per table).
2. **Recover parameters.** Before calling your handler the wrapper sets a
   `recv_subject` header to the *exact* subject the message arrived on; the handler
   splits it to recover the concrete params (e.g. the table name).
3. **Reply = output.** Whatever bytes the handler returns are sent back as the
   reply; on error it responds `{"error": "..."}`. The engine uses that reply as the
   node's output.
4. **Local registry.** Registrations are tracked in a process-local map keyed by
   `name` (`svcHandler.GetSvc(name)` / `GetAllSvcs()`), so a compiler can resolve a
   logical service name back to its subject pattern and fill placeholders with
   `SvcTopic.MakeReqSubjectWithParams(args)`.

## Control commands in the reply — an extrinsic can route

A reply is normally just the node's output, but it can also carry a **command**
that rewrites the node's outgoing edges before the flow moves on. `svcHandler`
ships the two helpers that build one:

```go
svcHandler.ImplHandlerOnSubject("risk", svcHandler.SvcTopic("svc.risk.check"),
	func(header nats.Header, data []byte) ([]byte, error) {
		if overLimit(data) {
			// keep only edges tagged "escalate"; {"score": 91} is still the output
			return svcHandler.FilterNextResponse(map[string]any{"score": 91}, []string{"escalate"})
		}
		return svcHandler.FilterNextResponse(map[string]any{"score": 12}, []string{"approve"})
	})
```

| Helper | Wire form | Effect on `Next` |
|---|---|---|
| `FilterNextResponse(data, tags)` | `{…, "_cmd":"next_tags", "_next_filter":["escalate"]}` | deactivate all, re-activate only edges carrying one of those tags |
| `StopHereResponse(data)` | `{…, "_cmd":"stop"}` | deactivate **every** edge — the branch ends here |
| plain bytes | no `_cmd` | nothing changes; edges are followed as compiled |

The keys are `models.SvcCmdResposeKey` (`_cmd`) and
`models.SvcCmdResponseNextFilterKey` (`_next_filter`); commands are
`models.CmdNextFilter` and `models.CmdStop`. Either way the rest of the reply is
still written into context at the node's `Key`, and a missing or malformed `_cmd`
is deliberately a no-op.

So an extrinsic node is only linear by convention. Give it several **tagged**
`Next` entries and your own backend decides the branch — the same mechanism a
[Contract](contract.md) uses, with the decision living in your service instead of
in a compiled rule. Note the inspector renders a single output handle for this
type, so authoring tagged outputs on an extrinsic node needs a palette that draws
them (the compiler already carries any edge's `Data.Tags` through). See
[../routing.md](../routing.md).

## Reference example: inspector-api

`inflow-inspector-api` is itself an instance built on this platform (via
inflow-fusion), which is why it registers its own extrinsic services. In
`inspector-api/inflow/port.go` → `LoadSvcNodehandlers`:

```go
func LoadSvcNodehandlers() error {
    svc_sub1 := "svc.add.issue.{TABLE_NAME}"
    err := svcHandler.ImplHandlerOnSubject("exports_db", svcHandler.SvcTopic(svc_sub1),
        func(header nats.Header, data []byte) ([]byte, error) {
            subject := header.Get("recv_subject")   // the concrete subject it arrived on
            table := strings.Split(subject, ".")[3] // recover {TABLE_NAME}
            return []byte(fmt.Sprintf(`{"status":"saved successfully on %s table"}`, table)), nil
        })
    // ...
}
```

That single registration turns `svc.add.issue.{TABLE_NAME}` into a callable service
named `exports_db`. A flow node pointed at it (with a concrete table) invokes the
handler, and `{"status":"..."}` becomes the node's output.

## Frontend representation (inspector)

Palette type `extrinsic` (`ExtrinsicNode.vue` + `ExtrinsicDrawer.vue`). Relevant
`node.data`:

| Field | Backend | Meaning |
|---|---|---|
| `serviceTopic` | `Subject` | the subject to publish to (often resolved from a logical service name) |
| `operationData{}` | `OperationData` (`op`) | key/value payload sent alongside the scoped data; values may carry `{{ }}` variables, resolved at run time |
| `timeout` | `ReqTimeoutSecound` | per-request timeout (seconds) |

The node has one input and one output handle — it's a linear step. `hasSettings`
lights it up (green) once any of the above is set.

## Compiles to

The compiler hook reads those fields, resolves the subject (often via
`svcHandler.GetSvc(name)` + `SvcTopic.MakeReqSubjectWithParams` to fill
placeholders), and builds `nodes.NewExtrinsicSvcNode(subject, …)` →
`models.ExtrinsicRule`. See `dev-backend/inflow/compiler.go`'s `NODE_MY_A` case and
[../compilers/vueflow.md](../compilers/vueflow.md).

## Extrinsic vs. Plugin

| | **Extrinsic** (this node) | **Plugin** ([plugin.md](plugin.md)) |
|---|---|---|
| Purpose | Call an **internal** service the backend owns | Full external **adapter** node |
| Call model | Engine publishes; handler's return is the output (req/reply) | Two-phase job: `jobId` ack, then async progress/context/done |
| UI | No per-node form builder | Own JSON Forms UI per action |
| Lifecycle | A plain subscription/handler | Long-running process; progress, context read/write, stop |
| Registered via | `svcHandler.ImplHandlerOnSubject(name, topic, handler)` | Plugin SDK `p.AddAction(...)` |
| Node model | `models.ExtrinsicRule` | `models.PluginRule` |

## Next

- [plugin.md](plugin.md) — the richer external-process node
- [../infra.md](../infra.md) — the wire-level subjects (see "your backend's own extrinsic services")
- [../compilers/vueflow.md](../compilers/vueflow.md) — resolving a service by logical name in the hook
- [from-frontend.md](from-frontend.md) · [../nodes.md](../nodes.md)

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
	Data              map[string]any // static payload merged into the request
	ReqTimeoutSecound uint8          // default 5
}
```

## Builder

```go
n := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.orders")
```

`WithIsolated(...)` scopes the call to a specific NATS account/space (the same
`spaces` concept plugins use — see the README's provisioning section). If
`InfraIsolated.Account` is empty, it defaults to the shared `inflow` account
connection.

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
| `operationData{}` | `Data` | static key/value payload merged into the request |
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

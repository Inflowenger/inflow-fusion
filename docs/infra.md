# Infra & wire reference

The concrete REST endpoints and NATS subjects `inflow-fusion` speaks. Useful if you're implementing the infra side, debugging traffic, or writing a backend in another language.

## Authentication

Every REST call from the SDK to infra carries:

```
Authorization: Bearer <JWT>
```

The JWT is minted locally (`jwt.SigningMethodHS256`, claim `{"admin": true}`) and signed with `INFLOW_INFRA_JWT_SECRET` — infra and your backend must share this secret out of band. There is no login flow; possession of the secret is the credential. The same helper (`makeTokenWithHs256`) is reused to authenticate against individual engine instances when they have their own `jwt_secret` configured in their `Portal`.

## REST: your backend → infra

| Method | Path | Called by | Purpose |
|---|---|---|---|
| `GET` | `/account/inflow/cred` | `inflow.InitBackend` → `getCred` | Fetch the NATS credential (`models.Cred`) for the `inflow` account |
| `GET` | `/inflow/resource?per_page={n}` | `inflow.InflowWire.ReloadResources` | List registered engine instances (`models.RegisteredInflow`) |
| `GET` | `/account/id/{accountId}` | `spaces.fetchAccount` (via `GetAccountById`/`GetAccountCred`) | Fetch an `Account` (seed, pub key, policy) by id, used to mint scoped plugin credentials |

Response envelopes are `{"data": ..., "error": ...}`; the SDK unwraps `.Data` and treats a non-null `.Error` or a non-2xx status as failure.

## REST: your backend → engine instance

| Method | Path | Called by | Purpose |
|---|---|---|---|
| `POST` | `{engineUrl}/engine` | `Process.Exec` | Start a process: body is `models.ProcessRequest` |
| `POST` | `{engineUrl}/ps/stop/{pid}` | `Process.Stop` / `inflow.StopProcess` | Stop a running process |

`engineUrl` comes from the round-robin pool (`inflow.GetResourceCandid`); if it has no scheme it's prefixed with `http://`, and if it has no port it's suffixed with `models.INFLOW_REST_PORT` (`9001`).

### `ProcessRequest`

```go
type ProcessRequest struct {
	Context      ContextTopicsPattern // NATS subjects the engine should use for this run's context
	Flow         FlowEngine           // NATS subject to fetch the flow definition
	PID          string               // process id, auto-generated (UUID) if empty
	StartNodeIds []string             // one or more nodes to start at (see below)
	Settings     Settings             // timeouts, node execution cap
	Meta         map[string]string    // free-form; also used to fill subject templates
	Resume       bool                 // continue an earlier run over the same context (see Resume)
}

type Settings struct {
	RequestTimeOut   int64  // per NATS request, seconds (default 5)
	ExecuteTimeOut   int64  // whole process, seconds (default 3600)
	ProcessNodeLimit uint16 // safety cap on nodes visited (default 500)
}
```

`Context.Getter`/`Context.Setter`/`Flow.GetFlow` are subject templates auto-filled from `contextId`/`flowId`/any `Meta` entries unless you set them explicitly (see [nodes.md](nodes.md) and `inflow.NewProcess`).

#### Start nodes

`StartNodeIds` is where the run enters. A **single** id is the historical trigger semantics: that node does not itself run — its successors are queued and it is recorded done. **Multiple** ids (or any resume) queue the named nodes directly, so they *run* as tasks. Build it with `inflow.NewProcess(ids)` or add more with `WithStartNode`.

#### Resume

`Resume` continues an earlier run over the **same context** (same `contextId`, typically the same `PID`). The pattern: a node such as a `continue after` gate returns `{"_cmd":"stop"}`, the process stops, and later — after whatever delay your backend schedules, which the engine knows nothing about — you start a new process whose `StartNodeIds` are the **successors** of that terminated node, with `Resume: true` (`inflow.WithResume()`).

Why the flag matters: node results already live in the context document, so a plain restart continues fine on a linear path. But a *join* downstream of the resume point checks the completion **generation** of its dependencies, not their data — and a dependency that completed before the stop is not re-run from the resume point. Without `Resume`, that join waits until the process times out. With it, the engine seeds the traversal snapshot the previous run left in the context header, so the join sees its already-completed dependencies and fires.

`Resume` is additive: omit it and the request behaves exactly as before. It only takes effect when a matching snapshot is present in the context header **and** the flow definition is unchanged since it was taken (the engine gates on a structural signature and falls back to a blank continue on drift).

### `ProcessResponse`

```go
type ProcessResponse struct {
	Data struct {
		PID string
	}
	Error any
}
```

## NATS: engine ↔ your backend

Your backend subscribes to three subjects on init (`inflow.InflowWire.connectAndListen`). The `{param}` placeholders are NATS wildcard subjects (`*`) when subscribing, and get filled with real values when the engine publishes a request.

| Subject template | Direction | Handler | Purpose |
|---|---|---|---|
| `inflow.req.flow.get.{flowId}` | engine → backend | `IInflowService.RetrieveFlow` | Return the compiled `models.Flow` for this id |
| `inflow.req.context.get.{contextId}` | engine → backend | `IInflowService.RetrieveContext` | Return the current `models.ContextDoc` |
| `inflow.req.context.set.{contextId}` | engine → backend | `IInflowService.UpdateContext` | Persist an updated `models.ContextDoc` |

These defaults live in `svcHandler.DefaultGetFlowSvc` / `DefaultGetContextSvc` / `DefaultSetContextSvc` and can be overridden by constructing your own `svcHandler.SvcTopic` patterns.

### `ContextDoc`

```go
type ContextDoc struct {
	Data   string         `json:"data"`   // opaque to the SDK — typically JSON-encoded per-node scope data
	Header map[string]any `json:"header"`
}
```

`Header` is per-context memory that survives across runs. Your backend stores and returns it verbatim — it is a `map[string]any`, so new reserved keys are forward-compatible. Two kinds of entry live there, both engine-managed; treat them as opaque:

- **Node registry** — one entry per call site (keyed `"<scope>:<nodeId>"`), holding what a node left for the next run, e.g. a plugin's `jobId` so a job that outlived the process can be reconnected. Handed back to the plugin on its next handshake as `_registry`.
- **Traversal snapshot** (`_sched`) — the previous run's completed node generations and join watermarks, written on every finish and read back only when a request sets `Resume`. This is what lets a continuation's downstream join see its already-done dependencies. It is keyed by node id and gated by a flow signature, so it is inert against a changed flow.

Neither key needs anything from your backend beyond storing the header as-is.

## NATS: your backend's own extrinsic services

You register arbitrary subjects with `svcHandler.ImplHandlerOnSubject(name, topic, handler)`. Internally this subscribes on `topic.ConvertToSubscribe()` (all `{param}` placeholders replaced with `*`), and before invoking your handler it sets a `recv_subject` header to the exact subject the message arrived on — this is how a wildcard subject like `my.internal.svc.persist.*` lets one handler serve many logical topics (e.g. `.persist.orders` vs `.persist.tasks`):

```go
svcHandler.ImplHandlerOnSubject("db_handler", svcHandler.SvcTopic("my.internal.svc.persist.*"),
	func(header nats.Header, data []byte) ([]byte, error) {
		table := strings.Split(header.Get("recv_subject"), ".")[4]
		// ...
		return []byte(`{"status":"saved"}`), nil
	})
```

Handlers registered this way are also tracked in a process-local registry (`svcHandler.GetSvc` / `GetAllSvcs`), keyed by the `name` argument — useful if a compiler needs to resolve a logical service name back to its subject pattern (see [compilers/vueflow.md](compilers/vueflow.md)).

## NATS: plugin-originated service calls

Plugins (inflow-plugin-sdk) can call your backend mid-job via `job.CmdSvcCall(action, data, op)`. These requests do **not** arrive on the infra connection above — the engine forwards them on the **plugin space** (builtin-plugins account), addressed to the bare action subject (`add.db.record`, `log`, …), origin-tagged `plugin:<node title>`, with a `models.ExtSvcRequestBody` body. Subscribe there via `spaces.GetCredOnBuiltinPluginAcc` + `natsHandler.GetNatsByInfraIsolate` — full wire contract and handler recipe in [plugin-svc-calls.md](plugin-svc-calls.md).

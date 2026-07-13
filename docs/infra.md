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
	StartNodeId  string
	Settings     Settings             // timeouts, node execution cap
	Meta         map[string]string    // free-form; also used to fill subject templates
}

type Settings struct {
	RequestTimeOut   int64  // per NATS request, seconds (default 5)
	ExecuteTimeOut   int64  // whole process, seconds (default 3600)
	ProcessNodeLimit uint16 // safety cap on nodes visited (default 500)
}
```

`Context.Getter`/`Context.Setter`/`Flow.GetFlow` are subject templates auto-filled from `contextId`/`flowId`/any `Meta` entries unless you set them explicitly (see [nodes.md](nodes.md) and `inflow.NewProcess`).

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

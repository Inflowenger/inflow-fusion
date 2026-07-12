# inflow-fusion

**The Go SDK for wiring a backend service into the Inflowenger workflow platform.**

`inflow-fusion` is not a standalone application — it's a library you import into your own Go backend so that backend can participate in Inflowenger: answer the platform's questions about flows and execution state, expose your own domain logic as callable steps inside a workflow, and kick off/stop workflow runs.

> **Status:** early-stage / pre-1.0. Public APIs (especially in `models`) may still change between releases.

---

## What is Inflowenger?

Inflowenger is a workflow automation platform. A *flow* is a graph of *nodes* (branching logic, code steps, calls out to your services, sub-flow jumps, third-party plugins) typically authored in a visual editor. The platform's job is to **compile and execute that graph**; your backend's job is to **own the data**: the flow definitions, the execution context (the working data a running process reads/writes), and any business logic the flow needs to call out to.

The two sides talk over [NATS](https://nats.io): the execution engine asks your backend for a flow definition or the current context, and your backend answers. This SDK implements that wiring so you only have to fill in the parts that are specific to your product — see [docs/architecture.md](docs/architecture.md) for the full picture and [docs/infra.md](docs/infra.md) for the exact subjects/endpoints involved.

## What does this SDK give you?

- **`inflow.InitBackend`** — registers your backend with the Inflowenger infra control-plane, pulls NATS credentials, and subscribes to the subjects the engine uses to ask for flows/context.
- **`IInflowService`** — the one interface you implement (`RetrieveFlow`, `RetrieveContext`, `UpdateContext`) to plug in your own storage.
- **`nodes`** — typed builders for every node kind a flow can contain: code (JS/OPA), contract/rule (branching), extrinsic service calls, plugins, goto, void.
- **`svcHandler`** — register your own domain functions as NATS-callable "extrinsic" services that flow nodes can invoke.
- **`compilers/vueFlow`** — a generic graph walker that turns an exported [Vue Flow](https://vueflow.dev) graph (nodes + edges) into the flat, executable node map the engine expects, with a hook so *you* decide how your product's custom node types map to `inflow-fusion` node types.
- **`inflow.NewProcess`** — starts (or stops) a workflow run on one of the registered engine instances, load-balanced round-robin.
- **`spaces`** — issues narrowly-scoped NATS credentials for isolated integrations (e.g. third-party plugins), so a plugin can only publish/subscribe on subjects it owns.

See [docs/nodes.md](docs/nodes.md) for the full node reference and [docs/compilers](docs/compilers) for compiling a frontend graph into nodes.

## Where it fits

```
 ┌────────────────────┐        REST (cred, resources, accounts)      ┌───────────────────────────┐
 │  Inflowenger Infra  │ <───────────────────────────────────────────│   Your Backend             │
 │  (control plane)    │                                             │   (imports inflow-fusion)  │
 │                     │        NATS: get flow / get,set context     │                             │
 │                     │ <───────────────────────────────────────────│  implements IInflowService  │
 └─────────┬───────────┘                                             └──────────────┬─────────────┘
           │ REST: registered engine instances                                      │ REST: POST /engine, /ps/stop
           ▼                                                                        ▼
 ┌────────────────────┐        NATS: get flow / get,set context      ┌───────────────────────────┐
 │  Inflow Engine(s)   │ ───────────────────────────────────────────>│   (back to Your Backend)   │
 │  (execution runtime)│                                             └───────────────────────────┘
 │                     │        NATS: isolated plugin account
 │                     │ ───────────────────────────────────────────> Plugins (jira, slack, ...)
 └────────────────────┘
```

Your backend never executes a flow itself — it answers questions (what's this flow, what's this process's context) and, if a node calls out to it, runs a bit of business logic. The engine instances do the traversal. Full details in [docs/architecture.md](docs/architecture.md).

`github.com/Inflowenger/dev-backend` is a real consumer of this SDK: it implements `IInflowService` on top of its own DB (`dev-backend/inflow/wire.go`), and defines its own Vue Flow node→`inflow-fusion` node mapping (`dev-backend/inflow/compiler.go`).

## Installation

```bash
go get github.com/Inflowenger/inflow-fusion
```

Requires the Go version pinned in [go.mod](go.mod).

## Configuration

The SDK reads two environment variables by default (both can be overridden with `inflow.With...` options):

| Env var | Purpose |
|---|---|
| `INFLOW_INFRA_API` | Base URL of the Inflowenger infra REST API (e.g. `http://localhost:8022`) |
| `INFLOW_INFRA_JWT_SECRET` | HMAC secret shared with infra, used to mint the bearer token for infra REST calls |

## Quickstart

```go
package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Inflowenger/inflow-fusion/inflow"
	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/Inflowenger/inflow-fusion/nodes"
	"github.com/Inflowenger/inflow-fusion/svcHandler"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

// MyBackend implements inflow.IInflowService.
type MyBackend struct{}

func (b *MyBackend) RetrieveFlow(msg *nats.Msg) {
	flowId := strings.Split(msg.Subject, ".")[4]
	flow := loadFlowFromDB(flowId) // your own storage
	b, _ := sonic.Marshal(flow)
	msg.Respond(b)
}

func (b *MyBackend) RetrieveContext(msg *nats.Msg) {
	ctx := loadContextFromDB(/* contextId from msg.Subject or header */)
	b, _ := sonic.Marshal(ctx)
	msg.Respond(b)
}

func (b *MyBackend) UpdateContext(msg *nats.Msg) {
	var doc models.ContextDoc
	sonic.Unmarshal(msg.Data, &doc)
	saveContextToDB(doc) // your own storage
	msg.Respond([]byte(`accepted`))
}

func main() {
	backend := &MyBackend{}
	if err := inflow.InitBackend(inflow.WithImplementedBackendBy(backend)); err != nil {
		panic(err)
	}
	slog.Default().Info("inflowenger backend initialized")

	// Expose a domain action to running flows via an Extrinsic node.
	persistNode := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.*")
	svcHandler.ImplHandlerOnSubject("db_handler", svcHandler.SvcTopic(persistNode.Subject),
		func(header nats.Header, data []byte) ([]byte, error) {
			table := strings.Split(header.Get("recv_subject"), ".")[4]
			saveOnTable(table, data)
			return []byte(`{"status":"saved"}`), nil
		})

	// Start a process (a running instance of a flow) on a registered engine.
	pid, err := inflow.NewProcess("start-node-id",
		inflow.WithFlowId("f-123"),
		inflow.WithContextDocument("ctx-123"),
	)
	if err != nil {
		panic(err)
	}
	res, err := pid.Exec(context.Background())
	_ = res

	select {} // keep the NATS subscriptions alive
}
```

A fully runnable version of this (with an in-memory example flow) lives in [backend_sample.go](backend_sample.go) and [main_test.go](main_test.go).

## Package layout

| Package | Purpose |
|---|---|
| [`inflow`](inflow) | Backend wiring: `InitBackend`, engine resource registry/round-robin, `Process` (start/stop a flow run) |
| [`models`](models) | Shared wire types: `Flow`/`Node`, node rule types, infra/account/credential models, process request/response, error codes |
| [`nodes`](nodes) | Typed constructors for each node kind (void, code, contract, extrinsic, plugin, goto) |
| [`svcHandler`](svcHandler) | Register and look up NATS-callable "extrinsic" service handlers |
| [`spaces`](spaces) | Infra account lookups and scoped NATS credential issuance (used for plugin isolation) |
| [`nats`](nats) | Low-level NATS connection pooling and idle-connection cleanup |
| [`compilers/vueFlow`](compilers/vueFlow) | Generic Vue Flow graph → engine `Node` map compiler |
| [`etc`](etc) | Small utilities: UUIDs, HTTP client helpers, subject-pattern templating |

## Documentation

- [docs/architecture.md](docs/architecture.md) — how infra, your backend, engine instances, and plugins fit together
- [docs/infra.md](docs/infra.md) — the concrete REST endpoints and NATS subjects/wire formats
- [docs/nodes.md](docs/nodes.md) — every node type, its rule fields, and its builder API
- [docs/compilers](docs/compilers) — how a frontend-authored graph is compiled into engine-ready nodes, and how to add support for a different graph library

## Contributing

Issues and PRs are welcome. Since the API is still moving, consider opening an issue to discuss larger changes before sending a PR.

## License

Not yet decided — do not assume a permissive license until a `LICENSE` file is added.

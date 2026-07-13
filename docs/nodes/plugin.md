# Plugin node — `models.PluginNodeType`

The **external adapter** primitive, and the most powerful node type. It hands
execution off to a **live external process** you own — running anywhere, holding its
own connections and state — that the runtime (Fractal) calls into over NATS. It is
the one primitive that does **not** compile down to the others: a plugin is a real
process, not compiled logic, which is exactly why it can do things the compiled
primitives can't.

> The full authoring side of a plugin — how you *write* the process — lives in the
> separate **`inflow-plugin-sdk`** (`sdkv1`) repository. This page covers the node
> as it exists in `inflow-fusion` (`models.PluginRule`) and how it fits the node
> taxonomy. For building a plugin, see that SDK's
> [README](https://github.com/Inflowenger/go-plugin-sdk) and its
> `docs/architecture.md`, `docs/jobs-and-commands.md`, `docs/protocol-inflowv1.md`,
> `cookbook.md`, and the `skills/inflow-plugin/SKILL.md` agent skill.

## Rule data (the node, in this SDK)

```go
type PluginRule struct {
	InfraIsolated   InfraIsolated  // account/seed/cred/url for this plugin's isolated NATS identity
	Request         string
	SubjectPrefix   string         // defaults to "inflow.cpu.{uniqId}"
	CancelAfterIdle int8           // minutes; default 15
	Body            map[string]any
}
```

```go
plugin, err := nodes.NewPluginNode("jira",
	nodes.WithIdleWaitMinutes(30),
)
```

If you don't supply `WithPluginIsolated`, `NewPluginNode` fetches (and caches)
credentials for the builtin `plugins` account via
`spaces.GetCredOnBuiltinPluginAcc` — this requires `inflow.InitBackend` to have run.
For per-instance credential scoping (so one plugin instance's client can't see
another's traffic), see `spaces.PluginCredentialStrictPermission` and
`spaces.CreateUserCredential`.

## Why it doesn't compile to primitives

A compiled primitive is inert logic the engine runs. A plugin is a persistent
process, so it can:

- **hold long-running state** — open DB/queue/socket connections and keep them warm;
- **originate events** — run background loops that surface external happenings as
  node activity;
- **be an adapter** — translate any external protocol into Inflowenger context;
- **carry its own UI** — every action ships a JSON-Schema/UI-Schema form (rendered
  by JSON Forms with `x-inflow-ui` renderers) so users configure the node visually;
- **read and write flow context mid-execution**, report progress, and even **stop
  the flow** from inside its handler.

Because of this, a single plugin node is enough to build an entire workflow-
automation system on top of Inflowenger — it is the platform's open-ended extension
point. Everything else is a closed set of compiled primitives; the plugin is the
escape hatch to the outside world.

## Two registers of interaction (subject naming)

The runtime talks to a plugin in two ways, and the subject tells you which:

- **UI / arguments (`@`-prefixed on `inflow.v1.<PLUGIN_ID>.*`)** — *describe/
  configure me*: `@intro`, `@settings`, `@actions`, `@form`. Pure metadata; nothing
  executes.
- **Execution (`inflow.cpu.<PLUGIN_ID>.*`)** — *run me*: the node's main call,
  issued by the Fractal at runtime. It starts a **Job** — the plugin acks with a
  `jobId`, then works asynchronously, streaming `progress`, reading/writing context
  (`context/current`, `context/path`, `commit`), and finally `done`, all keyed by
  that `jobId`.

The `SubjectPrefix` field above (default `inflow.cpu.{uniqId}`) is the execution-plane
prefix for this node instance.

## Frontend representation (inspector)

Palette type `pluginNative` (`PluginNativeNode.vue` + `PluginNativeDrawer.vue`).
Relevant `node.data`:

| Field | Backend | Meaning |
|---|---|---|
| `subject_prefix` | `SubjectPrefix` | execution-plane subject prefix |
| `request` | `Request` | which action/method to invoke |
| `idle_min` | `CancelAfterIdle` | cancel the call after N idle minutes |
| `body{}` | `Body` | payload passed to the action |
| `infra_isolated.account` | `InfraIsolated` | account/space to run the plugin's identity under |

A packaged plugin can also appear on the canvas as an **extension instance** —
palette type `my_a_ext` (`extension_raw` + `settings`), whose form was declared by
the extension itself and filled in via JSON Forms; it compiles to the plugin (or
extrinsic) it wraps. See [from-frontend.md](from-frontend.md).

## Compiles to

The compiler hook reads the fields above and builds `nodes.NewPluginNode(request, …)`
→ `models.PluginRule`, setting `node.Type = models.PluginNodeType`.

## Next

- [extrinsic.md](extrinsic.md) — the thin internal-call counterpart; see the
  comparison table there
- [../architecture.md](../architecture.md) — Context · Workflows · Fractals ·
  Adapters, and where plugins sit
- inflow-plugin-sdk (separate repo) — how to actually write a plugin process
- [from-frontend.md](from-frontend.md) · [../nodes.md](../nodes.md)

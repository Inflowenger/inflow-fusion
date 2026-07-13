# From a frontend node to a primitive — the whole claim

This is the doc that explains **why Inflowenger only needs a handful of primitive
nodes**. The promise the platform makes is strong:

> With only the small set of low-level node types documented here, you can build
> **any** higher-level node, and therefore **any** system a workflow builder could
> need — ERP steps, CRM actions, approval gates, integrations, long-running jobs.

This page shows *how* that reduction actually happens: how an arbitrary node a user
draws on a canvas — with any title, any inputs, any number of outputs, and any
custom configuration form — becomes one of the primitives the engine executes. The
reference implementation used throughout is the **inflow-inspector** Vue app
(`inflow-vue/inflow-inspector`), whose node components and drawers are a working
example of the mapping.

If you only read one node doc, read this one, then the per-node pages
([code](code.md), [contract](contract.md), [extrinsic](extrinsic.md),
[plugin](plugin.md), [goto](goto.md), [void](void.md)).

## The primitive set

Everything the engine can execute is one of these (see [../nodes.md](../nodes.md)
for the exact structs and builders):

| Primitive | `NodeType` | Role in one line |
|---|---|---|
| **Void** | `VoidNodeType` | No-op; start markers, join points, dead-ends |
| **Code** | `CodeNodeType` | Run JS/OPA logic, write result into context |
| **Contract** | `RuleNodeType` | Decide which outgoing branch(es) fire (tags) |
| **Extrinsic** | `ExtrinsicNodeType` | Call an internal service your backend owns (NATS req/reply) |
| **Plugin** | `PluginNodeType` | Hand off to a live external process (its own UI + job lifecycle) |
| **GoTo** | `GoToNodeType` | Jump into another (or the same) flow and return |

There is no seventh kind. Every "HTTP node", "send-email node", "wait-for-approval
node", "add-finding-to-DB node" a product ships is one of the six above with
specific configuration — or, for the richest cases, a **Plugin** (the one primitive
that does *not* compile away, because it's a real process; see
[plugin.md](plugin.md)).

## Every node has the same skeleton

On the canvas a node looks bespoke, but structurally they are identical. In the
inspector, **every** node component (`CustomNode.vue`, `CodeNode.vue`,
`ContractNode.vue`, `ExtrinsicNode.vue`, `PluginNativeNode.vue`, …) shares the same
three universal fields, edited the same way:

| Universal field | Frontend (`node.data`) | Backend (`models.Node`) | Meaning |
|---|---|---|---|
| **Title** | `title` | `Title` | Human label. Editing it also mirrors into `key`. |
| **Key** | `key` | `Key` | Where this node's output is written into the context. |
| **Scope** | `scope` | `Scope` | A JSONPath the node reads/writes under. |

So the three things true of *any* node — *what is it called, where does its output
go, what slice of context does it see* — are captured before you even pick a type.
Everything else is per-type rule data.

## Inputs and outputs are just handles → edges → `Next`

This is the crux of "any inputs/outputs can be converted to a primitive."

- **The input handle** (left side, `type="target"`, id `input`) is not data — it
  only means *"execution can arrive here."* What the node actually *reads* is
  governed by `scope`/context, not by the wire. So an arbitrary number of logical
  "inputs" a designer imagines all reduce to: this node runs, and it reads context.
- **Output handles** (`type="source"`) are what become transitions. Each edge drawn
  from an output handle is compiled into one `models.Next` entry on the node
  (`NodeId` = edge target, `Tags` = the handle's tags — see below).

A node with **one output** (Code, Extrinsic, Plugin, Void, GoTo in the inspector
all render a single right-side `output` handle) is a **linear** step: it runs, then
the engine follows its `Next`. No decision is needed.

A node with **many outputs** is a **branch**, and that is exactly what the
**Contract** primitive is for. In `ContractNode.vue` the default right-side output
is removed; instead the user adds any number of **handler** handles at the bottom,
each with:

- a set of **tags** (`tag1, tag2, …`), and
- a color (cosmetic).

When an edge is drawn from a handler, it **inherits that handler's tags** into
`edge.data.tags` (see `VueFlowPage.vue` `onConnect`). At runtime the Contract node's
rule returns a list of tags, and the engine follows only the `Next` entries whose
`Tags` match. That's the entire branching mechanism — "3 outputs: success / retry /
reject" is just three tagged handlers on one Contract node.

```
 designer's mental model            primitive reality
 ───────────────────────            ─────────────────────────────────
 node with 1 output          ──►    any linear primitive + one Next
 node with N labelled        ──►    Contract node with N tagged handlers;
   outputs / conditions             rule picks the matching tag(s)
 node that reads "inputs"    ──►    scope/context read inside the node
 node with a config form     ──►    form values stored in node.data,
                                    read by the compiler hook
```

## The pipeline: canvas → saved graph → compiler → engine

None of the frontend shapes reach the engine directly. The chain is:

1. **Author.** The user drags nodes from the palette (`NodePalette.vue`), each
   seeded with the right backend fields (`VueFlowPage.vue` `nodeTypeMap` +
   per-type `baseData`), configures them through drawers
   (`CodeDrawer`, `ContractDrawer`, `ExtrinsicDrawer`, `PluginNativeDrawer`, …),
   and connects handles.
2. **Save.** `saveDiagram()` ships the **raw Vue Flow graph** — `{ nodes, edges }`
   plus viewport — to the backend as `view_flow` (`POST /flow`). The frontend does
   **no** translation to `models.Node`; it persists its own shape verbatim.
3. **Compile.** The backend (here, `inspector-api`, itself built on this SDK) runs
   the [Vue Flow compiler](../compilers/vueflow.md). For every node it calls a
   **hook** — `func(VueFlowNode) (*models.Node, error)` — that switches on
   `VueFlowNode.Type` and reads `VueFlowNode.Data` to construct the matching
   `nodes.*` builder. The compiler fills in each node's `Next` from the edges
   (target + `Data.Tags`).
4. **Execute.** The resulting `map[string]*models.Node` is what an engine instance
   fetches and walks (see [../architecture.md](../architecture.md) and
   [../infra.md](../infra.md)).

The hook is the *only* place product-specific knowledge lives. This is the seam
that makes the claim real: **your** node types are entirely your frontend's
convention; the compiler hook is where you declare "this custom type means *that*
primitive with *these* fields."

## The mapping table (inspector reference implementation)

How each inspector frontend node type reduces to a primitive, and which
`node.data` fields carry the configuration:

| Frontend type | `node.data` fields | Primitive | Notes |
|---|---|---|---|
| `startNode` / `void` | — | **Void** | Pure marker / no-op ([void.md](void.md)) |
| `code` | `lang` (`js`/`opa`), `logic_rule`, `opa_result` | **Code** | `NewJsNode` / `NewOpaNode` ([code.md](code.md)) |
| `contract` | `lang`, `logic_rule`, `opa_result`, `conditions[]`, `handlers[]` (tags) | **Contract** | handlers → tagged `Next` ([contract.md](contract.md)) |
| `extrinsic` | `serviceTopic`, `timeout`, `operationData{}` | **Extrinsic** | subject + payload + timeout ([extrinsic.md](extrinsic.md)) |
| `pluginNative` | `subject_prefix`, `request`, `idle_min`, `body{}`, `infra_isolated.account` | **Plugin** | maps to `PluginRule` ([plugin.md](plugin.md)) |
| `my_a_ext` | `extension_raw`, `settings{}` (JSON Forms) | **Plugin / Extrinsic** | an *extension instance* dragged from the palette; a pre-packaged node whose form was declared by the extension, compiling to whichever primitive it wraps |
| `goto` | goto target/return fields | **GoTo** | ([goto.md](goto.md)) |
| `custom` | title/key/scope only | (any) | the bare skeleton before a type is chosen |

> `my_a_ext` is worth calling out: it is how a **higher-level, packaged node**
> appears. The extension ships a JSON-Schema/UI-Schema form (`extension_raw`), the
> user fills it in (`settings`), and the compiler turns that into the underlying
> primitive. From the canvas it feels like a first-class custom node; underneath
> it's still one of the six.

## Worked example: a "save finding to DB" node

Concretely, following the extrinsic path (see [extrinsic.md](extrinsic.md)):

1. Backend registers a service:
   `svcHandler.ImplHandlerOnSubject("exports_db", svcHandler.SvcTopic("svc.add.issue.{TABLE_NAME}"), handler)`.
2. On the canvas the user drops an **Extrinsic** node, opens its drawer, sets
   `serviceTopic` (resolved from the logical name `exports_db`), any `operationData`
   payload, and a `timeout`.
3. Save ships the graph; the compiler hook reads those fields and builds
   `nodes.NewExtrinsicSvcNode(subject, …)` → `models.ExtrinsicRule`.
4. At runtime the engine publishes to the subject, the handler writes the row and
   replies `{"status":"saved …"}`, and that reply becomes the node's output,
   written into context at the node's `key`.

A "send Slack message" node, a "call HTTP API" node, an "approval gate" node are
the same story with a different primitive (Plugin for the long-running/eventful
ones, Contract for the gate). **Nothing new in the engine is ever required.**

## Why this is enough (the design argument)

- **Computation** — Code (JS/OPA) covers arbitrary transformation of context.
- **Decision / control flow** — Contract covers all branching; GoTo covers
  composition and reuse across flows; Void covers structure.
- **Reaching your own system** — Extrinsic covers any internal service via one
  req/reply subject.
- **Reaching the outside world / long-running / stateful / event-driven** — Plugin,
  a live process, covers everything the compiled primitives can't (open
  connections, background loops, its own UI, progress, mid-flow context read/write,
  stopping the flow).

Those axes span what a workflow builder needs, which is why the primitive set is
closed. See the platform-level framing (Context · Workflows · Fractals · Adapters)
in [../architecture.md](../architecture.md).

## Building your own frontend / compiler

The inspector is one frontend. Any editor that can emit a `{ nodes, edges }`-shaped
graph can reuse the shipped compiler; a genuinely different shape gets its own
compiler. Both paths are covered in [../compilers/](../compilers/README.md) and
[../compilers/vueflow.md](../compilers/vueflow.md). The rule of thumb: **put all
node-type knowledge in the hook**, reuse the `nodes.*` builders, and let the
compiler own `Next`.

## Next

- [../nodes.md](../nodes.md) — the primitive reference (structs + builders) and hub
- per-node deep dives: [void](void.md) · [code](code.md) · [contract](contract.md)
  · [extrinsic](extrinsic.md) · [plugin](plugin.md) · [goto](goto.md)
- [../compilers/vueflow.md](../compilers/vueflow.md) — the compiler that runs the hook

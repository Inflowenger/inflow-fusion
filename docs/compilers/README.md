# Compilers

The engine only ever executes `map[string]*models.Node` (see [nodes.md](../nodes.md)). It never parses whatever JSON your visual editor produces. A **compiler** is the piece that sits in between: it takes *your* graph representation — whatever your frontend/editor exports — and turns it into that node map.

`inflow-fusion` ships one compiler today, for graphs shaped like a Vue Flow / React Flow export: [vueflow.md](vueflow.md). This document covers the general shape of a compiler in this repo, so that a second one (a different graph library, or no graph library at all) can be added consistently.

## The contract

There's no formal Go interface for this yet (each compiler lives in its own subpackage of `compilers/`), but every compiler here follows the same convention:

1. **A graph type** describing your external representation — typically `Nodes []YourNode` + `Edges []YourEdge`, but it doesn't have to be nodes-and-edges at all if your source format encodes flow differently.
2. **A constructor**, `NewXCompiler(opts ...func(*XCompiler)) *XCompiler`, that accepts a **hook** — `func(YourNode) (*models.Node, error)` — supplied via an option (e.g. `WithEachNodeFunc`). The hook is where all of the product-specific mapping happens: reading your node's custom form data and constructing the matching `nodes.*` builder from [nodes.md](../nodes.md).
3. **A `Compile(startNodeId string, graph X) (map[string]*models.Node, map[string]error)` method** that walks the graph from a start node, invokes the hook per node, and populates each resulting node's `Next` from the graph's transitions (edges, or whatever your format uses).

The only thing the engine actually cares about is that `Next` ends up correctly populated on each `models.Node` — position, dimensions, or any other editor-only metadata in your external graph type never needs to leave your compiler package.

## Available compilers

| Compiler | Source format | Docs |
|---|---|---|
| `compilers/vueFlow` | Vue Flow (and, in practice, React Flow — see below) node/edge exports | [vueflow.md](vueflow.md) |

## Adding a new compiler

Write one when your frontend/editor doesn't emit a Vue Flow/React Flow-shaped graph — a different graph library (Cytoscape.js, Litegraph, a custom canvas), a non-visual DSL, or any other struct that describes steps and transitions between them.

1. Create a new package under `compilers/<name>` (see `compilers/vueFlow` for reference).
2. Define structs for your external graph/node/edge shape.
3. Add a `NewXCompiler` constructor and a hook option, following the pattern above.
4. Implement `Compile`, resolving your format's transitions into `models.Next` entries (`NodeId`, `Tags`, `Meta` as appropriate — see [nodes.md](../nodes.md) for what `Next` means at runtime).
5. Reuse the `nodes.*` builders inside your hook rather than constructing `models.Node` rule fields by hand, so compiled output stays consistent with what `inflow.NewProcess`-driven flows expect.

PRs for new compilers are welcome — open an issue first if the graph shape is unusual enough that the contract above might need to flex.

## Next

- [vueflow.md](vueflow.md) — the shipped Vue Flow / React Flow compiler
- [../nodes.md](../nodes.md) — what to build inside a compiler's hook for each node type
- [../infra.md](../infra.md) — how a compiled node map is actually served to an engine instance

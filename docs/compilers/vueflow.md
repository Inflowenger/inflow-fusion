# Vue Flow / React Flow compiler

`compilers/vueFlow` is the shipped implementation of the [general compiler contract](README.md), for graphs shaped like a [Vue Flow](https://vueflow.dev) export (a list of nodes plus a list of edges connecting them).

## It works for React Flow too

Vue Flow's data model intentionally mirrors [React Flow](https://reactflow.dev)'s: both describe a graph as `{ nodes, edges }`, where each node carries `id`/`type`/`data`/`position` and each edge carries `id`/`source`/`target`/`sourceHandle`/`targetHandle`. In practice a graph exported from React Flow decodes into the same `VueFlowNode`/`Edges` structs below unchanged — the package name reflects which frontend this was first built against, not a hard dependency on Vue Flow specifically. If you're on React Flow, you can very likely use this compiler as-is; only reach for [writing a new compiler](README.md#adding-a-new-compiler) if your export genuinely diverges (custom edge shape, non-handle-based routing, etc).

## The moving parts

```go
type VueFlow struct {
	Nodes []VueFlowNode
	Edges []Edges
}

type VueFlowNode struct {
	ID   string
	Type string // your frontend's node type string, e.g. "code", "contract", "pluginNative"
	Data any    // your frontend's arbitrary per-node form data
	// ...position/dimension fields, irrelevant to compilation
}

type Edges struct {
	ID           string
	Source       string
	Target       string
	SourceHandle string      // which output handle on the source node this edge left from
	TargetHandle string
	Data         EdgePayload // Tags []string, EdgeType string
	Label        string
}
```

You provide a hook, `func(VueFlowNode) (*models.Node, error)`, that inspects `VueFlowNode.Data` (a `map[string]any` in practice, since it comes from decoded JSON) and returns the corresponding `models.Node` — everything **except** `Next`, which the compiler fills in for you from the edges.

```go
cmpr := compiler.NewVueFlowCompiler(compiler.WithEachNodeFunc(myNodeBuilder))
nodeMap, errsByNodeId := cmpr.Compile(startNodeId, vueFlowGraph)
```

## What `Compile` actually does

Starting from `startNodeId`, it walks the graph depth-first following outgoing edges:

1. Call your hook on the current `VueFlowNode` to get a `*models.Node`.
2. For every edge whose `Source` is this node, append a `models.Next` to the node's `Next` list:
   - `NodeId` ← edge `Target`
   - `Tags` ← edge `Data.Tags`
   - `Meta` ← `{"edgeId": ..., "label": ..., "edgeHandle": edge.SourceHandle}`
3. Recurse into each `Next.NodeId` not already visited.
4. With `WithDependsGuard()` — off by default — run the [depends guard](#the-depends-guard) over the result, which reports graphs that would deadlock at runtime.
5. Return the accumulated `map[nodeId]*models.Node`, plus any per-node errors your hook returned (compilation stops walking further from a node once its hook errors) and, when the guard is on, its errors.

This is why a conditional/branching frontend node (multiple output handles: `success`, `error`, `else`, ...) doesn't need special handling in the compiler itself — each handle is just an edge with its own `SourceHandle`/tags, and which tagged `Next` entries actually fire is decided at runtime. Note `Data.Tags` is carried through for **every** edge, whatever primitive it leaves, so the same wiring serves a **Contract** rule, an extrinsic reply that answers with tags, or a plugin job that routes itself — see [routing.md](../routing.md).

For the deeper mechanics of how a specific handle id ends up meaning something at runtime (and a worked traversal example), see [`compilers/vueFlow/vuewFlow.md`](../../compilers/vueFlow/vuewFlow.md) — that file predates some of the field names above, so treat `models/flow.go` as the source of truth for the current `Node`/`Next` shape.

## The depends guard

A node with `Depends` blocks until every id it lists has completed. That makes it
possible to draw a graph the engine accepts and then deadlocks on, so `Compile`
can run a guard over the compiled map and report the offending nodes.

It is **opt-in** (`compiler.WithDependsGuard()`), and it is meant as help for
whoever is EDITING the graph, not as a gate in front of a run. The rule below
answers a question about one traversal — the one that starts at the
`startNodeId` given to `Compile` — and a caller compiling a graph in order to
run it is often not making that traversal: a run resumed after a human task or
a timer enters at the nodes the parked node handed back, several of them, in the
middle of the graph, with the engine seeding the earlier run's traversal state,
so a join this rule calls deadlocked can be exactly the join whose dependencies
are already satisfied. Turn the guard on where a flow is saved or a draft is
checked; leave it off where a graph is lowered to be executed.

The invalid shape:

```text
start -> A ->> B, C        B, C -> D (depends: [B, C])      D -> E      E -> D
```

Wiring `E` back into `D`'s blocking input port grows the set to `[B, C, E]`. On
the first crossing `D` waits for `E`, while `E` is only ever queued *after* `D`
runs - nothing can advance, and the flow is stuck at `D` forever. Semantically
the node is saying "I depend on a step that comes after me", which has no valid
reading.

### The rule

> For every node `N` and every `dep` in `N.Depends`: `dep` must still be
> reachable from the start node when `N` is removed from the graph.

In dominator terms, a node may not dominate its own dependencies. This is
deliberately *not* cycle detection, because loops are valid topology in the
Inflowenger ecosystem and the guard must not reject them:

| Back edge | `D.Depends` | Verdict | Why |
|---|---|---|---|
| `E -> D` | `[B, C, E]` | **error** | every path to `E` runs through `D` |
| `E -> B` | `[B, C]` | valid | `B` is reachable via `start -> A -> B`, so it can complete before `D` |
| `E -> D` | `[B, C]` | valid | the loop re-enters `D`, but `D` never waits for `E` |

The middle row is the fix the error suggests: re-point the back edge at a node
that already runs before the join, and the loop re-enters the join through its
existing dependencies.

### Consuming the result

With the guard on, error severity issues are merged into the `map[nodeId]error`
that `Compile` already returns, and the structured report carries the rest -
warnings, the edge to re-point, and the legal targets for it:

```go
cmpr := compiler.NewVueFlowCompiler(
	compiler.WithEachNodeFunc(myNodeBuilder),
	compiler.WithDependsGuard(), // off unless you ask for it
)
nodeMap, errsByNodeId := cmpr.Compile(startNodeId, vueFlowGraph)

report := cmpr.Report()
for _, issue := range report.Issues {
	// show it on the canvas: issue.NodeId, issue.EdgeId, issue.Suggestion
	log.Printf("%s %s on node %s: %s", issue.Severity, issue.Code, issue.NodeId, issue.Message)
}
```

`report.Err()` collapses every error-severity issue into one error for a caller
that does want to refuse the flow - but prefer showing the issues: each one
names the node and the edge whose change would fix it.

Each `ValidationIssue` is JSON-serialisable and identifies the exact canvas
objects, which is what a frontend needs to highlight them:

```json
{
  "code": "depend_deadlock",
  "severity": "error",
  "nodeId": "D",
  "dependsOn": "E",
  "edgeId": "e-E-D",
  "message": "node \"D\" depends on \"E\", but \"E\" can only be reached by running \"D\" first: ...",
  "suggestion": "re-point edge \"e-E-D\" from \"E\" so it targets a node connected to \"D\" that already runs before it (one of: \"B\", \"C\"). ...",
  "suggestedTargets": ["B", "C"]
}
```

| Code | Severity | Meaning |
|---|---|---|
| `depend_deadlock` | error | the dependency only exists downstream of the waiting node |
| `depend_unreachable` | error | the dependency is not reachable from the start node at all |
| `depend_unknown_node` | error | `depends` names an id that is not a node of this graph |
| `depend_self` | error | a node waits for itself |
| `depend_without_edge` | warning | satisfiable, but the dependency has no transition into the node - usually a stale entry |
| `start_node_missing` | error | the start node is absent, so reachability cannot be judged |

The check itself is a plain function over a compiled node map, so it is reusable
by any other compiler or by a flow built by hand with the `nodes` builders:

```go
report := compiler.ValidateDepends(startNodeId, nodeMap)
```

This is also how a caller that left the guard off gets the same findings when it
wants them, without the guard merging errors into `Compile`'s map. Inactive
transitions (`Next.Active == -1`) are followed while judging reachability, so the
guard only ever reports a deadlock it is certain about.

## Writing your own hook

A hook typically switches on `VueFlowNode.Type` and constructs the matching `nodes.*` builder from [nodes.md](../nodes.md), pulling whatever fields your frontend form stored in `Data`:

```go
func myNodeBuilder(vfn compiler.VueFlowNode) (*inflowModels.Node, error) {
	data := vfn.Data.(map[string]any)
	node := inflowModels.Node{ID: vfn.ID, Title: data["title"].(string)}

	switch vfn.Type {
	case "code":
		node.Type = inflowModels.CodeNodeType
		n := inflowNodes.NewJsNode(data["logic_rule"].(string))
		node.Code = &n.CodeRule
	case "contract":
		node.Type = inflowModels.RuleNodeType
		n := inflowNodes.NewJsRuleLogicNode(inflowNodes.WithContractLogicCode(data["logic_rule"].(string)))
		node.Contract = &n.ContractRule
	// ... your other node types
	}
	return &node, nil
}
```

If your frontend needs to reference a backend-registered extrinsic service by a logical name rather than hardcoding its NATS subject, resolve it through `svcHandler.GetSvc(name)` inside the hook (see `dev-backend/inflow/compiler.go`'s `NODE_MY_A` case for a full example, including filling subject placeholders with `SvcTopic.MakeReqSubjectWithParams`).

## Debugging a graph

`GetOutboundsEdges(nodeId)` / `GetInboundsEdges(nodeId)` on `*VueFlowCompiler` let you inspect a node's edges directly if you need to debug why a walk produced (or omitted) a particular transition.

## Next

- [README.md](README.md) — the general compiler contract, and how to add one for a different graph library
- [../nodes.md](../nodes.md) — what to build inside your hook for each node type
- [../infra.md](../infra.md) — how the compiled node map is actually served to an engine instance

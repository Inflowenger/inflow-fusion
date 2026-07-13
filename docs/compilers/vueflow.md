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
4. Return the accumulated `map[nodeId]*models.Node`, plus any per-node errors your hook returned (compilation stops walking further from a node once its hook errors).

This is why a conditional/branching frontend node (multiple output handles: `success`, `error`, `else`, ...) doesn't need special handling in the compiler itself — each handle is just an edge with its own `SourceHandle`/tags, and it's the **Contract** node's own rule logic (see [nodes.md](../nodes.md#contract--modelsrulenodetype)) that decides which tagged `Next` entries actually fire at runtime.

For the deeper mechanics of how a specific handle id ends up meaning something at runtime (and a worked traversal example), see [`compilers/vueFlow/vuewFlow.md`](../../compilers/vueFlow/vuewFlow.md) — that file predates some of the field names above, so treat `models/flow.go` as the source of truth for the current `Node`/`Next` shape.

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

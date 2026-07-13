# GoTo node — `models.GoToNodeType`

Jumps execution to a node in another (or the same) flow, then returns. This is the
**composition / reuse** primitive: it lets one flow call into another the way a
program calls a subroutine, so shared logic is authored once and reused.

## Rule data

```go
type GoToRule struct {
	From  Next // where to jump from (flow + node id)
	EndAt Next // where control returns to afterwards
}
```

`Next` here is the same transition struct used everywhere (`FlowId` + `NodeId`; see
[../nodes.md](../nodes.md)). `From` names the target flow/node to enter; `EndAt`
names where control resumes when that sub-traversal is done.

## Builder

```go
g := nodes.NewGotoNode()
g.From("flow-a", "node-3") // jump into flow-a at node-3
g.To("flow-a", "node-8")   // return to node-8 afterwards
```

## Frontend representation (inspector)

Palette type `goto` (`GotoNode.vue`). The node's drawer collects the target flow/node
and return target; on save these populate the `From`/`EndAt` transitions. Rendered
with the standard input/output handles.

## Why it's a primitive

Sub-flows are how large systems stay maintainable: an "onboard customer" flow can
`GoTo` a shared "send-and-confirm-email" flow instead of duplicating those nodes.
Because a Fractal can treat an embedded flow as a node, GoTo is also the seam the
platform's "a node can be an embedded flow" idea rests on (see
[../architecture.md](../architecture.md)).

## Next

- [../nodes.md](../nodes.md) · [from-frontend.md](from-frontend.md)

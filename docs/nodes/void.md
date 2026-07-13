# Void node — `models.VoidNodeType`

A **no-op** node. It carries no rule data and executes nothing; the engine simply
passes through it and follows its `Next`. It exists for *structure*, not behavior.

## When you use it

- **Start marker.** A flow needs an entry node; a Void node is the conventional
  start (the inspector's `startNode` is a Void with a distinct look).
- **Join / fan-in point.** Several branches can converge on one Void node so
  downstream wiring has a single target (combine with `Depends` on the node to wait
  for multiple inbound nodes — see [../nodes.md](../nodes.md)).
- **Branch target that does nothing** — e.g. the "else" arm of a
  [Contract](contract.md) that should just continue.
- **Placeholder** while a flow is being authored.

## Rule data

None. `Code`/`Contract`/`Extrinsic`/`Plugin`/`GoTo` are all nil; only the universal
fields (`Title`, `Key`, `Scope`, `Next`, `Depends`) apply.

## Builder

```go
n := nodes.NewVoidNode(nodes.WithUniqId[*nodes.VoidNode]("start"))
```

## Frontend representation (inspector)

Two palette entries compile to this primitive:

- `startNode` → the flow's start marker.
- `void` → a generic pass-through.

Both render one input handle and one output handle (`CustomNode`-style) and expose
only the universal title/key/scope controls — there is nothing else to configure,
which is the whole point.

## Compiles to

The compiler hook sets `node.Type = models.VoidNodeType` and leaves all rule
pointers nil. See [from-frontend.md](from-frontend.md) for the pipeline.

## Next

- [../nodes.md](../nodes.md) · [from-frontend.md](from-frontend.md)
- [contract.md](contract.md) — the usual reason you need a join/else target

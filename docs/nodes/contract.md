# Contract node — `models.RuleNodeType`

The **decision / control-flow** primitive. A Contract node runs logic just like a
[Code](code.md) node, but its output is interpreted as a **list of tags** that
selects which of the node's outgoing transitions (`Next` entries) fire. Every
"branch", "condition", "gate", "router", or "switch" a designer imagines is a
Contract node — this is what makes the "a node with any number of outputs" case
reduce to a primitive (see [from-frontend.md](from-frontend.md)).

## Rule data

```go
type ContractRule struct {
	Lang       string         // "js" or "opa"
	LogicRule  string
	Conditions map[string]any // arbitrary criteria data available to the rule
	OpaResult  string         // for OPA: which binding holds the resulting tag list
}
```

Same two languages as Code; the difference is purely in how the result is used.

```go
// JS: the last expression must resolve to a tag list
rule := nodes.NewJsRuleLogicNode(
	nodes.WithContractLogicCode(`c = 8; if (c < data.threshold) { next = ["a"] } else { next = ["else"] }; next`),
	nodes.WithContractConditions(map[string]any{"threshold": 10}),
)

// OPA: OpaResult names the binding that holds the tag list
rule := nodes.NewOpaRuleLogicNode("next",
	nodes.WithContractLogicCode(`c = 8
	 default next := ["else"]
	 next := ["a", "b"] if { c < data.threshold }`),
	nodes.WithContractConditions(map[string]any{"threshold": 10}),
)
```

## How branching actually works

1. The rule returns a tag list, e.g. `["a"]` (or `["a","b"]` to fire multiple
   branches at once — fan-out).
2. The engine compares that list against each `Next.Tags` on this node.
3. Only the transitions whose tags match are followed.

So the branch *decision* lives in the rule, and the branch *wiring* lives in the
tagged `Next` entries. `Conditions` is the static criteria data the rule reads
(exposed as `data.*`), which is how the same rule logic can be parameterized per
node.

## Frontend representation (inspector) — handlers = tagged outputs

This is the most instructive node to see on the canvas (`ContractNode.vue` +
`ContractDrawer.vue`). Unlike other nodes, its default right-side output handle is
**removed**. Instead the user adds any number of **handler** handles along the
bottom, each with:

- **tags** (`tag1, tag2, …`) — the label(s) this branch responds to, and
- a **color** (cosmetic only).

Relevant `node.data`:

| Field | Meaning |
|---|---|
| `lang` | `"js"` or `"opa"` |
| `logic_rule` | the rule code (edited in the drawer) |
| `opa_result` | which OPA binding is the tag list (OPA only) |
| `conditions[]` | key/value criteria → `ContractRule.Conditions` |
| `handlers[]` | `{ id, tags[], color }` — one per output branch |

When the user draws an edge **from a handler**, the edge inherits that handler's
tags into `edge.data.tags` (see `VueFlowPage.vue` `onConnect`). At compile time each
edge becomes a `models.Next{NodeId, Tags}`; at runtime the rule's returned tags
select among them. Deleting a handler removes its edges (`removeHandlerEdges`).

> "Three outputs: `success` / `retry` / `reject`" is literally three handlers, each
> tagged, on one Contract node — plus a rule that returns one of those tag strings.

## Contract vs. Code

|  | [Code](code.md) | Contract |
|---|---|---|
| Languages | JS / OPA | JS / OPA |
| Output meaning | a value written to context | a tag list selecting branches |
| Handles (frontend) | one output | many tagged handler outputs |
| `NodeType` | `CodeNodeType` | `RuleNodeType` |

## Compiles to

```go
// inside the compiler hook, roughly:
n := inflowNodes.NewJsRuleLogicNode(inflowNodes.WithContractLogicCode(data["logic_rule"].(string)))
node.Type = inflowModels.RuleNodeType
node.Contract = &n.ContractRule
// the compiler fills node.Next from the edges, carrying each edge's Data.Tags
```

The compiler needs no special handling for branching — each handler is just an edge
with its own tags. See [../compilers/vueflow.md](../compilers/vueflow.md).

## Next

- [code.md](code.md) — same languages, output is a value not tags
- [void.md](void.md) — common "else"/join target for a branch
- [from-frontend.md](from-frontend.md) · [../nodes.md](../nodes.md)

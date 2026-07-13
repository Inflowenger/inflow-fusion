# Nodes — the knowledge base

This is the hub for everything about Inflowenger nodes: what the primitive node
types are, how to build each one, and — most importantly — **how any node a user
could want reduces to this small primitive set**. Each node type also has a
dedicated deep-dive page under [nodes/](nodes/).

## The primitives claim

Inflowenger is a runtime whose logic is a **workflow graph** — a substrate for
building large classes of systems (ERP, CRM, automation platforms) where an
operator can define and change logic without redeploying. The platform ships only a
**small set of primitive nodes**; every higher-level node compiles down to them, and
the one open-ended extension point ([Plugin](nodes/plugin.md)) covers the rest.

> With only these low-level node types you can build **any** higher-level node, and
> therefore **any** system a workflow builder needs.

The doc that makes that concrete — how an arbitrary custom node on a canvas (any
title, any inputs, any number of outputs, any config form) becomes a primitive —
is **[nodes/from-frontend.md](nodes/from-frontend.md)**. Read it after this page.

## The node types

| Node | `NodeType` | Role | Deep dive |
|---|---|---|---|
| **Void** | `VoidNodeType` | No-op: start markers, joins, dead-ends | [nodes/void.md](nodes/void.md) |
| **Code** | `CodeNodeType` | Run JS/OPA logic, write result to context | [nodes/code.md](nodes/code.md) |
| **Contract** | `RuleNodeType` | Branch: rule output = tags selecting `Next` | [nodes/contract.md](nodes/contract.md) |
| **Extrinsic** | `ExtrinsicNodeType` | Call an internal service you own (NATS req/reply) | [nodes/extrinsic.md](nodes/extrinsic.md) |
| **Plugin** | `PluginNodeType` | Hand off to a live external process (own UI + jobs) | [nodes/plugin.md](nodes/plugin.md) |
| **GoTo** | `GoToNodeType` | Jump into another/the same flow and return | [nodes/goto.md](nodes/goto.md) |

Primitive vs. higher-level: Code/Contract/Extrinsic/GoTo/Void are *compiled*
primitives. **Plugin** is the exception — it does not compile away; it's a real
external process, which is what makes it the platform's richest, open-ended node
(see [nodes/plugin.md](nodes/plugin.md)).

## The shared node shape

A flow (`models.Flow`) is a list of `models.Node`. Every node shares this shape:

```go
type Node struct {
	ID        string         `json:"uuid"`
	Type      NodeType       `json:"type"`
	Title     string         `json:"title"`
	Key       string         `json:"key"`      // where this node's output is written into the context, if not empty
	Scope     string         `json:"scope"`    // a JSON path into the context this node reads/writes under
	Code      *CodeRule      `json:"code,omitempty"`
	GoTo      *GoToRule      `json:"goto,omitempty"`
	Extrinsic *ExtrinsicRule `json:"extrinsic,omitempty"`
	Plugin    *PluginRule    `json:"plugin,omitempty"`
	Contract  *ContractRule  `json:"contract,omitempty"`
	Meta      map[string]any `json:"meta"`
	Tags      string         `json:"tags"`
	Next      []Next         `json:"next"`
	Depends   []string       `json:"depends"` // wait for all of these inbound node ids to finish first
}

type Next struct {
	FlowId string         `json:"flowId"` // defaults to the current flow's id, see Flow.ValidateNext
	NodeId string         `json:"nodeId"`
	Tags   []string       `json:"tags"`   // used to filter which Next entries are followed (branching)
	Active int8           `json:"active"` // 0: active, -1: inactive
	Meta   map[string]any `json:"meta"`
}
```

Exactly one of `Code`/`GoTo`/`Extrinsic`/`Plugin`/`Contract` is populated, matching
`Type`. The universal fields — `Title`, `Key` (output destination in context),
`Scope` (the JSONPath slice the node reads/writes) — apply to every type.
`nodes.INode` (`SetId`, `GetInflowNodeType`) is the small interface every builder
implements, which is what lets `nodes.WithUniqId[T](id)` work generically.

## Builders at a glance

Each type has a typed builder in the `nodes` package. Summaries below; full rule
structs, languages, gotchas, and the frontend mapping are on each type's deep-dive
page.

```go
// Void — nodes/void.md
n := nodes.NewVoidNode(nodes.WithUniqId[*nodes.VoidNode]("start"))

// Code — nodes/code.md  (Lang "js" | "opa")
js  := nodes.NewJsNode(`input.a = input.b * input.b; input`)
opa := nodes.NewOpaNode(`result = {"f": 60}`, "result", nodes.WithCriteriaData(map[string]any{"threshold": 10}))

// Contract — nodes/contract.md  (rule output is a tag list selecting Next entries)
rule := nodes.NewJsRuleLogicNode(
	nodes.WithContractLogicCode(`c = 8; if (c < data.threshold) { next = ["a"] } else { next = ["else"] }; next`),
	nodes.WithContractConditions(map[string]any{"threshold": 10}),
)

// Extrinsic — nodes/extrinsic.md  (publish to a subject; reply = output)
ext := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.orders")

// Plugin — nodes/plugin.md  (hand off to an external process)
plugin, _ := nodes.NewPluginNode("jira", nodes.WithIdleWaitMinutes(30))

// GoTo — nodes/goto.md  (jump into another flow and return)
g := nodes.NewGotoNode(); g.From("flow-a", "node-3"); g.To("flow-a", "node-8")
```

## How branching works (the one cross-cutting rule)

Only [Contract](nodes/contract.md) nodes decide branches. A Contract's rule returns
a tag list (e.g. `["a"]`); the engine follows only the `Next` entries whose
`Next.Tags` match. This is why a frontend node with multiple outputs maps to a
Contract with one tagged handler per output — full explanation in
[nodes/contract.md](nodes/contract.md) and
[nodes/from-frontend.md](nodes/from-frontend.md).

## Building `Next` manually

If you construct a `models.Flow` directly (rather than through a compiler), wire
transitions yourself:

```go
flow := models.Flow{
	UUID: "f-123",
	Nodes: []models.Node{
		{ID: "n0", Type: models.VoidNodeType, Next: []models.Next{{NodeId: "n1"}}},
		{ID: "n1", Type: models.CodeNodeType, Code: &models.CodeRule{Lang: "js", LogicRule: "input"}},
	},
}
flow.ValidateNext() // fills in FlowId on any Next entries that omitted it
```

In practice most flows are authored in a visual editor and turned into this node map
by a **compiler** (see below), not built by hand.

## Next

- [nodes/from-frontend.md](nodes/from-frontend.md) — **the core doc**: how any
  custom frontend node (any inputs/outputs, any form) compiles to a primitive
- per-node deep dives: [void](nodes/void.md) · [code](nodes/code.md) ·
  [contract](nodes/contract.md) · [extrinsic](nodes/extrinsic.md) ·
  [plugin](nodes/plugin.md) · [goto](nodes/goto.md)
- [compilers](compilers) — building this node map automatically from a
  frontend-authored graph
- [infra.md](infra.md) — the wire-level detail of how the engine invokes
  extrinsic/plugin subjects
- [architecture.md](architecture.md) — where nodes sit in the platform (Context ·
  Workflows · Fractals · Adapters)

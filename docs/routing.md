# Tag routing — the contract pattern

Branching in Inflowenger is **not a node type**. It is one runtime mechanism that
every node can use:

> A node emits a **list of tags** when it finishes. Every outgoing transition
> (`models.Next`) carries the tags it accepts, fixed at compile time. The engine
> deactivates every outgoing edge, then re-activates only the ones whose `Tags`
> match — and follows those.

The route is chosen at run time; the set of routes that could ever fire was drawn
on the canvas. That is the whole "contract" idea: the decision is dynamic, the
possibilities are static and inspectable.

```
        rule / service reply / plugin command
                     │  ["approve"]
                     ▼
   ┌──────────┐  Next{Tags:["approve"]}  ──► fires
   │  node    │  Next{Tags:["reject"]}   ──► Active = -1
   └──────────┘  Next{Tags:["escalate"]} ──► Active = -1
```

## Who can emit tags

Three primitives can decide a route, with **identical** filter semantics. The
[Contract](nodes/contract.md) node is the common case, not the only one.

| Decider | How the tags arrive | Where |
|---|---|---|
| **Contract** | the JS/OPA rule's result *is* the tag list | evaluated in-engine ([contract.md](nodes/contract.md)) |
| **Extrinsic** | the service reply carries `_cmd: "next_tags"` + `_next_filter: [...]` | your handler ([extrinsic.md](nodes/extrinsic.md)) |
| **Plugin** | the running job sends the `next_tags` command | external process ([plugin.md](nodes/plugin.md)) |

### Contract — a compiled rule

```go
rule := nodes.NewJsRuleLogicNode(
	nodes.WithContractLogicCode(`input.amount > data.threshold ? ["escalate"] : ["approve"]`),
	nodes.WithContractConditions(map[string]any{"threshold": 10000}),
)
```

Deterministic, evaluated outside any model, over the node's `Scope`.

### Extrinsic — your backend decides

`svcHandler` ships the response helpers; the engine applies the command to the
node's edges before writing the reply into context:

```go
svcHandler.ImplHandlerOnSubject("risk", svcHandler.SvcTopic("svc.risk.check"),
	func(header nats.Header, data []byte) ([]byte, error) {
		if risky(data) {
			return svcHandler.FilterNextResponse(map[string]any{"score": 91}, []string{"escalate"})
		}
		return svcHandler.FilterNextResponse(map[string]any{"score": 12}, []string{"approve"})
	})
```

| Helper | Wire form | Effect |
|---|---|---|
| `FilterNextResponse(data, tags)` | `{..., "_cmd":"next_tags", "_next_filter":["escalate"]}` | keep only edges tagged `escalate` |
| `StopHereResponse(data)` | `{..., "_cmd":"stop"}` | deactivate **every** edge — the branch ends here |
| plain bytes | no `_cmd` | no-op; edges stay as compiled |

The keys are `models.SvcCmdResposeKey` (`_cmd`) and
`models.SvcCmdResponseNextFilterKey` (`_next_filter`); commands are
`models.CmdNextFilter` / `models.CmdStop`. A missing or malformed command is
deliberately a no-op.

### Plugin — an external process decides, mid-job

A plugin node is a live process speaking the **inflow-v1 protocol**; routing is
just another job command, sent while the job is still running:

```go
p.AddAction(sdkv1.Action{Method: "classify", RequestHandler: func(job sdkv1.Job) {
	job.CmdNextFilter([]string{"escalate"})   // subject: <job>/next_tags, payload "escalate"
	job.Done(map[string]any{"reason": "amount over limit"})
}})
```

This is what makes a **model node route the diagram**. An LLM plugin binds its
functions/tools to the node's tagged outputs; when the model answers with a tool
call, the plugin translates that choice into `CmdNextFilter([...])` and the engine
fires only that port. The model picks among ports you drew — it cannot invent one,
and everything downstream of each port is already defined. The same is true of an
MCP node, or any plugin that chooses its own continuation.

See the SDK's `docs/protocol-inflowv1.md` and `docs/jobs-and-commands.md`
(`CmdNextFilter` → `next_tags`).

## Semantics, exactly

- **Deactivate-then-match.** All `Next` entries are set `Active = -1`; an entry is
  re-activated if *any* of its `Tags` is in the emitted list.
- **Multiple tags = fan-out.** `["a","b"]` fires every edge tagged `a` or `b`, in
  parallel. Join them again with a Void node using `Depends` ([void.md](nodes/void.md)).
- **No match = dead end.** If nothing matches, nothing continues on that branch —
  which is why an untagged catch-all edge (an "else") is worth drawing.
- **Untouched by default.** A linear node that emits no tags follows its `Next` as
  compiled. Tagging is opt-in per node.
- **It is recorded.** The tags that did the selecting are kept as the *select
  reason* and emitted with the `edge.select` event, so a run explains not just
  which edge fired but why ([logs.md](logs.md)).
- **Not on Code.** [Code](nodes/code.md) writes a value into context; it does not
  route. That is the only difference between Code and Contract.

## Why this is a primitive and not a feature

Every workflow builder eventually ships a drawer of comparison nodes — greater
than, less than, contains, is empty, matches, between. Inflowenger ships none of
them. It ships the primitive they are all made of.

A product author declares a node in their palette (*Greater than*, two fields) and
the compiler hook turns it into a Contract whose rule is a small JS or Rego body
built from those fields:

```go
// compiler hook, roughly — one case per palette node
case "op_gte":
	n := inflowNodes.NewJsRuleLogicNode(
		inflowNodes.WithContractLogicCode(`input[data.field] >= data.value ? ["true"] : ["false"]`),
		inflowNodes.WithContractConditions(map[string]any{
			"field": data["field"], "value": data["value"],
		}),
	)
	node.Type = inflowModels.RuleNodeType
	node.Contract = &n.ContractRule
```

The runtime never learns what "greater than" means, and the person on the canvas
never sees a line of JavaScript — they see two fields and a `true`/`false` output.
**Contract is the base class for decision nodes**: new operators are *authored* at
compile time, not implemented in the engine. Add fifty of them and the engine is
byte-for-byte unchanged.

The same argument extends past comparisons: a policy gate is a Rego Contract, an
approval gate is an Extrinsic that answers with tags, an agent's next step is a
plugin that answers with tags. One mechanism, three deciders.

## Next

- [nodes/contract.md](nodes/contract.md) — the node, its rule struct, and its
  tagged handler handles on the canvas
- [nodes/from-frontend.md](nodes/from-frontend.md) — how a canvas node with N
  outputs becomes N tagged `Next` entries
- [nodes/extrinsic.md](nodes/extrinsic.md) · [nodes/plugin.md](nodes/plugin.md) —
  the two external deciders
- [nodes.md](nodes.md) — the primitive set and the shared node shape
- [logs.md](logs.md) — `edge.select` and the recorded select reason

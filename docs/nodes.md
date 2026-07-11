# Node reference

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

Exactly one of `Code`/`GoTo`/`Extrinsic`/`Plugin`/`Contract` is populated, matching `Type`. `nodes.INode` (`SetId`, `GetInflowNodeType`) is the small interface every builder below implements, which is what lets `nodes.WithUniqId[T](id)` work generically across all of them.

## Void — `models.VoidNodeType`

A no-op placeholder node (frequently used as a flow's start node, or as a branch target that does nothing). No rule data.

```go
n := nodes.NewVoidNode(nodes.WithUniqId[*nodes.VoidNode]("start"))
```

## Code — `models.CodeNodeType`

Runs a snippet of logic and writes its result into the context. Supports two languages:

```go
type CodeRule struct {
	Lang      string         // "js" or "opa"
	LogicRule string         // the code
	OpaData   map[string]any // extra data made available to an OPA snippet (as `data.*`)
	OpaResult string         // for OPA: which binding in the rule to extract as the node's output
}
```

```go
jsNode := nodes.NewJsNode(`input.a = input.b * input.b; input`)

opaNode := nodes.NewOpaNode(
	`c = 5
	 allow if { c < 10 }
	 result = {"f": c*12, "allow": allow}`,
	"result", // OpaResult
	nodes.WithCriteriaData(map[string]any{"threshold": 10}), // -> OpaData
)
```

- JS code reads/writes a variable called `input` (the node's scoped context); the last expression's value becomes the node's output.
- OPA (Rego) code evaluates as a policy; `OpaResult` names the rule/binding whose value is extracted as the node's output, and `OpaData` is exposed to the policy as external `data`.

## Contract — `models.RuleNodeType`

A branching/decision node. Same two languages as Code, but its output is expected to be (or resolve to) a list of tags used to select which of the node's `Next` entries fire:

```go
type ContractRule struct {
	Lang       string         // "js" or "opa"
	LogicRule  string
	Conditions map[string]any // arbitrary criteria data available to the rule
	OpaResult  string         // for OPA: which binding holds the resulting tag list
}
```

```go
rule := nodes.NewJsRuleLogicNode(
	nodes.WithContractLogicCode(`c = 8; if (c < data.threshold) { next = ["a"] } else { next = ["else"] }; next`),
	nodes.WithContractConditions(map[string]any{"threshold": 10}),
)
// or, OPA flavored:
rule := nodes.NewOpaRuleLogicNode("next",
	nodes.WithContractLogicCode(`c = 8
	 default next := ["else"]
	 next := ["a", "b"] if { c < data.threshold }`),
	nodes.WithContractConditions(map[string]any{"threshold": 10}),
)
```

The resulting tag list (e.g. `["a"]`) is matched against each `Next.Tags` on this node — only matching transitions are followed, which is how conditional branching works in a compiled flow.

## Extrinsic — `models.ExtrinsicNodeType`

Calls out to a NATS subject your backend (or another service) owns — this is how a flow invokes your domain logic.

```go
type ExtrinsicRule struct {
	InfraIsolated     InfraIsolated  // optional: run this call over an isolated account instead of the default
	Subject           string         // NATS subject to call
	Data              map[string]any // static payload merged into the request
	ReqTimeoutSecound uint8          // default 5
}
```

```go
n := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.orders")
```

Pair this with `svcHandler.ImplHandlerOnSubject` on the receiving side (see [protocol.md](protocol.md)) so something actually answers on that subject. If `InfraIsolated.Account` is left empty, it defaults to the shared `inflow` account connection.

## Plugin — `models.PluginNodeType`

Hands execution off to an external plugin process (e.g. a Jira or Slack integration) running as its own isolated NATS participant.

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

If you don't supply `WithPluginIsolated`, `NewPluginNode` fetches (and caches) credentials for the builtin `plugins` account via `spaces.GetCredOnBuiltinPluginAcc` — this requires `inflow.InitBackend` to have already run. For finer-grained, per-instance credential scoping (so one plugin instance's client can't see another's traffic), see `spaces.PluginCredentialStrictPermission` and issue credentials with `spaces.CreateUserCredential`.

## GoTo — `models.GoToNodeType`

Jumps execution to a node in another (or the same) flow, then returns.

```go
type GoToRule struct {
	From  Next // where to jump from (flow + node id)
	EndAt Next // where control returns to afterwards
}
```

```go
g := nodes.NewGotoNode()
g.From("flow-a", "node-3")
g.To("flow-a", "node-8")
```

## Building `Next` manually

If you're constructing a `models.Flow` directly (rather than through a compiler), wire node transitions yourself:

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

## Next

- [compilers](compilers) — building this node map automatically from a frontend-authored graph instead of by hand
- [protocol.md](protocol.md) — the wire-level detail of how the engine invokes extrinsic/plugin subjects

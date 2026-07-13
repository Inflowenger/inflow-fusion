# Code node — `models.CodeNodeType`

Runs a snippet of logic against the node's scoped context and writes the result
back. This is the **computation** primitive: any pure transformation of context —
reshaping data, computing a value, deriving a field — is a Code node.

It is *not* a decision node. It produces an output value; it does not choose a
branch. For branching, use [Contract](contract.md) (which runs the same two
languages but interprets the result as tags).

## Rule data

```go
type CodeRule struct {
	Lang      string         // "js" or "opa"
	LogicRule string         // the code
	OpaData   map[string]any // extra data exposed to an OPA snippet (as data.*)
	OpaResult string         // for OPA: which binding to extract as the node's output
}
```

## The two languages

- **JS** — the code reads and writes a variable called `input` (the node's scoped
  context). The value of the last expression becomes the node's output.

  ```go
  jsNode := nodes.NewJsNode(`input.a = input.b * input.b; input`)
  ```

- **OPA (Rego)** — the code is evaluated as a policy. `OpaResult` names the
  rule/binding whose value is extracted as the node's output; `OpaData` is exposed
  to the policy as external `data`.

  ```go
  opaNode := nodes.NewOpaNode(
      `c = 5
       allow if { c < 10 }
       result = {"f": c*12, "allow": allow}`,
      "result", // OpaResult
      nodes.WithCriteriaData(map[string]any{"threshold": 10}), // -> OpaData
  )
  ```

Why two? JS is the pragmatic choice for arbitrary data-shaping; OPA is for
declarative, auditable policy logic where you want the rule engine's guarantees.

## Frontend representation (inspector)

Palette type `code` (`CodeNode.vue` + `CodeDrawer.vue`). Relevant `node.data`:

| Field | Meaning |
|---|---|
| `lang` | `"js"` or `"opa"` |
| `logic_rule` | the code (edited in the drawer) |
| `opa_result` | which OPA binding to return (OPA only) |

The node shows a single input and a single output handle — it's a linear step (see
[from-frontend.md](from-frontend.md)). `hasCode` lights the node up once
`logic_rule` is non-empty.

## Compiles to

```go
// inside the compiler hook, roughly:
n := inflowNodes.NewJsNode(data["logic_rule"].(string))   // or NewOpaNode(...)
node.Type = inflowModels.CodeNodeType
node.Code = &n.CodeRule
```

The node's `Key`/`Scope` (universal fields) decide where in the context the output
lands and which slice `input` refers to.

## Next

- [contract.md](contract.md) — same languages, but result is branch tags
- [../nodes.md](../nodes.md) · [from-frontend.md](from-frontend.md)

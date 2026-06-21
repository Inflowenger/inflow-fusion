# Edge Handles and Workflow Traversal

## Handle IDs and Edge Routing

In Vue Flow, each node can define one or more handles. A handle's `id` is an important identifier because it is exported into the workflow model and can later be used by the backend compiler and runtime engine to determine routing behavior.

Example:

```vue
<!-- Output handle on the right -->
<Handle
  id="output"
  type="source"
  :position="Position.Right"
  class="custom-handle output-handle"
/>
```

The `id` value (`output` in this example) is exported as the `sourceHandle` field of the corresponding edge:

```go
SourceHandle string `json:"sourceHandle"`
```

For conditional nodes, multiple output handles may exist. For example:

* `success`
* `error`
* `else`
* `timeout`

When a connection is created from one of these handles, the handle ID is preserved in the exported edge model. During compilation, this value can be used to:

* Tag outgoing transitions.
* Filter which downstream nodes should be executed.
* Determine the next execution path for conditional workflows.

## Finding the Next Node

Each edge also contains the target node identifier:

```go
Target string `json:"target"`
```

The compiler uses the edge's `Target` field to locate the next node in the workflow graph.

A simplified traversal flow is:

1. Read the current node.
2. Inspect outgoing edges.
3. Use `SourceHandle` to determine routing conditions or transition tags.
4. Use `Target` to locate the destination node.
5. Continue traversal until the workflow completes.

## Node Model

After graph traversal and compilation, execution is performed against the following node model:

```go
type Node struct {
    ID         string         `json:"uuid"`
    Type       NodeType       `json:"type"`
    Title      string         `json:"title"`
    Key        string         `json:"key"`
    Scope      string         `json:"scope"` // JSON path

    Code       *CodeRule      `json:"code,omitempty"`
    GoTo       *GoToRule      `json:"goto,omitempty"`
    Event      *EventRule     `json:"event,omitempty"`
    Plugin     *PluginRule    `json:"plugin,omitempty"`
    Contract   *ContractRule  `json:"contract,omitempty"`

    Meta       map[string]any `json:"meta"`
    Tags       string         `json:"tags"`

    Next       []Next         `json:"next"`
    Depends    []string       `json:"depends"`      // Wait until all inbound dependencies are completed.
    NextFilter []string       `json:"next_filters"` // Filter next nodes by edge tags.
}
```

## Relationship Between Edges and `Next`

During compilation, edge definitions are transformed into the `Next` collection of the node model.

The information stored in an edge—particularly `Target` and `SourceHandle`—is used to build the runtime transition graph:

* `Target` identifies the next node.
* `SourceHandle` can be converted into tags or routing metadata.
* `NextFilter` can be used to select only transitions matching specific edge tags.
* `Depends` can enforce synchronization when multiple incoming paths must complete before execution continues.

This design allows frontend-defined connection semantics to be preserved and utilized by the backend execution engine without requiring additional routing configuration.



## Compiled Workflow Model

After the compilation phase, the Vue Flow graph is transformed into the execution model required by the Inflowenger runtime.

The compiler resolves all nodes and edges and produces a flattened workflow definition where each node contains its executable transitions in the `next` field.

Example compiled model:

```json
{
  "start_node": {
    "uuid": "start_node",
    "type": "codeNodeType",
    "next": [
      {
        "flowId": "",
        "nodeId": "1",
        "tags": [],
        "meta": {
          "edgeId": "e-start_node-1-1781193850590",
          "label": "",
          "edgeHandle": "output"
        }
      }
    ]
  },
  "1": {
    "uuid": "1",
    "type": "codeNodeType",
    "next": [
      {
        "flowId": "",
        "nodeId": "2",
        "tags": ["gte", "lte"],
        "meta": {
          "edgeId": "e-1-2-1781193850590",
          "label": "",
          "edgeHandle": "output"
        }
      },
      {
        "flowId": "",
        "nodeId": "4",
        "tags": [],
        "meta": {
          "edgeId": "e-1-4-1781193861962",
          "label": "",
          "edgeHandle": "output"
        }
      }
    ]
  }
}
```

## Transition Mapping

During compilation, every edge is converted into a `Next` entry:

| Edge Property  | Compiled Field    | Description                                         |
| -------------- | ----------------- | --------------------------------------------------- |
| `target`       | `nodeId`          | Destination node identifier.                        |
| `sourceHandle` | `meta.edgeHandle` | Original handle used for the connection.            |
| Edge Tags      | `tags`            | Routing and filtering information used at runtime.  |
| Edge ID        | `meta.edgeId`     | Original edge identifier for debugging and tracing. |

For example, the following Vue Flow connection:

```text
Node 1 (output) --> Node 2
```

becomes:

```json
{
  "nodeId": "2",
  "tags": ["gte", "lte"],
  "meta": {
    "edgeId": "e-1-2-1781193850590",
    "edgeHandle": "output"
  }
}
```

## Runtime Traversal

The Inflowenger engine traverses the workflow using the compiled `next` collection.

Execution flow:

1. Start from `start_node`.
2. Execute the current node.
3. Read all entries in `next`.
4. Apply tag filtering (`tags` and `next_filters`) if configured.
5. Resolve the destination node using `nodeId`.
6. Continue until no further transitions exist.

Example traversal path:

```text
start_node
    ↓
    1
   ↙ ↘
  2   4
  ↓
  3
  ↓
  4
```

## Edge Handle Preservation

The original Vue Flow handle identifier is preserved inside the compiled model:

```json
{
  "meta": {
    "edgeHandle": "output"
  }
}
```

This allows runtime components to:

* Distinguish between multiple outgoing paths.
* Support conditional branching.
* Apply route-specific tags.
* Implement custom transition logic.
* Debug execution paths using the original workflow definition.

## Inflowenger Runtime Requirement

The compiled model is the canonical execution format expected by the Inflowenger system. Frontend graph definitions are not executed directly. Instead, they must first be compiled into this node map structure, where:

* Each node is indexed by its UUID.
* All outgoing transitions are represented by `next`.
* Edge metadata is preserved for runtime decisions.
* Tags are available for routing and filtering.
* Dependencies and synchronization rules can be resolved efficiently during execution.

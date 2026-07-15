# Logs & process events

The engine emits a **process event stream**: one message per meaningful thing that happens while a flow executes. It is the only way an outside observer learns that a process started, which nodes ran, **which edges were actually taken**, and whether the process finished.

This document is the contract between whoever **publishes** these events (the engine) and whoever **consumes** them (a backend, `inspector-api`, the inspector UI, a parser library). It deliberately describes only the wire format — not how the engine is built internally. For other subjects and endpoints see [infra.md](infra.md); for node semantics see [nodes.md](nodes.md).

> **Schema version 1.** Everything below describes `v: 1`, which is the only format there is. A message that does not carry `v: 1` is not an event under this contract — consumers reject it rather than interpret it.

## Why this stream is shaped the way it is

Two consumer needs drive the whole design:

1. **Flow movement** — render a process walking the graph: which node is running, which edge it left by, where it went. This is why `edge.select` exists and why every node-scoped event carries a flow id.
2. **Completion** — know that a process ended, and whether it ended well. This is why `proc.finish` carries a status and why nothing may follow it.

Everything else is diagnostics.

The consumer **already has the flow graph** — it loaded the definition in order to render it. The event stream therefore never re-transmits the graph. Events reference nodes by identity, and the consumer joins against the definition it already holds. This keeps events small and makes them safe to broadcast.

## Transport

Every event in this document arrives on **one subject: the registration's event log**. Payload is a single JSON object, UTF-8, no framing.

**That subject is per-registration, not fixed.** A portal may define its own in `subscribe_prefix`, and the engine publishes there; `inflow.event.log` is only the fallback for a portal that defines none. A consumer that hardcodes the fallback does not error against a portal that set a subject — it silently receives nothing. Resolve the subject from the registration. (`subscribe_prefix` is misnamed: it is the whole subject, not a prefix onto which anything is appended.)

Every message carries a NATS header `rs` — the resource fractal name: the name the publishing fractal instance registered itself with on infra. It is how a consumer tells apart instances that share a subject.

> Infra mirrors process lifecycle events onto a private subject of its own for internal accounting. It is not part of this contract, carries no guarantee, and is not available to consumers — everything a consumer needs is on the event log subject above.

The stream is **shared across every process on that engine**. It is not a per-process channel. Consumers must demultiplex — see [Consumer rules](#consumer-rules).

## The envelope

Every event is exactly this shape:

```json
{
  "v": 1,
  "pid": "33e2f7df-3d47-44c9-a670-38a5d334f238",
  "seq": 42,
  "ts": 1784112492271,
  "kind": "node.enter",
  "level": "info",
  "src": "rt",
  "flow": "flow:22",
  "node": "10",
  "detail": { "type": "ruleNodeType", "title": "Contract" }
}
```

| Field | Type | Required | Meaning |
|---|---|---|---|
| `v` | int | always | Schema version. `1`. |
| `pid` | string | always | Process id. The correlation key for the whole stream. |
| `seq` | int | always | Monotonic counter **per `pid`**, starting at `0`, no gaps. The ordering key. |
| `ts` | int64 | always | Wall clock, Unix milliseconds. **Display only — never order by this.** |
| `kind` | string | always | What happened. See [Event kinds](#event-kinds). |
| `level` | string | always | Severity only: `debug` \| `info` \| `warn` \| `error`. |
| `src` | string | always | Who emitted it. See [`src`](#src). |
| `flow` | string | node-scoped kinds | Flow id owning `node`. |
| `node` | string | node-scoped kinds | Node id — **unique only within `flow`**. |
| `detail` | object | always | Typed per `kind`. Real JSON. Never a formatted string. |

### Node identity

**A node is identified by the pair `(flow, node)`, never by `node` alone.**

Node ids are only unique within their flow. A single process routinely spans several flows — a [GoTo](nodes/goto.md) node jumps into another flow and back — so within one `pid` the id `9` can mean a code node in `flow:22` and a contract node in `flow:33`. Both are live at the same time when flows run in parallel.

Consumers must key all state on the composite `${flow}:${node}`. Producers must set `flow` on every node-scoped event.

### `seq` and ordering

`ts` has millisecond resolution and the engine emits many events per millisecond, so timestamps collide constantly and cannot order anything. `seq` is the only ordering authority.

`seq` is per-`pid` and gapless, which also lets a consumer detect dropped messages and know that a replay is complete.

### `level`

`level` is **severity, nothing else**. It never encodes what kind of event this is. A consumer filtering to `level >= warn` must still receive a coherent (if sparse) event stream; it must not lose lifecycle events because they happened to be classed `info`.

`error` means *this operation failed*. Normal completion is never `error`.

### `src`

Who produced the event, as `actor` or `actor:name`:

| `src` | Meaning |
|---|---|
| `rt` | The runtime itself — lifecycle, routing |
| `js` | JS compiler / user JS code |
| `rego` | OPA compiler / user policy |
| `contract` | Rule evaluation |
| `plugin:<title>` | A [Plugin](nodes/plugin.md) node, e.g. `plugin:RpcFn` |
| `extrinsic:<title>` | An [Extrinsic](nodes/extrinsic.md) node |

`src` is an actor, not a node title. Node identity lives in `flow`/`node`; a bare title in `src` is not a substitute.

## Event kinds

| `kind` | Scope | `detail` |
|---|---|---|
| `proc.start` | process | `{ flow, node, ctxId? }` — the entry point |
| `proc.finish` | process | `{ status, durationMs, error? }` |
| `node.enter` | node | `{ type, title, attempt }` |
| `node.exit` | node | `{ status, durationMs, error? }` |
| `edge.select` | node | `{ taken: EdgeRef[], pruned: EdgeRef[], reason? }` |
| `flow.jump` | node | `{ to: NodeRef, ret: NodeRef? }` |
| `log` | either | `{ msg, ...fields }` — free-form |

```
NodeRef  = { flow: string, node: string }
EdgeRef  = { flow: string, node: string, edgeId: string, tags: string[] }
```

`edgeId` matches the edge id in the flow definition, so the consumer can light up the exact edge it is already rendering.

### `proc.start` / `proc.finish`

Exactly one of each per `pid`. `proc.start` is always `seq: 0`. **`proc.finish` is always the last event for that `pid`** — nothing may follow it.

`detail.status` is one of:

| `status` | `level` | Meaning |
|---|---|---|
| `completed` | `info` | Ran to the end |
| `failed` | `error` | Aborted on an error |
| `stopped` | `info` | Cancelled by user or timeout |

A process that completes normally emits `proc.finish` with `status: "completed"` and `level: "info"`. It does **not** additionally emit an error. `failed` carries `detail.error`.

Note that a process can complete while individual nodes failed — node-level failure does not imply `failed`. `status` describes the process, and `node.exit` describes the node.

### `node.enter` / `node.exit`

Paired, one `exit` per `enter`, same `(flow, node)`. A node may run more than once in a process (loops, re-entry via GoTo); `detail.attempt` counts from `1` and disambiguates.

`node.exit` carries `status: "ok" | "error"` and, on error, `detail.error`. A node that fails still emits `node.exit` — failure is not a missing event.

### `edge.select`

The routing decision, emitted after `node.enter` and before `node.exit`.

```json
{
  "v": 1, "pid": "33e2f7df…", "seq": 7, "ts": 1784112492271,
  "kind": "edge.select", "level": "info", "src": "rt",
  "flow": "flow:22", "node": "10",
  "detail": {
    "taken":  [ { "flow": "flow:22", "node": "11", "edgeId": "e-10-11-1783103865000", "tags": ["T"] } ],
    "pruned": [ { "flow": "flow:22", "node": "12", "edgeId": "e-10-12-1783103869460", "tags": ["F"] } ],
    "reason": { "tags": ["T"] }
  }
}
```

**Every node with outgoing edges emits `edge.select` — not only branching nodes.** A [Code](nodes/code.md) node with two unconditional outputs emits one with both edges in `taken` and `pruned: []`. A [Contract](nodes/contract.md) node emits one with `reason.tags` set to the tags its rule returned.

**This includes the process's entry node**, which is a special case worth stating: the entry node does not execute as a task, but it still emits the edges out of itself, immediately after `proc.start`. Without it the first nodes of a flow would report entering with nothing explaining how control reached them, and the traversal graph would start disconnected.

This is the single most important event for reconstructing movement, and it is why the stream is self-sufficient: a consumer never has to infer traversal from the graph's static shape, so it stays correct even if its copy of the definition has drifted.

A terminal node (no outgoing edges) emits no `edge.select`.

**A node emits one `edge.select` per execution.** Loops are ordinary in Inflow, and a node that runs many times reports its routing decision each time — the decisions describe that pass, not the node's lifetime. See [Loops](#loops) for what that means for a consumer.

### `flow.jump`

Emitted by a [GoTo](nodes/goto.md) node. `detail.to` is where control transfers; `detail.ret` is where it resumes afterwards, omitted if it does not return. This is what tells a consumer that the `(flow, node)` namespace is about to change.

### `log`

Free-form diagnostics from user code, plugins, or the runtime. `detail.msg` is a human-readable string; any other keys are structured context. This is the only kind whose `detail` is open-ended, and the only one a consumer may safely ignore entirely.

## Producer rules

1. **Set `flow` on every node-scoped event.** Without it `node` is ambiguous and consumers corrupt state on GoTo. Non-negotiable.
2. **`seq` is monotonic and gapless per `pid`.**
3. **`detail` is real JSON.** Never format a struct into a string. Structured data that arrives as `"[{flow:22 11 [T] 0 map[edgeId:e-10-11 …]}]"` is unparseable and does not satisfy this contract.
4. **Never include the node definition.** No code source, rule source, plugin config, or extrinsic config. Consumers have the graph; see [Secrets](#secrets).
5. **Emit `edge.select` for every node with outgoing edges**, not only branching ones.
6. **Completion is `proc.finish`, not an error**, and nothing follows it.
7. **`level` is severity only**, orthogonal to `kind`.
8. **`kind` values are stable identifiers** — lowercase, dotted, machine-readable. Not sentences.

## Consumer rules

1. **Demultiplex by `pid`.** The stream carries every process on the engine, interleaved. Ignore events for pids you are not observing.
2. **Order by `seq`, never `ts`.** Timestamps collide within a millisecond.
3. **Key state on `${flow}:${node}`.**
4. **Expect a node to run many times.** See [Loops](#loops): accumulate, don't replace, and don't grow per pass.
5. **Tolerate unknown `kind` and unknown `detail` keys** — forward compatibility. Skip, do not fail.
6. **Tolerate non-conforming lines.** The transport is shared and may carry non-event traffic (connection banners, etc.). A line that is not a valid envelope is skipped, not an error.
7. **Do not trust `detail.msg` as markup.** It can contain user-authored content from a Code node.

### Reconstructing movement

The stream states movement explicitly; no inference is needed.

- `proc.start` → mark `detail.flow:detail.node` as the entry point. The `edge.select` that follows it is the entry node's own, and connects it to the first nodes that run.
- `node.enter` → mark `${flow}:${node}` running. `detail.attempt` says which pass this is.
- `edge.select` → for each `taken` entry, mark edge `edgeId` traversed and its target pending. Record each `pruned` entry's edge as evaluated-but-not-followed (useful to grey out the untaken branch of a [Contract](nodes/contract.md)) — without clearing a traversal an earlier pass already recorded.
- `node.exit` → mark the node settled, `ok` or `error`. A node failing is not the process failing.
- `flow.jump` → follow into `detail.to`; the namespace changes.
- `proc.finish` → the process is done; `detail.status` says how. Nothing follows it.

Concurrency is normal: many nodes are `running` at once, and events for different subtrees interleave. A consumer maintaining per-node state keyed on `${flow}:${node}` handles this without special cases.

### Loops

A flow can route back on itself, and a node can be processed many times in one process — often, and without bound. Two consequences a consumer must handle:

**Accumulate decisions; don't replace them.** The same edge is decided afresh on every pass, and a loop routinely *takes* an edge on one pass and *prunes* it on the next. Only the union describes the path the flow actually walked: once control has crossed an edge, that fact is permanent, and a later rejection must not erase it. Treat "was this edge ever travelled" and "was it chosen on the latest pass" as different questions — a path view wants the first.

**Keep state proportional to the graph, not the pass count.** `node.enter`'s `detail.attempt` and a per-edge traversal count carry a loop's history in two integers. A consumer that appends per pass — one entry per traversal, every log line retained against its node — grows without limit in exactly the flows that run longest.

Events still fire on every pass. That is deliberate: a consumer animating movement needs each traversal, even though its state only keeps the totals.

## Secrets

**Credentials must never appear in the event stream.**

The stream is broadcast to every observer of an engine. Node definitions contain live secrets — a [Plugin](nodes/plugin.md) node's `infra_isolated.cred` is a NATS user JWT **and its NKEY seed**, i.e. a private key granting whatever that account can do. Emitting a node definition into an event puts that private key on a broadcast channel, and into every consumer's log buffer, scrollback, and browser memory.

Producer rule 4 (never include the node definition) exists primarily for this reason. Size and redundancy are the secondary benefit.

Consumers of the event log must additionally scope what they forward: a consumer that relays events onward (a websocket fan-out, for instance) is responsible for filtering to the pids its client is entitled to see. Relaying the raw stream to every connected client gives each of them every other tenant's execution history.

## Reference consumers

Two libraries implement this document. Neither is required, but a consumer that hand-rolls the shapes tends to rediscover the rules above the hard way.

### Go — `inflow-fusion/logs`

For a **backend keeping process history**. Tokens, wire types, and capture helpers; no transport, because which pid a caller may see is its policy, not the package's.

```go
import "github.com/Inflowenger/inflow-fusion/logs"

// The subject is per-registration — see Transport.
subject := logs.EventLogSubjectOf(registration)

nc.Subscribe(subject, func(msg *nats.Msg) {
    if f, ok := logs.CaptureProcFinish(msg.Data); ok {
        // f.Pid has ended: f.Status, f.DurationMs, f.Error.
        // Nothing follows proc.finish, so this row will not be contradicted.
    }
})
```

`CaptureProcStart` / `CaptureProcFinish` return flattened, ready-to-store bodies for the two events a backend persists — they open and close a process's history row. Both return `false` rather than an error for anything else on the subject: on a shared stream, "not mine" is the normal case, not a fault.

For the other kinds, `logs.Parse` gives the envelope and `logs.DetailOf[T]` decodes a body:

```go
ev, err := logs.Parse(msg.Data)          // rejects non-events; Detail stays lazy
d, err := logs.DetailOf[logs.EdgeSelectDetail](ev)
```

Note `proc.finish` carries **no flow id** — the contract does not repeat one there. A backend that needs it takes it from this pid's `proc.start`, or already knows it because it started the process itself.

### JavaScript — `@inflowenger/flow-trace`

For **rendering movement**. In the `inflow-vue/inflow-inspector` workspace. It implements every consumer rule above: demultiplexes by pid, reorders by seq, keys state on `flow:node`, handles loops, and turns the stream into `move` / `finish` events. Its test suite runs against a captured engine run, so it doubles as an executable check that a producer still honours this document.

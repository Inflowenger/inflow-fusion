# Architecture

This document explains how the pieces of the Inflowenger platform relate to each other, and where a backend built with `inflow-fusion` sits in that picture. It intentionally stays at the concept level — for exact endpoints/subjects see [infra.md](infra.md).

## The four actors

**Infra (control plane).** A central service that owns accounts and identity: it issues NATS credentials, tracks which "inflow engine" instances exist, and hands out per-account NATS accounts (`sys`, `inflow`, `plugins`) that everything else authenticates against. Your backend talks to it over plain REST, authenticated with a bearer JWT signed with a secret both sides share (`INFLOW_INFRA_JWT_SECRET`).

**Your backend (this SDK).** Owns the actual product data: flow definitions, execution context, and whatever business entities your domain has. It does not execute flows. It answers three questions asked over NATS (get a flow, get a process's context, set a process's context — see `IInflowService`), and optionally exposes extra domain actions as "extrinsic" services that flow nodes can call.

**Inflow engine instance(s).** The execution runtime. Given a `ProcessRequest` (start node, flow id, context id, timeouts), an engine walks the compiled node graph, asking your backend for whatever it doesn't already have, and executing each node according to its type (run code, evaluate a rule, call an extrinsic subject, hand off to a plugin, jump to another flow). Multiple engine instances can be registered with infra; `inflow-fusion` round-robins new process requests across them (see `inflow.NewProcess` / `inflow.GetResourceCandid`).

**Plugins.** Isolated external integrations (e.g. Jira, Slack) that a flow's `Plugin` node talks to over NATS. Each plugin gets its own NATS account/credentials, scoped so it can only see the subjects that belong to it — see `spaces.PluginCredentialStrictPermission`.

## Request flow, end to end

1. Your process starts up and calls `inflow.InitBackend(...)`. This fetches NATS credentials for the `inflow` account from infra's REST API, connects to NATS, and subscribes to the three default subjects (`inflow.req.flow.get.*`, `inflow.req.context.get.*`, `inflow.req.context.set.*`).
2. `InitBackend` also calls `ReloadResources`, which asks infra for the list of currently registered engine instances and loads them into a round-robin pool.
3. Something in your app (an API handler, a scheduler, ...) calls `inflow.NewProcess(startNodeId, inflow.WithFlowId(...), inflow.WithContextDocument(...))` and then `.Exec(ctx)`. This picks the next engine instance from the round-robin pool and POSTs a `ProcessRequest` to `{engine}/engine`.
4. The engine instance, as it traverses nodes, publishes NATS requests back to your backend's subscriptions to fetch the flow definition and the running context, and to persist context updates as nodes execute.
5. If a node is an `Extrinsic` node, the engine publishes to whatever subject you registered with `svcHandler.ImplHandlerOnSubject`, and your handler's return value becomes that node's output.
6. If a node is a `Plugin` node, the engine talks to the external plugin process directly over NATS, using credentials scoped by `spaces` so the plugin can't see anything outside its own subject namespace.
7. Your app can stop a running process early with `inflow.StopProcess(ctx, pid, resourceUrl)`.

## Why NATS request/reply instead of a direct DB connection?

The engine is deliberately kept ignorant of your storage. It never touches your database — it only knows how to ask "give me the flow with this id" and "give me/take this context" over a well-known subject shape. That's what makes the same engine usable across different backends: each backend just needs to answer those three questions correctly, however it likes to store the data.

## Multi-tenancy / isolation

Infra models accounts as NATS accounts, not just database rows. Three built-in accounts exist (`sys`, `inflow`, `plugins` — see `models.BUILTIN_*_ACCOUNT*`). Your backend authenticates as the `inflow` account. Plugins get credentials scoped to their own account and, within it, to a subject prefix unique to that plugin instance (`spaces.GetInboxConfigWithPluginId`, `spaces.PluginCredentialStrictPermission`) — so one plugin instance cannot observe another's traffic even though they share NATS infrastructure.

## Next

- [infra.md](infra.md) — exact REST endpoints, NATS subjects, and payload shapes
- [logs.md](logs.md) — the process event stream engines publish while executing a flow
- [nodes.md](nodes.md) — what each node type does and how to build one
- [compilers](compilers) — turning a frontend-authored graph into the node map the engine consumes

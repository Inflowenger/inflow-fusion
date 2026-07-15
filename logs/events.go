// Package logs is the consumer-side contract for the process event stream.
//
// The engine publishes one event per meaningful thing that happens while a flow
// executes; this package is what a backend uses to read them without hand-rolling
// JSON shapes or string-matching raw payloads. The wire format it implements is
// documented in docs/logs.md — read that before changing anything here.
//
// It is deliberately dependency-light: types, tokens, and capture helpers, no
// transport. Subscribing is the caller's job, because who is entitled to see
// which pid is the caller's policy, not this package's.
package logs

import (
	"encoding/json"

	"github.com/Inflowenger/inflow-fusion/models"
)

// EventSchemaVersion is the `v` field of every event this package accepts.
// Events carrying any other version are rejected rather than guessed at.
const EventSchemaVersion = 1

// Subjects the engine publishes on.
const (
	// DefaultSubjectEventLog carries every event kind, for every process on the
	// engine. It is only the fallback: the subject is per-registration, so
	// resolve it with EventLogSubject rather than subscribing to this directly.
	DefaultSubjectEventLog = "inflow.event.log"
	// SubjectTraceProcs carries only proc.start and proc.finish — the subject to
	// subscribe to when all you maintain is process accounting. Unlike the event
	// log it is fixed, and shared by every registration.
	SubjectTraceProcs = "_infra.trace.ps"
	// HeaderRegistration names the publishing engine registration on every
	// message, which is how a consumer tells apart engines that share a subject.
	HeaderRegistration = "rs"
)

// EventLogSubject returns the subject a registration publishes its event log on.
//
// A portal may define a subject of its own, and the engine publishes there
// instead of on the default — so a consumer that hardcodes the default hears
// nothing at all from a portal that set one. Pass the registration's
// `subscribe_prefix`; empty means the portal defined none.
//
// Despite the name, subscribe_prefix is the whole subject, not a prefix onto
// which anything is appended.
func EventLogSubject(subsPrefix string) string {
	if subsPrefix == "" {
		return DefaultSubjectEventLog
	}
	return subsPrefix
}

// EventLogSubjectOf is EventLogSubject for a registration listed off the infra
// API, which is where a backend usually gets one.
func EventLogSubjectOf(r models.RegisteredInflow) string {
	return EventLogSubject(r.RegisterPortal.SubsPrefix)
}

// EventKind is what happened. It is orthogonal to Level: kind says which event
// this is, Level says only how severe it is. A consumer that filters on severity
// must still receive a coherent event stream, so never infer one from the other.
type EventKind string

const (
	KindProcStart  EventKind = "proc.start"
	KindProcFinish EventKind = "proc.finish"
	KindNodeEnter  EventKind = "node.enter"
	KindNodeExit   EventKind = "node.exit"
	KindEdgeSelect EventKind = "edge.select"
	KindFlowJump   EventKind = "flow.jump"
	KindLog        EventKind = "log"
)

// Level is severity, nothing else.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Src identifies the actor that produced an event, as "actor" or "actor:name".
// Node identity lives in ProcEvent.Flow/Node — a bare node title is not a Src.
const (
	SrcRuntime  = "rt"
	SrcJS       = "js"
	SrcRego     = "rego"
	SrcContract = "contract"
)

func SrcPlugin(title string) string    { return "plugin:" + title }
func SrcExtrinsic(title string) string { return "extrinsic:" + title }

// How a process ended (ProcFinish.Status).
const (
	FinishCompleted = "completed"
	FinishFailed    = "failed"
	FinishStopped   = "stopped"
)

// How a single node settled (NodeExit.Status).
const (
	ExitOK    = "ok"
	ExitError = "error"
)

// ProcEvent is the envelope every event arrives in.
//
// Detail stays undecoded: the stream is shared across every process on the
// engine, so most events a given consumer sees are for pids it does not care
// about, and paying to decode their bodies is waste. Use the capture helpers,
// or DetailOf, once you have decided the event is yours.
type ProcEvent struct {
	V     int       `json:"v" bson:"v"`
	Pid   string    `json:"pid" bson:"pid"`
	Seq   int64     `json:"seq" bson:"seq"`
	Ts    int64     `json:"ts" bson:"ts"`
	Kind  EventKind `json:"kind" bson:"kind"`
	Level Level     `json:"level" bson:"level"`
	Src   string    `json:"src" bson:"src"`
	// Flow scopes Node: node ids are unique only within a flow, and one process
	// spans several flows via GoTo. Both are set on node-scoped kinds and absent
	// on process-scoped ones.
	Flow   string          `json:"flow,omitempty" bson:"flow,omitempty"`
	Node   string          `json:"node,omitempty" bson:"node,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty" bson:"detail,omitempty"`
}

// NodeKey is the composite identity a consumer must key node state on.
//
// The bare node id is not an identity: it is unique only within its flow, and a
// process routinely spans several flows, so keying on it alone corrupts state
// the moment a GoTo enters a flow that reuses an id.
func (e ProcEvent) NodeKey() string { return NodeKey(e.Flow, e.Node) }

// NodeKey builds the composite `flow:node` identity.
func NodeKey(flow, node string) string { return flow + ":" + node }

// NodeRef identifies a node. Both fields are required — see ProcEvent.Flow.
type NodeRef struct {
	Flow string `json:"flow"`
	Node string `json:"node"`
}

func (r NodeRef) Key() string { return NodeKey(r.Flow, r.Node) }

// EdgeRef is an outgoing edge and where it lands. EdgeId matches the edge id in
// the flow definition, so a consumer can light up the exact edge it renders.
type EdgeRef struct {
	Flow   string   `json:"flow"`
	Node   string   `json:"node"`
	EdgeId string   `json:"edgeId"`
	Tags   []string `json:"tags"`
}

// ---- detail bodies, one per kind -------------------------------------------

type ProcStartDetail struct {
	Flow string `json:"flow"`
	Node string `json:"node"`
}

type ProcFinishDetail struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type NodeEnterDetail struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Attempt int    `json:"attempt"`
}

type NodeExitDetail struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// EdgeSelectDetail is the routing decision. Every node with outgoing edges emits
// one, not only branching nodes, so consumers never infer traversal from their
// own copy of the graph.
type EdgeSelectDetail struct {
	Taken  []EdgeRef   `json:"taken"`
	Pruned []EdgeRef   `json:"pruned"`
	Reason *EdgeReason `json:"reason,omitempty"`
}

// EdgeReason explains a non-trivial selection, e.g. the tags a contract returned.
type EdgeReason struct {
	Tags []string `json:"tags"`
}

type FlowJumpDetail struct {
	To  NodeRef  `json:"to"`
	Ret *NodeRef `json:"ret,omitempty"`
}

// LogDetail is free-form diagnostics. Msg is human-readable and may contain
// user-authored content from a Code node — never treat it as markup. Fields
// carries any other keys the emitter attached.
type LogDetail struct {
	Msg    string         `json:"msg"`
	Fields map[string]any `json:"-"`
}

// UnmarshalJSON keeps Msg typed while preserving the open-ended context keys
// that make a log event worth reading.
func (d *LogDetail) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if msg, ok := raw["msg"].(string); ok {
		d.Msg = msg
	}
	delete(raw, "msg")
	d.Fields = raw
	return nil
}

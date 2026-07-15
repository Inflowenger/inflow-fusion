package logs

import (
	"fmt"

	"github.com/bytedance/sonic"
)

// RejectReason says why a message was not a usable v1 event.
//
// Rejection is normal, not exceptional: the subject is shared and carries
// traffic that is not a process event at all, so nothing here panics or logs —
// callers skip what they cannot use. See consumer rule 6 in docs/logs.md.
type RejectReason string

const (
	// RejectNotJSON — not a JSON object.
	RejectNotJSON RejectReason = "not-json"
	// RejectUnsupportedVersion — a `v` this build does not know. An engine that
	// predates the versioned stream has no `v` at all and lands here too.
	RejectUnsupportedVersion RejectReason = "unsupported-version"
	// RejectMalformed — right shape, missing fields the envelope requires.
	RejectMalformed RejectReason = "malformed"
)

// RejectError carries the reason a message was not a usable event, so an
// operator can be told "your engine speaks a version I don't" rather than
// "garbage on the wire".
type RejectError struct {
	Reason  RejectReason
	Version int
}

func (e *RejectError) Error() string {
	if e.Reason == RejectUnsupportedVersion {
		return fmt.Sprintf("logs: %s (v=%d, want v=%d)", e.Reason, e.Version, EventSchemaVersion)
	}
	return fmt.Sprintf("logs: %s", e.Reason)
}

// Parse validates one message off the wire and returns its envelope.
//
// Detail is left undecoded — see ProcEvent.Detail for why. A caller holding the
// envelope can cheaply check Kind and Pid and drop the event before paying for
// the body.
func Parse(raw []byte) (ProcEvent, error) {
	var e ProcEvent
	if err := sonic.Unmarshal(raw, &e); err != nil {
		return ProcEvent{}, &RejectError{Reason: RejectNotJSON}
	}
	if e.V != EventSchemaVersion {
		return ProcEvent{}, &RejectError{Reason: RejectUnsupportedVersion, Version: e.V}
	}
	// Kind is intentionally not checked against the known set: an unknown kind is
	// forward compatibility, and it is the consumer's job to ignore it rather
	// than to reject the message.
	if e.Pid == "" || e.Kind == "" || e.Src == "" || e.Level == "" {
		return ProcEvent{}, &RejectError{Reason: RejectMalformed}
	}
	return e, nil
}

// IsEvent reports whether raw is a usable v1 event.
func IsEvent(raw []byte) bool {
	_, err := Parse(raw)
	return err == nil
}

// DetailOf decodes an event's body into the type its Kind implies.
//
// This is the escape hatch for the kinds without a flattened capture below —
// node.enter, node.exit, edge.select, flow.jump, log — which are the business of
// something rendering the graph rather than of a backend keeping accounts:
//
//	d, err := logs.DetailOf[logs.EdgeSelectDetail](ev)
//
// It does not check that T matches ev.Kind; asking for the wrong body yields a
// zero-valued one, not an error.
func DetailOf[T any](e ProcEvent) (T, error) {
	var d T
	if len(e.Detail) == 0 {
		return d, &RejectError{Reason: RejectMalformed}
	}
	if err := sonic.Unmarshal(e.Detail, &d); err != nil {
		return d, &RejectError{Reason: RejectMalformed}
	}
	return d, nil
}

// ---- process lifecycle -----------------------------------------------------
//
// proc.start and proc.finish get flattened, ready-to-store bodies because they
// are the two events a backend actually persists: they open and close a
// process's history row. Everything else in the stream describes movement
// through the graph, which is a renderer's concern rather than a record's.
//
// Both arrive on the event log subject like every other kind — filter for them
// with the captures below.

// ProcStart is the opening event of a process. Exactly one per pid, always
// Seq 0, and it is the only event that names the flow a process entered at —
// proc.finish does not repeat it.
type ProcStart struct {
	Pid string `json:"pid" bson:"pid"`
	Seq int64  `json:"seq" bson:"seq"`
	Ts  int64  `json:"ts" bson:"ts"`
	// Flow and Node are the entry point, read out of the event's detail rather
	// than the envelope: proc.start is process-scoped, so it carries no
	// envelope-level node identity.
	Flow string `json:"flow" bson:"flow"`
	Node string `json:"node" bson:"node"`
}

// ProcFinish is the terminal event of a process. Exactly one per pid, and
// nothing follows it — a record updated from this will not be contradicted later.
//
// It carries no flow id: the contract does not repeat one here. A consumer that
// needs the flow already has it, either from this pid's ProcStart or because it
// started the process itself.
type ProcFinish struct {
	Pid string `json:"pid" bson:"pid"`
	Seq int64  `json:"seq" bson:"seq"`
	Ts  int64  `json:"ts" bson:"ts"`
	// Status is FinishCompleted, FinishFailed or FinishStopped.
	Status     string `json:"status" bson:"status"`
	DurationMs int64  `json:"durationMs" bson:"durationMs"`
	// Error is set only when Status is FinishFailed.
	Error string `json:"error,omitempty" bson:"error,omitempty"`
}

// Completed reports whether the process ran to the end.
//
// Note that a process can complete while individual nodes failed: node-level
// failure does not imply the process failed. Status describes the process.
func (f ProcFinish) Completed() bool { return f.Status == FinishCompleted }

// Failed reports whether the process aborted on an error. Error holds the cause.
func (f ProcFinish) Failed() bool { return f.Status == FinishFailed }

// Stopped reports whether the process was cancelled by a user or a timeout.
// This is not a failure.
func (f ProcFinish) Stopped() bool { return f.Status == FinishStopped }

// CaptureProcStart returns the start body if raw is a v1 proc.start event, and
// false for anything else — a different kind, another version, or non-event
// traffic. It never errors: on a shared subject, "not mine" is the common case.
func CaptureProcStart(raw []byte) (ProcStart, bool) {
	e, err := Parse(raw)
	if err != nil {
		return ProcStart{}, false
	}
	return ProcStartOf(e)
}

// CaptureProcFinish returns the finish body if raw is a v1 proc.finish event.
//
// This is the whole point of the package for a backend keeping process history:
//
//	if f, ok := logs.CaptureProcFinish(msg.Data); ok {
//	    // f.Pid is done — f.Status, f.DurationMs, f.Error
//	}
func CaptureProcFinish(raw []byte) (ProcFinish, bool) {
	e, err := Parse(raw)
	if err != nil {
		return ProcFinish{}, false
	}
	return ProcFinishOf(e)
}

// ProcStartOf is CaptureProcStart for an envelope already parsed, so a consumer
// switching on Kind does not parse the same message twice.
func ProcStartOf(e ProcEvent) (ProcStart, bool) {
	if e.Kind != KindProcStart {
		return ProcStart{}, false
	}
	d, err := DetailOf[ProcStartDetail](e)
	if err != nil {
		return ProcStart{}, false
	}
	return ProcStart{Pid: e.Pid, Seq: e.Seq, Ts: e.Ts, Flow: d.Flow, Node: d.Node}, true
}

// ProcFinishOf is CaptureProcFinish for an envelope already parsed.
func ProcFinishOf(e ProcEvent) (ProcFinish, bool) {
	if e.Kind != KindProcFinish {
		return ProcFinish{}, false
	}
	d, err := DetailOf[ProcFinishDetail](e)
	if err != nil {
		return ProcFinish{}, false
	}
	return ProcFinish{
		Pid:        e.Pid,
		Seq:        e.Seq,
		Ts:         e.Ts,
		Status:     d.Status,
		DurationMs: d.DurationMs,
		Error:      d.Error,
	}, true
}

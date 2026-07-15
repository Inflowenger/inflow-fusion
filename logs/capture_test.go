package logs

import (
	"testing"

	"github.com/Inflowenger/inflow-fusion/models"
)

// The event log subject is per-registration: a portal may define its own, and
// the engine publishes there instead of on the default. A consumer that resolves
// this wrong does not error — it just silently hears nothing.
func TestEventLogSubject(t *testing.T) {
	if got := EventLogSubject(""); got != DefaultSubjectEventLog {
		t.Errorf("unset portal subject = %q, want the default %q", got, DefaultSubjectEventLog)
	}
	if got := EventLogSubject("acme.event.log"); got != "acme.event.log" {
		t.Errorf("subject = %q, want the portal's own", got)
	}

	reg := models.RegisteredInflow{RegisterPortal: models.Portal{SubsPrefix: "acme.event.log"}}
	if got := EventLogSubjectOf(reg); got != "acme.event.log" {
		t.Errorf("EventLogSubjectOf = %q, want acme.event.log", got)
	}
	if got := EventLogSubjectOf(models.RegisteredInflow{}); got != DefaultSubjectEventLog {
		t.Errorf("EventLogSubjectOf on a portal with no subject = %q, want the default", got)
	}
}

// Fixtures are real lines captured off `inflow.event.log` from an engine run —
// the same run the flow-trace package tests against. Hand-written JSON would
// only prove this package agrees with itself.
const (
	realProcStart  = `{"v":1,"pid":"cf31ecb0-0979-401e-b2e3-c9c6bfd5b713","seq":0,"ts":1784116836285,"kind":"proc.start","level":"info","src":"rt","detail":{"flow":"flow:22","node":"6"}}`
	realProcFinish = `{"v":1,"pid":"cf31ecb0-0979-401e-b2e3-c9c6bfd5b713","seq":58,"ts":1784116836313,"kind":"proc.finish","level":"info","src":"rt","detail":{"status":"completed","durationMs":74}}`
	realEdgeSelect = `{"v":1,"pid":"cf31ecb0-0979-401e-b2e3-c9c6bfd5b713","seq":1,"ts":1784116836285,"kind":"edge.select","level":"info","src":"rt","flow":"flow:22","node":"6","detail":{"taken":[{"flow":"flow:22","node":"10","edgeId":"e-6-10-1783103861016","tags":[]}],"pruned":[]}}`
)

func TestCaptureProcFinish(t *testing.T) {
	f, ok := CaptureProcFinish([]byte(realProcFinish))
	if !ok {
		t.Fatal("real proc.finish not captured")
	}
	if f.Pid != "cf31ecb0-0979-401e-b2e3-c9c6bfd5b713" {
		t.Errorf("pid = %q", f.Pid)
	}
	if f.Status != FinishCompleted || !f.Completed() {
		t.Errorf("status = %q, want completed", f.Status)
	}
	if f.Failed() || f.Stopped() {
		t.Error("a completed process reported failed or stopped")
	}
	if f.DurationMs != 74 {
		t.Errorf("durationMs = %d, want 74", f.DurationMs)
	}
	if f.Error != "" {
		t.Errorf("a completed process carried an error: %q", f.Error)
	}
}

// A failed finish carries its cause; a stopped one is not a failure. Both are
// what a backend writes onto the history row, so both must survive the capture.
func TestCaptureProcFinishOutcomes(t *testing.T) {
	failed := `{"v":1,"pid":"p1","seq":9,"ts":1,"kind":"proc.finish","level":"error","src":"rt","detail":{"status":"failed","durationMs":12,"error":"code: 3, message: boom"}}`
	f, ok := CaptureProcFinish([]byte(failed))
	if !ok || !f.Failed() {
		t.Fatalf("failed finish not captured: %+v ok=%v", f, ok)
	}
	if f.Error != "code: 3, message: boom" {
		t.Errorf("error = %q", f.Error)
	}

	stopped := `{"v":1,"pid":"p1","seq":9,"ts":1,"kind":"proc.finish","level":"info","src":"rt","detail":{"status":"stopped","durationMs":5}}`
	s, ok := CaptureProcFinish([]byte(stopped))
	if !ok || !s.Stopped() {
		t.Fatalf("stopped finish not captured: %+v ok=%v", s, ok)
	}
	if s.Failed() || s.Completed() {
		t.Error("a stopped process reported failed or completed")
	}
}

func TestCaptureProcStart(t *testing.T) {
	s, ok := CaptureProcStart([]byte(realProcStart))
	if !ok {
		t.Fatal("real proc.start not captured")
	}
	if s.Seq != 0 {
		t.Errorf("seq = %d, want 0 — proc.start is always the first event", s.Seq)
	}
	// The entry point lives in the detail, not the envelope: proc.start is
	// process-scoped and carries no envelope-level flow/node.
	if s.Flow != "flow:22" || s.Node != "6" {
		t.Errorf("entry = %s:%s, want flow:22:6", s.Flow, s.Node)
	}
}

// Each capture answers for its own kind only. Without this a consumer switching
// on the wrong helper would silently read a zero-valued body as real.
func TestCapturesAreKindScoped(t *testing.T) {
	if _, ok := CaptureProcFinish([]byte(realProcStart)); ok {
		t.Error("proc.start captured as a finish")
	}
	if _, ok := CaptureProcStart([]byte(realProcFinish)); ok {
		t.Error("proc.finish captured as a start")
	}
	if _, ok := CaptureProcFinish([]byte(realEdgeSelect)); ok {
		t.Error("edge.select captured as a finish")
	}
}

// The subject is shared and carries traffic that is not a process event. None of
// it may reach a caller as a usable event, and none of it may panic.
func TestParseRejects(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want RejectReason
	}{
		{"not json", `hello`, RejectNotJSON},
		{"not an object", `[1,2,3]`, RejectNotJSON},
		{"empty", ``, RejectNotJSON},
		// An engine predating the versioned stream sends no `v` at all. It is an
		// unsupported version, not corruption, and it is told apart so an operator
		// hears "your engine is too old" rather than "garbage on the wire".
		{"no version", `{"pid":"p1","kind":"proc.finish","level":"info","src":"rt"}`, RejectUnsupportedVersion},
		{"future version", `{"v":99,"pid":"p1","seq":0,"ts":1,"kind":"proc.finish","level":"info","src":"rt"}`, RejectUnsupportedVersion},
		{"no pid", `{"v":1,"seq":0,"ts":1,"kind":"proc.finish","level":"info","src":"rt"}`, RejectMalformed},
		{"no kind", `{"v":1,"pid":"p1","seq":0,"ts":1,"level":"info","src":"rt"}`, RejectMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.raw))
			if err == nil {
				t.Fatalf("%q accepted", tc.raw)
			}
			re, ok := err.(*RejectError)
			if !ok {
				t.Fatalf("err = %T, want *RejectError", err)
			}
			if re.Reason != tc.want {
				t.Errorf("reason = %q, want %q", re.Reason, tc.want)
			}
			if IsEvent([]byte(tc.raw)) {
				t.Error("IsEvent accepted a rejected message")
			}
			// Rejection must not reach a caller as a usable capture either.
			if _, ok := CaptureProcFinish([]byte(tc.raw)); ok {
				t.Error("CaptureProcFinish accepted a rejected message")
			}
		})
	}
}

// An unknown kind is forward compatibility, not corruption: a consumer skips it,
// it does not fail the parse.
func TestParseToleratesUnknownKind(t *testing.T) {
	raw := `{"v":1,"pid":"p1","seq":3,"ts":1,"kind":"node.retry","level":"info","src":"rt","detail":{"whatever":true}}`
	e, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unknown kind rejected: %v", err)
	}
	if e.Kind != "node.retry" {
		t.Errorf("kind = %q", e.Kind)
	}
}

func TestDetailOf(t *testing.T) {
	e, err := Parse([]byte(realEdgeSelect))
	if err != nil {
		t.Fatalf("real edge.select rejected: %v", err)
	}
	d, err := DetailOf[EdgeSelectDetail](e)
	if err != nil {
		t.Fatalf("DetailOf: %v", err)
	}
	if len(d.Taken) != 1 || d.Taken[0].EdgeId != "e-6-10-1783103861016" {
		t.Errorf("taken = %+v", d.Taken)
	}
	if len(d.Pruned) != 0 {
		t.Errorf("pruned = %+v, want none", d.Pruned)
	}
}

// Node ids are unique only within their flow, and one process spans several via
// GoTo, so state must be keyed on the pair.
func TestNodeKey(t *testing.T) {
	e, err := Parse([]byte(realEdgeSelect))
	if err != nil {
		t.Fatalf("real edge.select rejected: %v", err)
	}
	if got := e.NodeKey(); got != "flow:22:6" {
		t.Errorf("NodeKey = %q, want flow:22:6", got)
	}
	if got := (NodeRef{Flow: "flow:33", Node: "9"}).Key(); got != "flow:33:9" {
		t.Errorf("NodeRef.Key = %q", got)
	}
}

// `msg` is typed; every other key is context that must survive decoding.
func TestLogDetail(t *testing.T) {
	e, err := Parse([]byte(`{"v":1,"pid":"p1","seq":4,"ts":1,"kind":"log","level":"error","src":"js","flow":"flow:22","node":"11","detail":{"msg":"boom","code":3}}`))
	if err != nil {
		t.Fatalf("log event rejected: %v", err)
	}
	d, err := DetailOf[LogDetail](e)
	if err != nil {
		t.Fatalf("DetailOf: %v", err)
	}
	if d.Msg != "boom" {
		t.Errorf("msg = %q", d.Msg)
	}
	if _, ok := d.Fields["msg"]; ok {
		t.Error("msg leaked into Fields")
	}
	if code, ok := d.Fields["code"].(float64); !ok || code != 3 {
		t.Errorf("fields = %+v, want code 3", d.Fields)
	}
}

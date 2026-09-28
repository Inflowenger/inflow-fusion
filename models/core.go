package models

const (
	INFLOW_REST_PORT = "9001"
)

type ProcessResponse struct {
	Data struct {
		PID string `json:"pid"`
	} `json:"data"`
	Error any `json:"error"`
}
type ProcessRequest struct {
	Context      ContextTopicsPattern `json:"context"`
	Flow         FlowEngine           `json:"flow"`
	StartNodeIds []string             `json:"startNodeIds" validate:"required,min=1,dive,inflow_required,max=128"`
	PID          string               `json:"pid"`
	Settings     Settings             `json:"settings"`
	Meta         map[string]string    `json:"meta"`
	// Resume marks this process as a continuation of an earlier run over the same
	// context (same PID/contextId). The StartNodeIds are the successors of the
	// node the previous run terminated at, and the engine seeds the traversal
	// snapshot the previous run left in the context header so a join downstream of
	// the resume point sees its already-completed dependencies instead of locking.
	// Omitted (false) for a fresh run — the field is additive and older requests
	// decode unchanged.
	// Resume carries the traversal snapshot of the run this process continues.
	// A non-nil value means this process is a continuation over the same context:
	// its start nodes are the successors of a node the previous run terminated at,
	// and the seeded snapshot (nodeTraverse counts + joinGen watermarks) lets a
	// join downstream of the resume point see its already-completed dependencies
	// instead of locking. nil means a plain run with blank traversal state.
	//
	// The snapshot no longer travels through the shared context-document header
	// (one slot per contextId, which overlapping runs clobbered). The engine emits
	// it per-PID at run-end; the scheduler (inflow-fusion) stores it against the
	// source PID it saw at the continue-after extrinsic and hands the right one
	// back here — so each continuation seeds its own run's state, never a sibling's.
	Resume *ResumeState `json:"resume,omitempty"`
}

// ResumeState is a run's completed traversal state, handed to the continuation
// that resumes it. It is the wire form the engine writes at run-end and reads
// back off the resume request; the fields mirror the engine's own maps.
type ResumeState struct {
	// FlowSig gates the seed. Generations are keyed by node id, so a definition
	// that changed between runs would attach them to the wrong nodes. A mismatch
	// means "do not seed" — the run degrades to a blank continue, not a failure.
	FlowSig  string             `json:"flowSig"`
	Traverse map[string]NodeGen `json:"traverse"`
	JoinGen  map[string]int     `json:"joinGen"`
	// Flows lists every flow scope the run loaded — the main flow plus every
	// GoTo namespace mounted under it — taken from the store's per-flow count
	// map. A continuation only receives its start nodes as bare node ids, so on
	// resume this is what tells it which flow scope each of those nodes belongs
	// to when a node terminated (e.g. `_cmd:stop` from continue-after) inside a
	// GoTo target rather than the main flow.
	Flows []string `json:"flows,omitempty"`
}

// NodeGen is a node's traversal state on the wire: its generation (completion
// count) and status. Only genuinely-completed nodes are ever recorded.
type NodeGen struct {
	Count  int `json:"c"`
	Status int `json:"s"`
}

type Settings struct {
	RequestTimeOut   int64  `json:"svc_req_timeout" bson:"svc_req_timeout"`
	ExecuteTimeOut   int64  `json:"proc_timeout" bson:"proc_timeout"`
	ProcessNodeLimit uint16 `json:"proc_node_limit"`
	StopOnError       bool   `json:"stop_on_error"` 
}

type ContextTopicsPattern struct {
	Getter    string `json:"get"`    //eg. inflow.{spaceId}.context.get.{contextId}
	Setter    string `json:"update"` //eg. inflow.{spaceId}.context.set.{contextId}
	ContextId string `json:"contextId"`
}

type FlowEngine struct {
	GetFlow string `json:"get_flow"` //eg. inflow.{spaceId}.get.flow.{flowId}
	FlowId  string `json:"flowId"`
}

type ContextDoc struct {
	Data   string         `json:"data"`
	Header map[string]any `json:"header"`
}

// ContextHeaderErrors is the header slot a run leaves its error ledger in. The
// underscore prefix keeps it clear of the per-node registry entries, which are
// always "<flowScope>:<nodeId>".
const ContextHeaderErrors = "_errors"

// ErrorKind separates the two things that go wrong at a node, so a reader can
// tell what the flow got wrong from what the platform did.
type ErrorKind string

const (
	// ErrorKindNode is the node's own operation failing: the flow author's js or
	// rego, the node data they wrote, a path they addressed, or something the
	// node called that did not deliver. It is theirs to answer for, and it is
	// what a run started with Settings.StopOnError refuses to carry on past.
	ErrorKindNode ErrorKind = "node"
	// ErrorKindSystem is the engine's own machinery failing while serving the
	// node: infra that would not answer, a reply that would not marshal, a
	// context write that did not land. Nothing in the flow caused it and nothing
	// in the flow mends it.
	ErrorKindSystem ErrorKind = "system"
)

// NodeError is one error a run hit, as it is written into the context header.
//
// A flow does not stop for a node error — it is handled and the run carries on —
// so a run that hit several still finishes, and these are how anyone who was not
// watching the event stream finds out. Code is a StatusFractal; Loc is set only
// for a node whose scope fanned out, where naming the node does not place the
// failure.
type NodeError struct {
	Ts   int64     `json:"ts"`
	Kind ErrorKind `json:"kind"`
	Flow string    `json:"flow"`
	Node string    `json:"node"`
	// Src is the actor that raised it — rt / js / rego / plugin:<title> — the same
	// vocabulary ProcEvent.Src uses, so an entry here joins onto the event stream
	// the run also published.
	Src  string `json:"src"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Loc  string `json:"loc,omitempty"`
}

// RunErrors is the ledger under ContextHeaderErrors: every error one run
// recorded. It describes a single run — the engine clears the slot when it loads
// the document — so Pid is the run that wrote all of Items, and the slot being
// absent is a run that hit nothing.
//
// Count is the true total and Items may be shorter: a cascading run can raise an
// error per location per node and the document still has to be publishable, so
// the entries are capped at the earliest ones, which is where the cause of a
// cascade is.
type RunErrors struct {
	Pid   string      `json:"pid"`
	Count int         `json:"count"`
	Items []NodeError `json:"items"`
}

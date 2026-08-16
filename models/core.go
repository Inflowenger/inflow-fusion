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

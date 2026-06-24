package models
const (
	INFLOW_REST_PORT = "9001"
)

type ProcessResponse struct{
	Data struct{
		PID string `json:"pid"`
	} `json:"data"`
	Error any `json:"error"`
}
type ProcessRequest struct {
	Context  ContextTopicsPattern `json:"context"`
	Flow     FlowEngine           `json:"flow"`
	PID      string               `json:"pid"`
	StartNodeId               string `json:"startNodeId"`
	Settings Settings             `json:"settings"`
	Meta     map[string]string    `json:"meta"`
}

type Settings struct {
	RequestTimeOut   int64  `json:"svc_req_timeout" bson:"svc_req_timeout"`
	ExecuteTimeOut   int64  `json:"proc_timeout" bson:"proc_timeout"`
	ProcessNodeLimit uint16 `json:"proc_node_limit"`
}


type ContextTopicsPattern struct {
	Getter       string `json:"get"`    //eg. inflow.{spaceId}.context.get.{contextId}
	Setter       string `json:"update"` //eg. inflow.{spaceId}.context.set.{contextId}
	ContextId    string `json:"contextId"`
}

type FlowEngine struct {
	GetFlow    string `json:"get_flow"` //eg. inflow.{spaceId}.get.flow.{flowId}
	FlowId     string `json:"flowId"`

}


type ContextDoc struct {
	Data   string         `json:"data"`
	Header map[string]any `json:"header"`
}
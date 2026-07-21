package models

type PluginRule struct {
	InfraIsolated   InfraIsolated  `json:"infra_isolated" bson:"infra_isolated"`
	Request         string         `json:"request" bson:"request"`
	SubjectPrefix   string         `json:"subject_prefix" bson:"subject_prefix"`
	CancelAfterIdle int8           `json:"idle_min" bson:"idle_min"`
	Body            map[string]any `json:"body" bson:"body"`
}
type InfraIsolated struct {
	Account string `json:"account" bson:"account"`
	Seed    string `json:"seed" bson:"seed"`
	Cred    string `json:"cred" bson:"cred"`
	Url     string `json:"url" bson:"url"`
}
type ContractRule struct {
	Lang       string         `json:"lang" bson:"lang" validate:"oneof=js opa"`
	LogicRule  string         `json:"logic_rule" bson:"logic_rule"`
	Conditions map[string]any `json:"conditions" bson:"conditions"`
	OpaResult  string         `json:"opa_result"` // in opa contract, it's the query to specify which part of data to query
}

type CodeRule struct {
	Lang      string         `json:"lang" bson:"lang" validate:"oneof=js opa"`
	LogicRule string         `json:"logic_rule" bson:"logic_rule"`
	OpaData   map[string]any `json:"opa_data" bson:"opa_data"` // just for opa . as appendix or condition based on nature of opa
	OpaResult string         `json:"opa_result"`               // just for opa, to specify which part of data to query
}
type GoToRule struct {
	From  Next `json:"from"`
	EndAt Next `json:"end"`
}

type ExtrinsicRule struct {
	InfraIsolated     InfraIsolated  `json:"infra_isolated" bson:"infra_isolated"`
	Subject           string         `json:"subject"`
	Data              map[string]any `json:"data"`
	ReqTimeoutSecound uint8          `json:"request_timeout_sec"` // default is 5
}

type VoidRule struct {
}

type ExtSvcRequestBody struct {
	Data          any            `json:"data"`
	OperationData map[string]any `json:"op"`
	Node          *Node          `json:"node"`
}

package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
)

type ContractNode struct {
	models.ContractRule
	UniqId string `json:"uniqId"`
}

// impl INode
func (n *ContractNode) SetId(uniqId string) {
	n.UniqId = uniqId
}
func (n *ContractNode) GetInflowNodeType() models.NodeType {
	return models.RuleNodeType
}

/*
 * Contract-driven rule execution With OPA Language.
 * Evaluates policy rules against the provided context and resolves a designated result key.
 * The value assigned to that key serves as the authoritative outcome and determines the next execution state.
 */
func NewOpaRuleLogicNode(resultKey string, opts ...func(*ContractNode)) *ContractNode {
	cn := &ContractNode{
		UniqId:       etc.UUID(),
		ContractRule: models.ContractRule{Lang: "opa", OpaResult: resultKey},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

/*
 * Contract-driven rule execution With JavaScript Language.
 * Evaluates policy rules against the provided context and resolves a designated result key.
 * The value assigned to that key serves as the authoritative outcome and determines the next execution state.
 */
func NewJsRuleLogicNode(opts ...func(*ContractNode)) *ContractNode {
	cn := &ContractNode{
		UniqId:       etc.UUID(),
		ContractRule: models.ContractRule{Lang: "js"},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

func WithContractLogicCode(code string) func(*ContractNode) {
	return func(cn *ContractNode) {
		cn.LogicRule = code
	}
}

func WithContractConditions(criteria map[string]any) func(*ContractNode) {
	return func(cn *ContractNode) {
		cn.Conditions = criteria
	}
}
func WithContractUniqId(id string) func(*ContractNode) {
	return func(cn *ContractNode) {
		cn.UniqId = id
	}
}

package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
)

// OPA rego code node
type OpaCodeNode struct {
	models.CodeRule
	UniqId string `json:"uniqId"`
}

// impl INode interface
func (n *OpaCodeNode) SetId(uniqId string) {
	n.UniqId = uniqId
}

func (n *OpaCodeNode) GetInflowNodeType() models.NodeType {
	return models.CodeNodeType
}
func NewOpaNode(code, resultKey string, opts ...func(*OpaCodeNode)) *OpaCodeNode {
	cn := &OpaCodeNode{
		UniqId:   etc.UUID(),
		CodeRule: models.CodeRule{Lang: "opa", OpaResult: resultKey, LogicRule: code},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

func WithCriteriaData(criteria map[string]any) func(*OpaCodeNode) {
	return func(cn *OpaCodeNode) {
		cn.OpaData = criteria
	}
}

// JavaScript node
type JsCodeNode struct {
	models.CodeRule
	UniqId string `json:"uniqId"`
}

// impl INode interface
func (n *JsCodeNode) SetId(uniqId string) {
	n.UniqId = uniqId
}
func (n *JsCodeNode) GetInflowNodeType() models.NodeType {
	return models.CodeNodeType
}
func NewJsNode(code string, opts ...func(*JsCodeNode)) *JsCodeNode {
	cn := &JsCodeNode{
		UniqId:   etc.UUID(),
		CodeRule: models.CodeRule{Lang: "js", LogicRule: code},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

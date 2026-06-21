package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
)

type GotoNode struct {
	models.GoToRule
	UniqId string `json:"uniqId"`
}

// impl INode interface
func (n *GotoNode) SetId(uniqId string) {
	n.UniqId = uniqId
}
func (n *GotoNode) GetInflowNodeType() models.NodeType {
	return models.GoToNodeType
}

func NewGotoNode(opts ...func(*GotoNode)) *GotoNode {
	cn := &GotoNode{
		UniqId:   etc.UUID(),
		GoToRule: models.GoToRule{},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

// start from node in given flowId
func (n *GotoNode) From(flowId, nodeId string) {
	n.GoToRule.From = models.Next{FlowId: flowId, NodeId: nodeId}
}

// back to origin after this node
func (n *GotoNode) To(flowId, nodeId string) {
	n.GoToRule.EndAt = models.Next{FlowId: flowId, NodeId: nodeId}
}

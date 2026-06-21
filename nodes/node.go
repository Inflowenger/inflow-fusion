package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
)

type INode interface {
	SetId(string)
	GetInflowNodeType() models.NodeType
}

func WithUniqId[T INode](id string) func(T) {
	return func(n T) {
		n.SetId(id)
	}
}

type VoidNode struct {
	models.VoidRule
	UniqId string `json:"uniqId"`
}

// impl INode interface
func (n *VoidNode) SetId(uniqId string) {
	n.UniqId = uniqId
}
func (n *VoidNode) GetInflowNodeType() models.NodeType {
	return models.VoidNodeType
}

func NewVoidNode(opts ...func(*VoidNode)) *VoidNode {
	cn := &VoidNode{
		UniqId:   etc.UUID(),
		VoidRule: models.VoidRule{},
	}
	for _, opt := range opts {
		opt(cn)
	}
	return cn
}

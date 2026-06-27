package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
)

type EtrinsicSvcNode struct {
	models.ExtrinsicRule
	UniqId string `json:"uniqId"`
}

// impl INode
func (n *EtrinsicSvcNode) SetId(uniqId string) {
	n.UniqId = uniqId
}

func (n *EtrinsicSvcNode) GetInflowNodeType() models.NodeType {
	return models.ExtrinsicNodeType
}

func NewSvcNode(subject string, opts ...func(*EtrinsicSvcNode)) *EtrinsicSvcNode {

	svcNode := &EtrinsicSvcNode{
		ExtrinsicRule: models.ExtrinsicRule{
			Subject: subject,
		},
		UniqId: etc.UUID(),
	}

	for _, opt := range opts {
		opt(svcNode)
	}
	if svcNode.InfraIsolated.Account == "" {
		_, ok := natsHandler.GetNatsBox().Read(models.DEFAULT_ISOLATED_INFRA)
		if ok {
			svcNode.InfraIsolated = models.InfraIsolated{Account: models.DEFAULT_ISOLATED_INFRA}
		}
	}
	return svcNode
}

func WithIsolated(isolated models.InfraIsolated) func(*EtrinsicSvcNode) {
	return func(esn *EtrinsicSvcNode) {
		esn.InfraIsolated = isolated
	}
}

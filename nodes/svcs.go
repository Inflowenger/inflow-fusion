package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
)

type EventSvcNode struct {
	models.EventRule
	UniqId string `json:"uniqId"`
}

// impl INode
func (n *EventSvcNode) SetId(uniqId string) {
	n.UniqId = uniqId
}

func (n *EventSvcNode) GetInflowNodeType() models.NodeType {
	return models.EventNodeType
}

func NewSvcNode(subject string, opts ...func(*EventSvcNode)) *EventSvcNode {

	svcNode := &EventSvcNode{
		EventRule: models.EventRule{
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

func WithIsolated(isolated models.InfraIsolated) func(*EventSvcNode) {
	return func(esn *EventSvcNode) {
		esn.InfraIsolated = isolated
	}
}

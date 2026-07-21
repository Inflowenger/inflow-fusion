package nodes

import (
	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
)

type ExtrinsicSvcNode struct {
	models.ExtrinsicRule
	UniqId string `json:"uniqId"`
}

// impl INode
func (n *ExtrinsicSvcNode) SetId(uniqId string) {
	n.UniqId = uniqId
}

func (n *ExtrinsicSvcNode) GetInflowNodeType() models.NodeType {
	return models.ExtrinsicNodeType
}

func NewExtrinsicSvcNode(subject string, opts ...func(*ExtrinsicSvcNode)) *ExtrinsicSvcNode {

	svcNode := &ExtrinsicSvcNode{
		ExtrinsicRule: models.ExtrinsicRule{
			Subject:           subject,
			ReqTimeoutSecound: 5,
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

func WithIsolated(isolated models.InfraIsolated) func(*ExtrinsicSvcNode) {
	return func(esn *ExtrinsicSvcNode) {
		esn.InfraIsolated = isolated
	}
}

func WithOpData(op map[string]any)func(*ExtrinsicSvcNode) {
		return func(esn *ExtrinsicSvcNode) {
		esn.Data = op
	}
}
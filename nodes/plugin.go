package nodes

import (
	"fmt"

	"github.com/Inflowenger/inflow-fusion/etc"
	InfraSpaces "github.com/Inflowenger/inflow-fusion/spaces"

	"github.com/Inflowenger/inflow-fusion/models"
)

type PluginNode struct {
	models.PluginRule
	Name   string `json:"name"`
	UniqId string `json:"uniqId"`
}

// impl INode
func (n *PluginNode) SetId(uniqId string) {
	n.UniqId = uniqId
}
func (n *PluginNode) GetInflowNodeType() models.NodeType {
	return models.PluginNodeType
}
func NewPluginNode(name string, opts ...func(*PluginNode)) (*PluginNode, error) {

	pluginNode := &PluginNode{
		PluginRule: models.PluginRule{

			CancelAfterIdle: 15, // default cancel plugin node after 15 minutes idle, can be override by options
		},
		Name:   name,
		UniqId: etc.UUID(),
	}

	for _, opt := range opts {
		opt(pluginNode)
	}
	if pluginNode.InfraIsolated.Account == "" {
		cred, err := InfraSpaces.GetCredOnBuiltinPluginAcc(pluginNode.Name)
		if err != nil {
			return nil, err
		}
		if cred != nil {
			pluginNode.InfraIsolated = *cred
		}
	}
	if pluginNode.SubjectPrefix == "" {
		pluginNode.SubjectPrefix = fmt.Sprintf("%s.%s", models.INFLOW_PLUGIN_PROTO_PREFIX, pluginNode.UniqId)
	}
	return pluginNode, nil
}

func WithPluginIsolated(isolated models.InfraIsolated) func(*PluginNode) {
	return func(pn *PluginNode) {
		pn.InfraIsolated = isolated
	}
}

func WithIdleWaitMinutes(idleMinutes int8) func(*PluginNode) {
	return func(pn *PluginNode) {
		pn.CancelAfterIdle = idleMinutes
	}
}
func WithCustomPrefix(prefix string) func(*PluginNode) {
	return func(pn *PluginNode) {
		pn.SubjectPrefix = prefix
	}
}

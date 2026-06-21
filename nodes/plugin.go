package nodes

import (
	"context"
	"errors"
	"fmt"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/inflow"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	"github.com/nats-io/jwt/v2"

	"github.com/Inflowenger/inflow-fusion/models"
)

var pluginsIsolatedAccount *models.Account

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
		backendInstance := inflow.GetInflowBackend()
		if backendInstance == nil {
			return nil, errors.New("backend initial is required before any request")
		}
		if pluginsIsolatedAccount == nil {
			var err error
			pluginsIsolatedAccount, err = getDefaultPluginsAccount(backendInstance.Infra, backendInstance.GetBearerToken())
			if err != nil {
				return nil, err
			}
		}
		cred, err := natsHandler.CreateUserCredential(pluginsIsolatedAccount.Seed, models.UserCredGenInput{Name: pluginNode.Name, Account: pluginsIsolatedAccount.Pub})
		if err != nil {
			return nil, err
		}
		pluginNode.InfraIsolated = models.InfraIsolated{
			Account: pluginsIsolatedAccount.Pub,
			Cred:    cred.Base64Cred,
			Seed:    pluginsIsolatedAccount.Seed,
			Url:     backendInstance.GetInfraNatsUrl(),
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

func getDefaultPluginsAccount(baseurl, token string) (*models.Account, error) {

	response, err := etc.SendHttpGet(context.Background(), map[string]string{"Authorization": token},
		fmt.Sprintf("%s/account/id/%s", baseurl, models.BUILTIN_PLUGINS_ACCOUNT_INDEX),
		struct {
			Data  *models.Account `json:"data"`
			Error any             `json:"error"`
		}{},
	)
	if err != nil {
		return nil, err
	}
	if response.Data == nil || response.Error != nil {
		return nil, fmt.Errorf("given account not found or any internal error occurred")
	}

	return response.Data, nil
}

func PluginCredentialPermission(userName, pluginUniqId, accountPub string) models.UserCredGenInput {
	inboxPattern := natsHandler.GetInboxConfigWithPluginId(pluginUniqId)
	perm := models.UserCredGenInput{
		Name:    userName,
		Account: accountPub,
		Pub: jwt.Permission{Allow: []string{
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_PROTO_PREFIX, pluginUniqId),
		}},
		Sub: jwt.Permission{Allow: []string{
			fmt.Sprintf("%s.>", inboxPattern),
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_PROTO_PREFIX, pluginUniqId),
		}},
		Tags: jwt.TagList{inboxPattern},
	}
	return perm
}

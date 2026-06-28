package InfraSpaces

import (
	"errors"
	"fmt"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/inflow"
	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/nats-io/jwt/v2"
)

func GetPluginBuiltinAccount() (*models.Account, error) {
	acc, ok := GetInfraSpaces().get(models.BUILTIN_PLUGINS_ACCOUNT_INDEX)
	if !ok {
		var err error
		acc, err = fetchAccount(models.BUILTIN_PLUGINS_ACCOUNT_INDEX)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}
func GetCredOnBuiltinPluginAcc(pluginName string) (*models.InfraIsolated, error) {

	acc, ok := GetInfraSpaces().get(models.BUILTIN_PLUGINS_ACCOUNT_INDEX)
	if !ok {
		var err error
		acc, err = fetchAccount(models.BUILTIN_PLUGINS_ACCOUNT_INDEX)
		if err != nil {
			return nil, err
		}
	}
	cred, err := CreateUserCredential(acc.Seed, models.UserCredGenInput{Name: pluginName, Account: acc.Pub})
	if err != nil {
		return nil, err
	}
	backendInstance := inflow.GetInflowBackend()
	if backendInstance == nil {
		return nil, errors.New("backend initial is required before any request")
	}
	space := models.InfraIsolated{
		Account: acc.Pub,
		Cred:    cred.Base64Cred,
		Seed:    acc.Seed,
		Url:     backendInstance.GetInfraNatsUrl(),
	}
	return &space, nil
}
func GetAccountCred(accountId ,pluginName string) (*models.InfraIsolated,error){
	acc, ok := GetInfraSpaces().get(accountId)
	if !ok {
		var err error
		acc, err = fetchAccount(accountId)
		if err != nil {
			return nil, err
		}
	}
	cred, err := CreateUserCredential(acc.Seed, models.UserCredGenInput{Name: pluginName, Account: acc.Pub})
	if err != nil {
		return nil, err
	}
	backendInstance := inflow.GetInflowBackend()
	if backendInstance == nil {
		return nil, errors.New("backend initial is required before any request")
	}
	space := models.InfraIsolated{
		Account: acc.Pub,
		Cred:    cred.Base64Cred,
		Seed:    acc.Seed,
		Url:     backendInstance.GetInfraNatsUrl(),
	}
	return &space, nil
}
func fetchAccount(accountId string) (*models.Account, error) {
	backendInstance := inflow.GetInflowBackend()
	if backendInstance == nil {
		return nil, errors.New("backend initial is required before any request")
	}
	acc, err := backendInstance.GetAccountByKey(accountId)
	if err != nil {
		return nil, err
	}
	GetInfraSpaces().set(accountId, acc)
	return acc, nil
}
func PluginCredentialPermission(userName, pluginUniqId, accountPub string) models.UserCredGenInput {
	inboxPattern := GetInboxConfigWithPluginId(pluginUniqId)
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

func GetInboxConfigWithPluginId(pluginUid string) string {
	return fmt.Sprintf("_INBOX.%s", etc.UuidLastPart(pluginUid))
}

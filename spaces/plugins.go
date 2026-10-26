package InfraSpaces

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
func GetAccountById(accountId string) (*models.Account, error) {
	acc, ok := GetInfraSpaces().get(accountId)
	if !ok {
		var err error
		acc, err = fetchAccount(accountId)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}
func GetAccountCred(accountId, pluginName string) (*models.InfraIsolated, error) {
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

// pluginSubjectToken is the shape a pluginUniqId must have to be safe to
// interpolate into a permission subject: exactly one NATS token.
//
// This is what makes the id in `inflow.plugin.<ID>.up.>` an identity a
// subscriber can trust. A `.` would add a subject level, and a `*` or `>` would
// turn the grant into a wildcard over every plugin in the account — a
// credential minted that way could publish under any id it liked, and an
// upstream collector reading the id off the subject would faithfully report
// whatever the publisher chose. The broker only enforces the token if the token
// is a token.
//
// It is deliberately not a uuid check. Plugin ids are uuids when a PluginNode
// mints its own, but the builtin llm/mcp/cast nodes carry hard-coded seed ids
// ("aaaa-bbbb-cccc-llm0") and imported flowfiles carry their own, none of which
// parse as uuids.
var pluginSubjectToken = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ErrInvalidPluginId is returned when a pluginUniqId cannot be placed in a
// subject. Callers mint credentials from request input, so this is a caller
// error (HTTP 400), not an infrastructure failure.
var ErrInvalidPluginId = errors.New("invalid pluginId")

// PluginCredentialStrictPermission is the permission set for one plugin's
// credential: it may serve its own job transport, reply on its own inbox,
// receive the runtime's signals, and publish upstream under its own id.
//
// The publish and subscribe grants are deliberately NOT the same subject. The
// runtime broadcasts signals to the plugin on `inflow.plugin.<ID>.>` — the
// engine's job stops land on `.proc` there — so granting publish on that same
// wildcard would let a plugin forge a stop to its own replicas, which all
// subscribe it, and would put plugin-authored messages in the same namespace as
// runtime-authored ones. Upstream publishing gets its own leaf instead.
func PluginCredentialStrictPermission(userName, pluginUniqId, accountPub string) (models.UserCredGenInput, error) {
	if !pluginSubjectToken.MatchString(pluginUniqId) {
		return models.UserCredGenInput{}, fmt.Errorf("%w: %q is not a single subject token", ErrInvalidPluginId, pluginUniqId)
	}
	inboxPattern := GetInboxConfigWithPluginId(pluginUniqId)
	perm := models.UserCredGenInput{
		Name:    userName,
		Account: accountPub,
		Pub: jwt.Permission{Allow: []string{
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_PROTO_PREFIX, pluginUniqId),
			fmt.Sprintf("%s.>", "_INBOX"),
			// publish upstream, under this plugin's own id only
			PluginUpstreamSubject(pluginUniqId, ">"),
		}},
		Sub: jwt.Permission{Allow: []string{
			fmt.Sprintf("%s.>", inboxPattern),
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_PROTO_PREFIX, pluginUniqId),
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_V1_PREFIX, pluginUniqId),
			//subscribe to signals
			fmt.Sprintf("%s.%s.>", models.INFLOW_PLUGIN_SIG_CHANNEL_PREFIX, pluginUniqId),
		}},
		Tags: jwt.TagList{inboxPattern},
	}
	return perm, nil
}
func PluginCredentialOpenPermission(userName, accountPub string) models.UserCredGenInput {
	perm := models.UserCredGenInput{
		Name:    userName,
		Account: accountPub,
	}
	return perm
}

// CollectorCredentialPermission is the permission set for an upstream collector:
// a service that reads what every plugin in the account publishes upstream and
// attributes each message to its sender.
//
// It is subscribe-only, and the deny is load-bearing. An empty Pub allow-list is
// read by NATS as "allow all", so a collector minted from
// PluginCredentialOpenPermission could publish forged runtime signals to every
// plugin in the account — the one thing the subject-as-identity scheme assumes
// nobody can do. Denying `>` is what actually takes publishing away.
//
// The collector must connect on the SAME account as the plugins it collects
// from. NATS accounts are isolated subject spaces; no subject pattern crosses
// that boundary without an export/import.
func CollectorCredentialPermission(userName, accountPub string) models.UserCredGenInput {
	return models.UserCredGenInput{
		Name:    userName,
		Account: accountPub,
		Sub:     jwt.Permission{Allow: []string{PluginUpstreamWildcard()}},
		Pub:     jwt.Permission{Deny: []string{">"}},
	}
}

// PluginUpstreamSubject is where a plugin publishes upstream data: its own id,
// then the reserved upstream leaf, then whatever the plugin wants to say. Pass
// ">" for kind to build the grant pattern rather than a concrete subject.
func PluginUpstreamSubject(pluginUniqId, kind string) string {
	return fmt.Sprintf("%s.%s.%s.%s", models.INFLOW_PLUGIN_SIG_CHANNEL_PREFIX, pluginUniqId, models.INFLOW_PLUGIN_UPSTREAM_LEAF, kind)
}

// PluginUpstreamWildcard is what a collector subscribes to receive every
// plugin's upstream traffic. The `*` is the sending plugin's id — read it back
// with PluginIdFromUpstreamSubject.
func PluginUpstreamWildcard() string {
	return fmt.Sprintf("%s.*.%s.>", models.INFLOW_PLUGIN_SIG_CHANNEL_PREFIX, models.INFLOW_PLUGIN_UPSTREAM_LEAF)
}

// PluginIdFromUpstreamSubject reads the sending plugin's id off a subject
// received on PluginUpstreamWildcard.
//
// The id is trustworthy: the broker refused the publish unless the credential
// was minted for that id, and PluginCredentialStrictPermission refuses to mint
// an id that is not a single token. It identifies the credential, not a person —
// a plugin's credential is a bearer token living in a dotenv on a host you may
// not control, so treat the id as "which plugin credential", not "which trusted
// party".
func PluginIdFromUpstreamSubject(subject string) (string, bool) {
	parts := strings.Split(subject, ".")
	prefix := strings.Split(models.INFLOW_PLUGIN_SIG_CHANNEL_PREFIX, ".")
	// prefix tokens, the id, the upstream leaf, and at least one kind token
	if len(parts) < len(prefix)+3 {
		return "", false
	}
	for i, p := range prefix {
		if parts[i] != p {
			return "", false
		}
	}
	if parts[len(prefix)+1] != models.INFLOW_PLUGIN_UPSTREAM_LEAF {
		return "", false
	}
	id := parts[len(prefix)]
	if !pluginSubjectToken.MatchString(id) {
		return "", false
	}
	return id, true
}
func GetInboxConfigWithPluginId(pluginUid string) string {
	return fmt.Sprintf("_INBOX.%s", etc.UuidLastPart(pluginUid))
}

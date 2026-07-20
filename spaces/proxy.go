package InfraSpaces

import (
	"fmt"
	"time"

	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
)

// v1RequestTimeout bounds a single inflowv1 request/response round-trip to a
// plugin. Kept short: these back interactive palette/drawer fetches.
const v1RequestTimeout = 4 * time.Second

// defaultPluginClientName names the user credential minted on the builtin-plugins
// account for the backend's proxy connection. The connection is cached per account
// (see GetNatsByInfraIsolate), so one credential/connection serves every request.
const defaultPluginClientName = "inflow-fusion-proxy"

// --- inflowv1 protocol subjects -------------------------------------------
//
// The plugin wire protocol lives here so every backend talks to plugins the same
// way. A plugin exposes reserved (@-prefixed) descriptor subjects under
// `inflow.v1.<pluginID>`.

func v1IntroSubject(pluginID string) string {
	return fmt.Sprintf("%s.%s.@intro", models.INFLOW_PLUGIN_V1_PREFIX, pluginID)
}
func v1SettingsSubject(pluginID string) string {
	return fmt.Sprintf("%s.%s.@settings", models.INFLOW_PLUGIN_V1_PREFIX, pluginID)
}
func v1ActionsSubject(pluginID string) string {
	return fmt.Sprintf("%s.%s.@actions", models.INFLOW_PLUGIN_V1_PREFIX, pluginID)
}
func v1ActionFormSubject(pluginID, method string) string {
	return fmt.Sprintf("%s.%s.%s.@form", models.INFLOW_PLUGIN_V1_PREFIX, pluginID, method)
}

// RequestPluginV1 proxies one inflowv1 request over the given isolated-infra
// connection and returns the plugin's raw response bytes. It is the single choke
// point for talking to plugins, so every fetch degrades the same way when the
// plugin is offline or the connection is down.
//
// Plugins run in their own NATS account (a distinct "space"), so callers pass the
// InfraIsolated identifying that account; GetNatsByInfraIsolate turns it into (and
// caches) an account-scoped connection.
func RequestPluginV1(infra models.InfraIsolated, subject string) ([]byte, error) {
	nat, err := natsHandler.GetNatsByInfraIsolate(infra)
	if err != nil {
		return nil, fmt.Errorf("plugin connection failed: %w", err)
	}
	conn := nat.GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("plugin connection not available")
	}
	msg, err := conn.Request(subject, nil, v1RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("inflowv1 request %q: %w", subject, err)
	}
	return msg.Data, nil
}

// --- Generic fetches (explicit isolated-infra target) ----------------------
//
// Callers that already hold the account's InfraIsolated (e.g. a plugin living on a
// non-default space) supply it together with the pluginID.

// FetchPluginIntro returns `inflow.v1.<pluginID>.@intro` — the plugin's name,
// author, version and (optional) settings form descriptor.
func FetchPluginIntro(infra models.InfraIsolated, pluginID string) ([]byte, error) {
	return RequestPluginV1(infra, v1IntroSubject(pluginID))
}

// FetchPluginSettings returns `inflow.v1.<pluginID>.@settings` — the settings
// (required-fields) form the drawer renders before any action can run.
func FetchPluginSettings(infra models.InfraIsolated, pluginID string) ([]byte, error) {
	return RequestPluginV1(infra, v1SettingsSubject(pluginID))
}

// FetchPluginActions returns `inflow.v1.<pluginID>.@actions` — the array of
// actions the plugin exposes.
func FetchPluginActions(infra models.InfraIsolated, pluginID string) ([]byte, error) {
	return RequestPluginV1(infra, v1ActionsSubject(pluginID))
}

// FetchActionForm returns `inflow.v1.<pluginID>.<method>.@form` — the JSON-schema
// form for one action, rendered when that action is added on the canvas.
func FetchActionForm(infra models.InfraIsolated, pluginID, method string) ([]byte, error) {
	return RequestPluginV1(infra, v1ActionFormSubject(pluginID, method))
}

// --- DefaultPluginAccount fetches (builtin-plugins account, pluginID only) --
//
// These resolve the builtin-plugins account (00000003) themselves, mint a
// credential and reuse the cached account connection, so callers only supply a
// pluginID.

// defaultPluginInfra resolves an isolated-infra target on the builtin-plugins
// account for the backend's proxy connection.
func defaultPluginInfra() (*models.InfraIsolated, error) {
	infra, err := GetCredOnBuiltinPluginAcc(defaultPluginClientName)
	if err != nil {
		return nil, fmt.Errorf("plugin account unavailable: %w", err)
	}
	return infra, nil
}

// DefaultPluginIntro is FetchPluginIntro against the builtin-plugins account.
func DefaultPluginIntro(pluginID string) ([]byte, error) {
	infra, err := defaultPluginInfra()
	if err != nil {
		return nil, err
	}
	return FetchPluginIntro(*infra, pluginID)
}

// DefaultPluginSettings is FetchPluginSettings against the builtin-plugins account.
func DefaultPluginSettings(pluginID string) ([]byte, error) {
	infra, err := defaultPluginInfra()
	if err != nil {
		return nil, err
	}
	return FetchPluginSettings(*infra, pluginID)
}

// DefaultPluginActions is FetchPluginActions against the builtin-plugins account.
func DefaultPluginActions(pluginID string) ([]byte, error) {
	infra, err := defaultPluginInfra()
	if err != nil {
		return nil, err
	}
	return FetchPluginActions(*infra, pluginID)
}

// DefaultPluginActionForm is FetchActionForm against the builtin-plugins account.
func DefaultPluginActionForm(pluginID, method string) ([]byte, error) {
	infra, err := defaultPluginInfra()
	if err != nil {
		return nil, err
	}
	return FetchActionForm(*infra, pluginID, method)
}

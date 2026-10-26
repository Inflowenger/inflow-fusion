package models

// builtins account and index
const (
	BUILTIN_SYS_ACCOUNT_INDEX     = "00000001"
	BUILTIN_INFLOW_ACCOUNT_INDEX  = "00000002"
	BUILTIN_PLUGINS_ACCOUNT_INDEX = "00000003"

	BUILTIN_SYS_ACCOUNT     = "sys"
	BUILTIN_INFLOW_ACCOUNT  = "inflow"
	BUILTIN_PLUGINS_ACCOUNT = "plugins"
)

type NodeType string
type EvidenceType string
type CodeVariant string

type SvcReturnCommand string
type SvcReturnKey string

const (
	VoidNodeType      NodeType = "voidNodeType"
	CodeNodeType      NodeType = "codeNodeType"
	GoToNodeType      NodeType = "gotoNodeType"
	ExtrinsicNodeType NodeType = "extrinsicNodeType"
	PluginNodeType    NodeType = "pluginNodeType"
	RuleNodeType      NodeType = "ruleNodeType"

	JavaScriptLang CodeVariant = "js"
	OPALang        CodeVariant = "opa"
)

const (
	INFLOW_PLUGIN_PROTO_PREFIX       = "inflow.cpu"
	INFLOW_PLUGIN_V1_PREFIX          = "inflow.v1"
	INFLOW_PLUGIN_SIG_CHANNEL_PREFIX = "inflow.plugin"

	// INFLOW_PLUGIN_UPSTREAM_LEAF reserves one leaf under a plugin's signal
	// channel for traffic going the other way: plugin -> upstream collector.
	// The runtime's own signals (".proc" and whatever follows it) keep the rest
	// of the channel, so a collector never has to tell the two apart by payload.
	INFLOW_PLUGIN_UPSTREAM_LEAF = "up"
)

const (
	CmdStop       SvcReturnCommand = "stop"
	CmdNextFilter SvcReturnCommand = "next_tags"
)
const (
	SvcCmdResposeKey            SvcReturnKey = "_cmd"
	SvcCmdResponseNextFilterKey SvcReturnKey = "_next_filter"
)

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
	INFLOW_PLUGIN_PROTO_PREFIX = "inflow.cpu"
)

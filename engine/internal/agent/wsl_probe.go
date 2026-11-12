package agent

// The probe itself is wslProbeScript in wsl.go; these are what it adds for
// AgentDock: the distro's own path roots, read by place.getenv.

// wslPathEnvs are the path-root variables known adapters read, probed in
// the distro. Secrets, profile names and credential files are not listed.
var wslPathEnvs = []string{
	"ANTE_HOME", "ATOMCODE_HOME", "CLAUDE_CONFIG_DIR", "CLINE_DATA_DIR", "CLINE_DIR",
	"CODEBUDDY_CONFIG_DIR", "CODEX_HOME", "DEVECO_CONFIG_DIR", "DSH_HOME",
	"FACTORY_HOME_OVERRIDE", "GEMINI_CLI_HOME", "GROK_HOME", "HERMES_HOME",
	"KIMI_CODE_HOME", "KIMI_SHARE_DIR", "MIMOCODE_HOME", "MINIMAX_DATA_DIR",
	"MISTER_MORPH_CONFIG", "MOTHX_DIR", "NVM_DIR", "OMO_CODING_AGENT_DIR",
	"OPENCODE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "PRIME_AGENT_CODING_AGENT_DIR",
	"QODERCN_CONFIG_DIR", "QODER_CONFIG_DIR", "QWEN_HOME", "REASONIX_HOME",
	"SENPI_CODING_AGENT_DIR", "SNOW_CONFIG_DIR", "STEP_CODING_AGENT_DIR",
	"XDG_CONFIG_HOME", "XDG_DATA_HOME",
}

// wslEnvDirs maps a probed path variable to the wslKind.dir Has key it
// relocates, so a custom root still counts as the agent being there.
var wslEnvDirs = []struct{ env, dir string }{
	{"CODEX_HOME", ".codex"},
	{"CLAUDE_CONFIG_DIR", ".claude"},
	{"OPENCODE_CONFIG_DIR", ".config/opencode"},
	{"MIMOCODE_HOME", ".config/mimocode"},
	{"HERMES_HOME", ".hermes"},
	{"GROK_HOME", ".grok"},
	{"KIMI_CODE_HOME", ".kimi-code"},
	{"QWEN_HOME", ".qwen"},
	{"DSH_HOME", ".dsh"},
	{"ANTE_HOME", ".ante"},
	{"ATOMCODE_HOME", ".atomcode"},
	{"CLINE_DIR", ".cline"},
	{"CODEBUDDY_CONFIG_DIR", ".codebuddy"},
	{"FACTORY_HOME_OVERRIDE", ".factory"},
	{"MOTHX_DIR", ".mothx"},
	{"QODER_CONFIG_DIR", ".qoder"},
	{"QODERCN_CONFIG_DIR", ".qoder-cn"},
	{"SNOW_CONFIG_DIR", ".snow"},
	{"REASONIX_HOME", ".reasonix"},
	{"DEVECO_CONFIG_DIR", ".config/deveco"},
	{"MINIMAX_DATA_DIR", ".minimax"},
	{"PI_CODING_AGENT_DIR", ".pi"},
	{"OMO_CODING_AGENT_DIR", ".omo"},
	{"PRIME_AGENT_CODING_AGENT_DIR", ".prime/agent"},
	{"STEP_CODING_AGENT_DIR", ".stepcode"},
}

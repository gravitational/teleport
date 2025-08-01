package config

const (
	// vsCodeServersKey defines the JSON key used by VSCode to store MCP servers
	// config.
	//
	// https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_manage-mcp-servers
	vsCodeServersKey = "servers"
	// vsCodeProjectDir defines the VSCode project directory, where the MCP
	// configuration is located.
	//
	// https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_manage-mcp-servers
	vsCodeProjectDir = ".vscode"
)

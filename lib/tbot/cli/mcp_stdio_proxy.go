package cli

// TODO: probably rename this if we even implement it.
type MCPProxyCommand struct {
	*genericExecutorHandler[MCPProxyCommand]

	ProxyServer      string
	IdentityFilePath string
	MCPServerName    string
}

func NewMCPProxyCommand(app KingpinClause, action func(*MCPProxyCommand) error) *MCPProxyCommand {
	cmd := app.Command("mcp-connect", "TODO.")

	c := &MCPProxyCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").Envar(ProxyServerEnvVar).StringVar(&c.ProxyServer)
	cmd.Flag("identity-file", "").StringVar(&c.IdentityFilePath)
	cmd.Flag("mcp-server", "MCP server name").StringVar(&c.MCPServerName)

	return c
}

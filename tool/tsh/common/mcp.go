/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/tool/common"
)

type mcpCommands struct {
	login   *mcpLoginCommand
	list    *mcpListCommand
	connect *mcpConnectCommand
	// TODO(greedy52) implement logout command
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) mcpCommands {
	mcp := app.Command("mcp", "View and control available MCP servers")
	return mcpCommands{
		login:   newMCPLoginCommand(mcp, cf),
		list:    newMCPListCommand(mcp, cf),
		connect: newMCPConnectCommand(mcp, cf),
	}
}

func newMCPLoginCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLoginCommand {
	cmd := &mcpLoginCommand{
		CmdClause: parent.Command("login", "Login to MCP servers and update client configurations"),
	}

	cmd.Flag("all", "Login to all MCP servers. Mutually exclusive with --labels or --query.").Short('R').BoolVar(&cf.ListAll)
	cmd.Flag("labels", labelHelp).StringVar(&cf.Labels)
	cmd.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	cmd.Flag("format", "\"claude\" for updating Claude Desktop configuration. \"json\" for printing out the configuration in JSON.").Short('f').StringVar(&cf.Format)
	cmd.Arg("name", "Name of the MCP server").StringVar(&cf.AppName)
	return cmd
}

func newMCPListCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpListCommand {
	cmd := &mcpListCommand{
		CmdClause: parent.Command("ls", "List available MCP servers"),
	}

	// TODO(greeyd52) support verbose flag
	cmd.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	cmd.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	cmd.Arg("labels", labelHelp).StringVar(&cf.Labels)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaults.DefaultFormats...)
	return cmd
}

func newMCPConnectCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpConnectCommand {
	cmd := &mcpConnectCommand{
		CmdClause: parent.Command("connect", "Connect to a MCP server with stdio."),
	}
	cmd.Arg("name", "Name of the MCP server").Required().StringVar(&cf.AppName)
	return cmd
}

type mcpLoginCommand struct {
	*kingpin.CmdClause
}

func (c *mcpLoginCommand) run(cf *CLIConf) error {
	cf.Confirm = true
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	mcpServers, err := c.findMCPServers(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := c.loginAll(cf, tc, mcpServers); err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) maybe use template?
	fmt.Fprintln(cf.Stdout(), "Logged into Teleport MCP server(s).")
	for mcpServer := range slices.Values(mcpServers) {
		fmt.Fprintf(cf.Stdout(), "- %s\n", mcpServer.GetName())
	}
	fmt.Fprintln(cf.Stdout(), "")

	switch cf.Format {
	case "":
		return c.autoDetectOrJSON(cf, mcpServers)
	case "json":
		return c.printJSON(cf, mcpServers)
	case "claude":
		return c.maybeClaude(cf, mcpServers)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
}

func (c *mcpLoginCommand) loginAll(cf *CLIConf, tc *client.TeleportClient, mcpServers []types.Application) error {
	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) run in errgroup.
	for mcpServer := range slices.Values(mcpServers) {
		appCertParams := client.ReissueParams{
			RouteToCluster: tc.SiteName,
			RouteToApp: proto.RouteToApp{
				Name:        mcpServer.GetName(),
				PublicAddr:  mcpServer.GetPublicAddr(),
				ClusterName: tc.SiteName,
				URI:         mcpServer.GetURI(),
			},
			AccessRequests: profile.ActiveRequests,
		}

		key, _, err := clusterClient.IssueUserCertsWithMFA(cf.Context, appCertParams)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := tc.LocalAgent().AddAppKeyRing(key); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *mcpLoginCommand) findMCPServers(cf *CLIConf, tc *client.TeleportClient) ([]types.Application, error) {
	selectors := resourceSelectors{
		kind:   "app",
		name:   cf.AppName,
		labels: cf.Labels,
		query:  cf.PredicateExpression,
	}
	switch {
	case cf.ListAll && !selectors.IsEmpty():
		return nil, trace.BadParameter("cannot use --labels or --query with --all")
	case !cf.ListAll && selectors.IsEmpty():
		return nil, trace.BadParameter("MCP server name is required. Check 'tsh mcp ls' for a list of available MCP servers.")
	}

	return getMCPServers(cf, tc)
}

func (c *mcpLoginCommand) autoDetectOrJSON(cf *CLIConf, mcpServers []types.Application) error {
	foundClaude, _ := claude.ConfigExists()
	if foundClaude {
		if err := cf.PromptConfirmation("Found Claude Desktop configuration. Update it?"); err == nil {
			return trace.Wrap(c.updateAndPrintClaude(cf, mcpServers))
		}
	}
	return trace.Wrap(c.printJSON(cf, mcpServers))
}

func (c *mcpLoginCommand) maybeClaude(cf *CLIConf, mcpServers []types.Application) error {
	foundClaude, err := claude.ConfigExists()
	if err != nil {
		return trace.Wrap(err)
	}
	if foundClaude {
		return trace.Wrap(c.updateAndPrintClaude(cf, mcpServers))
	}
	fmt.Fprintln(cf.Stdout(), "Claude Desktop configuration not found. Printing out JSON configuration instead.")
	return trace.Wrap(c.printJSON(cf, mcpServers))
}

func (c *mcpLoginCommand) printJSON(cf *CLIConf, mcpServers []types.Application) error {
	fmt.Fprintln(cf.Stdout(), "Here is a sample JSON configuration for launching Teleport MCP servers:")
	config := &claude.Config{
		MCPServers: appsToMCPServersMap(cf, mcpServers),
	}
	dump, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), string(dump))
	return nil
}

func (c *mcpLoginCommand) updateAndPrintClaude(cf *CLIConf, mcpServers []types.Application) error {
	// TODO(greedy52) refactor, like we already found it
	configPath, err := claude.ConfigPath()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), `Found Claude Desktop configuration at:
%s

Claude Desktop configuration will be updated automatically. Logged in Teleport
MCP servers will be prefixed with "teleport-" in this configuration.

`, configPath)

	if err := claude.UpdateConfigWithMCPServers(cf.Context, appsToMCPServersMap(cf, mcpServers)); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), `Run "tsh mcp logout" to remove the configuration from Claude Desktop.

You may need to restart Claude Desktop to reload these new configurations. If
you encounter a "disconnected" error when tsh session expires, you may also need
to restart Claude Desktop after logging in a new tsh session.`)
	return nil
}

func appsToMCPServersMap(cf *CLIConf, mcpServers []types.Application) map[string]claude.MCPServer {
	var envs map[string]string
	if homeDir := os.Getenv(types.HomeEnvVar); homeDir != "" {
		envs = map[string]string{
			types.HomeEnvVar: filepath.Clean(homeDir),
		}
	}

	ret := make(map[string]claude.MCPServer)
	for name := range types.ResourceNames(mcpServers) {
		localName := "teleport-" + name
		ret[localName] = claude.MCPServer{
			Command: cf.executablePath,
			Args:    []string{"mcp", "connect", name},
			Envs:    envs,
		}
	}
	return ret
}

type mcpListCommand struct {
	*kingpin.CmdClause
}

func (c *mcpListCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	mcpServers, err := getMCPServers(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.print(cf, mcpServers))
}

func getMCPServers(cf *CLIConf, tc *client.TeleportClient) (mcpServers []types.Application, err error) {
	filter := tc.ResourceFilter(types.KindAppServer)
	if cf.AppName != "" {
		filter.PredicateExpression = makeNamePredicate(cf.AppName)
	} else {
		// Filter by MCP schema.
		filter.PredicateExpression = makePredicateConjunction(
			filter.PredicateExpression,
			"hasPrefix(resource.spec.uri, \"mcp+\")",
		)
	}

	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		mcpServers, err = tc.ListApps(cf.Context, filter)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Sort by app name.
	sort.Slice(mcpServers, func(i, j int) bool {
		return mcpServers[i].GetName() < mcpServers[j].GetName()
	})
	return mcpServers, nil
}

func (c *mcpListCommand) print(cf *CLIConf, mcpServers []types.Application) error {
	switch cf.Format {
	case "", teleport.Text:
		return trace.Wrap(c.printText(cf, mcpServers))
	case teleport.JSON, teleport.YAML:
		out, err := serializeApps(mcpServers, cf.Format)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(cf.Stdout(), out); err != nil {
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
}

func (c *mcpListCommand) printText(cf *CLIConf, mcpServers []types.Application) error {
	t := asciitable.MakeTable([]string{"Name", "Description", "Type", "labels"})
	for mcpServer := range slices.Values(mcpServers) {
		t.AddRow([]string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), cf.Verbose),
		})
	}
	fmt.Fprintf(os.Stdout, t.AsBuffer().String())
	return nil
}

type mcpConnectCommand struct {
	*kingpin.CmdClause
}

func (c *mcpConnectCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	mcpServers, err := getMCPServers(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	switch len(mcpServers) {
	case 0:
		return trace.NotFound("no MCP servers found")
	case 1:
	default:
		logger.WarnContext(cf.Context, "multiple MCP servers found, using the first one")
	}

	// TODO(greedy52) load active cert?
	mcpServer := mcpServers[0]
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) refactor
	appCertParams := client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToApp: proto.RouteToApp{
			Name:        mcpServer.GetName(),
			PublicAddr:  mcpServer.GetPublicAddr(),
			ClusterName: tc.SiteName,
			URI:         mcpServer.GetURI(),
		},
		AccessRequests: profile.ActiveRequests,
	}

	// Do NOT write the keyring to avoid race condition when AI clients connect
	// multiple of them at the same time.
	keyRing, err := tc.IssueUserCertsWithMFA(cf.Context, appCertParams)
	if err != nil {
		return trace.Wrap(err)
	}
	credential, ok := keyRing.AppTLSCredentials[mcpServer.GetName()]
	if !ok {
		return trace.BadParameter("failed to find certificate for %q", mcpServer.GetName())
	}
	cert, err := credential.TLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	in, out := net.Pipe()
	listener := listenerutils.NewSingleUseListener(out)
	defer listener.Close()

	// TODO(greedy52) use middleware to refresh cert.
	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(cf.Context, tc, listener, tc.InsecureSkipVerify),
		alpnproxy.WithALPNProtocol(alpncommon.ProtocolTCP),
		alpnproxy.WithClientCert(cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(cf.Context, tc.RootClusterCACertPool),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		defer lp.Close()
		if err = lp.Start(cf.Context); err != nil {
			logger.ErrorContext(cf.Context, "Failed to start local ALPN proxy", "error", err)
		}
	}()

	clientConn := utils.CombinedStdio{}
	return utils.ProxyConn(cf.Context, in, clientConn)
}

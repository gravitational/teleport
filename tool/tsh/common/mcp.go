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
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/iterutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	pgmcp "github.com/gravitational/teleport/lib/client/db/postgres/mcp"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/tool/common"
)

type mcpCommands struct {
	login   *mcpLoginCommand
	logout  *mcpLogoutCommand
	list    *mcpListCommand
	connect *mcpConnectCommand

	db      *mcpDBCommand
	toolbox *mcpToolboxCommand
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) mcpCommands {
	mcp := app.Command("mcp", "View and control available MCP servers")
	return mcpCommands{
		login:   newMCPLoginCommand(mcp, cf),
		logout:  newMCPLogoutCommand(mcp, cf),
		list:    newMCPListCommand(mcp, cf),
		connect: newMCPConnectCommand(mcp, cf),
		db:      newMCPDBCommand(mcp),
		toolbox: newMCPToolboxCommand(mcp, cf),
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
	cmd.Flag("toolbox", "Add Teleport's toolbox MCP server in your client configuration.").BoolVar(&cmd.toolbox)
	return cmd
}

func newMCPListCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpListCommand {
	cmd := &mcpListCommand{
		CmdClause: parent.Command("ls", "List available MCP servers"),
	}

	cmd.Flag("verbose", "Show extra MCP server fields.").Short('v').BoolVar(&cf.Verbose)
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
	toolbox bool
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

	if len(mcpServers) > 0 {
		if err := c.loginAll(cf, tc, mcpServers); err != nil {
			return trace.Wrap(err)
		}

		// TODO(greedy52) maybe use template?
		fmt.Fprintln(cf.Stdout(), "Logged into Teleport MCP server(s).")
		for mcpServer := range slices.Values(mcpServers) {
			fmt.Fprintf(cf.Stdout(), "- %s\n", mcpServer.GetName())
		}
		fmt.Fprintln(cf.Stdout(), "")
	}

	switch cf.Format {
	case "":
		return c.autoDetectOrJSON(cf, mcpServers)
	case "json":
		return c.printJSON(cf, mcpServers)
	case "claude":
		return c.detectClaudeOrJSON(cf, mcpServers)
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

		result, err := clusterClient.IssueUserCertsWithMFA(cf.Context, appCertParams)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := tc.LocalAgent().AddAppKeyRing(result.KeyRing); err != nil {
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
		if c.toolbox {
			return nil, nil
		}
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

func (c *mcpLoginCommand) detectClaudeOrJSON(cf *CLIConf, mcpServers []types.Application) error {
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
		MCPServers: c.populateMCPServersMap(cf, mcpServers),
	}
	dump, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), string(dump))
	return nil
}

func (c *mcpLoginCommand) populateMCPServersMap(cf *CLIConf, mcpServers []types.Application) map[string]claude.MCPServer {
	servers := appsToMCPServersMap(cf, mcpServers)
	if c.toolbox {
		servers["teleport-toolbox"] = addEnvsToMCPServer(cf, claude.MCPServer{
			Command: cf.executablePath,
			Args:    []string{"mcp", "toolbox"},
		})
	}
	return servers
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

	if err := claude.UpdateConfigWithMCPServers(cf.Context, c.populateMCPServersMap(cf, mcpServers)); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), `Run "tsh mcp logout" to remove the configuration from Claude Desktop.

You may need to restart Claude Desktop to reload these new configurations. If
you encounter a "disconnected" error when tsh session expires, you may also need
to restart Claude Desktop after logging in a new tsh session.`)
	return nil
}

func addEnvsToMCPServer(cf *CLIConf, mcpServer claude.MCPServer) claude.MCPServer {
	var envs map[string]string
	if homeDir := os.Getenv(types.HomeEnvVar); homeDir != "" {
		envs = map[string]string{
			types.HomeEnvVar: filepath.Clean(homeDir),
		}
	}
	mcpServer.Envs = envs
	return mcpServer
}

func appsToMCPServersMap(cf *CLIConf, mcpServers []types.Application) map[string]claude.MCPServer {
	ret := make(map[string]claude.MCPServer)
	for name := range types.ResourceNames(mcpServers) {
		localName := "teleport-" + name
		ret[localName] = addEnvsToMCPServer(cf, claude.MCPServer{
			Command: cf.executablePath,
			Args:    []string{"mcp", "connect", name},
		})
	}
	return ret
}

type mcpLogoutCommand struct {
	*kingpin.CmdClause
}

func newMCPLogoutCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLogoutCommand {
	cmd := &mcpLogoutCommand{
		CmdClause: parent.Command("logout", "Logout MCP servers and remove client configurations."),
	}

	cmd.Flag("format", "\"claude\" for updating Claude Desktop configuration.").Short('f').StringVar(&cf.Format)
	cmd.Arg("name", "Name of the MCP server").StringVar(&cf.AppName)
	return cmd
}

func (c *mcpLogoutCommand) run(cf *CLIConf) error {
	switch cf.Format {
	case "claude":
		errors := []error{
			c.logoutClaudeDesktop(cf),
			c.logoutApps(cf),
		}
		return trace.NewAggregate(errors...)
	default:
		return trace.Wrap(c.logoutApps(cf))
	}
}

func (c *mcpLogoutCommand) logoutApps(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	activeRoutes, err := profile.AppsForCluster(tc.SiteName, tc.ClientStore)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO filter activeRoutes by mcp servers.
	err = logoutApps(cf, tc, profile, activeRoutes)
	if err != nil && strings.Contains(err.Error(), "not logged into app") {
		fmt.Fprintln(cf.Stdout(), err.Error())
		err = nil
	}
	return trace.Wrap(err)
}

func (c *mcpLogoutCommand) logoutClaudeDesktop(cf *CLIConf) error {
	config, err := claude.LoadConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	var updated bool
	for name, server := range config.MCPServers {
		if !strings.HasPrefix(name, "teleport-") || server.Command != cf.executablePath {
			continue
		}
		if cf.AppName == "" {
			updated = true
			fmt.Fprintf(cf.Stdout(), "Removing %q from Claude Desktop configuration.\n", name)
			delete(config.MCPServers, name)
			continue
		}
		if len(server.Args) >= 3 &&
			server.Args[0] == "mcp" &&
			server.Args[1] == "connect" &&
			server.Args[2] == cf.AppName {
			updated = true
			fmt.Fprintf(cf.Stdout(), "Removing %q from Claude Desktop configuration.\n", name)
			delete(config.MCPServers, name)
		}
	}

	if err := claude.SaveConfig(cf.Context, config); err != nil {
		return trace.Wrap(err)
	}
	if updated {
		fmt.Fprintln(cf.Stdout(), "Claude Desktop configuration updated.")
	} else {
		fmt.Fprintln(cf.Stdout(), "No change made to Claude Desktop configuration.")
	}
	return nil
}

type mcpListCommand struct {
	*kingpin.CmdClause
}

func (c *mcpListCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterClient *client.ClusterClient
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), tc.SiteName, clusterClient.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	mcpServers, err := getMCPServers(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.print(cf, mcpServers, accessChecker))
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

func (c *mcpListCommand) print(cf *CLIConf, mcpServers []types.Application, accessChecker services.AccessChecker) error {
	mcpServersWithDetails := iterutils.Map(func(in types.Application) appWithDetails {
		return makeAppWithDetails(in, accessChecker)
	}, slices.Values(mcpServers))

	switch cf.Format {
	case "", teleport.Text:
		if cf.Verbose {
			return trace.Wrap(c.printTextVerbose(cf, mcpServersWithDetails))
		}
		return trace.Wrap(c.printText(cf, mcpServersWithDetails))
	case teleport.JSON:
		out, err := utils.FastMarshalIndent(slices.Collect(mcpServersWithDetails), "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = fmt.Fprintln(cf.Stdout(), string(out))
		return trace.Wrap(err)
	case teleport.YAML:
		out, err := yaml.Marshal(slices.Collect(mcpServersWithDetails))
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = fmt.Fprintln(cf.Stdout(), string(out))
		return trace.Wrap(err)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
}

func (c *mcpListCommand) printText(cf *CLIConf, mcpServers iter.Seq[appWithDetails]) error {
	t := asciitable.MakeTable([]string{"Name", "Description", "Type", "Labels"})
	for mcpServer := range mcpServers {
		t.AddRow([]string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), cf.Verbose),
		})
	}
	_, err := fmt.Fprintf(os.Stdout, t.AsBuffer().String())
	return trace.Wrap(err)
}

func (c *mcpListCommand) printTextVerbose(cf *CLIConf, mcpServers iter.Seq[appWithDetails]) error {
	t := asciitable.MakeTable([]string{"Name", "Description", "Type", "Labels", "Command", "Args", "Denied Tools", "Allowed Tools"})
	for mcpServer := range mcpServers {
		mcpSpec := mcpServer.GetMCP()
		if mcpSpec == nil {
			mcpSpec = &types.MCP{}
		}
		t.AddRow([]string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), cf.Verbose),
			mcpSpec.Command,
			strings.Join(mcpSpec.Args, " "),
			strings.Join(mcpServer.Permissions.MCP.Tools.Denied, ","),
			strings.Join(mcpServer.Permissions.MCP.Tools.Allowed, ","),
		})
	}
	_, err := fmt.Fprintf(os.Stdout, t.AsBuffer().String())
	return trace.Wrap(err)
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

// mcpDBCommand implements `tsh mcp db` command.
type mcpDBCommand struct {
	*kingpin.CmdClause

	databaseUser        string
	databaseName        string
	labels              string
	predicateExpression string
	dryRun              bool
}

func newMCPDBCommand(parent *kingpin.CmdClause) *mcpDBCommand {
	cmd := &mcpDBCommand{
		CmdClause: parent.Command("db", "Start a local MCP server for database access"),
	}

	cmd.Flag("db-user", "Database user to log in as.").Short('u').StringVar(&cmd.databaseUser)
	cmd.Flag("db-name", "Database name to log in to.").Short('n').StringVar(&cmd.databaseName)
	cmd.Flag("labels", labelHelp).StringVar(&cmd.labels)
	cmd.Flag("query", queryHelp).StringVar(&cmd.predicateExpression)
	return cmd
}

func (c *mcpDBCommand) run(cf *CLIConf) error {
	logger, _, err := logutils.Initialize(logutils.Config{
		Severity: slog.LevelInfo.String(),
		Format:   mcpLogFormat,
		Output:   mcpLogOutput,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	registry := defaultDBMCPRegistry
	if cf.databaseMCPRegistryOverride != nil {
		registry = cf.databaseMCPRegistryOverride
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Avoid any input request on the command execution. This is required,
	// otherwise the MCP clients will be stuck waiting for a response.
	tc.NonInteractive = false

	sc, err := newSharedDatabaseExecClient(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	dbs, err := c.getDatabases(cf.Context, sc)
	if err != nil {
		return trace.Wrap(err)
	}

	server := dbmcp.NewRootServer()
	allDatabases, closeLocalProxies, err := c.prepareDatabases(cf.Context, registry, dbs, logger, tc, sc.profile, server)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeLocalProxies()

	for protocol, newServerFunc := range registry {
		databases := allDatabases[protocol]
		if len(databases) == 0 {
			continue
		}

		srv, err := newServerFunc(cf.Context, &dbmcp.NewServerConfig{
			RootServer: server,
			Databases:  databases,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer srv.Close(cf.Context)
	}

	return trace.Wrap(server.ServeStdio(cf.Context, cf.Stdin(), cf.Stdout()))
}

// closeLocalProxyFunc function used to close local proxy listeners.
type closeLocalProxyFunc func() error

// prepareDatabases based on the available MCP servers, initialize the database
// local proxy and generate the MCP database.
func (c *mcpDBCommand) prepareDatabases(
	ctx context.Context,
	registry dbmcp.Registry,
	dbs []types.Database,
	logger *slog.Logger,
	tc *client.TeleportClient,
	profile *client.ProfileStatus,
	server *dbmcp.RootServer,
) (map[string][]*dbmcp.Database, closeLocalProxyFunc, error) {
	var (
		dbsPerProtocol = make(map[string][]*dbmcp.Database)
		closeFuncs     []closeLocalProxyFunc
	)

	for _, db := range dbs {
		if !registry.IsSupported(db.GetProtocol()) {
			logger.InfoContext(ctx, "database protocol unsupported, skipping it", "database", db.GetName(), "protocol", db.GetProtocol())
			continue
		}

		route := tlsca.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
		}

		listener, err := createLocalProxyListener("localhost:0", route, profile)
		if err != nil {
			logger.ErrorContext(ctx, "failed to start local proxy listener for database, skipping it", "database", db.GetName(), "error", err)
			continue
		}

		cc := client.NewDBCertChecker(tc, route, nil, client.WithTTL(tc.KeyTTL))
		lp, err := alpnproxy.NewLocalProxy(
			makeBasicLocalProxyConfig(ctx, tc, listener, tc.InsecureSkipVerify),
			alpnproxy.WithDatabaseProtocol(route.Protocol),
			alpnproxy.WithMiddleware(cc),
			alpnproxy.WithClusterCAsIfConnUpgrade(ctx, tc.RootClusterCACertPool),
		)
		if err != nil {
			_ = listener.Close()
			logger.ErrorContext(ctx, "failed to start local proxy for database, skipping it", "database", db.GetName(), "error", err)
			continue
		}
		go func() {
			defer lp.Close()
			if err = lp.Start(ctx); err != nil {
				logger.WarnContext(ctx, "failed to start local ALPN proxy", "error", err)
			}
		}()

		mcpDB := &dbmcp.Database{
			DB:           db,
			DatabaseUser: c.databaseUser,
			DatabaseName: c.databaseName,
			Addr:         listener.Addr().String(),
		}
		dbsPerProtocol[db.GetProtocol()] = append(dbsPerProtocol[db.GetProtocol()], mcpDB)
		server.RegisterDatabase(mcpDB)
		closeFuncs = append(closeFuncs, listener.Close)
	}

	return dbsPerProtocol, func() error {
		var errs []error
		for _, closeFunc := range closeFuncs {
			errs = append(errs, closeFunc())
		}

		return trace.NewAggregate(errs...)
	}, nil
}

func (c *mcpDBCommand) getDatabases(ctx context.Context, sc *sharedDatabaseExecClient) ([]types.Database, error) {
	labels, err := client.ParseLabelSpec(c.labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbsList, err := sc.listDatabasesWithFilter(ctx, &proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseServer,
		Namespace:           apidefaults.Namespace,
		Labels:              labels,
		PredicateExpression: c.predicateExpression,
	})

	return dbsList, trace.Wrap(err)
}

type mcpToolboxCommand struct {
	*kingpin.CmdClause
}

func newMCPToolboxCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpToolboxCommand {
	cmd := &mcpToolboxCommand{
		CmdClause: parent.Command("toolbox", "Start a local MCP server for various Teleport tools like access request, search audit events."),
	}
	return cmd
}

func (c *mcpToolboxCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterClient *client.ClusterClient
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	authClient := clusterClient.AuthClient

	mcpServer := server.NewMCPServer(
		"teleport_toolbox",
		teleport.Version,
		server.WithInstructions(`Teleport is the easiest, most secure way to access and protect all your infrastructure. Teleport logs cluster activity by emitting various events into its audit log.`),
	)
	mcpServer.AddTool(
		mcp.NewTool(
			"teleport_search_events",
			mcp.WithDescription(`Search Teleport audit events.

The tool takes in two mandatory parameters "from" and "to" which are the
searching time range. The time must be in RFC3339 formats. 
An optional "start_key"" param can be used to perform pagination where returned
by previous call.

The response is a list of audit events found in that time period, maximum 100
per call. If more events are available, it will return a "next_key"" to be used
as "start_key"" in the next call for pagination.
`),
			mcp.WithString("from", mcp.Required(), mcp.Description("oldest date of returned events, in RFC3339 format, in UTC")),
			mcp.WithString("to", mcp.Required(), mcp.Description("newest date of returned events, in RFC3339 format, in UTC")),
			mcp.WithString("start_key", mcp.Description("key to start pagination from, if any")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			fromStr, ok := request.Params.Arguments["from"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'from'")
			}
			toStr, ok := request.Params.Arguments["to"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'to'")
			}
			from, err := time.Parse(time.RFC3339, fromStr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse 'from' as RFC3339 format")
			}
			to, err := time.Parse(time.RFC3339, toStr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse 'to' as RFC3339 format")
			}
			req := libevents.SearchEventsRequest{
				From:  from,
				To:    to,
				Limit: 100,
			}
			startKey, ok := request.Params.Arguments["start_key"].(string)
			if ok {
				req.StartKey = startKey
			}

			events, nextKey, err := authClient.SearchEvents(cf.Context, req)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			result, err := json.Marshal(map[string]any{
				"events":   events,
				"next_key": nextKey,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return mcp.NewToolResultText(string(result)), nil
		},
	)

	mcpServer.AddTool(
		mcp.NewTool(
			"teleport_create_access_request",
			mcp.WithDescription(`Create Teleport access request.

The tool takes a mandatory "role" parameter that indicates a Teleport role
an access request should be submitted for.
`),
			mcp.WithString("role", mcp.Required(), mcp.Description("role name to request")),
			mcp.WithString("reason", mcp.Description("optional reason for the request")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			role, ok := request.Params.Arguments["role"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'role'")
			}

			accessRequest, err := types.NewAccessRequest(
				uuid.NewString(),
				tc.Username,
				role)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if reason, ok := request.Params.Arguments["reason"].(string); ok {
				accessRequest.SetRequestReason(reason)
			}

			createdRequest, err := authClient.CreateAccessRequestV2(cf.Context, accessRequest)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			result, err := json.Marshal(createdRequest)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return mcp.NewToolResultText(string(result)), nil
		},
	)

	mcpServer.AddTool(
		mcp.NewTool(
			"teleport_list_access_request",
			mcp.WithDescription(`List Teleport access request and show their details.

The "state" has the following possible values:
NONE = 0, PENDING = 1, APPROVED = 2, DENIED = 3, PROMOTED = 4
`),
			mcp.WithString("id", mcp.Description("Optional id to filter requests by")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filter := types.AccessRequestFilter{}
			id, ok := request.Params.Arguments["id"].(string)
			if ok && id != "" {
				filter.ID = id
			}
			resp, err := authClient.GetAccessRequests(ctx, filter)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			result, err := json.Marshal(resp)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return mcp.NewToolResultText(string(result)), nil
		},
	)

	return trace.Wrap(
		server.NewStdioServer(mcpServer).Listen(cf.Context, cf.Stdin(), cf.Stdout()),
	)
}

var (
	// defaultDBMCPRegistry is the default database access MCP servers registry.
	defaultDBMCPRegistry = map[string]dbmcp.NewServerFunc{
		defaults.ProtocolPostgres: pgmcp.NewServer,
	}
)

const (
	// mcpLogFormat defiens the log format of the MCP command.
	mcpLogFormat = "json"
	// mcpLogFormat defines to where the MCP command logs will be directed to.
	// The stdout is exclusively used as the MCP server transport, leaving only
	// stderr available.
	mcpLogOutput = "stderr"
)

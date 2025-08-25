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
	"cmp"
	"context"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/iterutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	mcpconfig "github.com/gravitational/teleport/lib/client/mcp/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

func newMCPConnectCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpConnectCommand {
	cmd := &mcpConnectCommand{
		CmdClause: parent.Command("connect", "Connect to an MCP server.").Hidden(),
		cf:        cf,
	}

	cmd.Arg("name", "Name of the MCP server.").Required().StringVar(&cf.AppName)
	cmd.Flag("auto-reconnect", mcpAutoReconnectHelp).Default("true").BoolVar(&cmd.autoReconnect)
	return cmd
}

func newMCPListCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpListCommand {
	cmd := &mcpListCommand{
		CmdClause: parent.Command("ls", "List available MCP server applications."),
		cf:        cf,
	}

	cmd.Flag("verbose", "Show extra MCP server fields.").Short('v').BoolVar(&cf.Verbose)
	cmd.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	cmd.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	cmd.Arg("labels", labelHelp).StringVar(&cf.Labels)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaults.DefaultFormats...)
	return cmd
}

func newMCPConfigCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpConfigCommand {
	cmd := &mcpConfigCommand{
		CmdClause: parent.Command("config", "Print client configuration details."),
		cf:        cf,
	}

	cmd.Flag("all", "Select all MCP servers. Mutually exclusive with --labels or --query.").Short('R').BoolVar(&cf.ListAll)
	cmd.Flag("labels", labelHelp).StringVar(&cf.Labels)
	cmd.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	cmd.Flag("auto-reconnect", mcpAutoReconnectHelp).IsSetByUser(&cmd.autoReconnectSetByUser).BoolVar(&cmd.autoReconnect)
	cmd.Arg("name", "Name of the MCP server.").StringVar(&cf.AppName)
	cmd.clientConfig.addToCmd(cmd.CmdClause)
	cmd.Alias(mcpConfigHelp)
	return cmd
}

// mcpListCommand implements `tsh mcp ls` command.
type mcpListCommand struct {
	*kingpin.CmdClause
	cf            *CLIConf
	accessChecker services.AccessChecker
	mcpServers    []types.Application
}

func (c *mcpListCommand) run() error {
	if err := c.fetch(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.print())
}

func (c *mcpListCommand) fetch() error {
	ctx := c.cf.Context
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterClient *client.ClusterClient
	err = client.RetryWithRelogin(ctx, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(ctx)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	c.accessChecker, err = makeAccessChecker(ctx, tc, clusterClient.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	c.mcpServers, err = fetchMCPServers(ctx, tc, clusterClient.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *mcpListCommand) print() error {
	mcpServers := iterutils.Map(func(app types.Application) mcpServerWithDetails {
		return newMCPServerWithDetails(app, c.accessChecker)
	}, slices.Values(c.mcpServers))

	switch c.cf.Format {
	case "", teleport.Text:
		if c.cf.Verbose {
			return trace.Wrap(printMCPServersInVerboseText(c.cf.Stdout(), mcpServers))
		}
		return trace.Wrap(printMCPServersInText(c.cf.Stdout(), mcpServers))

	case teleport.JSON:
		return trace.Wrap(common.PrintJSONIndent(c.cf.Stdout(), slices.Collect(mcpServers)))
	case teleport.YAML:
		return trace.Wrap(common.PrintYAML(c.cf.Stdout(), slices.Collect(mcpServers)))

	default:
		return trace.BadParameter("unsupported format %q", c.cf.Format)
	}
}

func fetchMCPServers(ctx context.Context, tc *client.TeleportClient, auth apiclient.GetResourcesClient) ([]types.Application, error) {
	if auth == nil {
		var clusterClient *client.ClusterClient
		var err error
		err = client.RetryWithRelogin(ctx, tc, func() error {
			clusterClient, err = tc.ConnectToCluster(ctx)
			return trace.Wrap(err)
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer clusterClient.Close()
		auth = clusterClient.AuthClient
	}

	ctx, span := tc.Tracer.Start(
		ctx,
		"fetchMCPServers",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	filter := tc.ResourceFilter(types.KindAppServer)
	filter.PredicateExpression = withMCPServerAppFilter(filter.PredicateExpression)

	appServers, err := apiclient.GetAllResources[types.AppServer](ctx, auth, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return slices.SortedFunc(
		types.DeduplicatedApps(types.AppServers(appServers).Applications()),
		types.CompareResourceByNames,
	), nil
}

func withMCPServerAppFilter(predicateExpression string) string {
	return makePredicateConjunction(
		predicateExpression,
		`resource.sub_kind == "mcp"`,
	)
}

// mcpServerWithDetails defines an MCP server application with permission
// details, for printing purpose.
type mcpServerWithDetails struct {
	// Use a real type for inline.
	*types.AppV3

	Permissions struct {
		MCP struct {
			Tools struct {
				Allowed []string `json:"allowed"`
				Denied  []string `json:"denied,omitempty"`
			} `json:"tools"`
		} `json:"mcp"`
	} `json:"permissions"`
}

func (a *mcpServerWithDetails) updateToolsPermissions(accessChecker services.AccessChecker) {
	if accessChecker == nil {
		return
	}

	mcpTools := accessChecker.EnumerateMCPTools(a.AppV3)
	a.Permissions.MCP.Tools.Allowed, a.Permissions.MCP.Tools.Denied = mcpTools.ToEntities()
}

func newMCPServerWithDetails(app types.Application, accessChecker services.AccessChecker) mcpServerWithDetails {
	a := mcpServerWithDetails{
		AppV3: app.Copy(),
	}
	a.updateToolsPermissions(accessChecker)
	return a
}

type mcpListRBACPrinter struct {
	showFootnote bool
}

func (p *mcpListRBACPrinter) formatAllowedTools(mcpServer mcpServerWithDetails) string {
	allowed := common.FormatAllowedEntities(mcpServer.Permissions.MCP.Tools.Allowed, mcpServer.Permissions.MCP.Tools.Denied)
	if len(mcpServer.Permissions.MCP.Tools.Allowed) == 0 {
		allowed += " [!]"
		p.showFootnote = true
	}
	return allowed
}

func (p *mcpListRBACPrinter) maybePrintFootnote(w io.Writer) error {
	if !p.showFootnote {
		return nil
	}
	_, err := fmt.Fprintf(w, `[!] Warning: you do not have access to any tools on the MCP server.
Please contact your Teleport administrator to ensure your Teleport role has
appropriate 'allow.mcp.tools' set. For details on MCP access RBAC, see:
https://goteleport.com/docs/enroll-resources/mcp-access/rbac/
`)
	return trace.Wrap(err)
}

func printMCPServersInText(w io.Writer, mcpServers iter.Seq[mcpServerWithDetails]) error {
	var rows [][]string
	var rbacPrinter mcpListRBACPrinter
	for mcpServer := range mcpServers {
		rows = append(rows, []string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			rbacPrinter.formatAllowedTools(mcpServer),
			common.FormatLabels(mcpServer.GetAllLabels(), false),
		})
	}
	t := asciitable.MakeTableWithTruncatedColumn([]string{"Name", "Description", "Type", "Allowed Tools", "Labels"}, rows, "Labels")
	if _, err := fmt.Fprintln(w, t.String()); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(rbacPrinter.maybePrintFootnote(w))
}

func printMCPServersInVerboseText(w io.Writer, mcpServers iter.Seq[mcpServerWithDetails]) error {
	t := asciitable.MakeTable([]string{"Name", "Description", "Type", "Labels", "Command", "Args", "Allowed Tools"})
	var rbacPrinter mcpListRBACPrinter
	for mcpServer := range mcpServers {
		mcpSpec := cmp.Or(mcpServer.GetMCP(), &types.MCP{})
		t.AddRow([]string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), true),
			mcpSpec.Command,
			strings.Join(mcpSpec.Args, " "),
			rbacPrinter.formatAllowedTools(mcpServer),
		})
	}
	if _, err := fmt.Fprintln(w, t.String()); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(rbacPrinter.maybePrintFootnote(w))
}

type mcpConfigCommand struct {
	*kingpin.CmdClause
	clientConfig           mcpClientConfigFlags
	cf                     *CLIConf
	autoReconnect          bool
	autoReconnectSetByUser bool

	mcpServerApps []types.Application

	// fetchFunc is for fetching MCP servers, defaults to fetchMCPServers. Can
	// be mocked in tests.
	fetchFunc func(context.Context, *client.TeleportClient, apiclient.GetResourcesClient) ([]types.Application, error)
}

func (c *mcpConfigCommand) run() error {
	if err := c.checkSelectorFlags(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(runMCPConfig(c.cf, &c.clientConfig, c))
}

func (c *mcpConfigCommand) checkSelectorFlags() error {
	// Some of them can technically be used together but make them mutually
	// exclusively for simplicity.
	var mutuallyExclusiveSelectors int
	for _, selectorEnabled := range []bool{
		c.cf.ListAll,
		c.cf.AppName != "",
		c.cf.PredicateExpression != "",
		c.cf.Labels != "",
	} {
		if selectorEnabled {
			mutuallyExclusiveSelectors++
		}
	}

	switch mutuallyExclusiveSelectors {
	case 0:
		return trace.BadParameter("no selector specified. Please provide the MCP server name or use one of the following flags: --all, --labels, or --query.")
	case 1:
		return nil
	default:
		return trace.BadParameter("only one selector is allowed. Specify either the MCP server name or one of --all, --labels, or --query flags.")
	}
}

func (c *mcpConfigCommand) fetchAndPrintResult() error {
	if err := c.fetch(); err != nil {
		return trace.Wrap(err)
	}

	printList := fmt.Sprintf(`Found MCP servers:
%v

`, strings.Join(slices.Collect(types.ResourceNames(c.mcpServerApps)), "\n"))
	_, err := fmt.Fprint(c.cf.Stdout(), printList)
	return trace.Wrap(err)
}

func (c *mcpConfigCommand) fetch() error {
	if c.cf.AppName != "" {
		c.cf.PredicateExpression = makeNamePredicate(c.cf.AppName)
	}
	if c.fetchFunc == nil {
		c.fetchFunc = fetchMCPServers
	}

	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	c.mcpServerApps, err = c.fetchFunc(c.cf.Context, tc, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(c.mcpServerApps) == 0 {
		return trace.NotFound("no MCP servers found")
	}
	return nil
}

func (c *mcpConfigCommand) addMCPServersToConfig(config mcpConfig) error {
	for _, app := range c.mcpServerApps {
		localName := mcpServerAppConfigPrefix + app.GetName()
		args := []string{"mcp", "connect", app.GetName()}
		args = c.maybeAddAutoReconnect(args)
		err := config.PutMCPServer(localName, makeLocalMCPServer(c.cf, args))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *mcpConfigCommand) maybeAddAutoReconnect(args []string) []string {
	if !c.autoReconnectSetByUser {
		return args
	}
	if c.autoReconnect {
		return append(args, "--auto-reconnect")
	}
	return append(args, "--no-auto-reconnect")
}

func (c *mcpConfigCommand) printInstructions(w io.Writer, configFormat mcpconfig.ConfigFormat) error {
	if err := c.fetchAndPrintResult(); err != nil {
		return trace.Wrap(err)
	}

	config := mcpconfig.NewConfig(configFormat)
	if err := c.addMCPServersToConfig(config); err != nil {
		return trace.Wrap(err)
	}

	if _, err := fmt.Fprintf(w, "Here is a sample JSON configuration for launching Teleport MCP servers using %s format:\n", configFormat); err != nil {
		return trace.Wrap(err)
	}
	if err := config.Write(w, mcpconfig.FormatJSONOption(c.clientConfig.jsonFormat)); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.clientConfig.printFooterNotes(w))
}

func (c *mcpConfigCommand) updateConfig(w io.Writer, config *mcpconfig.FileConfig) error {
	if err := c.fetchAndPrintResult(); err != nil {
		return trace.Wrap(err)
	}

	if err := c.addMCPServersToConfig(config); err != nil {
		return trace.Wrap(err)
	}

	if err := config.Save(mcpconfig.FormatJSONOption(c.clientConfig.jsonFormat)); err != nil {
		return trace.Wrap(err)
	}

	_, err := fmt.Fprintf(c.cf.Stdout(), `Updated client configuration at:
%s

Teleport MCP servers will be prefixed with "teleport-mcp-" in this
configuration. You may need to restart your client to reload these new
configurations.
`, config.Path())
	return trace.Wrap(err)
}

const (
	mcpServerAppConfigPrefix = "teleport-mcp-"
	mcpAutoReconnectHelp     = "Automatically starts a new remote MCP session " +
		"when the previous remote session is interrupted " +
		"by network issues or tsh session expirations. " +
		"Recommended for stateless MCP sessions. Defaults to true."
)

// mcpConnectCommand implements `tsh mcp connect` command.
type mcpConnectCommand struct {
	*kingpin.CmdClause
	cf            *CLIConf
	autoReconnect bool
}

func (c *mcpConnectCommand) run() error {
	_, err := initLogger(c.cf, utils.LoggingForMCP, getLoggingOptsForMCPServer(c.cf))
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.NonInteractive = true

	if c.autoReconnect {
		return clientmcp.ProxyStdioConnWithAutoReconnect(
			c.cf.Context,
			clientmcp.ProxyStdioConnWithAutoReconnectConfig{
				ClientStdio: utils.CombinedStdio{},
				DialServer: func(ctx context.Context) (io.ReadWriteCloser, error) {
					conn, err := tc.DialMCPServer(ctx, c.cf.AppName)
					return conn, trace.Wrap(err)
				},
				MakeReconnectUserMessage: makeMCPReconnectUserMessage,
			},
		)
	}

	serverConn, err := tc.DialMCPServer(c.cf.Context, c.cf.AppName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(utils.ProxyConn(c.cf.Context, utils.CombinedStdio{}, serverConn))
}

func makeMCPReconnectUserMessage(err error) string {
	var userMessage string
	switch {
	case clientmcp.IsLikelyTemporaryNetworkError(err):
		userMessage = "A network error occurred while trying to connect to Teleport." +
			" This issue is likely temporary — the server may be unavailable, or your internet connection may be unstable." +
			" Please check your network and try again in a few moments." +
			" If your network appears to be working, try restarting your MCP client to see if the problem is resolved."
	case client.IsErrorResolvableWithRelogin(err):
		userMessage = clientmcp.ReloginRequiredErrorMessage
	case clientmcp.IsServerInfoChangedError(err):
		userMessage = "The remote MCP server information has changed after the reconnection. " +
			" Please restart your MCP client to use the new version."
	default:
		userMessage = "An error was encountered while sending the request to Teleport." +
			" This does not appear to be a transient error." +
			" Please ensure your tsh session is valid and restart your MCP client to see if the problem is resolved."
	}

	userMessage += " If the issue persists, check the MCP logs for more details or contact your Teleport admin."
	return userMessage
}

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
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
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
	"github.com/gravitational/teleport/lib/utils/mcputils"
	"github.com/gravitational/teleport/tool/common"
)

func newMCPConnectCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpConnectCommand {
	cmd := &mcpConnectCommand{
		CmdClause: parent.Command("connect", "Connect to an MCP server.").Hidden(),
		cf:        cf,
	}

	cmd.Arg("name", "Name of the MCP server.").Required().SetValue(&cf.AppSQN)
	cmd.Flag("auto-reconnect", mcpAutoReconnectHelp).Default("true").BoolVar(&cmd.autoReconnect)
	cmd.Flag("header", "Extra custom headers used for streamable HTTP MCP servers.").Short('H').StringsVar(&cmd.httpHeaders)
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
	cmd.Arg("name", "Name of the MCP server.").SetValue(&cf.AppSQN)
	cmd.clientConfig.addToCmd(cmd.CmdClause)
	cmd.Alias(mcpConfigHelp)
	cmd.Flag("header", "Extra custom headers used for streamable HTTP MCP servers.").Short('H').StringsVar(&cmd.httpHeaders)
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
	httpHeaders            []string

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
		c.cf.AppSQN.Name != "",
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
	if c.cf.AppSQN.Name != "" {
		c.cf.PredicateExpression = makeNamePredicate(c.cf.AppSQN.Name)
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
		if types.GetMCPServerTransportType(app.GetURI()) == types.MCPTransportHTTP {
			if _, err := parseHTTPHeaders(c.httpHeaders); err != nil {
				return trace.Wrap(err)
			}
			for _, header := range c.httpHeaders {
				args = append(args, "-H", header)
			}
		}
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
	httpHeaders   []string
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

	httpHeaders, err := parseHTTPHeaders(c.httpHeaders)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer := client.NewMCPServerDialer(tc, c.cf.AppSQN.Name)

	// Wire up OAuth credentials from `tsh mcp login`, if there are any. The
	// Authorization header is produced per request so that expired access
	// tokens get silently refreshed. An explicit -H "Authorization: ..."
	// always wins.
	credsPath := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.SiteName, c.cf.AppSQN.Name)
	var oauthSource *mcpOAuthHeaderSource
	var authDetail string
	if _, ok := httpHeaders["Authorization"]; ok {
		logger.InfoContext(c.cf.Context, "Using the explicit Authorization header from -H; stored MCP OAuth credentials are ignored", "app", c.cf.AppSQN.Name)
		authDetail = "The explicit Authorization header from -H was sent; credentials from `tsh mcp login` are ignored while it is set."
	} else {
		oauthSource, err = newMCPOAuthHeaderSource(c.cf.Context, dialer, c.cf.HomePath, tc.WebProxyHost(), tc.SiteName, c.cf.AppSQN.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if oauthSource == nil {
			logger.InfoContext(c.cf.Context, "No stored MCP OAuth credentials found", "path", credsPath)
			authDetail = fmt.Sprintf("No stored credentials were found at %q; `tsh mcp login` may have run against a different profile, cluster, or TELEPORT_HOME.", credsPath)
		} else {
			logger.InfoContext(c.cf.Context, "Using stored MCP OAuth credentials", "path", credsPath)
			authDetail = fmt.Sprintf("Stored credentials from %q were sent but rejected.", credsPath)
		}
	}
	var getAuthHeader func(context.Context) (string, error)
	var refreshAuthHeader func(context.Context, string) (string, error)
	if oauthSource != nil {
		getAuthHeader = oauthSource.GetAuthHeader
		refreshAuthHeader = oauthSource.RefreshAuthHeader
	}

	return clientmcp.ProxyStdioConn(
		c.cf.Context,
		clientmcp.ProxyStdioConnConfig{
			ClientStdio: utils.CombinedStdio{},
			GetApp:      dialer.GetApp,
			DialServer:  dialer.DialALPN,
			MakeReconnectUserMessage: func(err error) string {
				return makeMCPReconnectUserMessageWithAuthDetail(c.cf.AppSQN.Name, authDetail, err)
			},
			AutoReconnect:         c.autoReconnect,
			HTTPHeaders:           httpHeaders,
			GetHTTPAuthHeader:     getAuthHeader,
			RefreshHTTPAuthHeader: refreshAuthHeader,
		},
	)
}

func parseHTTPHeaders(headerArgs []string) (map[string]string, error) {
	if len(headerArgs) == 0 {
		return nil, nil
	}
	httpHeaders := make(map[string]string)
	for _, header := range headerArgs {
		key, value, ok := strings.Cut(header, ":")
		if !ok {
			return nil, trace.BadParameter("malformed header %q", header)
		}
		httpHeaders[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return httpHeaders, nil
}

func makeMCPReconnectUserMessage(err error) string {
	return makeMCPReconnectUserMessageWithAuthDetail("", "", err)
}

func makeMCPReconnectUserMessageForApp(appName string, err error) string {
	return makeMCPReconnectUserMessageWithAuthDetail(appName, "", err)
}

func makeMCPReconnectUserMessageWithAuthDetail(appName, authDetail string, err error) string {
	server := "the MCP server"
	loginTarget := "<server-name>"
	if appName != "" {
		server = fmt.Sprintf("MCP server %q", appName)
		loginTarget = appName
	}

	var loginRequiredErr *mcpOAuthLoginRequiredError
	var httpServerErr *clientmcp.HTTPServerError
	switch {
	case errors.As(err, &loginRequiredErr):
		return fmt.Sprintf("[MCP_AUTH_REQUIRED] Authentication with MCP server %q is required or has expired."+
			" Run `tsh mcp login %s` in a terminal, complete authorization in the browser, then retry the request.",
			loginRequiredErr.appName, loginRequiredErr.appName)
	case isMCPProtectedResourceMismatch(err):
		return fmt.Sprintf("[MCP_AUTH_FLOW_MISMATCH] The MCP client tried to authenticate directly through Teleport's local endpoint, but the OAuth server only accepts its public resource URL."+
			" Run `tsh mcp login %s` in a terminal instead, then retry in the MCP client.", loginTarget)
	case errors.Is(err, mcpclienttransport.ErrUnauthorized):
		message := fmt.Sprintf("[MCP_AUTH_REQUIRED] %s rejected the request with HTTP 401."+
			" Run `tsh mcp login %s` in a terminal, then retry. Do not use the MCP client's built-in OAuth login for a Teleport endpoint.",
			server, loginTarget)
		if authDetail != "" {
			message += " " + authDetail
		}
		return message
	case client.IsErrorResolvableWithRelogin(err):
		return "[MCP_TELEPORT_LOGIN_REQUIRED] " + clientmcp.ReloginRequiredErrorMessage
	case errors.Is(err, mcpclienttransport.ErrSessionTerminated):
		return fmt.Sprintf("[MCP_SESSION_EXPIRED] %s rejected the saved MCP session (HTTP 404)."+
			" Restart the MCP client to create a new session. If this repeats, the remote server may be losing session state.", server)
	case errors.Is(err, mcpclienttransport.ErrLegacySSEServer):
		return fmt.Sprintf("[MCP_TRANSPORT_MISMATCH] %s is configured as Streamable HTTP, but its endpoint responded like a legacy SSE server."+
			" Ask your Teleport administrator to verify the MCP application URI and transport.", server)
	case errors.As(err, &httpServerErr):
		return makeMCPHTTPServerErrorMessage(server, httpServerErr)
	case clientmcp.IsNetworkTimeoutError(err) || isMCPHTTPTimeout(err):
		return makeMCPTimeoutUserMessage(server)
	case clientmcp.IsServerInfoChangedError(err):
		return fmt.Sprintf("[MCP_SERVER_CHANGED] %s reported a different name or version after reconnecting."+
			" Restart the MCP client so it can load the server's current tools and capabilities.", server)
	case clientmcp.IsLikelyTemporaryNetworkError(err):
		return fmt.Sprintf("[MCP_NETWORK_UNAVAILABLE] tsh could not reach Teleport or %s."+
			" Check your network and retry. If other Teleport commands work, check the remote MCP server and the Teleport Application Service logs.", server)
	case isMCPUpstreamServerError(err):
		return fmt.Sprintf("[MCP_UPSTREAM_ERROR] %s or the Teleport Application Service returned an HTTP 5xx error."+
			" Retry once, then check the remote server's health and the Application Service logs.", server)
	case strings.Contains(strings.ToLower(err.Error()), "expected array, received null"):
		return fmt.Sprintf("[MCP_PROTOCOL_ERROR] %s returned null where the MCP client requires an array."+
			" Update tsh to a build containing the empty-tools-array fix and reconnect the MCP client.", server)
	default:
		return fmt.Sprintf("[MCP_REQUEST_FAILED] tsh could not complete the request to %s."+
			" Check the tsh MCP logs for the underlying error. If `tsh status` shows an expired session, run `tsh login`; otherwise check the remote server and Teleport Application Service logs.", server)
	}
}

func makeMCPTimeoutUserMessage(server string) string {
	return fmt.Sprintf("[MCP_CONNECTION_TIMEOUT] The request to %s timed out before a response arrived."+
		" Retry once. If it times out again, check the remote MCP server's health and the Teleport Application Service logs; restarting the MCP client will not fix a repeatedly slow upstream.", server)
}

// makeMCPHTTPServerErrorMessage builds the user message for an HTTP 5xx
// failure, using the error origin reported by the Teleport Application Service
// to attribute the failure instead of guessing.
func makeMCPHTTPServerErrorMessage(server string, httpErr *clientmcp.HTTPServerError) string {
	detail := httpErr.Body
	if detail == "" {
		detail = http.StatusText(httpErr.StatusCode)
	}
	switch httpErr.Origin {
	case mcputils.ErrorOriginAppService:
		return fmt.Sprintf("[MCP_TELEPORT_ERROR] The Teleport Application Service failed to proxy the request to %s (HTTP %d: %s)."+
			" This is a Teleport-side failure; check the Teleport Application Service logs for details.", server, httpErr.StatusCode, detail)
	case mcputils.ErrorOriginUpstreamUnreachable:
		return fmt.Sprintf("[MCP_UPSTREAM_UNREACHABLE] The Teleport Application Service could not reach %s (HTTP %d: %s)."+
			" Check the remote server's health and its network connectivity from the Application Service.", server, httpErr.StatusCode, detail)
	case mcputils.ErrorOriginUpstream:
		return fmt.Sprintf("[MCP_UPSTREAM_ERROR] %s returned HTTP %d: %s."+
			" Retry once, then check the remote server's health.", server, httpErr.StatusCode, detail)
	default:
		// No attribution header: the response came from an older Application
		// Service or an intermediate proxy, so the origin cannot be determined.
		if httpErr.StatusCode == http.StatusGatewayTimeout {
			return makeMCPTimeoutUserMessage(server)
		}
		return fmt.Sprintf("[MCP_UPSTREAM_ERROR] %s or the Teleport Application Service returned HTTP %d: %s."+
			" Retry once, then check the remote server's health and the Application Service logs.", server, httpErr.StatusCode, detail)
	}
}

func isMCPProtectedResourceMismatch(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "protected resource") &&
		strings.Contains(message, "does not match expected")
}

func isMCPUpstreamServerError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "internal server error") ||
		strings.Contains(message, "status 500") ||
		strings.Contains(message, "status 502") ||
		strings.Contains(message, "status 503") ||
		strings.Contains(message, "status 504")
}

func isMCPHTTPTimeout(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "gateway timeout") ||
		strings.Contains(message, "status 504")
}

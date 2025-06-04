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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

func newMCPListCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpListCommand {
	cmd := &mcpListCommand{
		CmdClause: parent.Command("ls", "List available MCP server applications"),
		cf:        cf,
	}

	cmd.Flag("verbose", "Show extra MCP server fields.").Short('v').BoolVar(&cf.Verbose)
	cmd.Flag("search", searchHelp).StringVar(&cf.SearchKeywords)
	cmd.Flag("query", queryHelp).StringVar(&cf.PredicateExpression)
	cmd.Arg("labels", labelHelp).StringVar(&cf.Labels)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&cf.Format, defaults.DefaultFormats...)
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

func printMCPServersInText(w io.Writer, mcpServers iter.Seq[mcpServerWithDetails]) error {
	var rows [][]string
	for mcpServer := range mcpServers {
		rows = append(rows, []string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), false),
		})
	}
	t := asciitable.MakeTableWithTruncatedColumn([]string{"Name", "Description", "Type", "Labels"}, rows, "Labels")
	_, err := fmt.Fprintln(w, t.AsBuffer().String())
	return trace.Wrap(err)
}

func printMCPServersInVerboseText(w io.Writer, mcpServers iter.Seq[mcpServerWithDetails]) error {
	t := asciitable.MakeTable([]string{"Name", "Description", "Type", "Labels", "Command", "Args", "Allowed Tools"})
	for mcpServer := range mcpServers {
		mcpSpec := cmp.Or(mcpServer.GetMCP(), &types.MCP{})
		t.AddRow([]string{
			mcpServer.GetName(),
			mcpServer.GetDescription(),
			types.GetMCPServerTransportType(mcpServer.GetURI()),
			common.FormatLabels(mcpServer.GetAllLabels(), true),
			mcpSpec.Command,
			strings.Join(mcpSpec.Args, " "),
			common.FormatAllowedEntities(mcpServer.Permissions.MCP.Tools.Allowed, mcpServer.Permissions.MCP.Tools.Denied),
		})
	}
	_, err := fmt.Fprintln(w, t.AsBuffer().String())
	return trace.Wrap(err)
}

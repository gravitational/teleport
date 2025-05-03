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
	"fmt"
	"io"
	"net"
	"os"
	"sort"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/tool/common"
)

type mcpCommands struct {
	list    *mcpListCommand
	connect *mcpConnectCommand
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) mcpCommands {
	mcp := app.Command("mcp", "View and control available MCP servers")
	return mcpCommands{
		list:    newMCPListCommand(mcp, cf),
		connect: newMCPConnectCommand(mcp, cf),
	}
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

type mcpListCommand struct {
	*kingpin.CmdClause
}

func (c *mcpListCommand) run(cf *CLIConf) error {
	mcpServers, err := c.getMCPServers(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.print(cf, mcpServers))
}

func (c *mcpListCommand) getMCPServers(cf *CLIConf) ([]types.Application, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter by MCP schema.
	filter := tc.ResourceFilter(types.KindAppServer)
	filter.PredicateExpression = makePredicateConjunction(
		filter.PredicateExpression,
		"hasPrefix(resource.spec.uri, \"mcp+\")",
	)

	var mcpServers []types.Application
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
	for _, mcpServer := range mcpServers {
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
	// Avoid printing to stdout from onAppLogin.
	// TODO(greedy52) refactor onAppLogin.
	cf.OverrideStdout = io.Discard
	err := onAppLogin(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := loadAppCertificate(tc, cf.AppName)
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

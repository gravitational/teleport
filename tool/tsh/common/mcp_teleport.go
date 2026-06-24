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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	mcpconfig "github.com/gravitational/teleport/lib/client/mcp/config"
	libmcp "github.com/gravitational/teleport/lib/mcp"
	"github.com/gravitational/teleport/lib/utils"
)

func newMCPTeleportInstallCmd(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return parent.Command("install", "Install Teleport MCP server to the local Claude Desktop config")
}

func newMCPTeleportRunCmd(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return parent.Command("run", "Run local Teleport MCP server")
}

func newMCPTeleportUninstallCmd(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return parent.Command("uninstall", "Uninstall Teleport MCP server from the local Claude Desktop config")
}

type mcpTeleportInstaller struct {
}

func (i *mcpTeleportInstaller) printInstructions(io.Writer, mcpconfig.ConfigFormat) error {
	return trace.NotImplemented("not implemented")
}

func (i *mcpTeleportInstaller) updateConfig(w io.Writer, config *mcpconfig.FileConfig) error {
	if _, ok := config.GetMCPServers()["teleport"]; ok {
		return trace.AlreadyExists("❌ Teleport MCP server is already installed")
	}
	self, err := os.Executable()
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	err = config.PutMCPServer("teleport", mcpconfig.MCPServer{
		Command: self,
		Args:    []string{"mcp", "teleport", "run"},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = config.Save("")
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("✅ Successfully installed Teleport MCP server!")
	return nil
}

func onMCPTeleportInstall(cf *CLIConf) error {
	return runMCPConfig(cf, &mcpClientConfigFlags{
		clientConfig: mcpClientConfigClaude,
	}, &mcpTeleportInstaller{})
}

type mcpTeleportUninstaller struct {
}

func (i *mcpTeleportUninstaller) printInstructions(io.Writer, mcpconfig.ConfigFormat) error {
	return trace.NotImplemented("not implemented")
}

func (i *mcpTeleportUninstaller) updateConfig(w io.Writer, config *mcpconfig.FileConfig) error {
	if _, ok := config.GetMCPServers()["teleport"]; !ok {
		return trace.NotFound("❌ Teleport MCP server is not installed")
	}
	err := config.RemoveMCPServer("teleport")
	if err != nil {
		return trace.Wrap(err)
	}
	err = config.Save("")
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("✅ Successfully uninstalled Teleport MCP server!")
	return nil
}

func onMCPTeleportUninstall(cf *CLIConf) error {
	return runMCPConfig(cf, &mcpClientConfigFlags{
		clientConfig: mcpClientConfigClaude,
	}, &mcpTeleportUninstaller{})
}

func onMCPTeleportRun(cf *CLIConf) error {
	log, err := initLogger(cf, utils.LoggingForMCP, getLoggingOptsForMCPServer(cf))
	if err != nil {
		return trace.Wrap(err)
	}

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

	mcpServer, err := libmcp.NewMCPServer(libmcp.Config{
		Auth:         clusterClient.AuthClient,
		WebProxyAddr: tc.WebProxyAddr,
		Log:          log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return mcpServer.ListenStdio(cf.Context, cf.Stdin(), cf.Stdout())
}

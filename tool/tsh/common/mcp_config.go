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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
)

type claudeDesktopConfigMCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}
type claudeDesktopConfig struct {
	MCPServers map[string]claudeDesktopConfigMCPServer `json:"mcpServers"`
}

func claudeDesktopConfigPath() (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(userHome, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
}

func openClaudeDesktopConfig(cf *CLIConf) (*claudeDesktopConfig, map[string]any, error) {
	configPath, err := claudeDesktopConfigPath()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Opening Claude Desktop config at %q\n\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	//TODO quick hack to read everything to avoid removing non-mcpServers settings.
	all := make(map[string]any)
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var config claudeDesktopConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	logger.DebugContext(cf.Context, "Claude Desktop config", "config", config)
	return &config, all, nil
}

func onMCPConfig(cf *CLIConf) error {
	config, _, err := openClaudeDesktopConfig(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	var rows []mcpConfigTableRow
	for localName, mcpServer := range config.MCPServers {
		if mcpServer.Command != cf.executablePath {
			continue
		}
		if len(mcpServer.Args) < 2 {
			continue
		}
		if mcpServer.Args[0] != "mcp" {
			continue
		}
		switch mcpServer.Args[1] {
		case "start":
			if len(mcpServer.Args) < 3 {
				return trace.BadParameter("missing args for entry %q: %q", mcpServer, mcpServer.Args[1])
			}
			rows = append(rows, mcpConfigTableRow{
				Type:            "Remote MCP",
				ResourceName:    mcpServer.Args[2],
				LocalServerName: localName,
				Args:            strings.Join(mcpServer.Args[3:], " "),
			})
		case "start-db":
			if len(mcpServer.Args) < 3 {
				return trace.BadParameter("missing args for entry %q: %q", mcpServer, mcpServer.Args[1])
			}
			rows = append(rows, mcpConfigTableRow{
				Type:            "Teleport Databases",
				LocalServerName: localName,
				Args:            strings.Join(mcpServer.Args[3:], " "),
			})
		case "start-teleport":
			rows = append(rows, mcpConfigTableRow{
				Type:            "Teleport Tools",
				LocalServerName: localName,
			})
		}
	}

	columns := makeTableColumnTitles(mcpConfigTableRow{})
	printRows := makeTableRows(rows)
	t := asciitable.MakeTable(columns, printRows...)
	fmt.Fprintln(cf.Stdout(), t.AsBuffer().String())
	return nil
}

func onMCPConfigUpdate(cf *CLIConf) error {
	cf.OverrideStdout = io.Discard
	if err := onAppLogin(cf); err != nil {
		return trace.Wrap(err)
	}
	cf.OverrideStdout = nil

	config, all, err := openClaudeDesktopConfig(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]claudeDesktopConfigMCPServer)
	}

	localName := "teleport-" + cf.AppName
	config.MCPServers[localName] = claudeDesktopConfigMCPServer{
		Command: cf.executablePath,
		Args:    []string{"mcp", "start", cf.AppName},
	}

	all["mcpServers"] = config.MCPServers
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	configPath, err := claudeDesktopConfigPath()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Saving Claude Desktop config with entry %v\n", config.MCPServers[localName].Args)
	if err := os.WriteFile(configPath, data, teleport.FileMaskOwnerOnly); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func onMCPConfigUpdateDB(cf *CLIConf) error {
	config, all, err := openClaudeDesktopConfig(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]claudeDesktopConfigMCPServer)
	}

	localName := "teleport-databases"
	args := []string{"mcp", "start-db", "--query", cf.PredicateExpression}

	config.MCPServers[localName] = claudeDesktopConfigMCPServer{
		Command: cf.executablePath,
		Args:    args,
	}

	all["mcpServers"] = config.MCPServers
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	configPath, err := claudeDesktopConfigPath()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Saving Claude Desktop config with entry %v\n", config.MCPServers[localName].Args)
	if err := os.WriteFile(configPath, data, teleport.FileMaskOwnerOnly); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

type mcpConfigTableRow struct {
	Type            string
	ResourceName    string `title:"Resource Name"`
	LocalServerName string `title:"Local Server Name"`
	Args            string `title:"Extra Args"`
}

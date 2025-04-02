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
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
)

type claudeDesktopConfigMCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}
type claudeDesktopConfig struct {
	MCPServers map[string]claudeDesktopConfigMCPServer `json:"mcpServers"`
}

func onMCPConfig(cf *CLIConf) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(userHome, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	fmt.Fprintf(cf.Stdout(), "Opening Claude Desktop config at %q\n\n", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var config claudeDesktopConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(cf.Context, "==== ", "config", config)
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
			// TODO Better way?
			var resourceName string
			for _, arg := range mcpServer.Args[3:] {
				if strings.HasPrefix(arg, "-") {
					continue
				}
				resourceName = arg
			}
			rows = append(rows, mcpConfigTableRow{
				Type:            "Database",
				ResourceName:    resourceName,
				LocalServerName: localName,
				Args:            strings.Join(mcpServer.Args[3:], " "),
			})
		}
	}

	columns := makeTableColumnTitles(mcpConfigTableRow{})
	printRows := makeTableRows(rows)
	t := asciitable.MakeTable(columns, printRows...)
	fmt.Fprintln(cf.Stdout(), t.AsBuffer().String())
	return nil
}

type mcpConfigTableRow struct {
	Type            string
	ResourceName    string `title:"Resource Name"`
	LocalServerName string `title:"Local Server Name"`
	Args            string `title:"Extra Args"`
}

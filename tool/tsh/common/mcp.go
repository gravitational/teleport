// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
)

type mcpCommands struct {
	dbStart  *mcpDBStartCommand
	dbConfig *mcpDBConfigCommand

	config  *mcpConfigCommand
	list    *mcpListCommand
	connect *mcpConnectCommand

	platformStart *mcpPlatformStartCommand
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) *mcpCommands {
	mcp := app.Command("mcp", "View and control proxied MCP servers.")
	db := mcp.Command("db", "Database access for MCP servers.")
	platform := mcp.Command("platform", "MCP servers for Teleport APIs")
	return &mcpCommands{
		dbStart:  newMCPDBCommand(db, cf),
		dbConfig: newMCPDBconfigCommand(db, cf),

		list:    newMCPListCommand(mcp, cf),
		config:  newMCPConfigCommand(mcp, cf),
		connect: newMCPConnectCommand(mcp, cf),

		platformStart: newMCPPlatformStartCommand(platform, cf),
	}
}

type mcpClientConfigFlags struct {
	clientConfig string
	jsonFormat   string
}

const (
	mcpClientConfigClaude = "claude"
)

func (m *mcpClientConfigFlags) addToCmd(cmd *kingpin.CmdClause) {
	cmd.Flag(
		"client-config",
		fmt.Sprintf(
			"If specified, update the specified client config. %q for default Claude Desktop config, or specify a JSON file path. Can also be set with environment variable %s.",
			mcpClientConfigClaude,
			mcpClientConfigEnvVar,
		)).
		Envar(mcpClientConfigEnvVar).
		StringVar(&m.clientConfig)

	cmd.Flag(
		"json-format",
		fmt.Sprintf(
			"Format the JSON file (%s). auto saves in compact if the file is already compact, otherwise pretty. Can also be set with environment variable %s. Default is %s.",
			strings.Join(m.jsonFormatOptions(), ", "),
			mcpConfigJSONFormatEnvVar,
			claude.FormatJSONAuto,
		)).
		Envar(mcpConfigJSONFormatEnvVar).
		Default(string(claude.FormatJSONAuto)).
		EnumVar(&m.jsonFormat, m.jsonFormatOptions()...)
}

func (m *mcpClientConfigFlags) isSet() bool {
	return m.clientConfig != ""
}

func (m *mcpClientConfigFlags) loadConfig() (*claude.FileConfig, error) {
	switch m.clientConfig {
	case mcpClientConfigClaude:
		return claude.LoadConfigFromDefaultPath()
	default:
		return claude.LoadConfigFromFile(m.clientConfig)
	}
}

func (m *mcpClientConfigFlags) jsonFormatOptions() []string {
	return []string{
		string(claude.FormatJSONPretty),
		string(claude.FormatJSONCompact),
		string(claude.FormatJSONAuto),
		string(claude.FormatJSONNone),
	}
}

func (m *mcpClientConfigFlags) printHint(w io.Writer) error {
	_, err := fmt.Fprintln(w, `Tip: use --client-config=claude to update your Claude Desktop configuration.
You can also specify a custom config path with --client-config=<path> to update
a config file compatible with the "mcpServer" mapping.`)
	return trace.Wrap(err)
}

// claudeConfig defines a subset of functions from claude.Config.
type claudeConfig interface {
	PutMCPServer(string, claude.MCPServer) error
	GetMCPServers() map[string]claude.MCPServer
}

func makeLocalMCPServer(cf *CLIConf, args []string) claude.MCPServer {
	s := claude.MCPServer{
		Command: cf.executablePath,
		Args:    args,
	}

	// Use the same TELEPORT_HOME the current tsh uses.
	if homeDir := os.Getenv(types.HomeEnvVar); homeDir != "" {
		s.AddEnv(types.HomeEnvVar, homeDir)
	}

	// Disable debug through env var. MCP server commands should enable debug by
	// default.
	opts := getLoggingOptsForMCPServer(cf)
	if !opts.debug {
		s.AddEnv(debugEnvVar, "false")
	}
	if opts.osLog {
		s.AddEnv(osLogEnvVar, "true")
	}

	// TODO(greedy52) anything else? maybe cluster, login-related, etc?
	return s
}

func getLoggingOptsForMCPServer(cf *CLIConf) loggingOpts {
	return getLoggingOptsWithDefault(cf, loggingOpts{
		debug: true,
	})
}

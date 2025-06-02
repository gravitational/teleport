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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

type mcpCommands struct {
	dbStart *mcpDBStartCommand

	login  *mcpLoginCommand
	logout *mcpLogoutCommand
	list   *mcpListCommand
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) *mcpCommands {
	mcp := app.Command("mcp", "View and control proxied MCP servers.")
	db := mcp.Command("db", "Database access for MCP servers.")
	return &mcpCommands{
		dbStart: newMCPDBCommand(db),

		list:   newMCPListCommand(mcp, cf),
		login:  newMCPLoginCommand(mcp, cf),
		logout: newMCPLogoutCommand(mcp, cf),
	}
}

type mcpConfigFileFlags struct {
	claude     bool
	jsonFile   string
	jsonFormat string
}

func (m *mcpConfigFileFlags) addToCmd(cmd *kingpin.CmdClause) {
	cmd.Flag(
		"claude",
		fmt.Sprintf("Updates claude_desktop_config.json from default Claude Desktop location. Can also be set with environment variable %s=true. Mutually exclusive with --json-file.",
			mcpConfigClaudeEnvVar,
		)).
		Envar(mcpConfigClaudeEnvVar).
		BoolVar(&m.claude)

	cmd.Flag(
		"json-file",
		fmt.Sprintf(
			"Updates \"mcpServer\" mapping in the provided json file. Can also be set with environment variable %s. Mutually exclusive with --claude.",
			mcpConfigJSONFileEnvVar,
		)).
		Envar(mcpConfigJSONFileEnvVar).
		StringVar(&m.jsonFile)

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

// isSet checks if any of the config file option set. Commands like "login" may
// allow the case where none of the file options is set. Note that check() will
// return an error if nothing is set.
func (m *mcpConfigFileFlags) isSet() bool {
	return m.claude || m.jsonFile != ""
}

func (m *mcpConfigFileFlags) checkIfSet() error {
	if m.isSet() {
		return trace.Wrap(m.check())
	}
	return nil
}

func (m *mcpConfigFileFlags) check() error {
	switch {
	case m.claude && m.jsonFile != "":
		return trace.BadParameter("only one of --claude and --json-file can be specified")
	case !m.claude && m.jsonFile == "":
		return trace.BadParameter("one of --claude and --json-file must be specified")
	default:
		return nil
	}
}

func (m *mcpConfigFileFlags) loadConfig() (*claude.FileConfig, error) {
	if m.claude {
		return claude.LoadConfigFromDefaultPath()
	}
	// For now assume all json configs use the same claude format.
	return claude.LoadConfigFromFile(m.jsonFile)
}

func (m *mcpConfigFileFlags) jsonFormatOptions() []string {
	return []string{
		string(claude.FormatJSONPretty),
		string(claude.FormatJSONCompact),
		string(claude.FormatJSONAuto),
		string(claude.FormatJSONNone),
	}
}

func (m *mcpConfigFileFlags) printHint(w io.Writer) error {
	_, err := fmt.Fprintf(w, `Use the --claude flag to automatically update your Claude Desktop
configuration. You can also set the environment variable
%s=true to achieve the same.

Similarly, use the --json-file <path> flag for any client configuration file
that supports the "mcpServer" mapping. You can also set the environment variable
%s=<path> to achieve the same.
`, mcpConfigClaudeEnvVar, mcpConfigJSONFileEnvVar)
	return trace.Wrap(err)
}

// claudeConfig defines a subset of functions from claude.Config.
type claudeConfig interface {
	PutMCPServer(string, claude.MCPServer) error
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

	// Enable debug through env var if debug is on.
	debugByEnvVar, _ := strconv.ParseBool(os.Getenv(debugEnvVar))
	if cf.Debug || debugByEnvVar {
		s.AddEnv(debugEnvVar, "true")
	}

	// TODO(greedy52) anything else? maybe cluster, login-related, etc?
	return s
}

func isLocalMCPServerFromTeleport(cf *CLIConf, localName string, server claude.MCPServer, nameCheck func(string) bool, startWithArgs []string) bool {
	if !nameCheck(localName) {
		return false
	}

	// Double check binary path.
	if cf.executablePath != server.Command {
		return false
	}

	// Check args.
	if !sliceutils.StartsWith(server.Args, startWithArgs) {
		return false
	}

	// Compare home path.
	var serverHomePath string
	if value, ok := server.GetEnv(types.HomeEnvVar); ok {
		serverHomePath = filepath.Clean(value)
	}
	return cf.HomePath == serverHomePath
}

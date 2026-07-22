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
	mcpconfig "github.com/gravitational/teleport/lib/client/mcp/config"
)

type mcpCommands struct {
	dbStart  *mcpDBStartCommand
	dbConfig *mcpDBConfigCommand

	config  *mcpConfigCommand
	list    *mcpListCommand
	connect *mcpConnectCommand
}

func newMCPCommands(app *kingpin.Application, cf *CLIConf) *mcpCommands {
	mcp := app.Command("mcp", "View and control proxied MCP servers.")
	db := mcp.Command("db", "Database access for MCP servers.")
	return &mcpCommands{
		dbStart:  newMCPDBCommand(db, cf),
		dbConfig: newMCPDBconfigCommand(db, cf),

		list:    newMCPListCommand(mcp, cf),
		config:  newMCPConfigCommand(mcp, cf),
		connect: newMCPConnectCommand(mcp, cf),
	}
}

type mcpClientConfigFlags struct {
	clientConfig          string
	jsonFormat            string
	configFormat          string
	configFormatSetByUser bool
}

const (
	mcpClientConfigClaude = "claude"
	mcpClientConfigCursor = "cursor"
)

// cursorConfigFormatAlias is an alias for Cursor config format.
const cursorConfigFormatAlias = "cursor"

func (m *mcpClientConfigFlags) addToCmd(cmd *kingpin.CmdClause) {
	cmd.Flag(
		"client-config",
		fmt.Sprintf(
			"If specified, update the specified client config, assuming its format. %q for default Claude Desktop config, %q for global Cursor MCP servers config, or specify a JSON file path. Can also be set with environment variable %s.",
			mcpClientConfigClaude,
			mcpClientConfigCursor,
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
			mcpconfig.FormatJSONAuto,
		)).
		Envar(mcpConfigJSONFormatEnvVar).
		Default(string(mcpconfig.FormatJSONAuto)).
		EnumVar(&m.jsonFormat, m.jsonFormatOptions()...)

	cmd.Flag(
		"format",
		fmt.Sprintf(
			"Format specifies the configuration format (%s, %s, %s). If not provided it will assume format from the configuration file, When no configuration file is provided it defaults to %q.",
			string(mcpconfig.ConfigFormatClaude),
			string(mcpconfig.ConfigFormatVSCode),
			cursorConfigFormatAlias,
			string(mcpconfig.DefaultConfigFormat),
		)).
		IsSetByUser(&m.configFormatSetByUser).
		StringVar(&m.configFormat)
}

func (m *mcpClientConfigFlags) loadConfig(configFormat mcpconfig.ConfigFormat) (*mcpconfig.FileConfig, error) {
	switch m.clientConfig {
	case mcpClientConfigClaude:
		return mcpconfig.LoadClaudeConfigFromDefaultPath()
	case mcpClientConfigCursor:
		return mcpconfig.LoadConfigFromGlobalCursor()
	default:
		return mcpconfig.LoadConfigFromFile(m.clientConfig, configFormat)
	}
}

func (m *mcpClientConfigFlags) jsonFormatOptions() []string {
	return []string{
		string(mcpconfig.FormatJSONPretty),
		string(mcpconfig.FormatJSONCompact),
		string(mcpconfig.FormatJSONAuto),
		string(mcpconfig.FormatJSONNone),
	}
}

func (m *mcpClientConfigFlags) printFooterNotes(w io.Writer) error {
	if m.configFormatSetByUser {
		return nil
	}

	_, err := fmt.Fprintln(w, mcpConfigHint)
	return trace.Wrap(err)
}

func (m *mcpClientConfigFlags) format() (mcpconfig.ConfigFormat, error) {
	var (
		configFormat mcpconfig.ConfigFormat
		flagFormat   mcpconfig.ConfigFormat
	)

	switch m.clientConfig {
	case mcpClientConfigClaude, mcpClientConfigCursor:
		configFormat = mcpconfig.ConfigFormatClaude
	case "":
	default:
		configFormat = mcpconfig.ConfigFormatFromPath(m.clientConfig)
	}

	if m.configFormat != "" {
		// Solve alias.
		if m.configFormat == cursorConfigFormatAlias {
			m.configFormat = string(mcpconfig.ConfigFormatClaude)
		}

		var err error
		flagFormat, err = mcpconfig.ParseConfigFormat(m.configFormat)
		if err != nil {
			return mcpconfig.ConfigFormatUnspecified, trace.Wrap(err)
		}
	}

	// If both the format and config file path are presented, we must ensure
	// both have the same format.
	if configFormat.IsSpecified() && flagFormat.IsSpecified() {
		if configFormat == flagFormat {
			return configFormat, nil
		}

		return mcpconfig.ConfigFormatUnspecified, trace.BadParameter(
			"Configuration format mismatch. --client-config=%s option uses %s config format, but --format=%s was set. You can drop one of the flags or adjust the values to match.",
			m.clientConfig,
			configFormat,
			m.configFormat,
		)
	}

	if configFormat.IsSpecified() {
		return configFormat, nil
	}

	if flagFormat.IsSpecified() {
		return flagFormat, nil
	}

	// In case of unspecified format, use default value.
	return mcpconfig.DefaultConfigFormat, nil
}

// runMCPConfig runs the MCP config based on flags.
func runMCPConfig(cf *CLIConf, flags *mcpClientConfigFlags, exec mcpConfigExec) error {
	// Ensure the format options are correct, otherwise return error to the
	// user instead of ignoring the values.
	format, err := flags.format()
	if err != nil {
		return trace.Wrap(err)
	}

	switch flags.clientConfig {
	case "":
		return trace.Wrap(exec.printInstructions(cf.Stdout(), format))
	default:
		config, err := flags.loadConfig(format)
		if err != nil {
			return trace.BadParameter("failed to load mcp configuration file at %q: %v", flags.clientConfig, err)
		}
		return trace.Wrap(exec.updateConfig(cf.Stdout(), config))
	}
}

// mcpConfig defines a subset of functions from mcpconfig.Config.
type mcpConfig interface {
	PutMCPServer(string, mcpconfig.MCPServer) error
	GetMCPServers() map[string]mcpconfig.MCPServer
}

// mcpConfigExec defines the interface for generating install instructions and
// directly updating the MCP config.
type mcpConfigExec interface {
	// printInstructions prints instructions on how to configure the MCP server.
	printInstructions(io.Writer, mcpconfig.ConfigFormat) error
	// updateConfig directly updates the client config. It might also print
	// information.
	updateConfig(io.Writer, *mcpconfig.FileConfig) error
}

func makeLocalMCPServer(cf *CLIConf, args []string) mcpconfig.MCPServer {
	s := mcpconfig.MCPServer{
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

// mcpConfigHint is the hint message displayed when the configuration is shown
// to users.
const mcpConfigHint = `Tip: You can use this command to update your MCP servers configuration file automatically.
- For Claude Desktop, use --client-config=claude to update the default configuration.
- For Cursor, use --client-config=cursor to update the global MCP servers configuration.
In addition, you can use --client-config=<path> to specify a config file location to directly update configuration of the supported clients.
For example, you can update a VSCode project using --client-config=<path-to-project>/.vscode/mcp.json`

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

package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
)

// claudeServersKey defines the JSON key used by Claude to store MCP servers
// config.
const claudeServersKey = "mcpServers"

// DefaultClaudeConfigPath returns the default path for the Claude Desktop config.
//
// https://modelcontextprotocol.io/quickstart/user
//
// macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
// Windows: %APPDATA%\Claude\claude_desktop_config.json
func DefaultClaudeConfigPath() (string, error) {
	switch runtime.GOOS {
	case "darwin", "windows":
		// os.UserConfigDir:
		// On Darwin, it returns $HOME/Library/Application Support.
		// On Windows, it returns %AppData%.
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
		return filepath.Join(configDir, "Claude", "claude_desktop_config.json"), nil

	default:
		// TODO(greedy52) there is no official Claude Desktop for linux yet. The
		// unofficial one uses the same path as above.
		return "", trace.NotImplemented("Claude Desktop is not supported on OS %s", runtime.GOOS)
	}
}

// LoadClaudeConfigFromDefaultPath loads the Claude Desktop's config from the
// default path.
func LoadClaudeConfigFromDefaultPath() (*FileConfig, error) {
	configPath, err := DefaultClaudeConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "finding Claude Desktop config path")
	}
	config, err := LoadConfigFromFile(configPath, ConfigFormatClaude)
	return config, trace.Wrap(err)
}

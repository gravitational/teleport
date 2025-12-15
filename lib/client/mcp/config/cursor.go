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

	"github.com/gravitational/trace"
)

const (
	// cursorProjectDir defines the Cursor project directory, where the MCP
	// configuration is located.
	//
	// https://docs.cursor.com/en/context/mcp#configuration-locations
	cursorProjectDir = ".cursor"
)

// GlobalCursorPath returns the default path for Cursor global MCP configuration.
//
// https://docs.cursor.com/context/mcp#configuration-locations
func GlobalCursorPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return filepath.Join(homeDir, ".cursor", "mcp.json"), nil
}

// LoadConfigFromGlobalCursor loads the Cursor global MCP server configuration.
func LoadConfigFromGlobalCursor() (*FileConfig, error) {
	configPath, err := GlobalCursorPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := LoadConfigFromFile(configPath, ConfigFormatClaude)
	return config, trace.Wrap(err)
}

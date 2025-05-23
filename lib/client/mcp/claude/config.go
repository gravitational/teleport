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

package claude

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
)

// Config represents the Claude Desktop config.
type Config struct {
	// MCPServers specifies a list of MCP servers.
	MCPServers map[string]MCPServer `json:"mcpServers,omitempty"`
	// AllFields preserves all fields of the config.
	AllFields map[string]any `json:"-"`
}

// AddMCPServer adds a new MCP server to the config.
func (c *Config) AddMCPServer(name string, mcpServer MCPServer) {
	if c.MCPServers == nil {
		c.MCPServers = make(map[string]MCPServer)
	}
	c.MCPServers[name] = mcpServer
}

// MarshalJSON implements json.Marshaler.
func (c Config) MarshalJSON() ([]byte, error) {
	// Shallow copy.
	c.AllFields = maps.Clone(c.AllFields)
	c.updateFields()
	data, err := json.Marshal(c.AllFields)
	return data, trace.Wrap(err)
}

func (c *Config) updateFields() {
	if c.AllFields == nil {
		c.AllFields = make(map[string]any)
	}
	if len(c.MCPServers) != 0 {
		c.AllFields["mcpServers"] = c.MCPServers
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Config) UnmarshalJSON(data []byte) error {
	type Alias Config
	if err := json.Unmarshal(data, (*Alias)(c)); err != nil {
		return trace.Wrap(err)
	}
	var allFields map[string]any
	if err := json.Unmarshal(data, &allFields); err != nil {
		return trace.Wrap(err)
	}
	c.AllFields = allFields
	c.updateFields()
	return nil
}

// MCPServer contains details to launch a MCP server.
type MCPServer struct {
	// Command specifies the command to execute.
	Command string `json:"command"`
	// Args specifies the arguments for the command.
	Args []string `json:"args,omitempty"`
	// Envs specifies extra environment variable.
	Envs map[string]string `json:"env,omitempty"`
	// AllFields preserves all fields of the MCPServer.
	AllFields map[string]any `json:"-"`
}

// MarshalJSON implements json.Marshaler.
func (s MCPServer) MarshalJSON() ([]byte, error) {
	// Shallow copy.
	s.AllFields = maps.Clone(s.AllFields)
	s.updateFields()
	data, err := json.Marshal(s.AllFields)
	return data, trace.Wrap(err)
}

func (s *MCPServer) updateFields() {
	if s.AllFields == nil {
		s.AllFields = make(map[string]any)
	}
	s.AllFields["command"] = s.Command
	if len(s.Args) > 0 {
		s.AllFields["args"] = s.Args
	}
	if len(s.Envs) > 0 {
		s.AllFields["env"] = s.Envs
	}
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *MCPServer) UnmarshalJSON(data []byte) error {
	type Alias MCPServer
	if err := json.Unmarshal(data, (*Alias)(s)); err != nil {
		return trace.Wrap(err)
	}

	var allFields map[string]any
	if err := json.Unmarshal(data, &allFields); err != nil {
		return trace.Wrap(err)
	}
	s.AllFields = allFields
	s.updateFields()
	return nil
}

// LoadConfigFromDefaultPath loads the Claude Desktop's config from the default
// path.
func LoadConfigFromDefaultPath() (*Config, error) {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "finding Claude Desktop config path")
	}
	config, err := LoadConfig(configPath)
	return config, trace.Wrap(err)
}

// LoadConfig loads the Claude Desktop's config from the provided path.
func LoadConfig(configPath string) (*Config, error) {
	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading Claude Desktop config")
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, trace.Wrap(err, "parsing Claude Desktop config")
	}
	return &config, nil
}

// SaveConfigToDefaultPath saves the Claude Desk config to the default path.
func SaveConfigToDefaultPath(config *Config) error {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return trace.Wrap(err, "finding Claude Desktop config path")
	}

	return trace.Wrap(SaveConfig(config, configPath))
}

// SaveConfig saves the Claude Desktop config to specified path.
func SaveConfig(config *Config, configPath string) error {
	if config == nil {
		return trace.BadParameter("config is nil")
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return trace.Wrap(err, "marshalling Claude Desktop config")
	}
	// Claude Desktop creates the config with 0644.
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return trace.Wrap(err, "opening Claude Desktop config")
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return trace.Wrap(err, "writing Claude Desktop config")
	}
	return nil
}

// DefaultConfigPath returns the default path for the Claude Desktop config.
// https://modelcontextprotocol.io/quickstart/user
//
// Windows: %APPDATA%\Claude\claude_desktop_config.json
func DefaultConfigPath() (string, error) {
	switch runtime.GOOS {
	// macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
	case "darwin":
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
		return filepath.Join(userHome, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil

	// windows: %APPDATA%\Claude\claude_desktop_config.json
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", trace.BadParameter("APPDATA environment variable not found")
		}

		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil

	default:
		return "", trace.NotImplemented("Claude Desktop is not supported on OS %s", runtime.GOOS)
	}
}

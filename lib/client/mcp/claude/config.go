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
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

type Config struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
	AllFields  map[string]any       `json:"-"`
}

func (c *Config) AddMCPServer(name string, mcpServer MCPServer) {
	if c.MCPServers == nil {
		c.MCPServers = make(map[string]MCPServer)
	}
	c.MCPServers[name] = mcpServer
}

func (c *Config) MarshalJSON() ([]byte, error) {
	c.updateFields()
	data, err := json.Marshal(c.AllFields)
	return data, trace.Wrap(err)
}

func (c *Config) updateFields() {
	if c.AllFields == nil {
		c.AllFields = make(map[string]any)
	}
	c.AllFields["mcpServers"] = c.MCPServers
}

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

type MCPServer struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Envs      map[string]string `json:"env,omitempty"`
	AllFields map[string]any    `json:"-"`
}

func (s *MCPServer) MarshalJSON() ([]byte, error) {
	s.updateFields()
	data, err := json.Marshal(s.AllFields)
	return data, trace.Wrap(err)
}

func (s *MCPServer) updateFields() {
	if s.AllFields == nil {
		s.AllFields = make(map[string]any)
	}
	s.AllFields["command"] = s.Command
	s.AllFields["args"] = s.Args
	s.AllFields["env"] = s.Envs
}

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

func UpdateConfigWithMCPServers(ctx context.Context, mcpServers map[string]MCPServer) error {
	config, err := LoadConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	// Overwrite existing MCP servers with the new ones.
	for name, mcpServer := range mcpServers {
		config.AddMCPServer(name, mcpServer)
	}
	return trace.Wrap(SaveConfig(ctx, config))
}

func ConfigExists() (bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return false, trace.Wrap(err)
	}
	return utils.FileExists(path), nil
}

func LoadConfig() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "finding Claude Desktop config path")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading Claude Desktop config")
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, trace.Wrap(err, "parsing Claude Desktop config")
	}
	return &config, nil
}

func SaveConfig(ctx context.Context, config *Config) error {
	configPath, err := ConfigPath()
	if err != nil {
		return trace.Wrap(err, "finding Claude Desktop config path")
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return trace.Wrap(err, "marshalling Claude Desktop config")
	}
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return trace.Wrap(err, "opening Claude Desktop config")
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return trace.Wrap(err, "writing Claude Desktop config")
	}
	return nil
}

func ConfigPath() (string, error) {
	if runtime.GOOS == "darwin" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
		return filepath.Join(userHome, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	}
	return "", trace.NotImplemented("Claude Desktop is not supported on %s", runtime.GOOS)
}

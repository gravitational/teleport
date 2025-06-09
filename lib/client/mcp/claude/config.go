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
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

// DefaultConfigPath returns the default path for the Claude Desktop config.
//
// https://modelcontextprotocol.io/quickstart/user
//
// macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
// Windows: %APPDATA%\Claude\claude_desktop_config.json
func DefaultConfigPath() (string, error) {
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

// MCPServer contains details to launch an MCP server.
//
// https://modelcontextprotocol.io/quickstart/user
type MCPServer struct {
	// Command specifies the command to execute.
	Command string `json:"command"`
	// Args specifies the arguments for the command.
	Args []string `json:"args,omitempty"`
	// Envs specifies extra environment variable.
	Envs map[string]string `json:"env,omitempty"`
}

// Config represents a Claude Desktop config.
//
// Config preserves unknown fields and ordering from the original JSON when
// saving, by using the sjson lib.
//
// Config functions are not thread-safe.
type Config struct {
	mcpServers            map[string]MCPServer
	configData            []byte
	isOriginalJSONCompact bool
}

// NewConfig creates an empty config.
func NewConfig() *Config {
	return &Config{
		mcpServers:            make(map[string]MCPServer),
		configData:            []byte("{}"),
		isOriginalJSONCompact: false,
	}
}

// NewConfigFromJSON creates a config from JSON.
func NewConfigFromJSON(data []byte) (*Config, error) {
	config := struct {
		MCPServers map[string]MCPServer `json:"mcpServers"`
	}{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, trace.Wrap(err, "parsing Claude Desktop config")
	}

	if config.MCPServers == nil {
		config.MCPServers = map[string]MCPServer{}
	}
	isOriginalJSONCompact, err := isJSONCompact(data)
	if err != nil {
		return nil, trace.Wrap(err, "parsing Claude Desktop config")
	}

	return &Config{
		mcpServers:            config.MCPServers,
		configData:            data,
		isOriginalJSONCompact: isOriginalJSONCompact,
	}, nil
}

// GetMCPServers returns a shallow copy of the MCP servers.
func (c *Config) GetMCPServers() map[string]MCPServer {
	return maps.Clone(c.mcpServers)
}

// PutMCPServer adds a new MCP server or replace an existing one.
func (c *Config) PutMCPServer(serverName string, server MCPServer) (err error) {
	c.mcpServers[serverName] = server
	c.configData, err = sjson.SetBytes(c.configData, c.mcpServerJSONPath(serverName), server)
	return trace.Wrap(err)
}

// RemoveMCPServer removes an MCP server by name.
func (c *Config) RemoveMCPServer(serverName string) (err error) {
	if _, ok := c.mcpServers[serverName]; !ok {
		return trace.NotFound("mcp server %v not found", serverName)
	}

	delete(c.mcpServers, serverName)
	c.configData, err = sjson.DeleteBytes(c.configData, c.mcpServerJSONPath(serverName))
	return trace.Wrap(err)
}

// FormatJSONOption specifies the option on how to format the JSON output.
type FormatJSONOption string

const (
	// FormatJSONPretty prettifies the JSON output.
	FormatJSONPretty FormatJSONOption = "pretty"
	// FormatJSONCompact minifies the JSON output.
	FormatJSONCompact FormatJSONOption = "compact"
	// FormatJSONNone skips formatting.
	FormatJSONNone FormatJSONOption = "none"
	// FormatJSONAuto minifies the JSON output if the original JSON is already
	// minified. Otherwise, the JSON output is prettified. If the original JSON
	// is "{}", the JSON output is also prettified.
	FormatJSONAuto FormatJSONOption = "auto"
)

// Write writes the config to provided writer.
func (c *Config) Write(w io.Writer, format FormatJSONOption) error {
	data, err := c.formatConfigData(format)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

// FileConfig represents a Config read from a file.
//
// Note that outside changes to the config file after LoadConfigFromFile will be
// ignored when saving.
type FileConfig struct {
	*Config
	configPath   string
	configExists bool
}

// LoadConfigFromFile loads the Claude Desktop's config from the provided path.
func LoadConfigFromFile(configPath string) (*FileConfig, error) {
	data, err := os.ReadFile(configPath)
	switch {
	case os.IsNotExist(err):
		return &FileConfig{
			Config:       NewConfig(),
			configPath:   configPath,
			configExists: false,
		}, nil

	case err != nil:
		return nil, trace.Wrap(trace.ConvertSystemError(err), "reading Claude Desktop config")

	default:
		config, err := NewConfigFromJSON(data)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &FileConfig{
			Config:       config,
			configPath:   configPath,
			configExists: true,
		}, nil
	}
}

// LoadConfigFromDefaultPath loads the Claude Desktop's config from the default
// path.
func LoadConfigFromDefaultPath() (*FileConfig, error) {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "finding Claude Desktop config path")
	}
	config, err := LoadConfigFromFile(configPath)
	return config, trace.Wrap(err)
}

// Exists returns true if config file exists.
func (c *FileConfig) Exists() bool {
	return c.configExists
}

// Save saves the updated config to the config path. Format defaults to "auto"
// if empty.
func (c *FileConfig) Save(format FormatJSONOption) error {
	// Claude Desktop creates the config with 0644.
	file, err := os.OpenFile(c.configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()
	return trace.Wrap(c.Write(file, format))
}

func (c *Config) mcpServerJSONPath(serverName string) string {
	return "mcpServers." + serverName
}

func (c *Config) formatConfigData(format FormatJSONOption) ([]byte, error) {
	return formatJSON(c.configData, format, c.isOriginalJSONCompact)
}

func formatJSON(data []byte, format FormatJSONOption, isOriginalCompact bool) ([]byte, error) {
	switch format {
	case FormatJSONPretty:
		// pretty.Pretty is more human-readable than json.Indent.
		return pretty.Pretty(data), nil
	case FormatJSONCompact:
		return pretty.Ugly(data), nil
	case FormatJSONNone:
		return data, nil
	case FormatJSONAuto, "":
		if isOriginalCompact {
			return pretty.Ugly(data), nil
		}
		return pretty.Pretty(data), nil
	default:
		return nil, trace.BadParameter("invalid JSON format option %q", format)
	}
}

func isJSONCompact(data []byte) (bool, error) {
	data = bytes.TrimSpace(data)

	// Do not treat empty object as compact.
	if bytes.Equal(data, []byte("{}")) {
		return false, nil
	}

	var buf bytes.Buffer
	err := json.Compact(&buf, data)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return bytes.Equal(buf.Bytes(), data), nil
}

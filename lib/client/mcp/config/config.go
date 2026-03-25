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

package config

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

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

// AddEnv adds an environment variable.
func (s *MCPServer) AddEnv(name, value string) {
	if s.Envs == nil {
		s.Envs = map[string]string{}
	}
	s.Envs[name] = value
}

// GetEnv gets the value of an environment variable.
func (s *MCPServer) GetEnv(key string) (string, bool) {
	if s.Envs == nil {
		return "", false
	}
	value, ok := s.Envs[key]
	return value, ok
}

// Config represents a MCP servers config.
//
// Config preserves unknown fields and ordering from the original JSON when
// saving, by using the sjson lib.
//
// Config functions are not thread-safe.
type Config struct {
	mcpServers            map[string]MCPServer
	configData            []byte
	isOriginalJSONCompact bool
	format                ConfigFormat
}

// NewConfig creates an empty config.
func NewConfig(format ConfigFormat) *Config {
	return &Config{
		mcpServers:            make(map[string]MCPServer),
		configData:            []byte("{}"),
		isOriginalJSONCompact: false,
		format:                format,
	}
}

// NewConfigFromJSON creates a config from JSON.
func NewConfigFromJSON(format ConfigFormat, data []byte) (*Config, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, trace.Wrap(err, "parsing config")
	}

	servers := make(map[string]MCPServer)
	if rawServers, ok := config[format.serversKey()]; ok {
		if err := json.Unmarshal(rawServers, &servers); err != nil {
			return nil, trace.Wrap(err, "parsing mcp servers config")
		}
	}

	isOriginalJSONCompact, err := isJSONCompact(data)
	if err != nil {
		return nil, trace.Wrap(err, "parsing mcp servers config")
	}

	return &Config{
		mcpServers:            servers,
		configData:            data,
		isOriginalJSONCompact: isOriginalJSONCompact,
		format:                format,
	}, nil
}

// GetMCPServers returns a shallow copy of the MCP servers.
func (c *Config) GetMCPServers() map[string]MCPServer {
	return maps.Clone(c.mcpServers)
}

// PutMCPServer adds a new MCP server or replaces an existing one.
func (c *Config) PutMCPServer(serverName string, server MCPServer) (err error) {
	c.mcpServers[serverName] = server

	// We require a custom marshal to improve MCP Resources URI readability when
	// it includes query params. By default the encoding/json escapes some
	// characters like `&` causing the final URI to be harder to read.
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(server); err != nil {
		return trace.Wrap(err)
	}
	c.configData, err = sjson.SetRawBytes(c.configData, c.mcpServerJSONPath(serverName), b.Bytes())
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

// ConfigFormat specifies what is the MCP servers configuration format.
type ConfigFormat string

const (
	// ConfigFormatUnspecified represents an unspecified format.
	ConfigFormatUnspecified ConfigFormat = ""
	// ConfigFormatClaude represents the Claude desktop and Claude Code formats.
	ConfigFormatClaude ConfigFormat = "claude"
	// ConfigFormatVSCode represents the VSCode format.
	ConfigFormatVSCode ConfigFormat = "vscode"
)

// DefaultConfigFormat determines the dafault config format. This can be used
// in cases where the format wasn't specified.
const DefaultConfigFormat = ConfigFormatClaude

// ParseConfigFormat parses configuration format from string.
func ParseConfigFormat(s string) (ConfigFormat, error) {
	switch ConfigFormat(s) {
	case ConfigFormatClaude:
		return ConfigFormatClaude, nil
	case ConfigFormatVSCode:
		return ConfigFormatVSCode, nil
	case ConfigFormatUnspecified:
		return ConfigFormatUnspecified, nil
	}

	return ConfigFormatUnspecified, trace.BadParameter("unsupported %q config format", s)
}

// IsSpecified returns whether the config format was specified or not.
func (cf ConfigFormat) IsSpecified() bool {
	return cf != ConfigFormatUnspecified
}

// serversKey returns the MCP servers JSON key for the format.
func (cf ConfigFormat) serversKey() string {
	switch cf {
	case ConfigFormatClaude:
		return claudeServersKey
	case ConfigFormatVSCode:
		return vsCodeServersKey
	default:
		return ""
	}
}

// String returns human readable config format name.
func (cf ConfigFormat) String() string {
	switch cf {
	case ConfigFormatClaude:
		return "Claude/Cursor"
	case ConfigFormatVSCode:
		return "VSCode"
	default:
		return "Unspecified"
	}
}

// ConfigFormatFromPath tries to determine the config format based on its path.
func ConfigFormatFromPath(configPath string) ConfigFormat {
	switch {
	case pathContains(configPath, vsCodeProjectDir):
		return ConfigFormatVSCode
	case pathContains(configPath, cursorProjectDir), pathContains(configPath, claudeCodeFileName):
		// Works for both, global and projects settings.
		return ConfigFormatClaude // Cursor uses the same format as Claude.
	default:
		return ConfigFormatUnspecified
	}
}

func pathContains(path, dir string) bool {
	return slices.Contains(strings.Split(filepath.Clean(path), string(filepath.Separator)), dir)
}

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

// LoadConfigFromFile loads the MCP config from the provided path.
func LoadConfigFromFile(configPath string, format ConfigFormat) (*FileConfig, error) {
	data, err := os.ReadFile(configPath)
	switch {
	case os.IsNotExist(err):
		return &FileConfig{
			Config:       NewConfig(format),
			configPath:   configPath,
			configExists: false,
		}, nil

	case err != nil:
		return nil, trace.Wrap(trace.ConvertSystemError(err), "reading mcp servers config")

	default:
		config, err := NewConfigFromJSON(format, data)
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

// Exists returns true if config file exists.
func (c *FileConfig) Exists() bool {
	return c.configExists
}

// Path returns the config file path.
func (c *FileConfig) Path() string {
	return c.configPath
}

// Save saves the updated config to the config path. Format defaults to "auto"
// if empty.
func (c *FileConfig) Save(format FormatJSONOption) error {
	// Creates the config with 0644.
	file, err := os.OpenFile(c.configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()
	return trace.Wrap(c.Write(file, format))
}

func (c *Config) mcpServerJSONPath(serverName string) string {
	return c.format.serversKey() + "." + serverName
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

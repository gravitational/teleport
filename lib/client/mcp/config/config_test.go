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
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestFileConfig_fileNotExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	config, err := LoadConfigFromFile(configPath, ConfigFormatClaude)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.False(t, config.Exists())

	require.NoError(t, config.PutMCPServer("test", MCPServer{
		Command: "command",
	}))
	require.NoError(t, config.Save(FormatJSONCompact))
	requireFileWithData(t, configPath, `{"mcpServers":{"test":{"command":"command"}}}`)
}

func TestFileConfig_sampleFile(t *testing.T) {
	const sampleConfigJSON = `{
  "someUnknownField": "someUnknownValue",
  "mcpServers": {
    "Puppeteer": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
      "someUnknownField": "someUnknownValue"
    },
    "teleport-my-mcp": {
      "command": "tsh",
      "args": ["mcp", "connect", "my-mcp"],
      "env": {
        "TELEPORT_HOME": "/tsh-home/"
      }
    }
  }
}
`
	var sampleMCPServers = map[string]MCPServer{
		"Puppeteer": {
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-puppeteer"},
		},
		"teleport-my-mcp": {
			Command: "tsh",
			Args:    []string{"mcp", "connect", "my-mcp"},
			Envs: map[string]string{
				"TELEPORT_HOME": "/tsh-home/",
			},
		},
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(sampleConfigJSON), 0600))

	// load
	config, err := LoadConfigFromFile(configPath, ConfigFormatClaude)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.True(t, config.Exists())
	require.Equal(t, configPath, config.Path())
	require.Equal(t, sampleMCPServers, config.GetMCPServers())

	// remove
	require.True(t, trace.IsNotFound(config.RemoveMCPServer("not-found")))
	require.NoError(t, config.RemoveMCPServer("teleport-my-mcp"))
	require.NoError(t, config.Save(FormatJSONPretty))
	requireFileWithData(t, configPath, `{
  "someUnknownField": "someUnknownValue",
  "mcpServers": {
    "Puppeteer": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
      "someUnknownField": "someUnknownValue"
    }
  }
}
`)

	// add it back
	require.NoError(t, config.PutMCPServer("teleport-my-mcp", sampleMCPServers["teleport-my-mcp"]))
	require.NoError(t, config.Save(FormatJSONAuto))
	requireFileWithData(t, configPath, sampleConfigJSON)

	// replace
	require.NoError(t, config.PutMCPServer("Puppeteer", MCPServer{
		Command: "custom-script",
	}))
	require.NoError(t, config.Save(""))
	requireFileWithData(t, configPath, `{
  "someUnknownField": "someUnknownValue",
  "mcpServers": {
    "Puppeteer": {
      "command": "custom-script"
    },
    "teleport-my-mcp": {
      "command": "tsh",
      "args": ["mcp", "connect", "my-mcp"],
      "env": {
        "TELEPORT_HOME": "/tsh-home/"
      }
    }
  }
}
`)
}

func TestConfig_Write(t *testing.T) {
	for _, format := range []ConfigFormat{ConfigFormatClaude, ConfigFormatVSCode} {
		t.Run(format.serversKey(), func(t *testing.T) {
			config := NewConfig(format)

			mcpServer := MCPServer{
				Command: "command",
			}
			mcpServer.AddEnv("foo", "bar")
			require.NoError(t, config.PutMCPServer("test", mcpServer))
			var buf bytes.Buffer

			require.NoError(t, config.Write(&buf, FormatJSONCompact))
			require.Equal(t, `{"`+format.serversKey()+`":{"test":{"command":"command","env":{"foo":"bar"}}}}`, buf.String())
		})
	}
}

func Test_isJSONCompact(t *testing.T) {
	tests := []struct {
		name           string
		in             string
		checkError     require.ErrorAssertionFunc
		checkIsCompact require.BoolAssertionFunc
	}{
		{
			name:           "bad JSON",
			in:             "{",
			checkError:     require.Error,
			checkIsCompact: require.False,
		},
		{
			name:           "empty object",
			in:             "{}",
			checkError:     require.NoError,
			checkIsCompact: require.False,
		},
		{
			name:           "compact",
			in:             `{"a":"b"}`,
			checkError:     require.NoError,
			checkIsCompact: require.True,
		},
		{
			name: "not compact",
			in: `{
  "a": "b"
}`,
			checkError:     require.NoError,
			checkIsCompact: require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isCompact, err := isJSONCompact([]byte(tt.in))
			tt.checkError(t, err)
			tt.checkIsCompact(t, isCompact)
		})
	}
}

func Test_formatJSON(t *testing.T) {
	notFormatted := `{"a":       "b"}`
	compact := `{"a":"b"}`
	pretty := `{
  "a": "b"
}
`
	tests := []struct {
		name              string
		in                string
		format            FormatJSONOption
		isOriginalCompact bool
		out               string
	}{
		{
			name:   "to compact",
			in:     notFormatted,
			format: FormatJSONCompact,
			out:    compact,
		},
		{
			name:   "to pretty",
			in:     notFormatted,
			format: FormatJSONPretty,
			out:    pretty,
		},
		{
			name:   "none",
			in:     notFormatted,
			format: FormatJSONNone,
			out:    notFormatted,
		},
		{
			name:              "auto compact",
			in:                notFormatted,
			format:            FormatJSONAuto,
			isOriginalCompact: true,
			out:               compact,
		},
		{
			name:              "auto pretty",
			in:                notFormatted,
			format:            FormatJSONAuto,
			isOriginalCompact: false,
			out:               pretty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted, err := formatJSON([]byte(tt.in), tt.format, tt.isOriginalCompact)
			require.NoError(t, err)
			require.Equal(t, tt.out, string(formatted))
		})
	}
}

// TestPrettyResourceURIs given a MCP server that includes a Resource URI as
// arguments it must encode and output those URIs in a readable format.
func TestReadableResourceURIs(t *testing.T) {
	for name, uri := range map[string]string{
		"uri with query params":    "teleport://clusters/root/databases/pg",
		"uri without query params": "teleport://clusters/root/databases/pg?dbName=postgres&dbUser=readonly",
		"random uri with params":   "teleport://random?hello=world&random=resource",
	} {
		t.Run(name, func(t *testing.T) {
			config := NewConfig(claudeServersKey)
			mcpServer := MCPServer{
				Command: "command",
				Args:    []string{uri},
			}
			require.NoError(t, config.PutMCPServer("test", mcpServer))

			var buf bytes.Buffer
			require.NoError(t, config.Write(&buf, FormatJSONCompact))
			require.Contains(t, buf.String(), uri)
		})
	}
}

func TestReadDifferentConfigFormats(t *testing.T) {
	fileNamePathWithDir := func(dir, fileName string) func(*testing.T, string) string {
		return func(t *testing.T, basePath string) string {
			path := filepath.Join(basePath, dir)
			require.NoError(t, os.Mkdir(path, 0700))
			return filepath.Join(path, fileName)
		}
	}

	fileNamePath := func(fileName string) func(*testing.T, string) string {
		return func(t *testing.T, basePath string) string {
			return filepath.Join(basePath, fileName)
		}
	}

	for name, tc := range map[string]struct {
		filePath     func(*testing.T, string) string
		contents     string
		expectErr    require.ErrorAssertionFunc
		expectConfig require.ValueAssertionFunc
	}{
		"vscode": {
			filePath:  fileNamePathWithDir(vsCodeProjectDir, "mcp.json"),
			contents:  `{"servers":{"everything": {"command": "npx", "args": []}}}`,
			expectErr: require.NoError,
			expectConfig: func(tt require.TestingT, i1 any, i2 ...any) {
				config := i1.(*FileConfig)
				servers := config.GetMCPServers()

				srv, ok := servers["everything"]
				require.True(tt, ok, `expected config to have "everything" mcp server configured`)
				require.Equal(tt, "npx", srv.Command)
			},
		},
		"cursor": {
			filePath:  fileNamePathWithDir(cursorProjectDir, "mcp.json"),
			contents:  `{"mcpServers":{"everything": {"command": "npx", "args": []}}}`,
			expectErr: require.NoError,
			expectConfig: func(tt require.TestingT, i1 any, i2 ...any) {
				config := i1.(*FileConfig)
				servers := config.GetMCPServers()

				srv, ok := servers["everything"]
				require.True(tt, ok, `expected config to have "everything" mcp server configured`)
				require.Equal(tt, "npx", srv.Command)
			},
		},
		"claude-code": {
			filePath:  fileNamePath(".mcp.json"),
			contents:  `{"mcpServers":{"everything": {"command": "npx", "args": []}}}`,
			expectErr: require.NoError,
			expectConfig: func(tt require.TestingT, i1 any, i2 ...any) {
				config := i1.(*FileConfig)
				servers := config.GetMCPServers()

				srv, ok := servers["everything"]
				require.True(tt, ok, `expected config to have "everything" mcp server configured`)
				require.Equal(tt, "npx", srv.Command)
			},
		},
		"empty config": {
			filePath:     fileNamePath("file.json"),
			contents:     ``,
			expectErr:    require.Error,
			expectConfig: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			configPath := tc.filePath(t, t.TempDir())
			require.NoError(t, os.WriteFile(configPath, []byte(tc.contents), 0600))

			format := ConfigFormatFromPath(configPath)
			config, err := LoadConfigFromFile(configPath, format)
			tc.expectErr(t, err)
			tc.expectConfig(t, config)
		})
	}
}

func requireFileWithData(t *testing.T, path string, want string) {
	t.Helper()
	read, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, string(read))
}

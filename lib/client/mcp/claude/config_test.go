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
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestFileConfig_fileNotExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	config, err := LoadConfigFromFile(configPath)
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
	config, err := LoadConfigFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, config)
	require.True(t, config.Exists())
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
	config := NewConfig()

	require.NoError(t, config.PutMCPServer("test", MCPServer{
		Command: "command",
	}))
	var buf bytes.Buffer

	require.NoError(t, config.Write(&buf, FormatJSONCompact))
	require.Equal(t, `{"mcpServers":{"test":{"command":"command"}}}`, buf.String())
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

func requireFileWithData(t *testing.T, path string, want string) {
	t.Helper()
	read, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, string(read))
}

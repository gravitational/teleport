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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

const sampleConfigJSON = `{
  "someUnknownField": "someUnknownValue",
  "mcpServers": {
    "Puppeteer": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-puppeteer"
      ],
      "someUnknownField": "someUnknownValue"
    },
    "teleport-my-mcp": {
      "command": "tsh",
      "args": [
        "mcp",
        "connect",
        "my-mcp"
      ],
      "env": {
        "TELEPORT_HOME": "/tsh-home/"
      }
    }
  }
}
`

var sampleConfig = Config{
	MCPServers: map[string]MCPServer{
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
	},
}

func TestConfig_marshaling(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantConfig Config
	}{
		{
			name:       "empty",
			input:      "{}",
			wantConfig: Config{},
		},
		{
			name:       "sample",
			input:      sampleConfigJSON,
			wantConfig: sampleConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := json.Unmarshal([]byte(tt.input), &config)
			require.NoError(t, err)

			require.Empty(t, diffConfig(&tt.wantConfig, &config))

			output, err := json.Marshal(config)
			require.NoError(t, err)
			require.JSONEq(t, tt.input, string(output))
		})
	}
}

func TestConfig_file(t *testing.T) {
	dir := t.TempDir()
	orgPath := filepath.Join(dir, "config_org.json")
	require.NoError(t, os.WriteFile(orgPath, []byte(sampleConfigJSON), 0600))
	savePath := filepath.Join(dir, "config_save.json")

	t.Run("LoadConfig no file exists", func(t *testing.T) {
		_, err := LoadConfig("no_exist.json")
		require.Error(t, err)
	})

	var config *Config
	t.Run("LoadConfig", func(t *testing.T) {
		var err error
		config, err = LoadConfig(orgPath)
		require.NoError(t, err)
		require.NotNil(t, config)
		require.Empty(t, diffConfig(&sampleConfig, config))
	})

	t.Run("SaveConfig", func(t *testing.T) {
		err := SaveConfig(config, savePath)
		require.NoError(t, err)

		// Double check all fields are preserved.
		orgData, err := os.ReadFile(orgPath)
		require.NoError(t, err)
		savedData, err := os.ReadFile(savePath)
		require.NoError(t, err)
		require.JSONEq(t, string(orgData), string(savedData))
	})
}

func diffConfig(a, b *Config) string {
	return cmp.Diff(a, b,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(Config{}, "AllFields"),
		cmpopts.IgnoreFields(MCPServer{}, "AllFields"),
	)
}

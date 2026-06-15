/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnConfigModify(t *testing.T) {
	t.Run("set token via named flag", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "auth_token: new-token")
		require.Contains(t, string(got), "data_dir: /var/lib/teleport")
	})

	t.Run("enable existing service", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("ssh_service:\n  enabled: \"no\"\n  listen_addr: 0.0.0.0:3022\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:         inputFile,
			output:        "file://" + outputFile,
			enableService: []string{"ssh_service"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), `enabled: "yes"`)
	})

	t.Run("enable non-existent service fails", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		err := onConfigModify(modifyFlags{
			input:         inputFile,
			output:        "stdout",
			enableService: []string{"ssh_service"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not exist")
	})

	t.Run("disable existing service", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("ssh_service:\n  enabled: \"yes\"\n  listen_addr: 0.0.0.0:3022\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:          inputFile,
			output:         "file://" + outputFile,
			disableService: []string{"ssh_service"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), `enabled: "no"`)
	})

	t.Run("node-labels produces map", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("ssh_service:\n  enabled: \"yes\"\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:      inputFile,
			output:     "file://" + outputFile,
			nodeLabels: "env=staging,cloud=aws",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "labels:")
		require.Contains(t, string(got), "env: staging")
		require.Contains(t, string(got), "cloud: aws")
	})

	t.Run("unset removes field", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n  auth_token: old-token\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			unset:  []string{"teleport.auth_token"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.NotContains(t, string(got), "auth_token")
		require.Contains(t, string(got), "data_dir")
	})

	t.Run("--set arbitrary path", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			set:    []string{"teleport.log.severity=DEBUG"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "severity: DEBUG")
	})

	t.Run("output file exists without overwrite fails", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")
		require.NoError(t, os.WriteFile(outputFile, []byte("existing"), 0644))

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "new-token",
		})
		require.Error(t, err)
	})

	t.Run("output file exists with overwrite succeeds", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")
		require.NoError(t, os.WriteFile(outputFile, []byte("existing"), 0644))

		err := onConfigModify(modifyFlags{
			input:     inputFile,
			output:    "file://" + outputFile,
			overwrite: true,
			token:     "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "auth_token: new-token")
	})

	t.Run("stdout output", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "stdout",
			token:  "new-token",
		})
		require.NoError(t, err)
	})

	t.Run("input file does not exist", func(t *testing.T) {
		err := onConfigModify(modifyFlags{
			input:  "/nonexistent/path.yaml",
			output: "stdout",
			token:  "new-token",
		})
		require.Error(t, err)
	})

	t.Run("--set invalid format", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "stdout",
			set:    []string{"no-equals-sign"},
		})
		require.Error(t, err)
	})

	t.Run("--set value containing equals sign", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			set:    []string{"teleport.auth_token=https://auth.example.com?token=abc=def"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "auth_token: https://auth.example.com?token=abc=def")
	})
}

func TestOnConfigModifyRoles(t *testing.T) {
	t.Run("creates ssh_service section", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			roles:  "node",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "ssh_service:")
		require.Contains(t, string(got), `enabled: "yes"`)
		require.Contains(t, string(got), "listen_addr: 0.0.0.0:3022")
	})

	t.Run("roles does not overwrite existing section", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\nssh_service:\n  enabled: \"no\"\n  listen_addr: 0.0.0.0:9999\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			roles:  "node",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), `enabled: "yes"`)
		require.Contains(t, string(got), "listen_addr: 0.0.0.0:9999")
	})

	t.Run("multiple roles", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			roles:  "node,proxy,auth",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "ssh_service:")
		require.Contains(t, string(got), "proxy_service:")
		require.Contains(t, string(got), "auth_service:")
	})

	t.Run("unknown role fails", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "stdout",
			roles:  "nonexistent",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown role")
	})
}

func TestOnConfigModifyPreservesComments(t *testing.T) {
	input := "# Main teleport configuration\nteleport:\n  # Data directory for teleport state\n  data_dir: /var/lib/teleport\n  auth_token: old-token\n# SSH service configuration\nssh_service:\n  enabled: \"yes\"\n  listen_addr: 0.0.0.0:3022\n"
	inputFile := filepath.Join(t.TempDir(), "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte(input), 0644))

	outputFile := filepath.Join(t.TempDir(), "output.yaml")

	err := onConfigModify(modifyFlags{
		input:  inputFile,
		output: "file://" + outputFile,
		token:  "new-token",
	})
	require.NoError(t, err)

	got, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	require.Contains(t, string(got), "# Main teleport configuration")
	require.Contains(t, string(got), "# Data directory for teleport state")
	require.Contains(t, string(got), "# SSH service configuration")
	require.Contains(t, string(got), "auth_token: new-token")
	require.NotContains(t, string(got), "old-token")
}

func TestOnConfigModifyRealisticConfigs(t *testing.T) {
	const fullConfig = `# Teleport configuration file
version: v3
teleport:
  nodename: node1.example.com
  data_dir: /var/lib/teleport
  auth_token: original-token
  auth_server: auth.example.com:3025
  # CA pin for secure cluster joining
  ca_pin: sha256:abc123def456
  log:
    severity: INFO
    output: /var/log/teleport.log

# SSH access service
ssh_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3022
  labels:
    env: production
    region: us-east-1

# Web proxy service
proxy_service:
  enabled: "yes"
  web_listen_addr: 0.0.0.0:3080
  # Public address for external access
  public_addr: proxy.example.com:3080

# Application service
app_service:
  enabled: "yes"
  apps:
    - name: grafana
      uri: http://localhost:3000
    - name: jenkins
      uri: http://localhost:8080

# Database service
db_service:
  enabled: "no"
`

	t.Run("modify token and add labels in full config", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte(fullConfig), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:      inputFile,
			output:     "file://" + outputFile,
			token:      "new-cluster-token",
			nodeLabels: "team=platform,tier=frontend",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		result := string(got)

		// Modifications applied
		require.Contains(t, result, "auth_token: new-cluster-token")
		require.Contains(t, result, "team: platform")
		require.Contains(t, result, "tier: frontend")

		// Comments preserved
		require.Contains(t, result, "# Teleport configuration file")
		require.Contains(t, result, "# CA pin for secure cluster joining")
		require.Contains(t, result, "# SSH access service")
		require.Contains(t, result, "# Web proxy service")
		require.Contains(t, result, "# Public address for external access")
		require.Contains(t, result, "# Application service")
		require.Contains(t, result, "# Database service")

		// Unmodified values preserved
		require.Contains(t, result, "nodename: node1.example.com")
		require.Contains(t, result, "ca_pin: sha256:abc123def456")
		require.Contains(t, result, "web_listen_addr: 0.0.0.0:3080")
		require.Contains(t, result, "name: grafana")
		require.Contains(t, result, "name: jenkins")

		// Original token gone
		require.NotContains(t, result, "original-token")
	})

	t.Run("unset deeply nested key", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte(fullConfig), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			unset:  []string{"teleport.log.severity"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		result := string(got)

		// Severity removed
		require.NotContains(t, result, "severity: INFO")

		// Sibling key under log preserved
		require.Contains(t, result, "output: /var/log/teleport.log")

		// Rest of config intact
		require.Contains(t, result, "version: v3")
		require.Contains(t, result, "nodename: node1.example.com")
		require.Contains(t, result, "# SSH access service")
		require.Contains(t, result, "proxy_service:")
	})

	t.Run("enable and disable services in full config", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte(fullConfig), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onConfigModify(modifyFlags{
			input:          inputFile,
			output:         "file://" + outputFile,
			enableService:  []string{"db_service"},
			disableService: []string{"proxy_service"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		result := string(got)

		// db_service enabled
		require.Contains(t, result, "db_service:")

		// All comments preserved
		require.Contains(t, result, "# Teleport configuration file")
		require.Contains(t, result, "# Web proxy service")
		require.Contains(t, result, "# Database service")

		// Unmodified sections remain
		require.Contains(t, result, "nodename: node1.example.com")
		require.Contains(t, result, "name: grafana")
		require.Contains(t, result, "listen_addr: 0.0.0.0:3022")
	})
}

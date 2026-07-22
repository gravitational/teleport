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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestReconfigure(t *testing.T) {
	t.Run("set token via named flag", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "token_name: new-token")
		require.Contains(t, string(got), "data_dir: /var/lib/teleport")
	})

	t.Run("token with join-method migrates legacy auth_token to join_params", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n  auth_token: old-token\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:      inputFile,
			output:     "file://" + outputFile,
			token:      "new-token",
			joinMethod: "iam",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "token_name: new-token")
		require.Contains(t, string(got), "method: iam")
		require.NotContains(t, string(got), "auth_token")
	})

	t.Run("data-dir named flag", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:   inputFile,
			output:  "file://" + outputFile,
			dataDir: "/var/lib/teleport_teamA",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "data_dir: /var/lib/teleport_teamA")
	})

	t.Run("enable existing service", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("ssh_service:\n  enabled: \"no\"\n  listen_addr: 0.0.0.0:3022\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
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

		err := onReconfigure(reconfigureFlags{
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

		err := onReconfigure(reconfigureFlags{
			input:          inputFile,
			output:         "file://" + outputFile,
			disableService: []string{"ssh_service"},
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), `enabled: "no"`)
	})

	t.Run("roles creates a missing service section", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			roles:  "app",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "app_service:")
		require.Contains(t, string(got), `enabled: "yes"`)
	})

	t.Run("output file exists without overwrite fails", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")
		require.NoError(t, os.WriteFile(outputFile, []byte("existing"), 0644))

		err := onReconfigure(reconfigureFlags{
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

		err := onReconfigure(reconfigureFlags{
			input:     inputFile,
			output:    "file://" + outputFile,
			overwrite: true,
			token:     "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "token_name: new-token")
	})

	t.Run("stdout output", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "stdout",
			token:  "new-token",
		})
		require.NoError(t, err)
	})

	t.Run("input file does not exist", func(t *testing.T) {
		err := onReconfigure(reconfigureFlags{
			input:  "/nonexistent/path.yaml",
			output: "stdout",
			token:  "new-token",
		})
		require.Error(t, err)
	})

	t.Run("input that cannot be parsed is refused", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  bogus_field: oops\n"), 0644))

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "stdout",
			token:  "new-token",
		})
		require.Error(t, err)
	})
}

func TestReconfigureJoinParams(t *testing.T) {
	t.Run("token on config with no join_params sets default method token", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "my-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "token_name: my-token")
		require.Contains(t, string(got), "method: token")
	})

	t.Run("token with existing join_params.method preserves existing method", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n  join_params:\n    method: iam\n    token_name: old\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "token_name: new-token")
		require.Contains(t, string(got), "method: iam")
		require.NotContains(t, string(got), "method: token")
	})

	t.Run("token without join-method preserves legacy auth_token format", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("teleport:\n  data_dir: /var/lib/teleport\n  auth_token: old-token\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			token:  "new-token",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "auth_token: new-token")
		require.NotContains(t, string(got), "join_params")
	})
}

func TestReconfigureEndpoints(t *testing.T) {
	t.Run("setting proxy clears auth_server", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("version: v3\nteleport:\n  data_dir: /var/lib/teleport\n  auth_server: auth.example.com:3025\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			proxy:  "proxy.example.com:443",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "proxy_server: proxy.example.com:443")
		require.NotContains(t, string(got), "auth_server")
	})

	t.Run("setting auth_server clears proxy_server", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("version: v3\nteleport:\n  data_dir: /var/lib/teleport\n  proxy_server: proxy.example.com:443\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:      inputFile,
			output:     "file://" + outputFile,
			authServer: "auth.example.com:3025",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "auth_server: auth.example.com:3025")
		require.NotContains(t, string(got), "proxy_server")
	})

	t.Run("setting proxy clears legacy auth_servers", func(t *testing.T) {
		inputFile := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(inputFile, []byte("version: v3\nteleport:\n  data_dir: /var/lib/teleport\n  auth_servers:\n  - auth.example.com:3025\n"), 0644))

		outputFile := filepath.Join(t.TempDir(), "output.yaml")

		err := onReconfigure(reconfigureFlags{
			input:  inputFile,
			output: "file://" + outputFile,
			proxy:  "proxy.example.com:443",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "proxy_server: proxy.example.com:443")
		require.NotContains(t, string(got), "auth_servers")
	})

	t.Run("overwrite preserves atomic write when target is the input", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "teleport.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("version: v3\nteleport:\n  data_dir: /var/lib/teleport\n  auth_server: auth.example.com:3025\n"), 0644))

		err := onReconfigure(reconfigureFlags{
			input:     configFile,
			output:    "file://" + configFile,
			overwrite: true,
			proxy:     "proxy.example.com:443",
		})
		require.NoError(t, err)

		got, err := os.ReadFile(configFile)
		require.NoError(t, err)
		require.Contains(t, string(got), "proxy_server: proxy.example.com:443")
		require.NotContains(t, string(got), "auth_server")

		// The atomic rename must not leave temp files behind in the target directory.
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "teleport.yaml", entries[0].Name())
	})
}

// reconfigureToString runs onReconfigure to a temp file and returns the output.
func reconfigureToString(t *testing.T, in string, flags reconfigureFlags) (string, error) {
	t.Helper()
	inputFile := filepath.Join(t.TempDir(), "input.yaml")
	require.NoError(t, os.WriteFile(inputFile, []byte(in), 0o644))
	outputFile := filepath.Join(t.TempDir(), "output.yaml")

	flags.input = inputFile
	flags.output = "file://" + outputFile
	if err := onReconfigure(flags); err != nil {
		return "", err
	}
	got, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	return string(got), nil
}

func TestReconfigureJoinInvariants(t *testing.T) {
	t.Run("join-method without token migrates legacy auth_token into join_params", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n  auth_token: old-token\n",
			reconfigureFlags{joinMethod: "iam"})
		require.NoError(t, err)
		require.Contains(t, got, "token_name: old-token")
		require.Contains(t, got, "method: iam")
		require.NotContains(t, got, "auth_token")
	})

	t.Run("invalid join-method is rejected", func(t *testing.T) {
		_, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{token: "t", joinMethod: "not-a-real-method"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "join method must be one of")
	})

	t.Run("bound_keypair registration secret is written", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{joinMethod: "bound_keypair", registrationSecret: "super-secret", token: "bk-token"})
		require.NoError(t, err)
		require.Contains(t, got, "method: bound_keypair")
		require.Contains(t, got, "token_name: bk-token")
		require.Contains(t, got, "registration_secret_value: super-secret")
		require.NotContains(t, got, "auth_token")
	})

	t.Run("method change clears the previous method's sub-block", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n  join_params:\n    method: bound_keypair\n    token_name: old\n    bound_keypair:\n      registration_secret_value: leaked\n",
			reconfigureFlags{joinMethod: "token", token: "new-token"})
		require.NoError(t, err)
		require.Contains(t, got, "method: token")
		require.Contains(t, got, "token_name: new-token")
		require.NotContains(t, got, "bound_keypair")
		require.NotContains(t, got, "leaked")
	})
}

func TestReconfigureVersionGuard(t *testing.T) {
	t.Run("endpoint on pre-v3 config without --config-version is rejected", func(t *testing.T) {
		_, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{proxy: "proxy.example.com:443"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "config version v3")
	})

	t.Run("endpoint with --config-version v3 on pre-v3 config succeeds", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{proxy: "proxy.example.com:443", configVersion: "v3"})
		require.NoError(t, err)
		require.Contains(t, got, "version: v3")
		require.Contains(t, got, "proxy_server: proxy.example.com:443")
	})

	t.Run("proxy and auth-server together is rejected", func(t *testing.T) {
		_, err := reconfigureToString(t,
			"version: v3\nteleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{proxy: "proxy.example.com:443", authServer: "auth.example.com:3025"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "only one of")
	})

	t.Run("invalid config-version is rejected", func(t *testing.T) {
		_, err := reconfigureToString(t,
			"teleport:\n  data_dir: /var/lib/teleport\n",
			reconfigureFlags{configVersion: "v9"})
		require.Error(t, err)
	})
}

func TestReconfigureSSHNetworking(t *testing.T) {
	t.Run("ssh-listen-addr moves the node off its port", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"ssh_service:\n  enabled: \"yes\"\n  listen_addr: 0.0.0.0:3022\n",
			reconfigureFlags{sshListenAddr: "0.0.0.0:3122"})
		require.NoError(t, err)
		require.Contains(t, got, "listen_addr: 0.0.0.0:3122")
		require.NotContains(t, got, "0.0.0.0:3022")
	})

	t.Run("force-listen and public-addr are set", func(t *testing.T) {
		got, err := reconfigureToString(t,
			"ssh_service:\n  enabled: \"yes\"\n",
			reconfigureFlags{sshListenAddr: "0.0.0.0:3122", sshPublicAddr: "node.example.com:3122", forceListen: true})
		require.NoError(t, err)
		require.Contains(t, got, "force_listen: true")
		require.Contains(t, got, "node.example.com:3122")
	})
}

// TestReconfigureProducesStartableConfig proves the generated config loads
// through the same path the agent uses at startup (ApplyFileConfig), for a
// node config that references no host-local files.
func TestReconfigureProducesStartableConfig(t *testing.T) {
	in := "version: v3\n" +
		"teleport:\n" +
		"  data_dir: /var/lib/teleport\n" +
		"  proxy_server: proxy.example.com:443\n" +
		"  auth_token: old-token\n" +
		"auth_service:\n" +
		"  enabled: \"no\"\n" +
		"proxy_service:\n" +
		"  enabled: \"no\"\n" +
		"ssh_service:\n" +
		"  enabled: \"yes\"\n"

	got, err := reconfigureToString(t, in, reconfigureFlags{
		joinMethod:    "token",
		token:         "new-token",
		dataDir:       "/var/lib/teleport_teamA",
		sshListenAddr: "0.0.0.0:3122",
	})
	require.NoError(t, err)
	require.Contains(t, got, "data_dir: /var/lib/teleport_teamA")
	require.Contains(t, got, "token_name: new-token")

	// The reconfigured output must load the way teleport start loads it.
	fc, err := config.ReadConfig(strings.NewReader(got))
	require.NoError(t, err)
	require.NoError(t, config.ApplyFileConfig(fc, servicecfg.MakeDefaultConfig()))
}

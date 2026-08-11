/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/config"
)

const reconfigureTestInput = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
proxy_service:
  enabled: no
ssh_service:
  enabled: yes
`

// reconfigureCollisionTestInput pins pid_file, so reconfiguring without
// --pid-file collides.
const reconfigureCollisionTestInput = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  pid_file: /var/run/teleport.pid
proxy_service:
  enabled: no
ssh_service:
  enabled: yes
`

func writeReconfigureInputFile(t *testing.T, content string) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), "teleport.yaml")
	require.NoError(t, os.WriteFile(input, []byte(content), 0o600))
	return input
}

func writeReconfigureInput(t *testing.T) string {
	t.Helper()
	return writeReconfigureInputFile(t, reconfigureTestInput)
}

func TestRunReconfigure(t *testing.T) {
	t.Run("writes valid YAML to stdout when output omitted", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:       writeReconfigureInput(t),
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.NoError(t, err)
		fc, err := config.ReadConfig(strings.NewReader(stdout.String()))
		require.NoError(t, err)
		require.Equal(t, "new.example.com:443", fc.ProxyServer)
	})

	t.Run("collision with the input agent is a hard error", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:       writeReconfigureInputFile(t, reconfigureCollisionTestInput),
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.ErrorContains(t, err, `pid_file "/var/run/teleport.pid" (--pid-file)`)
		require.Empty(t, stdout.String())
	})

	t.Run("collision error prevents writing the output file", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInputFile(t, reconfigureCollisionTestInput)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.Error(t, err)
		require.NoFileExists(t, output)
	})

	t.Run("passing --pid-file resolves the collision", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:       writeReconfigureInputFile(t, reconfigureCollisionTestInput),
			proxyServer: "new.example.com:443",
			pidFile:     "/run/teleport_new.pid",
		}, &stdout)
		require.NoError(t, err)
	})

	t.Run("writes the output file with 0600 permissions", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.NoError(t, err)
		info, err := os.Stat(output)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		_, err = config.ReadFromFile(output)
		require.NoError(t, err)
		require.Empty(t, stdout.String())
	})

	t.Run("refuses an existing output without --overwrite", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		require.NoError(t, os.WriteFile(output, []byte("old"), 0o600))
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.ErrorContains(t, err, "already exists; use --overwrite to replace it")
	})

	t.Run("replaces an existing output with --overwrite", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		require.NoError(t, os.WriteFile(output, []byte("old"), 0o600))
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			overwrite:   true,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.NoError(t, err)
		fc, err := config.ReadFromFile(output)
		require.NoError(t, err)
		require.Equal(t, "new.example.com:443", fc.ProxyServer)
	})

	t.Run("overwrite forces 0600 on a world-readable existing output", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		require.NoError(t, os.WriteFile(output, []byte("old"), 0o644))
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			overwrite:   true,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.NoError(t, err)
		info, err := os.Stat(output)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("output symlink pointing at the input is refused", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		link := filepath.Join(filepath.Dir(input), "link.yaml")
		require.NoError(t, os.Symlink(input, link))
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      link,
			overwrite:   true,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.ErrorContains(t, err, "refusing to write over the input config")
	})

	t.Run("never writes over the input, even with --overwrite", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      input,
			overwrite:   true,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.ErrorContains(t, err, "refusing to write over the input config")
	})

	t.Run("overwrite without output errors", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:     writeReconfigureInput(t),
			overwrite: true,
		}, &stdout)
		require.ErrorContains(t, err, "--overwrite requires --output")
	})

	t.Run("stdout write failure returns an error", func(t *testing.T) {
		err := runReconfigure(reconfigureFlags{
			input:       writeReconfigureInput(t),
			proxyServer: "new.example.com:443",
		}, errWriter{})
		require.ErrorContains(t, err, "pipe closed")
	})

	t.Run("missing input file errors", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input: filepath.Join(t.TempDir(), "missing.yaml"),
		}, &stdout)
		require.Error(t, err)
	})

	t.Run("unknown field in input errors", func(t *testing.T) {
		var stdout bytes.Buffer
		input := filepath.Join(t.TempDir(), "teleport.yaml")
		require.NoError(t, os.WriteFile(input, []byte("teleport:\n  not_a_real_field: x\n"), 0o600))
		err := runReconfigure(reconfigureFlags{input: input}, &stdout)
		require.ErrorContains(t, err, "not_a_real_field")
	})

	t.Run("output directory must exist", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:       writeReconfigureInput(t),
			output:      filepath.Join(t.TempDir(), "missing-dir", "out.yaml"),
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.Error(t, err)
	})

	t.Run("dangling symlink at output is rejected without --overwrite", func(t *testing.T) {
		var stdout bytes.Buffer
		input := writeReconfigureInput(t)
		output := filepath.Join(filepath.Dir(input), "teleport_new.yaml")
		require.NoError(t, os.Symlink(filepath.Join(filepath.Dir(input), "nope"), output))
		err := runReconfigure(reconfigureFlags{
			input:       input,
			output:      output,
			proxyServer: "new.example.com:443",
		}, &stdout)
		require.ErrorContains(t, err, "already exists; use --overwrite to replace it")
	})

	t.Run("registration-secret-path lands in the output config", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:                              writeReconfigureInput(t),
			joinMethod:                         "bound_keypair",
			boundKeypairRegistrationSecretPath: "/etc/teleport-secret",
		}, &stdout)
		require.NoError(t, err)
		fc, err := config.ReadConfig(strings.NewReader(stdout.String()))
		require.NoError(t, err)
		require.Equal(t, "/etc/teleport-secret", fc.JoinParams.BoundKeypair.RegistrationSecretPath)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
	})

	t.Run("registration-secret-path without a bound_keypair method errors", func(t *testing.T) {
		var stdout bytes.Buffer
		err := runReconfigure(reconfigureFlags{
			input:                              writeReconfigureInput(t),
			boundKeypairRegistrationSecretPath: "/etc/teleport-secret",
		}, &stdout)
		require.ErrorContains(t, err, "--join-method bound_keypair")
	})
}

// TestTeleportReconfigure drives `teleport reconfigure` through the real
// kingpin app, so a mis-registered flag or broken dispatch case fails here
// even though the handler-level tests pass.
func TestTeleportReconfigure(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "teleport.yaml")
	require.NoError(t, os.WriteFile(input, []byte(reconfigureTestInput), 0o600))
	output := filepath.Join(dir, "teleport_new.yaml")

	_, cmd, _ := Run(Options{
		Args: []string{"reconfigure",
			"--input", input,
			"--output", output,
			"--proxy-server", "new.example.com:443",
			"--ca-pin", "sha256:bbb", "--ca-pin", "sha256:ccc",
			"--token", "new-token",
			"--join-method", "bound_keypair",
			"--registration-secret-path", "/etc/teleport-secret",
			"--node-labels", "team=a",
			"--data-dir", "/var/lib/teleport_new",
			"--pid-file", "/run/teleport_new.pid",
			"--diag-addr", "127.0.0.1:3001",
			"--ssh-listen-addr", "0.0.0.0:3122",
			"--kube-listen-addr", "0.0.0.0:3127",
			"--metrics-listen-addr", "127.0.0.1:3081",
		},
		InitOnly: true,
	})
	require.Equal(t, "reconfigure", cmd)

	fc, err := config.ReadFromFile(output)
	require.NoError(t, err)
	require.Equal(t, "new.example.com:443", fc.ProxyServer)
	require.Equal(t, apiutils.Strings{"sha256:bbb", "sha256:ccc"}, fc.CAPin)
	require.Equal(t, "new-token", fc.JoinParams.TokenName)
	require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
	require.Equal(t, "/etc/teleport-secret", fc.JoinParams.BoundKeypair.RegistrationSecretPath)
	require.Equal(t, map[string]string{"team": "a"}, fc.SSH.Labels)
	require.Equal(t, "/var/lib/teleport_new", fc.DataDir)
	require.Equal(t, "/run/teleport_new.pid", fc.PIDFile)
	require.Equal(t, "127.0.0.1:3001", fc.DiagAddr)
	require.Equal(t, "0.0.0.0:3122", fc.SSH.ListenAddress)
	require.Equal(t, "0.0.0.0:3127", fc.Kube.ListenAddress)
	require.Equal(t, "127.0.0.1:3081", fc.Metrics.ListenAddress)
}

// errWriter fails every write, standing in for a closed pipe or full disk.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("pipe closed")
}

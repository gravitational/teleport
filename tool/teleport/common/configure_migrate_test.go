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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestConfigureMigrateWritesOverEmptyOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	outputPath := filepath.Join(dir, "teleport_scope.yaml")
	require.NoError(t, os.WriteFile(outputPath, []byte(" \n\t"), 0o640))
	secretPath := filepath.Join(dir, "token-secret")
	require.NoError(t, os.WriteFile(secretPath, []byte("secret-value"), 0o600))

	var stdout bytes.Buffer
	err := onConfigureMigrate(configureMigrateFlags{
		input:           inputPath,
		installSuffix:   "scope",
		output:          "file://" + outputPath,
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: secretPath,
		dataDir:         filepath.Join(dir, "data"),
		stdout:          &stdout,
		stderr:          &bytes.Buffer{},
	})
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "Wrote migrated Teleport configuration")

	rendered, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(rendered), "token_secret: "+secretPath)
	require.NotContains(t, string(rendered), "secret-value")
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	fc, err := config.ReadFromFile(outputPath)
	require.NoError(t, err)
	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	secret, err := cfg.TokenSecret()
	require.NoError(t, err)
	require.Equal(t, "secret-value", secret)
}

func TestConfigureMigrateRefusesNonEmptyOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	outputPath := filepath.Join(dir, "teleport_scope.yaml")
	require.NoError(t, os.WriteFile(outputPath, []byte("already here"), 0o640))

	err := onConfigureMigrate(configureMigrateFlags{
		input:           inputPath,
		installSuffix:   "scope",
		output:          "file://" + outputPath,
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: filepath.Join(dir, "token-secret"),
		dataDir:         filepath.Join(dir, "data"),
		stdout:          &bytes.Buffer{},
		stderr:          &bytes.Buffer{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "will not overwrite existing non-empty file")
}

func TestConfigureMigrateTestSuppressesWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	outputPath := filepath.Join(dir, "teleport_scope.yaml")

	var stderr bytes.Buffer
	err := onConfigureMigrate(configureMigrateFlags{
		input:           inputPath,
		installSuffix:   "scope",
		output:          "file://" + outputPath,
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: filepath.Join(dir, "token-secret"),
		dataDir:         filepath.Join(dir, "data"),
		test:            true,
		stdout:          &bytes.Buffer{},
		stderr:          &stderr,
	})
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "OK "+inputPath+" (migrated output validated)")
	require.NoFileExists(t, outputPath)
}

func TestConfigureMigrateDiffIsRedactedAndDoesNotWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	outputPath := filepath.Join(dir, "teleport_scope.yaml")
	require.NoError(t, os.WriteFile(outputPath, []byte("already here"), 0o640))

	var stdout, stderr bytes.Buffer
	err := onConfigureMigrate(configureMigrateFlags{
		input:           inputPath,
		installSuffix:   "scope",
		output:          "file://" + outputPath,
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: filepath.Join(dir, "token-secret"),
		dataDir:         filepath.Join(dir, "data"),
		diff:            true,
		stdout:          &stdout,
		stderr:          &stderr,
	})
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "writing would require --force")
	require.Contains(t, stdout.String(), "<redacted>")
	require.NotContains(t, stdout.String(), "SUPERSECRET")
	require.NotContains(t, stdout.String(), "scope-migrate-ip-10-2-4-17")
	onDisk, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "already here", string(onDisk))
}

func TestConfigureMigrateStdoutIsRedacted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := onConfigureMigrate(configureMigrateFlags{
		input:           inputPath,
		installSuffix:   "scope",
		output:          "stdout://",
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: filepath.Join(dir, "token-secret"),
		dataDir:         filepath.Join(dir, "data"),
		stdout:          &stdout,
		stderr:          &stderr,
	})
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "NOTICE: stdout output is redacted; use --output=file:// to write a usable config")
	require.Contains(t, stdout.String(), "token_secret: <redacted>")
	require.NotContains(t, stdout.String(), "SUPERSECRET")
	require.NotContains(t, stdout.String(), "scope-migrate-ip-10-2-4-17")
}

func TestConfigureMigrateFlagValidation(t *testing.T) {
	t.Parallel()

	base := configureMigrateFlags{
		input:         "/tmp/teleport.yaml",
		installSuffix: "scope",
		proxyServer:   "target.example.com:443",
		tokenName:     "scope-migrate-ip-10-2-4-17",
	}

	tokenNoSecret := base
	tokenNoSecret.joinMethod = string(types.JoinMethodToken)
	require.Error(t, tokenNoSecret.CheckAndSetDefaults())

	iamWithSecret := base
	iamWithSecret.joinMethod = string(types.JoinMethodIAM)
	iamWithSecret.tokenSecretFile = "/tmp/secret"
	require.Error(t, iamWithSecret.CheckAndSetDefaults())

	iamNoSecret := base
	iamNoSecret.joinMethod = string(types.JoinMethodIAM)
	require.NoError(t, iamNoSecret.CheckAndSetDefaults())

	fileDefaultNoSuffix := base
	fileDefaultNoSuffix.installSuffix = ""
	fileDefaultNoSuffix.output = "file"
	fileDefaultNoSuffix.dataDir = "/tmp/data"
	fileDefaultNoSuffix.joinMethod = string(types.JoinMethodToken)
	fileDefaultNoSuffix.tokenSecretFile = "/tmp/secret"
	err := fileDefaultNoSuffix.CheckAndSetDefaults()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--install-suffix is required when --output=file uses the default migrated config path")
}

func TestConfigureMigrateInstallSuffixValidation(t *testing.T) {
	t.Parallel()

	base := configureMigrateFlags{
		input:           "/tmp/teleport.yaml",
		proxyServer:     "target.example.com:443",
		joinMethod:      string(types.JoinMethodToken),
		tokenName:       "scope-migrate-ip-10-2-4-17",
		tokenSecretFile: "/tmp/secret",
	}

	for _, suffix := range []string{
		"bad/suffix",
		"bad suffix",
		"-bad",
		"default",
		"system",
	} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			flags := base
			flags.installSuffix = suffix
			require.Error(t, flags.CheckAndSetDefaults())
		})
	}
}

func TestParseMigrateLabelsTrimsValues(t *testing.T) {
	t.Parallel()

	labels, err := parseMigrateLabels([]string{" env = prod "})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"env": "prod"}, labels)
}

func TestConfigureMigrateBoundKeypairNotice(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := writeMigrateInput(t, dir)
	var stderr bytes.Buffer
	err := onConfigureMigrate(configureMigrateFlags{
		input:         inputPath,
		installSuffix: "scope",
		output:        "stdout://",
		proxyServer:   "target.example.com:443",
		joinMethod:    string(types.JoinMethodBoundKeypair),
		tokenName:     "scope-migrate-ip-10-2-4-17",
		dataDir:       filepath.Join(dir, "data"),
		stdout:        &bytes.Buffer{},
		stderr:        &stderr,
	})
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "bound_keypair joins require")
}

func writeMigrateInput(t *testing.T, dir string) string {
	t.Helper()
	inputPath := filepath.Join(dir, "teleport.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`
version: v3
teleport:
  auth_token: SUPERSECRET
ssh_service:
  enabled: yes
auth_service:
  enabled: no
proxy_service:
  enabled: no
`), 0o600))
	return inputPath
}

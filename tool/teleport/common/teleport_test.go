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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/server/installer"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

const initTestSentinel = "init_test"

func TestMain(m *testing.M) {
	if slices.Contains(os.Args, initTestSentinel) {
		os.Exit(0)
	}

	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func BenchmarkInit(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping heavy benchmark")
	}
	executable, err := os.Executable()
	require.NoError(b, err)

	for b.Loop() {
		cmd := exec.Command(executable, initTestSentinel)
		err := cmd.Run()
		assert.NoError(b, err)
	}
}

// bootstrap check
func TestTeleportMain(t *testing.T) {
	// get the hostname
	hostname, err := os.Hostname()
	require.NoError(t, err)

	fixtureDir := t.TempDir()
	// generate the fixture config file
	configFile := filepath.Join(fixtureDir, "teleport.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configData), 0660))

	// generate the fixture bootstrap file
	bootstrapEntries := []struct {
		fileName string
		kind     string
		name     string
		content  string
	}{
		{fileName: "role.yaml", kind: types.KindRole, name: "role_name"},
		{fileName: "github.yaml", kind: types.KindGithubConnector, name: "github"},
		{fileName: "user.yaml", kind: types.KindRole, name: "user"},
		{
			fileName: "workload-identity.yaml",
			kind:     types.KindWorkloadIdentity,
			name:     "example-workload-identity",
			content: `kind: workload_identity
version: v1
metadata:
  name: example-workload-identity
spec:
  spiffe:
    id: /svc/example
`,
		},
	}
	var bootstrapData []byte
	for _, entry := range bootstrapEntries {
		data := []byte(entry.content)
		if entry.content == "" {
			fileData, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "resources", entry.fileName))
			require.NoError(t, err)
			data = fileData
		}
		bootstrapData = append(bootstrapData, data...)
		bootstrapData = append(bootstrapData, "\n---\n"...)
	}
	bootstrapFile := filepath.Join(fixtureDir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(bootstrapFile, bootstrapData, 0660))

	// set defaults to test-mode (non-existing files&locations)
	defaults.ConfigFilePath = filepath.Join(fixtureDir, "missing", "etc", "teleport.yaml")
	defaults.DataDir = filepath.Join(fixtureDir, "var", "lib", "teleport")

	t.Run("Default", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start"},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Equal(t, hostname, conf.Hostname)
		require.Equal(t, filepath.Join(fixtureDir, "var", "lib", "teleport"), conf.DataDir)
		require.True(t, conf.Auth.Enabled)
		require.True(t, conf.SSH.Enabled)
		require.True(t, conf.Proxy.Enabled)
		require.True(t, slog.Default().Handler().Enabled(context.Background(), slog.LevelError))
	})

	t.Run("RolesFlag", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node"},
			InitOnly: true,
		})
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		_, cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=proxy"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.True(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		_, cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=auth"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.True(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)
	})

	t.Run("ConfigFile", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node", "--labels=a=a1,b=b1", "--config=" + configFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.True(t, slog.Default().Handler().Enabled(context.Background(), slog.LevelDebug))
		require.Equal(t, "hvostongo.example.org", conf.Hostname)

		token, err := conf.Token()
		require.NoError(t, err)
		require.Equal(t, "xxxyyy", token)
		require.Equal(t, "10.5.5.5", conf.AdvertiseIP)
		require.Equal(t, map[string]string{"a": "a1", "b": "b1"}, conf.SSH.Labels)
	})

	t.Run("Bootstrap", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--bootstrap", bootstrapFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Len(t, bootstrapEntries, len(conf.Auth.BootstrapResources))
		for i, entry := range bootstrapEntries {
			require.Equal(t, entry.kind, conf.Auth.BootstrapResources[i].GetKind(), entry.fileName)
			require.Equal(t, entry.name, conf.Auth.BootstrapResources[i].GetName(), entry.fileName)
			require.NoError(t, services.CheckAndSetDefaults(conf.Auth.BootstrapResources[i]), entry.fileName)
		}
	})
	t.Run("ApplyOnStartup", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--apply-on-startup", bootstrapFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Len(t, bootstrapEntries, len(conf.Auth.ApplyOnStartupResources))
		for i, entry := range bootstrapEntries {
			require.Equal(t, entry.kind, conf.Auth.ApplyOnStartupResources[i].GetKind(), entry.fileName)
			require.Equal(t, entry.name, conf.Auth.ApplyOnStartupResources[i].GetName(), entry.fileName)
			require.NoError(t, services.CheckAndSetDefaults(conf.Auth.ApplyOnStartupResources[i]), entry.fileName)
		}
	})
}

func TestConfigure(t *testing.T) {
	t.Run("Dump", func(t *testing.T) {
		err := onConfigDump(dumpFlags{
			// typo
			output: "sddout",
		})
		require.ErrorAs(t, err, new(*trace.BadParameterError))

		err = onConfigDump(dumpFlags{
			output: "file://" + filepath.Join(t.TempDir(), "test"),
			SampleFlags: config.SampleFlags{
				ClusterName: "example.com",
			},
		})
		require.NoError(t, err)

		// stdout
		err = onConfigDump(dumpFlags{
			output: "stdout",
		})
		require.NoError(t, err)
	})

	t.Run("Defaults", func(t *testing.T) {
		flags := dumpFlags{}
		err := flags.CheckAndSetDefaults()
		require.NoError(t, err)
	})

	t.Run("Suppress output", func(t *testing.T) {
		tempDir := t.TempDir()
		var stdout bytes.Buffer
		err := onConfigDump(dumpFlags{
			SampleFlags: config.SampleFlags{
				Silent: true,
			},
			output: filepath.Join(tempDir, "teleport.yaml"),
			stdout: &stdout,
		})
		require.NoError(t, err)
		require.Empty(t, stdout.Bytes())
	})
}

func TestConfigureCommandParsing(t *testing.T) {
	// Swaps os.Stdout/os.Stderr globally; this test (and anything using
	// captureRunOutput) must not call t.Parallel.
	t.Run("bare configure routes to hidden dump command", func(t *testing.T) {
		_, stderr, command := captureRunOutput(t, []string{"configure"})
		require.Empty(t, stderr)
		require.Equal(t, "configure dump", command)
	})

	t.Run("configure output stdout routes to hidden dump command", func(t *testing.T) {
		_, stderr, command := captureRunOutput(t, []string{"configure", "--output=stdout"})
		require.Empty(t, stderr)
		require.Equal(t, "configure dump", command)
	})

	t.Run("configure test routes to hidden dump command", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "teleport.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("version: v3\nteleport:\n  data_dir: /tmp/teleport\n"), 0o600))
		_, stderr, command := captureRunOutput(t, []string{"configure", "--test=" + configPath})
		require.Contains(t, stderr, "OK "+configPath)
		require.Equal(t, "configure dump", command)
	})

	t.Run("configure migrate routes to migrate command", func(t *testing.T) {
		dir := t.TempDir()
		inputPath := writeMigrateInput(t, dir)
		secretPath := filepath.Join(dir, "token-secret")
		require.NoError(t, os.WriteFile(secretPath, []byte("secret-value"), 0o600))
		_, stderr, command := captureRunOutput(t, []string{
			"configure",
			"--output=stdout",
			"--data-dir=" + filepath.Join(dir, "data"),
			"migrate",
			"--input=" + inputPath,
			"--install-suffix=scope",
			"--proxy-server=target.example.com:443",
			"--token-name=scope-migrate-ip-10-2-4-17",
			"--token-secret-file=" + secretPath,
		})
		require.Contains(t, stderr, "stdout output is redacted")
		require.Equal(t, "configure migrate", command)
	})
}

func TestConfigureHelpHidesDefaultChild(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_CONFIGURE_HELP") == "1" {
		Run(Options{Args: []string{"configure", "--help"}, InitOnly: true})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestConfigureHelpHidesDefaultChild")
	cmd.Env = append(os.Environ(), "TELEPORT_TEST_CONFIGURE_HELP=1")
	out, _ := cmd.CombinedOutput()
	output := string(out)
	require.Contains(t, output, "usage: teleport configure")
	commandsStart := strings.Index(output, "Commands:")
	require.NotEqual(t, -1, commandsStart)
	commands := output[commandsStart:]
	if flagsStart := strings.Index(commands, "\n\nFlags:"); flagsStart >= 0 {
		commands = commands[:flagsStart]
	}
	require.Regexp(t, regexp.MustCompile(`(?m)^\s+migrate\s+`), commands)
	require.NotRegexp(t, regexp.MustCompile(`(?m)^\s+dump\s+`), commands)
}

func TestConfigureMigrateFlagWiring(t *testing.T) {
	// Swaps os.Stdout/os.Stderr globally; this test (and anything using
	// captureRunOutput) must not call t.Parallel.
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "teleport.yaml")
	require.NoError(t, os.WriteFile(inputPath, []byte(`
version: v3
teleport:
  auth_token: SUPERSECRET
ssh_service:
  enabled: yes
app_service:
  enabled: yes
auth_service:
  enabled: no
proxy_service:
  enabled: no
`), 0o600))
	secretPath := filepath.Join(dir, "token-secret")
	require.NoError(t, os.WriteFile(secretPath, []byte("secret-value"), 0o600))

	t.Run("scoped flags disable and label file output", func(t *testing.T) {
		outputPath := filepath.Join(dir, "scoped.yaml")
		_, stderr, command := captureRunOutput(t, []string{
			"configure",
			"--output=file://" + outputPath,
			"migrate",
			"--input=" + inputPath,
			"--install-suffix=scope",
			"--proxy-server=target.example.com:443",
			"--token-name=tok-name",
			"--token-secret-file=" + secretPath,
			"--disable-services=app",
			"--label=scope=target",
		})
		require.Equal(t, "configure migrate", command)
		rendered, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		require.Contains(t, string(rendered), "app_service:\n  enabled: no")
		require.Contains(t, string(rendered), "scope: target")
		require.Contains(t, stderr, "NOTICE: disabled app_service in the migrated config; the original agent continues serving it.")
	})

	t.Run("legacy token file output", func(t *testing.T) {
		outputPath := filepath.Join(dir, "legacy.yaml")
		_, _, command := captureRunOutput(t, []string{
			"configure",
			"--token=legacy-tok-value",
			"--output=file://" + outputPath,
			"--data-dir=" + filepath.Join(dir, "legacy-data"),
			"migrate",
			"--input=" + inputPath,
			"--install-suffix=scope",
			"--proxy-server=target.example.com:443",
		})
		require.Equal(t, "configure migrate", command)
		rendered, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		require.Contains(t, string(rendered), "token_name: legacy-tok-value")
		require.NotContains(t, string(rendered), "token_secret:")
	})

	t.Run("legacy token stdout output", func(t *testing.T) {
		stdout, _, command := captureRunOutput(t, []string{
			"configure",
			"--token=legacy-tok-value",
			"--output=stdout",
			"--data-dir=" + filepath.Join(dir, "legacy-stdout-data"),
			"migrate",
			"--input=" + inputPath,
			"--install-suffix=scope",
			"--proxy-server=target.example.com:443",
		})
		require.Equal(t, "configure migrate", command)
		require.Contains(t, stdout, "token_name: <redacted>")
		require.NotContains(t, stdout, "legacy-tok-value")
	})
}

func captureRunOutput(t *testing.T, args []string) (stdout string, stderr string, command string) {
	t.Helper()
	// Swaps os.Stdout/os.Stderr globally; this test (and anything using
	// captureRunOutput) must not call t.Parallel.

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	require.NoError(t, err)
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	require.NoError(t, err)

	os.Stdout = stdoutFile
	os.Stderr = stderrFile
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	_, command, _ = Run(Options{Args: args, InitOnly: true})
	require.NoError(t, stdoutFile.Close())
	require.NoError(t, stderrFile.Close())

	stdoutRaw, err := os.ReadFile(stdoutFile.Name())
	require.NoError(t, err)
	stderrRaw, err := os.ReadFile(stderrFile.Name())
	require.NoError(t, err)

	return strings.TrimSpace(string(stdoutRaw)), strings.TrimSpace(string(stderrRaw)), command
}

func TestDumpConfigFile(t *testing.T) {
	tt := []struct {
		name      string
		outputURI string
		contents  string
		comment   string
		assert    require.ErrorAssertionFunc
	}{
		{
			name:      "errors on relative path",
			assert:    require.Error,
			outputURI: "../",
		},
		{
			name:      "doesn't error on unexisting config path",
			assert:    require.NoError,
			outputURI: fmt.Sprintf("%s/unexisting/dir/%s", t.TempDir(), "config.yaml"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dumpConfigFile(tc.outputURI, tc.contents, tc.comment)
			tc.assert(t, err)
		})
	}
}

func TestWriteInstallJoinFailureError(t *testing.T) {
	t.Parallel()

	t.Run("all fields populated", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		writeInstallJoinFailureError(&stderr, &installer.JoinFailureError{
			Message:            "node did not become ready (join cluster) within 5m0s",
			ServiceDiagnostics: `systemd service state: ActiveState="failed"`,
			JournalOutput:      "error: token expired",
		})

		out := stderr.String()
		require.Contains(t, out, "ERROR: agent failed to join the cluster\n")
		require.Contains(t, out, "node did not become ready (join cluster) within 5m0s\n")
		require.Contains(t, out, `systemd service state: ActiveState="failed"`)
		require.Contains(t, out, "Journal output:\nerror: token expired\n")
	})

	t.Run("message only", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		writeInstallJoinFailureError(&stderr, &installer.JoinFailureError{
			Message: "node did not become ready (join cluster) within 5m0s",
		})

		out := stderr.String()
		require.Contains(t, out, "ERROR: agent failed to join the cluster\n")
		require.Contains(t, out, "node did not become ready (join cluster) within 5m0s\n")
	})

	t.Run("no journal output", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		writeInstallJoinFailureError(&stderr, &installer.JoinFailureError{
			Message:            "node did not become ready (join cluster) within 5m0s",
			ServiceDiagnostics: "systemd service state: unavailable",
		})

		out := stderr.String()
		require.Contains(t, out, "ERROR: agent failed to join the cluster\n")
		require.Contains(t, out, "node did not become ready (join cluster) within 5m0s\n")
		require.Contains(t, out, "systemd service state: unavailable\n")
		require.NotContains(t, out, "Journal output:")
	})
}

const configData = `
version: v3
teleport:
  advertise_ip: 10.5.5.5
  nodename: hvostongo.example.org
  auth_server: auth.server.example.org:3024
  auth_token: xxxyyy
  log:
    output: stderr
    severity: DEBUG
  connection_limits:
    max_connections: 90
    max_users: 91
    rates:
    - period: 1m1s
      average: 70
      burst: 71
    - period: 10m10s
      average: 170
      burst: 171

auth_service:
  enabled: yes
  listen_addr: tcp://auth

ssh_service:
  enabled: no
  listen_addr: tcp://ssh
  labels:
    name: mondoserver
    role: follower
  commands:
  - name: hostname
    command: [/bin/hostname]
    period: 10ms
  - name: date
    command: [/bin/date]
    period: 20ms
`

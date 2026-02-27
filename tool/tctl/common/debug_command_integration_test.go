// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// shortTempDir creates a temp dir with a short path to avoid Unix socket path
// length limits (104-108 chars on macOS/Linux).
func shortTempDir(t *testing.T, prefix string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll("./tmp", 0o755))
	dir, err := os.MkdirTemp("./tmp", prefix)
	require.NoError(t, err)
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(abs) })
	return abs
}

// newAuthClientNoBreaker creates an admin auth client with the circuit breaker
// disabled. This is needed for integration tests that poll RPCs which may fail
// temporarily (e.g. before a reverse tunnel is established).
func newAuthClientNoBreaker(t *testing.T, process *service.TeleportProcess) *authclient.Client {
	t.Helper()
	cfg := process.Config
	identity, err := storage.ReadLocalIdentityForRole(
		filepath.Join(cfg.DataDir, teleport.ComponentProcess),
		types.RoleAdmin,
	)
	require.NoError(t, err)
	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)
	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Logger
	authConfig.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)
	return client
}

// waitForDebugReady polls until a debug command (get-log-level) succeeds
// against the given server ID. This confirms the tunnel and HTTP debug
// service are established.
func waitForDebugReady(t *testing.T, ctx context.Context, client *authclient.Client, serverID string) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	var lastErr error
	for {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"get-log-level", serverID})
		if err == nil {
			return
		}
		lastErr = err
		t.Logf("waitForDebugReady(%s): %v", serverID, err)
		select {
		case <-time.After(500 * time.Millisecond):
		case <-deadline:
			t.Fatalf("debug service for %s not ready within 30s: %v", serverID, lastErr)
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for debug service for %s", serverID)
		}
	}
}

// runDebugCmd creates a DebugCommand with the given output writers and
// fields, then parses args and runs via TryRun.
func runDebugCmd(t *testing.T, ctx context.Context, client *authclient.Client, cmd *DebugCommand, args []string) error {
	t.Helper()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	app := utils.InitCLIParser("tctl", GlobalHelpString)
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	args = append([]string{"debug"}, args...)
	selectedCmd, err := app.Parse(args)
	require.NoError(t, err)

	_, err = cmd.TryRun(ctx, selectedCmd, func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		return client, func(context.Context) {}, nil
	})
	return err
}

func TestDebugCommandIntegration(t *testing.T) {
	// Speed up instance heartbeats so hostname resolution works in tests.
	apidefaults.SetTestTimeouts(3*time.Second, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Start a combined auth+proxy+node process with debug service enabled.
	// The debug service on the auth side tunnels HTTP to the node's debug
	// HTTP server via a local in-process connection (LazyLocalDebugDialer).
	authProcess, err := testenv.NewTeleportProcess(shortTempDir(t, "auth-"),
		testenv.WithLogger(logtest.NewLogger()),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.DebugService.Enabled = true
			cfg.LogBroadcaster = logutils.NewLogBroadcaster()
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	// Get an admin auth client with the circuit breaker disabled.
	client := newAuthClientNoBreaker(t, authProcess)
	t.Cleanup(func() { _ = client.Close() })

	// Get the server's host UUID.
	serverID, err := authProcess.WaitForHostID(ctx)
	require.NoError(t, err)

	// Wait for the debug service to be reachable.
	waitForDebugReady(t, ctx, client, serverID)

	t.Run("GetLogLevel", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"get-log-level", serverID})
		require.NoError(t, err)
		out := strings.TrimSpace(stdout.String())
		assert.NotEmpty(t, out, "should return a log level")
		validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR"}
		found := false
		for _, l := range validLevels {
			if strings.EqualFold(out, l) {
				found = true
				break
			}
		}
		assert.True(t, found, "unexpected log level %q", out)
	})

	t.Run("SetLogLevel", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"set-log-level", serverID, "DEBUG"})
		require.NoError(t, err)
		out := strings.TrimSpace(stdout.String())
		assert.Contains(t, out, "DEBUG", "should mention the new level")

		// Verify it was actually set.
		var verifyBuf bytes.Buffer
		verifyCmd := &DebugCommand{stdout: &verifyBuf}
		err = runDebugCmd(t, ctx, client, verifyCmd, []string{"get-log-level", serverID})
		require.NoError(t, err)
		assert.Equal(t, "DEBUG", strings.TrimSpace(verifyBuf.String()))
	})

	t.Run("Readyz", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"readyz", serverID})
		require.NoError(t, err)
		out := stdout.String()
		assert.Contains(t, out, "PID")
		assert.Contains(t, out, "status:")
	})

	t.Run("Metrics", func(t *testing.T) {
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			var stdout bytes.Buffer
			cmd := &DebugCommand{stdout: &stdout}
			err := runDebugCmd(t, ctx, client, cmd, []string{"metrics", serverID})
			assert.NoError(collect, err)
			out := stdout.String()
			assert.Contains(collect, out, "# HELP", "should contain Prometheus metric help text")
			assert.Contains(collect, out, "# TYPE", "should contain Prometheus metric type")
		}, 10*time.Second, time.Second)
	})

	t.Run("Profile", func(t *testing.T) {
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			profileDir := t.TempDir()
			var stderr bytes.Buffer
			cmd := &DebugCommand{
				stderr:     &stderr,
				profileDir: profileDir,
			}
			err := runDebugCmd(t, ctx, client, cmd, []string{"profile", serverID, "--seconds=1", "goroutine"})
			if !assert.NoError(collect, err) {
				return
			}

			// Verify profile file was created.
			pattern := filepath.Join(profileDir, "*-goroutine.pb.gz")
			files, err := filepath.Glob(pattern)
			if !assert.NoError(collect, err) {
				return
			}
			if !assert.Len(collect, files, 1, "expected exactly one goroutine profile file") {
				return
			}

			info, err := os.Stat(files[0])
			assert.NoError(collect, err)
			assert.Greater(collect, info.Size(), int64(50), "profile should have meaningful content")
		}, 10*time.Second, time.Second)
	})

	t.Run("LogStream", func(t *testing.T) {
		// Open a Connect tunnel and stream logs via HTTP.
		// Inject a log entry into the broadcaster and verify it arrives.
		streamCtx, streamCancel := context.WithCancel(ctx)
		defer streamCancel()

		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout, serverID: serverID}
		debugClt, cleanup, err := cmd.debugClient(streamCtx, client)
		require.NoError(t, err)
		defer cleanup()

		// Start reading the log-stream in a goroutine.
		type result struct {
			lines []string
			err   error
		}
		resultC := make(chan result, 1)
		go func() {
			body, err := debugClt.GetLogStream(streamCtx, "")
			if err != nil {
				resultC <- result{err: err}
				return
			}
			defer body.Close()

			var lines []string
			scanner := json.NewDecoder(body)
			for scanner.More() {
				var obj map[string]any
				if err := scanner.Decode(&obj); err != nil {
					break
				}
				msg, _ := obj["message"].(string)
				lines = append(lines, msg)
				if msg == "integration-test-marker" {
					resultC <- result{lines: lines}
					return
				}
			}
			resultC <- result{lines: lines}
		}()

		// The stream traverses the Connect tunnel so the subscription
		// may not be registered yet. Poll-broadcast until the marker arrives.
		broadcaster := authProcess.Config.LogBroadcaster
		marker := &debugpb.LogEntry{Level: "INFO", Message: "integration-test-marker"}
		deadline := time.After(10 * time.Second)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			broadcaster.Broadcast(marker, slog.LevelInfo)
			select {
			case r := <-resultC:
				require.NoError(t, r.err)
				require.Contains(t, r.lines, "integration-test-marker")
				return
			case <-ticker.C:
				continue
			case <-deadline:
				t.Fatal("did not receive any log entry within 10s")
				return
			}
		}
	})

	t.Run("HostnameResolution", func(t *testing.T) {
		// Hostname resolution requires the instance to be heartbeated
		// to the backend. Poll until the instance appears.
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			var stdout bytes.Buffer
			cmd := &DebugCommand{stdout: &stdout}
			err := runDebugCmd(t, ctx, client, cmd, []string{"get-log-level", "server01"})
			assert.NoError(collect, err)
			assert.NotEmpty(collect, strings.TrimSpace(stdout.String()))
		}, 30*time.Second, 500*time.Millisecond, "hostname resolution should work")
	})
}

// TestDebugCommandSplitDeployment verifies the debug service works in
// auth-only deployments without a proxy or reverse tunnel.
func TestDebugCommandSplitDeployment(t *testing.T) {
	apidefaults.SetTestTimeouts(3*time.Second, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Start an auth-only process (no proxy, no node) with debug enabled.
	authProcess, err := testenv.NewTeleportProcess(shortTempDir(t, "auth-"),
		testenv.WithLogger(logtest.NewLogger()),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Proxy.Enabled = false
			cfg.SSH.Enabled = false
			cfg.DebugService.Enabled = true
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	client := newAuthClientNoBreaker(t, authProcess)
	t.Cleanup(func() { _ = client.Close() })

	authID, err := authProcess.WaitForHostID(ctx)
	require.NoError(t, err)

	// Wait for the debug service to become reachable via self-targeting
	// (LazyLocalDebugDialer handles in-process connections).
	waitForDebugReady(t, ctx, client, authID)

	t.Run("SelfTarget/GetLogLevel", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"get-log-level", authID})
		require.NoError(t, err)
		out := strings.TrimSpace(stdout.String())
		assert.NotEmpty(t, out, "should return a log level")
	})

	t.Run("SelfTarget/SetLogLevel", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"set-log-level", authID, "DEBUG"})
		require.NoError(t, err)
		out := strings.TrimSpace(stdout.String())
		assert.Contains(t, out, "DEBUG", "should mention the new level")

		// Verify it was actually set.
		var verifyBuf bytes.Buffer
		verifyCmd := &DebugCommand{stdout: &verifyBuf}
		err = runDebugCmd(t, ctx, client, verifyCmd, []string{"get-log-level", authID})
		require.NoError(t, err)
		assert.Equal(t, "DEBUG", strings.TrimSpace(verifyBuf.String()))
	})

	t.Run("SelfTarget/Readyz", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := &DebugCommand{stdout: &stdout}
		err := runDebugCmd(t, ctx, client, cmd, []string{"readyz", authID})
		require.NoError(t, err)
		out := stdout.String()
		assert.Contains(t, out, "PID")
		assert.Contains(t, out, "status:")
	})
}

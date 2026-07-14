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

package tbot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/tbot/services/application"
	"github.com/gravitational/teleport/lib/tbot/services/database"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

type tunnelRecoveryTestBot struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func startTunnelRecoveryTestBot(t *testing.T, parent context.Context, bot *Bot) *tunnelRecoveryTestBot {
	t.Helper()

	ctx, cancel := context.WithCancel(parent)
	run := &tunnelRecoveryTestBot{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() {
		run.err = bot.Run(ctx)
		close(run.done)
	}()

	t.Cleanup(func() {
		run.cancel()
		select {
		case <-run.done:
			if !t.Failed() {
				assert.NoError(t, run.err, "bot should exit cleanly")
			}
		case <-time.After(10 * time.Second):
			assert.Fail(t, "bot did not stop within 10 seconds")
		}
	})
	return run
}

func (r *tunnelRecoveryTestBot) requireRunning(t *testing.T) {
	t.Helper()

	select {
	case <-r.done:
		require.Fail(t, "bot exited unexpectedly", "error: %v", r.err)
	default:
	}
}

func newTunnelRecoveryDiagnosticsClient(t *testing.T, socketPath string) *http.Client {
	t.Helper()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Second,
	}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func waitForTunnelRecoveryServiceStatus(
	ctx context.Context,
	run *tunnelRecoveryTestBot,
	client *http.Client,
	serviceName string,
	want readyz.Status,
) (*readyz.ServiceStatus, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastStatus *readyz.ServiceStatus
	var lastErr error
	for {
		select {
		case <-run.done:
			return nil, fmt.Errorf("tbot exited before %q became %s: %w", serviceName, want, run.err)
		default:
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://unix/readyz/"+serviceName,
			nil,
		)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil {
			status := new(readyz.ServiceStatus)
			decodeErr := json.NewDecoder(resp.Body).Decode(status)
			resp.Body.Close()
			if decodeErr == nil {
				lastStatus = status
				if status.Status == want {
					return status, nil
				}
			} else {
				lastErr = decodeErr
			}
		} else {
			lastErr = err
		}

		select {
		case <-run.done:
			return nil, fmt.Errorf("tbot exited before %q became %s: %w", serviceName, want, run.err)
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"waiting for %q to become %s (last status: %v, last error: %v): %w",
				serviceName,
				want,
				lastStatus,
				lastErr,
				ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

// TestBotTunnelServicesRecoverWhenServersAppear verifies recovery when existing
// resources begin to be served after tbot starts. Label matching exercises the
// real agent reconciliation and heartbeat paths, but not cold agent joining.
func TestBotTunnelServicesRecoverWhenServersAppear(t *testing.T) {
	t.Parallel()

	const (
		matcherLabel       = "tbot-test"
		matcherValue       = "enabled"
		appName            = "delayed-app"
		databaseService    = "delayed-database"
		databaseName       = "testdb"
		databaseUser       = "llama"
		appTunnelName      = "delayed-app-tunnel"
		databaseTunnelName = "delayed-database-tunnel"
	)

	ctx := t.Context()
	log := logtest.NewLogger()
	matcher := services.ResourceMatcher{
		Labels: types.Labels{matcherLabel: []string{matcherValue}},
	}

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(log),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Apps.Enabled = true
			cfg.Apps.ResourceMatchers = []services.ResourceMatcher{matcher}
			cfg.Databases.Enabled = true
			cfg.Databases.ResourceMatchers = []services.ResourceMatcher{matcher}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })

	wantStatus := http.StatusTeapot
	wantBody := "application tunnel response"
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(wantStatus)
		_, _ = io.WriteString(w, wantBody)
	}))
	t.Cleanup(httpServer.Close)

	postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: rootClient,
		Users:      []string{databaseUser},
	})
	require.NoError(t, err)
	postgresDone := make(chan error, 1)
	go func() {
		postgresDone <- postgresServer.Serve()
	}()
	t.Cleanup(func() {
		postgresServer.Close()
		select {
		case err := <-postgresDone:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			assert.Fail(t, "Postgres test server did not stop within 10 seconds")
		}
	})

	app, err := types.NewAppV3(types.Metadata{
		Name: appName,
		Labels: map[string]string{
			matcherLabel: "disabled",
		},
	}, types.AppSpecV3{
		URI:        httpServer.URL,
		PublicAddr: appName + ".example.com",
	})
	require.NoError(t, err)
	require.NoError(t, rootClient.CreateApp(ctx, app))

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: databaseService,
		Labels: map[string]string{
			matcherLabel: "disabled",
		},
	}, types.DatabaseSpecV3{
		Protocol: libdefaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresServer.Port()),
	})
	require.NoError(t, err)
	require.NoError(t, rootClient.CreateDatabase(ctx, db))

	role, err := types.NewRole("tunnel-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseNames: []string{databaseName},
			DatabaseUsers: []string{databaseUser},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	appListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = appListener.Close() })
	databaseListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = databaseListener.Close() })

	diagDir, err := os.MkdirTemp("", "tbot-tunnel-recovery-")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(diagDir)) })
	diagSocketPath := filepath.Join(diagDir, "diag.sock")

	onboarding, _ := makeBot(t, rootClient, "tunnel-recovery", role.GetName())
	botConfig := defaultBotConfig(
		t,
		process,
		onboarding,
		config.ServiceConfigs{
			&application.TunnelConfig{
				Name:     appTunnelName,
				Listener: appListener,
				AppName:  appName,
			},
			&database.TunnelConfig{
				Name:     databaseTunnelName,
				Listener: databaseListener,
				Service:  databaseService,
				Database: databaseName,
				Username: databaseUser,
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		},
	)
	botConfig.Oneshot = false
	botConfig.DiagSocketForUpdater = diagSocketPath

	run := startTunnelRecoveryTestBot(t, ctx, New(botConfig, log))
	diagnosticsClient := newTunnelRecoveryDiagnosticsClient(t, diagSocketPath)

	// Before startup retries are implemented, either tunnel may return its
	// missing-server error first and cancel the whole bot. The done-channel
	// error from this wait is the intended pre-fix regression signal; do not
	// depend on which tunnel wins that race.
	statusCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	appStatus, err := waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, appTunnelName, readyz.Unhealthy,
	)
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, appStatus.Reason)
	require.Contains(t, appStatus.Reason, appName)

	statusCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	databaseStatus, err := waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, databaseTunnelName, readyz.Unhealthy,
	)
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, databaseStatus.Reason)
	require.Contains(t, databaseStatus.Reason, databaseService)
	run.requireRunning(t)

	app.SetStaticLabels(map[string]string{matcherLabel: matcherValue})
	require.NoError(t, rootClient.UpdateApp(ctx, app))
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		servers, err := rootClient.GetApplicationServers(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		for _, server := range servers {
			if server.GetApp().GetName() == appName {
				return
			}
		}
		require.Fail(t, "application server heartbeat not found")
	}, 30*time.Second, 100*time.Millisecond)

	statusCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	appStatus, err = waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, appTunnelName, readyz.Healthy,
	)
	cancel()
	require.NoError(t, err)
	require.Empty(t, appStatus.Reason)

	statusCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	databaseStatus, err = waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, databaseTunnelName, readyz.Unhealthy,
	)
	cancel()
	require.NoError(t, err)
	require.Contains(t, databaseStatus.Reason, databaseService)
	run.requireRunning(t)

	httpClient := &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   2 * time.Second,
	}
	t.Cleanup(httpClient.CloseIdleConnections)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := httpClient.Get("http://" + appListener.Addr().String())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, wantStatus, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, wantBody, string(body))
	}, 30*time.Second, 100*time.Millisecond)

	db.SetStaticLabels(map[string]string{matcherLabel: matcherValue})
	require.NoError(t, rootClient.UpdateDatabase(ctx, db))
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		servers, err := rootClient.GetDatabaseServers(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		for _, server := range servers {
			if server.GetDatabase().GetName() == databaseService {
				return
			}
		}
		require.Fail(t, "database server heartbeat not found")
	}, 30*time.Second, 100*time.Millisecond)

	statusCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	databaseStatus, err = waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, databaseTunnelName, readyz.Healthy,
	)
	cancel()
	require.NoError(t, err)
	require.Empty(t, databaseStatus.Reason)
	run.requireRunning(t)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		conn, err := pgconn.Connect(
			attemptCtx,
			fmt.Sprintf(
				"postgres://%s/%s?user=%s",
				databaseListener.Addr().String(),
				databaseName,
				databaseUser,
			),
		)
		require.NoError(t, err)
		defer conn.Close(attemptCtx)
		results, err := conn.Exec(attemptCtx, "SELECT 1;").ReadAll()
		require.NoError(t, err)
		require.NotEmpty(t, results)
	}, 30*time.Second, 100*time.Millisecond)

	statusCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	appStatus, err = waitForTunnelRecoveryServiceStatus(
		statusCtx, run, diagnosticsClient, appTunnelName, readyz.Healthy,
	)
	cancel()
	require.NoError(t, err)
	require.Empty(t, appStatus.Reason)
	run.requireRunning(t)

}

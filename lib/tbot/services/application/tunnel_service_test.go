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

package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	jointoken "github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestE2E_ApplicationTunnelService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()

	// Spin up a test HTTP server
	wantStatus := http.StatusTeapot
	wantBody := []byte("hello this is a test")
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(wantStatus)
		w.Write(wantBody)
	}))
	t.Cleanup(httpSrv.Close)

	// Make a new auth server.
	appName := "my-test-app"
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(log),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: appName,
					URI:  httpSrv.URL,
				},
			}
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

	// Create role that allows the bot to access the app.
	role, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(t.Context(), role)
	require.NoError(t, err)

	botListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		botListener.Close()
	})

	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}
	b, err := bot.New(bot.Config{
		Connection: connCfg,
		Logger:     log,
		Onboarding: *onboarding,
		Services: []bot.ServiceBuilder{
			TunnelServiceBuilder(
				&TunnelConfig{
					Listener: botListener,
					AppName:  appName,
				},
				connCfg,
				bot.DefaultCredentialLifetime,
				time.Minute,
			),
		},
	})
	require.NoError(t, err)

	// Spin up goroutine for bot to run in
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancel()
		wg.Wait()
	})

	// We can't predict exactly when the tunnel will be ready so we use
	// EventuallyWithT to retry.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		proxyUrl := url.URL{
			Scheme: "http",
			Host:   botListener.Addr().String(),
		}
		resp, err := http.Get(proxyUrl.String())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, wantStatus, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, wantBody, body)
	}, 10*time.Second, 100*time.Millisecond)
}

func makeTunnelRequest(t *testing.T, botListener net.Listener, wantStatus int, wantBody []byte) {
	t.Helper()

	// Need a custom client: the default http.Client will "helpfully" reuse the
	// connection and keep our `OnNewConnectionFunc` from triggering, preventing
	// cert refreshes.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		proxyUrl := url.URL{
			Scheme: "http",
			Host:   botListener.Addr().String(),
		}
		resp, err := client.Get(proxyUrl.String())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, wantStatus, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, wantBody, body)
	}, 10*time.Second, 100*time.Millisecond)
}

func TestE2E_ApplicationTunnelService_Leeway(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()
	clock := clockwork.NewFakeClockAt(time.Now())

	// Spin up a test HTTP server
	wantStatus := http.StatusTeapot
	wantBody := []byte("hello this is a test")
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(wantStatus)
		w.Write(wantBody)
	}))
	t.Cleanup(httpSrv.Close)

	// Make a new auth server.
	appName := "my-test-app"
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(log),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: appName,
					URI:  httpSrv.URL,
				},
			}
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

	// Create role that allows the bot to access the app.
	role, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(t.Context(), role)
	require.NoError(t, err)

	botListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		botListener.Close()
	})

	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	certsIssued := atomic.Uint32{}

	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}
	b, err := bot.New(bot.Config{
		Connection: connCfg,
		Logger:     log,
		Onboarding: *onboarding,
		Services: []bot.ServiceBuilder{
			TunnelServiceBuilder(
				&TunnelConfig{
					Listener: botListener,
					AppName:  appName,
					clock:    clock,
					certIssuedHook: func() {
						t.Logf("!! new cert issued")
						certsIssued.Add(1)
					},
				},
				connCfg,
				bot.DefaultCredentialLifetime,
				5*time.Minute,
			),
		},
	})
	require.NoError(t, err)

	// Spin up goroutine for bot to run in
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancel()
		wg.Wait()
	})

	// One cert should be issued at startup (eventually).
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.EqualValues(t, 1, certsIssued.Load())
	}, 10*time.Second, 20*time.Millisecond)

	// Make a first request. It should not cause a new cert to be issued.
	makeTunnelRequest(t, botListener, wantStatus, wantBody)
	require.EqualValues(t, 1, certsIssued.Load())

	// Advance the clock a bit and try again. No cert should be issued (<TTL)
	clock.Advance(bot.DefaultCredentialLifetime.RenewalInterval)
	makeTunnelRequest(t, botListener, wantStatus, wantBody)
	require.EqualValues(t, 1, certsIssued.Load())

	// Advance the clock into the leeway period, and try once more. A new cert
	// should be issued.
	clock.Advance(bot.DefaultCredentialLifetime.TTL - bot.DefaultCredentialLifetime.RenewalInterval - time.Minute)
	makeTunnelRequest(t, botListener, wantStatus, wantBody)
	require.EqualValues(t, 2, certsIssued.Load())
}

func TestTunnelService_Run_CancellationDuringRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pinger := &fakePinger{err: errors.New("proxy unreachable")}
		registry := readyz.NewRegistry()
		reporter := registry.AddService("application-tunnel", "test")
		readyCh := make(chan struct{})
		close(readyCh)

		listener, err := net.Listen("tcp", "127.0.0.1:")
		require.NoError(t, err)

		svc := &TunnelService{
			cfg:                &TunnelConfig{AppName: "test", Listener: listener},
			proxyPinger:        pinger,
			botIdentityReadyCh: readyCh,
			statusReporter:     reporter,
			log:                logtest.NewLogger(),
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait()

		// Verify that the reporter wiring is correct and it's reporting Unhealthy
		status, ok := registry.ServiceStatus("test")
		require.True(t, ok)
		require.Equal(t, readyz.Unhealthy, status.Status)
		require.Contains(t, status.Reason, "proxy unreachable")

		// Verify that Run has not returned, therefore it's retrying.
		select {
		case <-done:
			require.Fail(t, "Run returned instead of retrying")
		default:
		}

		cancel()
		synctest.Wait()

		// Verify that context cancellation is not propagating an error
		// from the retry wrapper.
		require.NoError(t, <-done)
	})
}

type fakePinger struct {
	err error
}

func (p fakePinger) Ping(_ context.Context) (*connection.ProxyPong, error) {
	return nil, p.err
}

// TestE2E_ScopedApplicationTunnelService tests that the application tunnel
// service works in scoped mode, issuing scoped app certs and proxying
// traffic through to the backend.
func TestE2E_ScopedApplicationTunnelService(t *testing.T) {
	if !scopes.FeaturesFromEnv().AgentPinEnabled {
		t.Skip("test requires TELEPORT_UNSTABLE_AGENT_SCOPE_PIN=yes")
	}
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()

	const (
		scopeName      = "/test-scope"
		scopedRoleName = "scoped-app-access"
		botName        = "scoped-tunnel-bot"
		appName        = "scoped-tunnel-app"
	)

	// Spin up a test HTTP server.
	wantStatus := http.StatusTeapot
	wantBody := []byte("hello from scoped tunnel")
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(wantStatus)
		w.Write(wantBody)
	}))
	t.Cleanup(httpSrv.Close)

	// Start the main Teleport process (auth + proxy, no app service).
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(log),
		testenv.WithScopesFeatures(scopes.Features{Enabled: true, AgentPinEnabled: true}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })

	// Create a scoped role that grants app access.
	makeScopedRole(t, ctx, rootClient, scopedRoleName, scopeName)

	// Create the scoped bot, token, and role assignment.
	botOnboarding := makeScopedBot(t, process, rootClient, botName, scopeName, scopedRoleName)

	// Start a scoped app agent.
	appTokenResp := makeScopedAppAgent(t, ctx, process, log, scopeName, appName, httpSrv.URL)
	_ = appTokenResp

	// Wait for the app to be visible.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		servers, err := rootClient.GetApplicationServers(ctx, "default")
		if !assert.NoError(ct, err) {
			return
		}
		for _, s := range servers {
			if s.GetApp().GetName() == appName {
				return
			}
		}
		assert.Fail(ct, "scoped app not yet visible")
	}, 10*time.Second, 100*time.Millisecond)

	// Configure and start the scoped bot with the tunnel service.
	botListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { botListener.Close() })

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)
	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}

	b, err := bot.New(bot.Config{
		Connection: connCfg,
		Logger:     log,
		Onboarding: *botOnboarding,
		Scoped:     true,
		Services: []bot.ServiceBuilder{
			TunnelServiceBuilder(
				&TunnelConfig{
					Listener: botListener,
					AppName:  appName,
				},
				connCfg,
				bot.DefaultCredentialLifetime,
				time.Minute,
			),
		},
	})
	require.NoError(t, err)

	// Run bot in background.
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Go(func() {
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	})
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	// Verify the tunnel proxies traffic to the backend.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := http.Get("http://" + botListener.Addr().String())
		if !assert.NoError(t, err) {
			return
		}
		defer resp.Body.Close()
		assert.Equal(t, wantStatus, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, wantBody, body)
	}, 10*time.Second, 100*time.Millisecond)
}

// makeScopedBot creates a scoped bot with a bound keypair token and a scoped
// role assignment, returning the onboarding config for tbot.
func makeScopedBot(
	t *testing.T,
	process *service.TeleportProcess,
	rootClient *authclient.Client,
	botName, scopeName, scopedRoleName string,
) *onboarding.Config {
	t.Helper()
	ctx := t.Context()

	_, err := rootClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
		Bot: machineidv1pb.Bot_builder{
			Metadata: headerv1.Metadata_builder{Name: botName}.Build(),
			Scope:    scopeName,
			Spec:     &machineidv1pb.BotSpec{},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	botKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	botPublicKey := strings.TrimSpace(string(botKey.MarshalSSHPublicKey()))
	botKeyPath := filepath.Join(t.TempDir(), "bot_key.pem")
	require.NoError(t, os.WriteFile(botKeyPath, botKey.PrivateKeyPEM(), 0600))

	botTokenResp, err := process.GetAuthServer().ScopedTokenService.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: joiningv1.ScopedToken_builder{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: botName + "-token"}.Build(),
			Scope:    scopeName,
			Spec: joiningv1.ScopedTokenSpec_builder{
				Roles:      []string{types.RoleBot.String()},
				JoinMethod: string(types.JoinMethodBoundKeypair),
				UsageMode:  jointoken.TokenUsageModeBot,
				Bot:        scopes.QualifiedName{Scope: scopeName, Name: botName}.String(),
				BoundKeypair: joiningv1.BoundKeypairSpec_builder{
					Onboarding: joiningv1.BoundKeypairSpec_OnboardingSpec_builder{
						InitialPublicKey: botPublicKey,
					}.Build(),
					Recovery: joiningv1.BoundKeypairSpec_RecoverySpec_builder{
						Limit: 10,
						Mode:  "insecure",
					}.Build(),
				}.Build(),
			}.Build(),
			Status: joiningv1.ScopedTokenStatus_builder{
				Usage: joiningv1.UsageStatus_builder{
					BoundKeypair: &joiningv1.BoundKeypairStatus{},
				}.Build(),
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	sraResp, err := rootClient.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:     scopedaccess.KindScopedRoleAssignment,
			Version:  types.V1,
			SubKind:  scopedaccess.SubKindDynamic,
			Metadata: headerv1.Metadata_builder{Name: uuid.NewString()}.Build(),
			Scope:    scopeName,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: scopes.QualifiedName{Scope: scopeName, Name: botName}.String(),
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  scopes.QualifiedName{Scope: scopeName, Name: scopedRoleName}.String(),
						Scope: scopeName,
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Wait for the SRA to be visible in the cache.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		_, err := process.GetAuthServer().ScopedAccessCache.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    sraResp.GetAssignment().GetMetadata().GetName(),
			SubKind: sraResp.GetAssignment().GetSubKind(),
			Scope:   sraResp.GetAssignment().GetScope(),
		}.Build())
		assert.NoError(ct, err)
	}, 10*time.Second, 100*time.Millisecond)

	return &onboarding.Config{
		TokenValue: scopes.QualifiedName{Scope: scopeName, Name: botTokenResp.GetToken().GetMetadata().GetName()}.String(),
		JoinMethod: types.JoinMethodBoundKeypair,
		BoundKeypair: onboarding.BoundKeypairOnboardingConfig{
			StaticPrivateKeyPath: botKeyPath,
		},
	}
}

// makeScopedAppAgent starts a Teleport process as an app-only agent, joining
// with a scoped token so the app is visible to scoped bots.
func makeScopedAppAgent(
	t *testing.T,
	ctx context.Context,
	process *service.TeleportProcess,
	log *slog.Logger,
	scopeName, appName, appURI string,
) *joiningv1.CreateScopedTokenResponse {
	t.Helper()

	tokenResp, err := process.GetAuthServer().ScopedTokenService.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
		Token: joiningv1.ScopedToken_builder{
			Kind:     types.KindScopedToken,
			Version:  types.V1,
			Metadata: headerv1.Metadata_builder{Name: appName + "-agent-token"}.Build(),
			Scope:    scopeName,
			Spec: joiningv1.ScopedTokenSpec_builder{
				AssignedScope: scopeName,
				Roles:         []string{types.RoleApp.String()},
				JoinMethod:    string(types.JoinMethodToken),
				UsageMode:     string(jointoken.TokenUsageModeUnlimited),
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	agentCfg := servicecfg.MakeDefaultConfig()
	agentCfg.ScopesFeatures = scopes.Features{Enabled: true, AgentPinEnabled: true}
	agentCfg.Hostname = appName + "-agent"
	agentCfg.DataDir = t.TempDir()
	agentCfg.SetToken(scopes.QualifiedName{
		Scope: scopeName,
		Name:  jointoken.EncodeScopedToken(tokenResp.GetToken().GetMetadata().GetName(), tokenResp.GetToken().GetStatus().GetSecret()),
	}.String())
	agentCfg.SetAuthServerAddress(*proxyAddr)
	agentCfg.InsecureMode = true
	agentCfg.Auth.Enabled = false
	agentCfg.Proxy.Enabled = false
	agentCfg.SSH.Enabled = false
	agentCfg.Apps.Enabled = true
	agentCfg.Apps.Apps = []servicecfg.App{
		{
			Name: appName,
			URI:  appURI,
		},
	}
	agentCfg.CachePolicy.Enabled = false
	agentCfg.InstanceMetadataClient = nil
	agentCfg.DebugService.Enabled = false
	agentCfg.Logger = log

	agentProcess, err := service.NewTeleport(agentCfg)
	require.NoError(t, err)
	require.NoError(t, agentProcess.Start())
	t.Cleanup(func() {
		require.NoError(t, agentProcess.Close())
		require.NoError(t, agentProcess.Wait())
	})

	_, err = agentProcess.WaitForEvent(ctx, service.AppsReady)
	require.NoError(t, err)

	return tokenResp
}

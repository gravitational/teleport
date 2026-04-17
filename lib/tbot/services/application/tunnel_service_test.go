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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
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
		if !assert.NoError(t, err) {
			return
		}
		defer resp.Body.Close()
		assert.Equal(t, wantStatus, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, wantBody, body)
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

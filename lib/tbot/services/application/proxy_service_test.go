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

package application

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// Struct to encapsulate the request the HTTP Test Server got.
// It contains the request and the extracted body, since the default golang HTTP Server closes the body io stream.
type proxyReqRes struct {
	request *http.Request
	body    []byte
}

func TestE2E_ApplicationProxyService(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	log := logtest.NewLogger()

	// Spin up a test HTTP server
	receivedRequestsCh := make(chan proxyReqRes, 1)

	// Spin up 2 servers
	httpSrvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the requestBody
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		// Copy the request and the received body into the channel.
		// We do this because the default http server closes the body io stream to clear the memory.
		receivedRequestsCh <- proxyReqRes{
			request: r,
			body:    requestBody,
		}

		w.Header().Set("X-From-Server", "server-a-header-value")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello from server a"))
	}))
	t.Cleanup(httpSrvA.Close)

	httpSrvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the requestBody
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		// Copy the request and the received body into the channel.
		// We do this because the default http server closes the body io stream to clear the memory.
		receivedRequestsCh <- proxyReqRes{
			request: r,
			body:    requestBody,
		}
		w.Header().Set("X-From-Server", "server-b-header-value")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello from server b"))
	}))
	t.Cleanup(httpSrvB.Close)

	// Make a new auth server.
	appNameA := "app-a"
	appNameB := "app-b"

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(log),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: appNameA,
					URI:  httpSrvA.URL,
				},
				{
					Name: appNameB,
					URI:  httpSrvB.URL,
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

	proxyServiceConfig := &ProxyServiceConfig{
		Listen:   "localhost:12345",
		Listener: botListener,
	}

	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}
	alpnUpgradeCache := internal.NewALPNUpgradeCache(log)
	b, err := bot.New(bot.Config{
		Connection: connCfg,
		Logger:     log,
		Onboarding: *onboarding,
		Services: []bot.ServiceBuilder{
			ProxyServiceBuilder(
				proxyServiceConfig,
				connCfg,
				bot.DefaultCredentialLifetime,
				alpnUpgradeCache,
			),
		},
	})
	require.NoError(t, err)

	// Spin up goroutine for bot to run in
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Go(func() {
		err := b.Run(ctx)
		assert.NoError(t, err, "bot should not exit with error")
		cancel()
	})
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancel()
		wg.Wait()
	})

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   botListener.Addr().String(),
			}),
		},
	}

	// We can't predict exactly when the tunnel will be ready so we use
	// EventuallyWithT to retry.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requestUrl, _ := url.Parse("http://" + appNameA)
		request := &http.Request{
			URL: requestUrl,
		}
		resp, err := httpClient.Do(request)
		if !assert.NoError(t, err) {
			return
		}
		defer resp.Body.Close()
		// Drain channel since we don't care to much about this first request.
		<-receivedRequestsCh
	}, 10*time.Second, 1*time.Second)

	// Request A
	outgoingReqA := &http.Request{
		Method: http.MethodPost,
		Body:   io.NopCloser(bytes.NewReader([]byte("hello from client"))),
		Header: http.Header{
			"X-From-Client": []string{"client-header-value"},
		},
		URL: &url.URL{
			Scheme:   "http",
			Host:     appNameA,
			Path:     "/some/path",
			RawQuery: "queryParam=value-a",
		},
	}
	outgoingReqA = outgoingReqA.WithContext(ctx)
	resp, err := httpClient.Do(outgoingReqA)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert server received the request we sent
	proxyRequestResponseA := <-receivedRequestsCh
	serverGotReqA := proxyRequestResponseA.request
	require.NotNil(t, serverGotReqA)
	require.Equal(t, http.MethodPost, serverGotReqA.Method)
	require.Equal(t, "client-header-value", serverGotReqA.Header.Get("X-From-Client"))
	require.Equal(t, outgoingReqA.URL.Path, serverGotReqA.URL.Path)
	require.Equal(t, "hello from client", string(proxyRequestResponseA.body))
	require.Equal(t, "value-a", serverGotReqA.URL.Query().Get("queryParam"))

	// Assert client receives the response the server sent
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	require.Equal(t, "server-a-header-value", resp.Header.Get("X-From-Server"))
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello from server a", string(respBody))

	// Request B
	outgoingReqB := &http.Request{
		Method: http.MethodPost,
		Body:   io.NopCloser(bytes.NewReader([]byte("hello from client"))),
		Header: http.Header{
			"X-From-Client": []string{"client-header-value"},
		},
		URL: &url.URL{
			Scheme:   "http",
			Host:     appNameB,
			Path:     "/some/path",
			RawQuery: "queryParam=value-b",
		},
	}
	outgoingReqB = outgoingReqB.WithContext(ctx)
	resp, err = httpClient.Do(outgoingReqB)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert server received the request we sent
	proxyRequestResponseB := <-receivedRequestsCh
	serverGotReqB := proxyRequestResponseB.request
	require.NotNil(t, serverGotReqB)
	require.Equal(t, http.MethodPost, serverGotReqB.Method)
	require.Equal(t, "client-header-value", serverGotReqB.Header.Get("X-From-Client"))
	require.Equal(t, outgoingReqB.URL.Path, serverGotReqB.URL.Path)
	require.Equal(t, "hello from client", string(proxyRequestResponseB.body))
	require.Equal(t, "value-b", serverGotReqB.URL.Query().Get("queryParam"))

	// Assert client receives the response the server sent
	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	require.Equal(t, "server-b-header-value", resp.Header.Get("X-From-Server"))
	respBody, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello from server b", string(respBody))
}

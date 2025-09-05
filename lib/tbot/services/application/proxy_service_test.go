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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

var echoHeaders = map[string]struct{}{
	"X-Test-Header-A": {},
	"X-Test-Header-B": {},
}

func TestE2E_ApplicationProxyService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()

	// Spin up a test HTTP server
	wantStatus := http.StatusTeapot
	wantBody := []byte("X-Test-Header-A: test-value-a\n")
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This stub HTTP Server just echoes back all headers it receives.
		// This lets us verify that headers are being properly forwarded by the HTTP Proxy Service.
		w.WriteHeader(wantStatus)

		var body string
		for header, values := range r.Header {
			for _, value := range values {
				body += fmt.Sprintf("%s: %s\n", header, value)
			}
		}

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

	proxyServiceConfig := &ProxyServiceConfig{
		Listen:   "localhost:12345",
		Listener: botListener,
	}

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
			ProxyServiceBuilder(
				proxyServiceConfig,
				connCfg,
				bot.DefaultCredentialLifetime,
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

	proxyUrl, err := url.Parse("http://" + proxyServiceConfig.Listen)
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}

	// We can't predict exactly when the tunnel will be ready so we use
	// EventuallyWithT to retry.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requestUrl, _ := url.Parse("http://" + appName)
		request := &http.Request{
			URL: requestUrl,
		}
		resp, err := httpClient.Do(request)

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
	}, 10*time.Second, 1*time.Second)

	// Do a second request to test caching
	resp, err := httpClient.Get("http://" + appName)
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

	// Validate Application Caching is disabled
	assert.Equal(t, strconv.FormatBool(proxyServiceConfig.CertificateCaching), resp.Header.Get("X-Teleport-Application-Cached"))
}

func TestE2E_ApplicationProxyServiceWithCaching(t *testing.T) {
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

	proxyServiceConfig := &ProxyServiceConfig{
		Listen:             "localhost:12345",
		Listener:           botListener,
		CertificateCaching: true,
	}

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
			ProxyServiceBuilder(
				proxyServiceConfig,
				connCfg,
				bot.DefaultCredentialLifetime,
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

	proxyUrl, err := url.Parse("http://" + proxyServiceConfig.Listen)
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}

	time.Sleep(120 * time.Second)

	// We can't predict exactly when the tunnel will be ready so we use
	// EventuallyWithT to retry.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := httpClient.Get("http://" + appName)
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

	}, 10*time.Second, 1*time.Second)

	// Do a second request to test caching
	resp, err := httpClient.Get("http://" + appName)
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
	assert.Equal(t, resp.Header.Get("X-Teleport-Application-Cached"), strconv.FormatBool(proxyServiceConfig.CertificateCaching))
}

//func TestE2E_ApplicationProxyServiceLoadTest(t *testing.T) {
//	t.Parallel()
//	ctx := context.Background()
//	log := logtest.NewLogger()
//
//	// Spin up a test HTTP server
//	wantStatus := http.StatusTeapot
//	wantBody := []byte("hello this is a test\n")
//	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		// This stub HTTP Server just echoes back all headers it receives.
//		// This lets us verify that headers are being properly forwarded by the HTTP Proxy Service.
//		w.WriteHeader(wantStatus)
//		w.Write(wantBody)
//	}))
//	t.Cleanup(httpSrv.Close)
//
//	// Make a new auth server.
//	appName := "my-test-app"
//
//	var appList []servicecfg.App
//
//	// Create 100 apps to test caching and performance
//	var nrOfApps = 100
//
//	// Number of apps to randomly test
//	var testedAppCount = 20
//
//	for i := 0; i < nrOfApps; i++ {
//		appList = append(appList, servicecfg.App{
//			Name: fmt.Sprintf("%s-%d", appName, i),
//			URI:  httpSrv.URL,
//		})
//	}
//
//	process, err := testenv.NewTeleportProcess(
//		t.TempDir(),
//		defaultTestServerOpts(log),
//		testenv.WithConfig(func(cfg *servicecfg.Config) {
//			cfg.Apps.Enabled = true
//			cfg.Apps.Apps = appList
//			cfg.Testing.ClientTimeout = 5 * time.Minute
//		}),
//	)
//	require.NoError(t, err)
//	t.Cleanup(func() {
//		require.NoError(t, process.Close())
//		require.NoError(t, process.Wait())
//	})
//	rootClient, err := testenv.NewDefaultAuthClient(process)
//	require.NoError(t, err)
//	t.Cleanup(func() { _ = rootClient.Close() })
//
//	// Create role that allows the bot to access the app.
//	role, err := types.NewRole("app-access", types.RoleSpecV6{
//		Allow: types.RoleConditions{
//			AppLabels: types.Labels{
//				"*": apiutils.Strings{"*"},
//			},
//		},
//	})
//	require.NoError(t, err)
//	role, err = rootClient.UpsertRole(t.Context(), role)
//	require.NoError(t, err)
//
//	botListener, err := net.Listen("tcp", "127.0.0.1:0")
//	require.NoError(t, err)
//	t.Cleanup(func() {
//		botListener.Close()
//	})
//
//	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
//
//	proxyAddr, err := process.ProxyWebAddr()
//	require.NoError(t, err)
//
//	proxyServiceConfig := &ProxyServiceConfig{
//		Listen:             "localhost:12345",
//		Listener:           botListener,
//		CertificateCaching: true,
//	}
//
//	connCfg := connection.Config{
//		Address:     proxyAddr.Addr,
//		AddressKind: connection.AddressKindProxy,
//		Insecure:    true,
//	}
//	b, err := bot.New(bot.Config{
//		Connection: connCfg,
//		Logger:     log,
//		Onboarding: *onboarding,
//		Services: []bot.ServiceBuilder{
//			ProxyServiceBuilder(
//				proxyServiceConfig,
//				connCfg,
//				bot.DefaultCredentialLifetime,
//			),
//		},
//	})
//	require.NoError(t, err)
//
//	// Spin up goroutine for bot to run in
//	ctx, cancel := context.WithCancel(ctx)
//	wg := sync.WaitGroup{}
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		err := b.Run(ctx)
//		assert.NoError(t, err, "bot should not exit with error")
//		cancel()
//	}()
//	t.Cleanup(func() {
//		// Shut down bot and make sure it exits.
//		cancel()
//		wg.Wait()
//	})
//
//	proxyUrl, err := url.Parse("http://" + proxyServiceConfig.Listen)
//	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
//
//	var testCases []struct {
//		app string
//	}
//
//	for i := 0; i < testedAppCount; i++ {
//		// Randomly select an app from the 1000 we created
//		appName = fmt.Sprintf("%s-%d", appName, rand.Intn(nrOfApps)+1)
//		testCases = append(testCases, struct{ app string }{app: appName})
//	}
//
//	for _, c := range testCases {
//		c := c
//		t.Run(c.app, func(t *testing.T) {
//			t.Parallel()
//			require.EventuallyWithT(t, func(t *assert.CollectT) {
//				requestUrl, _ := url.Parse("http://" + c.app)
//				resp, err := httpClient.Get(requestUrl.String())
//
//				if !assert.NoError(t, err) {
//					return
//				}
//				defer resp.Body.Close()
//				assert.Equal(t, appName, resp.Header.Get("X-Teleport-Application"))
//				assert.Equal(t, strconv.FormatBool(false), resp.Header.Get("X-Teleport-Application-Cached"))
//			}, 10*time.Second, 1*time.Second)
//		})
//	}
//
//	// Run the same tests again to test caching
//	for _, c := range testCases {
//		c := c
//		t.Run(c.app, func(t *testing.T) {
//			t.Parallel()
//			require.EventuallyWithT(t, func(t *assert.CollectT) {
//				requestUrl, _ := url.Parse("http://" + c.app)
//				resp, err := httpClient.Get(requestUrl.String())
//
//				if !assert.NoError(t, err) {
//					return
//				}
//				defer resp.Body.Close()
//				assert.Equal(t, appName, resp.Header.Get("X-Teleport-Application"))
//				assert.Equal(t, strconv.FormatBool(true), resp.Header.Get("X-Teleport-Application-Cached"))
//			}, 10*time.Second, 1*time.Second)
//		})
//	}
//
//	//// We can't predict exactly when the tunnel will be ready so we use
//	//// EventuallyWithT to retry.
//	//require.EventuallyWithT(t, func(t *assert.CollectT) {
//	//	requestUrl, _ := url.Parse("http://" + appName)
//	//	request := &http.Request{
//	//		URL: requestUrl,
//	//	}
//	//	resp, err := httpClient.Do(request)
//	//
//	//	if !assert.NoError(t, err) {
//	//		return
//	//	}
//	//	defer resp.Body.Close()
//	//	assert.Equal(t, wantStatus, resp.StatusCode)
//	//	body, err := io.ReadAll(resp.Body)
//	//	if !assert.NoError(t, err) {
//	//		return
//	//	}
//	//	assert.Equal(t, wantBody, body)
//	//}, 10*time.Second, 1*time.Second)
//
//	//// Do a second request to test caching
//	//resp, err := httpClient.Get("http://" + appName)
//	//if !assert.NoError(t, err) {
//	//	return
//	//}
//	//defer resp.Body.Close()
//	//assert.Equal(t, wantStatus, resp.StatusCode)
//	//body, err := io.ReadAll(resp.Body)
//	//if !assert.NoError(t, err) {
//	//	return
//	//}
//	//assert.Equal(t, wantBody, body)
//	//
//	//// Validate Application Caching is disabled
//	//assert.Equal(t, strconv.FormatBool(proxyServiceConfig.CertificateCaching), resp.Header.Get("X-Teleport-Application-Cached"))
//}

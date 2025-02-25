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

package tbot

import (
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
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestE2E_ApplicationTunnelService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

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
	process := testenv.MakeTestServer(
		t,
		defaultTestServerOpts(t, log),
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
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create role that allows the bot to access the app.
	role, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	botListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		botListener.Close()
	})

	// Prepare the bot config
	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
	botConfig := defaultBotConfig(
		t, process, onboarding, config.ServiceConfigs{
			&config.ApplicationTunnelService{
				Listener: botListener,
				AppName:  appName,
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			// insecure required as the db tunnel will connect to proxies
			// self-signed.
			insecure: true,
		},
	)
	botConfig.Oneshot = false
	b := New(botConfig, log)

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

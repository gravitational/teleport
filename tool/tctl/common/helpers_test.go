/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type options struct {
	CertPool *x509.CertPool
	Insecure bool
}

type optionsFunc func(o *options)

func withRootCertPool(pool *x509.CertPool) optionsFunc {
	return func(o *options) {
		o.CertPool = pool
	}
}

func withInsecure(insecure bool) optionsFunc {
	return func(o *options) {
		o.Insecure = insecure
	}
}

func runResourceCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) (*bytes.Buffer, error) {
	var options options
	for _, v := range opts {
		v(&options)
	}
	var stdoutBuff bytes.Buffer
	command := &ResourceCommand{
		stdout: &stdoutBuff,
	}
	cfg := service.MakeDefaultConfig()

	app := utils.InitCLIParser("tctl", GlobalHelpString)
	command.Initialize(app, cfg)

	selectedCmd, err := app.Parse(args)
	require.NoError(t, err)

	var ccf GlobalCLIFlags
	ccf.ConfigString = mustGetBase64EncFileConfig(t, fc)
	ccf.Insecure = options.Insecure

	clientConfig, err := ApplyConfig(&ccf, cfg)
	require.NoError(t, err)

	if options.CertPool != nil {
		clientConfig.TLS.RootCAs = options.CertPool
	}

	ctx := context.Background()
	client, err := connectToAuthService(ctx, cfg, clientConfig)
	require.NoError(t, err)

	_, err = command.TryRun(ctx, selectedCmd, client)
	if err != nil {
		return nil, err
	}
	return &stdoutBuff, nil
}

func mustDecodeJSON(t *testing.T, r io.Reader, i interface{}) {
	err := json.NewDecoder(r).Decode(i)
	require.NoError(t, err)
}

func mustGetFreeLocalListenerAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().String()
}

func mustGetBase64EncFileConfig(t *testing.T, fc *config.FileConfig) string {
	configYamlContent, err := yaml.Marshal(fc)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(configYamlContent)
}

type testServerOptions struct {
	fileConfig *config.FileConfig
}

type testServerOptionFunc func(options *testServerOptions)

func withFileConfig(fc *config.FileConfig) testServerOptionFunc {
	return func(options *testServerOptions) {
		options.fileConfig = fc
	}
}

func makeAndRunTestAuthServer(t *testing.T, opts ...testServerOptionFunc) (auth *service.TeleportProcess) {
	var options testServerOptions
	for _, opt := range opts {
		opt(&options)
	}

	var err error
	cfg := service.MakeDefaultConfig()
	if options.fileConfig != nil {
		err = config.ApplyFileConfig(options.fileConfig, cfg)
		require.NoError(t, err)
	}

	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	auth, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		auth.Close()
	})

	eventCh := make(chan service.Event, 1)
	auth.WaitForEvent(auth.ExitContext(), service.AuthTLSReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(30 * time.Second):
		// in reality, the auth server should start *much* sooner than this.  we use a very large
		// timeout here because this isn't the kind of problem that this test is meant to catch.
		t.Fatal("auth server didn't start after 30s")
	}
	return auth
}

func waitForBackendDatabaseResourcePropagation(t *testing.T, authServer *auth.Server) {
	deadlineC := time.After(5 * time.Second)
	for {
		select {
		case <-time.Tick(100 * time.Millisecond):
			databases, err := authServer.GetDatabaseServers(context.Background(), defaults.Namespace)
			if err != nil {
				logrus.WithError(err).Debugf("GetDatabaseServer call failed")
				continue
			}
			if len(databases) == 0 {
				logrus.Debugf("Database servers not found")
				continue
			}
			return
		case <-deadlineC:
			t.Fatal("Failed to fetch database servers")
		}
	}
}

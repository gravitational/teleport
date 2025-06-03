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

package resource

import (
	"bytes"
	"context"
	"encoding/json"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/service"
	"github.com/jonboulle/clockwork"
	"io"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type options struct {
	Editor func(string) error
}

type optionsFunc func(o *options)

func withEditor(editor func(string) error) optionsFunc {
	return func(o *options) {
		o.Editor = editor
	}
}

type cliCommand interface {
	Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config)
	TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error)
}

func runCommand(t *testing.T, client *authclient.Client, cmd cliCommand, args []string) error {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	app := utils.InitCLIParser("tctl", "test CLI")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selectedCmd, err := app.Parse(args)
	require.NoError(t, err)

	_, err = cmd.TryRun(context.Background(), selectedCmd, func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		return client, func(context.Context) {}, nil
	})
	return err
}

func runResourceCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &ResourceCommand{
		stdout: &stdoutBuff,
	}
	return &stdoutBuff, runCommand(t, client, command, args)
}

func runEditCommand(t *testing.T, client *authclient.Client, args []string, opts ...optionsFunc) (*bytes.Buffer, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	var stdoutBuff bytes.Buffer
	command := &EditCommand{
		Editor: o.Editor,
	}
	return &stdoutBuff, runCommand(t, client, command, args)
}

func mustDecodeJSON[T any](t *testing.T, r io.Reader) T {
	var out T
	err := json.NewDecoder(r).Decode(&out)
	require.NoError(t, err)
	return out
}

func mustTranscodeYAMLToJSON(t *testing.T, r io.Reader) []byte {
	decoder := kyaml.NewYAMLToJSONDecoder(r)
	var resource services.UnknownResource
	require.NoError(t, decoder.Decode(&resource))
	return resource.Raw
}

// Copied from `common`

type testServerOptions struct {
	fileConfig      *config.FileConfig
	fileDescriptors []*servicecfg.FileDescriptor
	fakeClock       *clockwork.FakeClock
}

type testServerOptionFunc func(options *testServerOptions)

func withFileConfig(fc *config.FileConfig) testServerOptionFunc {
	return func(options *testServerOptions) {
		options.fileConfig = fc
	}
}

func withFileDescriptors(fds []*servicecfg.FileDescriptor) testServerOptionFunc {
	return func(options *testServerOptions) {
		options.fileDescriptors = fds
	}
}

func withFakeClock(fakeClock *clockwork.FakeClock) testServerOptionFunc {
	return func(options *testServerOptions) {
		options.fakeClock = fakeClock
	}
}

func makeAndRunTestAuthServer(t *testing.T, opts ...testServerOptionFunc) (auth *service.TeleportProcess) {
	var options testServerOptions
	for _, opt := range opts {
		opt(&options)
	}

	var err error
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.FileDescriptors = options.fileDescriptors
	if options.fileConfig != nil {
		err = config.ApplyFileConfig(options.fileConfig, cfg)
		require.NoError(t, err)
	}

	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	cfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	if options.fakeClock != nil {
		cfg.Clock = options.fakeClock
	}
	auth, err = service.NewTeleport(cfg)

	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		require.NoError(t, auth.Close())
		require.NoError(t, auth.Wait())
	})

	_, err = auth.WaitForEventTimeout(30*time.Second, service.AuthTLSReady)
	// in reality, the auth server should start *much* sooner than this.  we use a very large
	// timeout here because this isn't the kind of problem that this test is meant to catch.
	require.NoError(t, err, "auth server didn't start after 30s")

	// Wait for proxy to start up if it's enabled. Otherwise we may get racy
	// behavior between startup and shutdown.
	if cfg.Proxy.Enabled {
		_, err = auth.WaitForEventTimeout(30*time.Second, service.ProxyWebServerReady)
		require.NoError(t, err, "proxy server didn't start after 30s")
	}

	if cfg.Auth.Enabled && cfg.Databases.Enabled {
		waitForDatabases(t, auth, cfg.Databases.Databases)
	}
	return auth
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, dbs []servicecfg.Database) {
	if len(dbs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)

			// Count how many input "dbs" are registered.
			var registered int
			for _, db := range dbs {
				for _, a := range all {
					if a.GetName() == db.Name {
						registered++
						break
					}
				}
			}

			if registered == len(dbs) {
				return
			}
		case <-ctx.Done():
			t.Fatal("databases not registered after 10s")
		}
	}
}

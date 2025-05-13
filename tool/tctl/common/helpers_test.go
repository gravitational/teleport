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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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

func getAuthClient(ctx context.Context, t *testing.T, fc *config.FileConfig, opts ...optionsFunc) *authclient.Client {
	var options options
	for _, v := range opts {
		v(&options)
	}
	cfg := servicecfg.MakeDefaultConfig()

	var ccf GlobalCLIFlags
	ccf.ConfigString = mustGetBase64EncFileConfig(t, fc)
	ccf.Insecure = options.Insecure

	clientConfig, err := ApplyConfig(&ccf, cfg)
	require.NoError(t, err)

	if options.CertPool != nil {
		clientConfig.TLS.RootCAs = options.CertPool
	}

	client, err := authclient.Connect(ctx, clientConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

type cliCommand interface {
	Initialize(app *kingpin.Application, cfg *servicecfg.Config)
	TryRun(ctx context.Context, cmd string, client *authclient.Client) (bool, error)
}

func runCommand(t *testing.T, fc *config.FileConfig, cmd cliCommand, args []string, opts ...optionsFunc) error {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	app := utils.InitCLIParser("tctl", GlobalHelpString)
	cmd.Initialize(app, cfg)

	selectedCmd, err := app.Parse(args)
	require.NoError(t, err)

	ctx := context.Background()
	client := getAuthClient(ctx, t, fc, opts...)
	_, err = cmd.TryRun(ctx, selectedCmd, client)
	return err
}

func runResourceCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &ResourceCommand{
		stdout: &stdoutBuff,
	}
	return &stdoutBuff, runCommand(t, fc, command, args, opts...)
}

func runLockCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) error {
	command := &LockCommand{}
	args = append([]string{"lock"}, args...)
	return runCommand(t, fc, command, args, opts...)
}

func runTokensCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &TokensCommand{
		stdout: &stdoutBuff,
	}

	args = append([]string{"tokens"}, args...)
	return &stdoutBuff, runCommand(t, fc, command, args, opts...)
}

func runUserCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) error {
	command := &UserCommand{}
	args = append([]string{"users"}, args...)
	return runCommand(t, fc, command, args, opts...)
}

func runAuthCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) error {
	command := &AuthCommand{}
	args = append([]string{"auth"}, args...)
	return runCommand(t, fc, command, args, opts...)
}

func runIdPSAMLCommand(t *testing.T, fc *config.FileConfig, args []string, opts ...optionsFunc) error {
	command := &IdPCommand{}
	args = append([]string{"idp"}, args...)
	return runCommand(t, fc, command, args, opts...)
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

func mustDecodeYAMLDocuments[T any](t *testing.T, r io.Reader, out *[]T) error {
	t.Helper()
	decoder := yaml.NewDecoder(r)
	for {
		var entry T
		if err := decoder.Decode(&entry); err != nil {
			// Break when there are no more documents to decode
			if err != io.EOF {
				return err
			}
			break
		}
		*out = append(*out, entry)
	}
	return nil
}

func mustDecodeYAML[T any](t *testing.T, r io.Reader) T {
	var out T
	err := yaml.NewDecoder(r).Decode(&out)
	require.NoError(t, err)
	return out
}
func mustGetBase64EncFileConfig(t *testing.T, fc *config.FileConfig) string {
	configYamlContent, err := yaml.Marshal(fc)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(configYamlContent)
}

func mustWriteFileConfig(t *testing.T, fc *config.FileConfig) string {
	fileConfPath := filepath.Join(t.TempDir(), "teleport.yaml")
	fileConfYAML, err := yaml.Marshal(fc)
	require.NoError(t, err)
	err = os.WriteFile(fileConfPath, fileConfYAML, 0600)
	require.NoError(t, err)
	return fileConfPath
}

func mustAddUser(t *testing.T, fc *config.FileConfig, username string, roles ...string) {
	err := runUserCommand(t, fc, []string{"add", username, "--roles", strings.Join(roles, ",")})
	require.NoError(t, err)
}

func mustWriteIdentityFile(t *testing.T, fc *config.FileConfig, username string) string {
	identityFilePath := filepath.Join(t.TempDir(), "identity")
	err := runAuthCommand(t, fc, []string{"sign", "--user", username, "--out", identityFilePath})
	require.NoError(t, err)
	return identityFilePath
}

type testServerOptions struct {
	fileConfig      *config.FileConfig
	fileDescriptors []*servicecfg.FileDescriptor
	fakeClock       clockwork.FakeClock
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

func withFakeClock(fakeClock clockwork.FakeClock) testServerOptionFunc {
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

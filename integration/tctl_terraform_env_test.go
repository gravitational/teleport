/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/wait"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/tctl/common"
)

// TestTCTLTerraformCommand_ProxyJoin validates that the command `tctl terraform env` can run against a Teleport Proxy
// service and generates valid credentials Terraform can use to connect to Teleport.
func TestTCTLTerraformCommand_ProxyJoin(t *testing.T) {
	// test is not Parallel because of the metrics black hole
	testDir := t.TempDir()
	prometheus.DefaultRegisterer = metricRegistryBlackHole{}

	// Test setup: creating a teleport instance running auth and proxy
	clusterName := "root.example.com"
	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logtest.NewLogger(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v3"
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	testUsername := "test-user"
	createTCTLTerraformUserAndRole(t, testUsername, rc)

	// Test setup: starting the Teleport instance
	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
	})

	// Test setup: obtaining and authclient connected via the proxy
	clt := getAuthClientForProxy(t, rc, testUsername, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err = clt.Ping(ctx)
	require.NoError(t, err)

	addr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)

	// Test execution, running the tctl command
	tctlCfg := &servicecfg.Config{}
	err = tctlCfg.SetAuthServerAddresses([]utils.NetAddr{*addr})
	require.NoError(t, err)
	tctlCommand := common.TerraformCommand{}

	app := kingpin.New("test", "test")
	tctlCommand.Initialize(app, nil, tctlCfg)
	_, err = app.Parse([]string{"terraform", "env"})
	require.NoError(t, err)
	// Create io buffer writer
	stdout := &bytes.Buffer{}

	err = tctlCommand.RunEnvCommand(ctx, clt, stdout, os.Stderr)
	require.NoError(t, err)

	vars := parseExportedEnvVars(t, stdout)
	require.Contains(t, vars, constants.EnvVarTerraformAddress)
	require.Contains(t, vars, constants.EnvVarTerraformIdentityFileBase64)

	// Test validation: connect with the credentials in env vars and do a ping
	require.Equal(t, addr.String(), vars[constants.EnvVarTerraformAddress])

	connectWithCredentialsFromVars(t, vars, clt)
}

// TestTCTLTerraformCommand_AuthJoin validates that the command `tctl terraform env` can run against a Teleport Auth
// service and generates valid credentials Terraform can use to connect to Teleport.
func TestTCTLTerraformCommand_AuthJoin(t *testing.T) {
	// test is not Parallel because of the metrics black hole
	testDir := t.TempDir()
	prometheus.DefaultRegisterer = metricRegistryBlackHole{}

	// Test setup: creating a teleport instance running auth and proxy
	clusterName := "root.example.com"
	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logtest.NewLogger(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = false
	rcConf.SSH.Enabled = false
	rcConf.Version = "v3"

	testUsername := "test-user"
	createTCTLTerraformUserAndRole(t, testUsername, rc)

	// Test setup: starting the Teleport instance
	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
	})

	// Test setup: obtaining and authclient connected via the proxy
	clt := getAuthClientForAuth(t, rc, testUsername, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err = clt.Ping(ctx)
	require.NoError(t, err)

	addr, err := rc.Process.AuthAddr()
	require.NoError(t, err)

	// Test execution, running the tctl command
	tctlCfg := &servicecfg.Config{}
	err = tctlCfg.SetAuthServerAddresses([]utils.NetAddr{*addr})
	require.NoError(t, err)
	tctlCommand := common.TerraformCommand{}

	app := kingpin.New("test", "test")
	tctlCommand.Initialize(app, nil, tctlCfg)
	_, err = app.Parse([]string{"terraform", "env"})
	require.NoError(t, err)
	// Create io buffer writer
	stdout := &bytes.Buffer{}

	err = tctlCommand.RunEnvCommand(ctx, clt, stdout, os.Stderr)
	require.NoError(t, err)

	vars := parseExportedEnvVars(t, stdout)
	require.Contains(t, vars, constants.EnvVarTerraformAddress)
	require.Contains(t, vars, constants.EnvVarTerraformIdentityFileBase64)

	// Test validation: connect with the credentials in env vars and do a ping
	require.Equal(t, addr.String(), vars[constants.EnvVarTerraformAddress])

	connectWithCredentialsFromVars(t, vars, clt)
}

// TestTCTLTerraformCommand_ScopedProxyJoin validates that the command `tctl terraform env`
// can run from an unscoped admin against a Teleport Proxy service and generates valid
// scoped credentials Terraform can use to connect to Teleport.
func TestTCTLTerraformCommand_ScopedProxyJoin(t *testing.T) {
	// test is not Parallel because of the metrics black hole
	testDir := t.TempDir()
	prometheus.DefaultRegisterer = metricRegistryBlackHole{}

	// Test setup: creating a teleport instance running auth and proxy
	clusterName := "root.example.com"
	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      logtest.NewLogger(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v3"
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.ScopesFeatures = scopes.Features{Enabled: true}

	testUsername := "test-user"
	createTCTLTerraformCrossScopeAdmin(t, testUsername, rc)

	// Test setup: starting the Teleport instance
	err := rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, rc.StopAll())
	})

	// Test setup: obtaining and authclient connected via the proxy
	clt := getAuthClientForProxy(t, rc, testUsername, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err = clt.Ping(ctx)
	require.NoError(t, err)

	createDefaultTerraformScopedRole(t, ctx, clt)

	addr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)

	// Test execution, running the tctl command
	tctlCfg := &servicecfg.Config{}
	err = tctlCfg.SetAuthServerAddresses([]utils.NetAddr{*addr})
	require.NoError(t, err)
	tctlCommand := common.TerraformCommand{}

	const testScope = "/test"
	app := kingpin.New("test", "test")
	tctlCommand.Initialize(app, nil, tctlCfg)
	_, err = app.Parse([]string{"terraform", "env", "--scope", testScope})
	require.NoError(t, err)
	// Create io buffer writer
	stdout := &bytes.Buffer{}

	err = tctlCommand.RunEnvCommand(ctx, clt, stdout, os.Stderr)
	require.NoError(t, err)

	vars := parseExportedEnvVars(t, stdout)
	require.Contains(t, vars, constants.EnvVarTerraformAddress)
	require.Contains(t, vars, constants.EnvVarTerraformIdentityFileBase64)

	// Test validation: connect with the credentials in env vars and do a ping
	require.Equal(t, addr.String(), vars[constants.EnvVarTerraformAddress])

	connectWithCredentialsFromVars(t, vars, clt)

	// Test validation: make sure no role assignment is left. We don't know the assignment's name so we have to list them all initially.
	resp, err := clt.ScopedAccessServiceClient().ListScopedRoleAssignments(ctx, scopedaccessv1.ListScopedRoleAssignmentsRequest_builder{
		ScopeFilter: scopesv1.Filter_builder{
			Scope: testScope,
			Mode:  scopesv1.Mode_MODE_EXACT,
		}.Build(),
	}.Build())
	require.NoError(t, err)
	switch len(resp.GetAssignments()) {
	case 0:
		// assignment already deleted
		return
	case 1:
		// Assignment still exists, this might be a race against the cache.
		// Will retry a few times.
		_, err = wait.UntilNotFound(ctx, func(ctx context.Context) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
			return clt.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
				Name:    resp.GetAssignments()[0].GetMetadata().GetName(),
				SubKind: resp.GetAssignments()[0].GetSubKind(),
				Scope:   resp.GetAssignments()[0].GetScope(),
			}.Build())
		})
		require.NoError(t, err)
	default:
		require.Fail(t, "unexpected number of role assignments: %v", len(resp.GetAssignments()))
	}
}

func createDefaultTerraformScopedRole(t *testing.T, ctx context.Context, clt *authclient.Client) {
	reader, err := utils.OpenFileNoUnsafeLinks("../integrations/terraform/examples/provider/scoped-provider-role.yaml")
	require.NoError(t, err)

	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	var raw services.UnknownResource
	err = decoder.Decode(&raw)
	require.NoError(t, err)
	r, err := services.UnmarshalProtoResource[*scopedaccessv1.ScopedRole](raw.Raw, services.DisallowUnknown())
	require.NoError(t, err)

	_, err = clt.ScopedAccessServiceClient().CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: r,
	}.Build())
	require.NoError(t, err)

	// Wait for the role to enter the cache, there is a race. If the test wins, tctl will fail to find the role.
	_, err = wait.UntilFound(ctx, func(ctx context.Context) (*scopedaccessv1.GetScopedRoleResponse, error) {
		return clt.ScopedAccessServiceClient().GetScopedRole(ctx,
			scopedaccessv1.GetScopedRoleRequest_builder{
				Name:  r.GetMetadata().GetName(),
				Scope: r.GetScope(),
			}.Build())
	}, wait.WithMaxTries(7))
	require.NoError(t, err)
}

func createTCTLTerraformUserAndRole(t *testing.T, username string, instance *helpers.TeleInstance) {
	// Test setup: creating a test user and its role
	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindToken, types.KindRole, types.KindBot},
					Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbList, types.VerbUpdate},
				},
			},
		},
	})
	require.NoError(t, err)

	instance.AddUserWithRole(username, role)
}

func createTCTLTerraformCrossScopeAdmin(t *testing.T, username string, instance *helpers.TeleInstance) {
	// Test setup: creating a test user and its role
	role, err := types.NewRole("test-role-full-admin", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindScopedToken},
					Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbList, types.VerbUpdate, types.VerbDelete},
				},
				{
					Resources: []string{scopedaccess.KindScopedRole, scopedaccess.KindScopedRoleAssignment},
					Verbs:     []string{types.VerbReadNoSecrets, types.VerbCreate, types.VerbList, types.VerbUpdate, types.VerbDelete},
				},
				{
					Resources: []string{types.KindBot},
					Verbs:     []string{types.VerbRead, types.VerbCreate, types.VerbList, types.VerbUpdate, types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)

	instance.AddUserWithRole(username, role)
}

// getAuthCLientForProxy builds an authclient.CLient connecting to the auth through the proxy
// (with a web client resolver hitting /v1/wenapi/ping and a tunnel auth dialer reaching the auth through the proxy).
// For the tests, the client is configured to trust the proxy TLS certs on first connection.
func getAuthClientForProxy(t *testing.T, tc *helpers.TeleInstance, username string, ttl time.Duration) *authclient.Client {
	// Get TLS and SSH material
	keyRing := helpers.MustCreateUserKeyRing(t, tc, username, ttl)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tlsConfig, err := keyRing.TeleportClientTLSConfig(nil, []string{tc.Config.Auth.ClusterName.GetClusterName()})
	require.NoError(t, err)
	tlsConfig.InsecureSkipVerify = true
	proxyAddr, err := tc.Process.ProxyWebAddr()
	require.NoError(t, err)
	sshConfig, err := keyRing.ProxyClientSSHConfig(proxyAddr.Host())
	require.NoError(t, err)

	// Build auth client configuration
	authAddr, err := tc.Process.AuthAddr()
	require.NoError(t, err)
	clientConfig := &authclient.Config{
		TLS:                  tlsConfig,
		SSH:                  sshConfig,
		AuthServers:          []utils.NetAddr{*authAddr},
		Log:                  logtest.NewLogger(),
		CircuitBreakerConfig: breaker.Config{},
		DialTimeout:          0,
		DialOpts:             nil,
		// Insecure:             true,
		ProxyDialer: nil,
	}

	// Configure the resolver and dialer to connect to the auth via a proxy
	resolver, err := reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: clientConfig.AuthServers[0].String(),
			Insecure:  clientConfig.Insecure,
			Timeout:   clientConfig.DialTimeout,
		}),
		nil /* clock */)
	require.NoError(t, err)

	dialer, err := reversetunnelclient.NewAuthDialerThroughProxy(reversetunnelclient.AuthDialerThroughProxyConfig{
		Resolver:              resolver,
		ClientConfig:          clientConfig.SSH,
		Log:                   slog.Default(),
		InsecureSkipTLSVerify: clientConfig.Insecure,
		GetClusterCAs:         client.ClusterCAsFromCertPool(clientConfig.TLS.RootCAs),
	})
	require.NoError(t, err)

	clientConfig.ProxyDialer = dialer

	// Finally, build a client and connect
	clt, err := authclient.Connect(ctx, clientConfig)
	require.NoError(t, err)
	return clt
}

// getAuthClientForAuth builds an authclient.CLient connecting to the auth directly.
// This client only has TLSConfig set (as opposed to TLSConfig+SSHConfig).
func getAuthClientForAuth(t *testing.T, tc *helpers.TeleInstance, username string, ttl time.Duration) *authclient.Client {
	// Get TLS and SSH material
	keyRing := helpers.MustCreateUserKeyRing(t, tc, username, ttl)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tlsConfig, err := keyRing.TeleportClientTLSConfig(nil, []string{tc.Config.Auth.ClusterName.GetClusterName()})
	require.NoError(t, err)

	// Build auth client configuration
	authAddr, err := tc.Process.AuthAddr()
	require.NoError(t, err)
	clientConfig := &authclient.Config{
		TLS:                  tlsConfig,
		AuthServers:          []utils.NetAddr{*authAddr},
		Log:                  logtest.NewLogger(),
		CircuitBreakerConfig: breaker.Config{},
		DialTimeout:          0,
		DialOpts:             nil,
		ProxyDialer:          nil,
	}

	// Build the client and connect
	clt, err := authclient.Connect(ctx, clientConfig)
	require.NoError(t, err)
	return clt
}

// parseExportedEnvVars parses a buffer corresponding to the program's stdout and returns a map {env: value}
// of the exported variables. The buffer content should looks like:
//
//	export VAR1="VALUE1"
//	export VAR2="VALUE2"
//	# this is a comment
func parseExportedEnvVars(t *testing.T, stdout *bytes.Buffer) map[string]string {
	// Test validation: parse the output and extract exported envs
	vars := map[string]string{}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == '#' {
			continue
		}
		require.True(t, strings.HasPrefix(line, "export "))
		parts := strings.Split(line, "=")
		env := strings.TrimSpace(parts[0][7:])
		value := strings.Trim(strings.Join(parts[1:], "="), `"' `)
		require.NotEmpty(t, env)
		require.NotEmpty(t, value)
		vars[env] = value
	}
	return vars
}

// connectWithCredentialsFromVars takes the environment variables exported by the `tctl terraform env` command,
// builds a Teleport client from them, and validates it can ping the cluster.
func connectWithCredentialsFromVars(t *testing.T, vars map[string]string, clt *authclient.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	identity, err := base64.StdEncoding.DecodeString(vars[constants.EnvVarTerraformIdentityFileBase64])
	require.NoError(t, err)
	creds := client.LoadIdentityFileFromString(string(identity))
	require.NotNil(t, creds)
	botClt, err := client.New(ctx, client.Config{
		Addrs:                    []string{vars[constants.EnvVarTerraformAddress]},
		Credentials:              []client.Credentials{creds},
		InsecureAddressDiscovery: clt.Config().InsecureSkipVerify,
		Context:                  ctx,
	})
	require.NoError(t, err)
	_, err = botClt.Ping(ctx)
	require.NoError(t, err)
}

// metricRegistryBlackHole is a fake prometheus.Registerer that accepts every metric and do nothing.
// This is a workaround for different teleport component using the global registry but registering incompatible metrics.
// Those issues can surface during integration tests starting Teleport auth, proxy, and tbot.
// The long-term fix is to have every component use its own registry instead of the global one.
type metricRegistryBlackHole struct {
}

func (m metricRegistryBlackHole) Register(_ prometheus.Collector) error {
	return nil
}

func (m metricRegistryBlackHole) MustRegister(_ ...prometheus.Collector) {}

func (m metricRegistryBlackHole) Unregister(_ prometheus.Collector) bool {
	return true
}

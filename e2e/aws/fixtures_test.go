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

package e2e

import (
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// username is the name of the host user used for tests.
var username string

func init() {
	me, err := user.Current()
	if err != nil {
		panic(err)
	}
	username = me.Username
}

// mustGetEnv is a test helper that fetches an env variable or fails with an
// error describing the missing env variable.
func mustGetEnv(t *testing.T, key string) string {
	t.Helper()
	val := os.Getenv(key)
	require.NotEmpty(t, val, "%s environment variable must be set and not empty", key)
	return val
}

func mustGetDiscoveryMatcherLabels(t *testing.T) types.Labels {
	t.Helper()
	labelSpec := mustGetEnv(t, discoveryMatcherLabelsEnv)
	labels, err := client.ParseLabelSpec(labelSpec)
	require.NoError(t, err)
	out := make(types.Labels)
	for k, v := range labels {
		out[k] = []string{v}
	}
	return out
}

// testOptionsFunc is a test option configuration func.
type testOptionsFunc func(*testOptions)

// testOptions are options to pass to createTeleportCluster.
type testOptions struct {
	// instanceConfigFuncs are a list of functions that configure the
	// TeleInstance before it is used to create Teleport services.
	instanceConfigFuncs []func(*helpers.InstanceConfig)
	// serviceConfigFuncs are a list of functions that configure the Teleport
	// cluster before it starts.
	serviceConfigFuncs []func(*servicecfg.Config)
	// userRoles are roles that will be bootstrapped and added to the Teleport
	// user under test.
	userRoles []types.Role
}

// createTeleportCluster sets up a Teleport cluster for tests.
func createTeleportCluster(t *testing.T, opts ...testOptionsFunc) *helpers.TeleInstance {
	t.Helper()
	var options testOptions
	for _, opt := range opts {
		opt(&options)
	}

	cfg := newInstanceConfig(t)
	for _, optFn := range options.instanceConfigFuncs {
		optFn(&cfg)
	}
	teleport := helpers.NewInstance(t, cfg)

	// Create a new user with the role created above.
	teleport.AddUserWithRole(username, options.userRoles...)

	tconf := newTeleportConfig(t)
	for _, optFn := range options.serviceConfigFuncs {
		optFn(tconf)
	}
	// Create a new teleport instance with the auth server.
	err := teleport.CreateEx(t, nil, tconf)
	require.NoError(t, err)
	// Start the teleport instance and wait for it to be ready.
	err = teleport.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, teleport.StopAll())
	})
	return teleport
}

func newInstanceConfig(t *testing.T) helpers.InstanceConfig {
	// Create the CA authority that will be used in Auth.
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	const (
		host   = helpers.Host
		site   = helpers.Site
		hostID = helpers.HostID
	)
	return helpers.InstanceConfig{
		ClusterName: site,
		HostID:      host,
		NodeName:    host,
		Priv:        priv,
		Pub:         pub,
		Log:         utils.NewLoggerForTests(),
	}
}

func newTeleportConfig(t *testing.T) *servicecfg.Config {
	tconf := servicecfg.MakeDefaultConfig()
	// Replace the default auth and proxy listeners with the ones so we can
	// run multiple tests in parallel.
	tconf.Console = nil
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.Testing.ClientTimeout = time.Second
	tconf.Testing.ShutdownTimeout = 2 * tconf.Testing.ClientTimeout
	return tconf
}

// withUserRole creates a new role that will be bootstraped and then granted to
// the Teleport user under test.
func withUserRole(t *testing.T, name string, spec types.RoleSpecV6) testOptionsFunc {
	t.Helper()
	// Create a new role with full access to all databases.
	role, err := types.NewRole(name, spec)
	require.NoError(t, err)
	return func(options *testOptions) {
		options.userRoles = append(options.userRoles, role)
	}
}

// withSingleProxyPort sets up a single proxy port listener config and
// sets `auth.proxy_listener_mode` to "multiplex".
func withSingleProxyPort(t *testing.T) testOptionsFunc {
	t.Helper()
	// enable proxy single port mode
	return func(options *testOptions) {
		options.instanceConfigFuncs = append(options.instanceConfigFuncs, func(cfg *helpers.InstanceConfig) {
			cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
		})
		options.serviceConfigFuncs = append(options.serviceConfigFuncs, func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		})
	}
}

// withDiscoveryService sets up the discovery service to watch for resources
// in the AWS account.
func withDiscoveryService(t *testing.T, discoveryGroup string, awsMatchers ...types.AWSMatcher) testOptionsFunc {
	t.Helper()
	return func(options *testOptions) {
		options.serviceConfigFuncs = append(options.serviceConfigFuncs, func(cfg *servicecfg.Config) {
			cfg.Discovery.Enabled = true
			cfg.Discovery.DiscoveryGroup = discoveryGroup
			// Reduce the polling interval to speed up the test execution
			// in the case of a failure of the first attempt.
			// The default polling interval is 5 minutes.
			cfg.Discovery.PollInterval = 1 * time.Minute
			cfg.Discovery.AWSMatchers = append(cfg.Discovery.AWSMatchers, awsMatchers...)
		})
	}
}

// withDatabaseService sets up the databases service to watch for discovered
// database resources in the AWS account.
func withDatabaseService(t *testing.T, matchers ...services.ResourceMatcher) testOptionsFunc {
	t.Helper()
	return func(options *testOptions) {
		options.serviceConfigFuncs = append(options.serviceConfigFuncs, func(cfg *servicecfg.Config) {
			cfg.Databases.Enabled = true
			cfg.Databases.ResourceMatchers = matchers
		})
	}
}

// withFullDatabaseAccessUserRole creates a Teleport role with full access to
// databases.
func withFullDatabaseAccessUserRole(t *testing.T) testOptionsFunc {
	t.Helper()
	// Create a new role with full access to all databases.
	return withUserRole(t, "db-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseUsers:  []string{types.Wildcard},
			DatabaseNames:  []string{types.Wildcard},
		},
	})
}

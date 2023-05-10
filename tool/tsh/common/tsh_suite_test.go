/*
Copyright 2021 Gravitational, Inc.

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
	"net"
	"os/user"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

type suite struct {
	root      *service.TeleportProcess
	leaf      *service.TeleportProcess
	connector types.OIDCConnector
	user      types.User
}

func newTestSuite(t *testing.T, opts ...testSuiteOptionFunc) *suite {
	var options testSuiteOptions
	for _, opt := range opts {
		opt(&options)
	}
	s := &suite{}

	s.setupRootCluster(t, options)

	if options.leafCluster || options.leafConfigFunc != nil {
		s.setupLeafCluster(t, options)
		// Wait for root/leaf to find each other.
		if s.root.Config.Auth.NetworkingConfig.GetProxyListenerMode() == types.ProxyListenerMode_Multiplex {
			require.Eventually(t, func() bool {
				rt, err := s.root.GetAuthServer().GetTunnelConnections(s.leaf.Config.Auth.ClusterName.GetClusterName())
				require.NoError(t, err)
				return len(rt) == 1
			}, time.Second*10, time.Second)
		} else {
			require.Eventually(t, func() bool {
				_, err := s.leaf.GetAuthServer().GetReverseTunnel(s.root.Config.Auth.ClusterName.GetClusterName())
				return err == nil
			}, time.Second*10, time.Second)
		}
	}

	if options.validationFunc != nil {
		require.Eventually(t, func() bool {
			return options.validationFunc(s)
		}, 10*time.Second, 500*time.Millisecond)
	}

	return s
}

func (s *suite) setupRootCluster(t *testing.T, options testSuiteOptions) {
	sshListenAddr := localListenerAddr()
	_, sshListenPort, err := net.SplitHostPort(sshListenAddr)
	require.NoError(t, err)
	fileConfig := &config.FileConfig{
		Version: "v2",
		Global: config.Global{
			DataDir:  t.TempDir(),
			NodeName: "rootnode",
		},
		SSH: config.SSH{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: localListenerAddr(),
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: sshListenAddr,
			},
			SSHPublicAddr: []string{net.JoinHostPort("localhost", sshListenPort)},
			WebAddr:       localListenerAddr(),
			TunAddr:       localListenerAddr(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: localListenerAddr(),
			},
			ClusterName: "root",
		},
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Log = utils.NewLoggerForTests()
	err = config.ApplyFileConfig(fileConfig, cfg)
	require.NoError(t, err)

	cfg.Proxy.DisableWebInterface = true
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleNode, types.RoleTrustedCluster},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	require.NoError(t, err)

	user, err := user.Current()
	require.NoError(t, err)

	s.connector = mockConnector(t)
	sshLoginRole, err := types.NewRole("ssh-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{user.Username},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
		Options: types.RoleOptions{
			ForwardAgent: true,
		},
	})
	require.NoError(t, err)
	kubeLoginRole, err := types.NewRole("kube-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			KubeGroups: []string{user.Username},
			KubernetesLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	s.user, err = types.NewUser("alice")
	require.NoError(t, err)
	s.user.SetRoles([]string{"access", "ssh-login", "kube-login"})
	cfg.Auth.BootstrapResources = []types.Resource{s.connector, s.user, sshLoginRole, kubeLoginRole}

	if options.rootConfigFunc != nil {
		options.rootConfigFunc(cfg)
	}

	s.root = runTeleport(t, cfg)
	t.Cleanup(func() { require.NoError(t, s.root.Close()) })
}

func (s *suite) setupLeafCluster(t *testing.T, options testSuiteOptions) {
	sshListenAddr := localListenerAddr()
	_, sshListenPort, err := net.SplitHostPort(sshListenAddr)
	require.NoError(t, err)
	fileConfig := &config.FileConfig{
		Version: "v2",
		Global: config.Global{
			DataDir:  t.TempDir(),
			NodeName: "leafnode",
		},
		SSH: config.SSH{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: localListenerAddr(),
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: sshListenAddr,
			},
			SSHPublicAddr: []string{net.JoinHostPort("localhost", sshListenPort)},
			WebAddr:       localListenerAddr(),
			TunAddr:       localListenerAddr(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: localListenerAddr(),
			},
			ClusterName:       "leaf1",
			ProxyListenerMode: types.ProxyListenerMode_Multiplex,
		},
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Log = utils.NewLoggerForTests()
	err = config.ApplyFileConfig(fileConfig, cfg)
	require.NoError(t, err)

	user, err := user.Current()
	require.NoError(t, err)

	cfg.Proxy.DisableWebInterface = true
	sshLoginRole, err := types.NewRole("ssh-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{user.Username},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	tc, err := types.NewTrustedCluster("root-cluster", types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                staticToken,
		ProxyAddress:         s.root.Config.Proxy.WebAddr.String(),
		ReverseTunnelAddress: s.root.Config.Proxy.WebAddr.String(),
		RoleMap: []types.RoleMapping{
			{
				Remote: "access",
				Local:  []string{"access", "ssh-login"},
			},
		},
	})
	require.NoError(t, err)
	cfg.Auth.BootstrapResources = []types.Resource{sshLoginRole}
	if options.leafConfigFunc != nil {
		options.leafConfigFunc(cfg)
	}
	s.leaf = runTeleport(t, cfg)

	_, err = s.leaf.GetAuthServer().UpsertTrustedCluster(s.leaf.ExitContext(), tc)
	require.NoError(t, err)
}

type testSuiteOptions struct {
	rootConfigFunc func(cfg *servicecfg.Config)
	leafConfigFunc func(cfg *servicecfg.Config)
	leafCluster    bool
	validationFunc func(*suite) bool
}

type testSuiteOptionFunc func(o *testSuiteOptions)

func withRootConfigFunc(fn func(cfg *servicecfg.Config)) testSuiteOptionFunc {
	return func(o *testSuiteOptions) {
		o.rootConfigFunc = fn
	}
}

func withLeafConfigFunc(fn func(cfg *servicecfg.Config)) testSuiteOptionFunc {
	return func(o *testSuiteOptions) {
		o.leafConfigFunc = fn
	}
}

func withLeafCluster() testSuiteOptionFunc {
	return func(o *testSuiteOptions) {
		o.leafCluster = true
	}
}

func withValidationFunc(f func(*suite) bool) testSuiteOptionFunc {
	return func(o *testSuiteOptions) {
		o.validationFunc = f
	}
}

func (s *suite) setMockSSOLogin(t *testing.T) CliOption {
	return func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user.GetName())
		cf.AuthConnector = s.connector.GetName()
		return nil
	}
}

func (s *suite) mustLogin(t *testing.T, opts ...CliOption) CliOption {
	_, _, opt := mustLoginHome(t, s.root.GetAuthServer(), s.root.Config.Proxy.WebAddr.String(), s.user.GetName(), s.connector.GetName(), opts...)
	return opt
}

func (s *suite) mustLoginIdentity(t *testing.T, opts ...CliOption) CliOption {
	_, opt := mustLoginIdentity(t, s.root.GetAuthServer(), s.root.Config.Proxy.WebAddr.String(), s.user.GetName(), s.connector.GetName(), opts...)
	return opt
}

// login with new temp tshHome and set it in Env. This is useful
// when running "ssh" commands with a tsh "ProxyCommand".
func (s *suite) mustLoginSetEnv(t *testing.T, opts ...CliOption) string {
	tshHome, _, _ := mustLoginHome(t, s.root.GetAuthServer(), s.root.Config.Proxy.WebAddr.String(), s.user.GetName(), s.connector.GetName(), opts...)
	t.Setenv(types.HomeEnvVar, tshHome)
	return tshHome
}

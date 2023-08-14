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
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
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

func runTeleport(t *testing.T, cfg *servicecfg.Config) *service.TeleportProcess {
	if cfg.InstanceMetadataClient == nil {
		// Disables cloud auto-imported labels when running tests in cloud envs
		// such as Github Actions.
		//
		// This is required otherwise Teleport will import cloud instance
		// labels, and use them for example as labels in Kubernetes Service and
		// cause some tests to fail because the output includes unexpected
		// labels.
		//
		// It is also found that Azure metadata client can throw "Too many
		// requests" during CI which fails services.NewTeleport.
		cfg.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	}
	process, err := service.NewTeleport(cfg)
	require.NoError(t, err, trace.DebugReport(err))
	require.NoError(t, process.Start())
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	var serviceReadyEvents []string
	if cfg.Proxy.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.ProxyWebServerReady)
	}
	if cfg.SSH.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.NodeSSHReady)
	}
	if cfg.Databases.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.DatabasesReady)
	}
	if cfg.Apps.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.AppsReady)
	}
	if cfg.Auth.Enabled {
		serviceReadyEvents = append(serviceReadyEvents, service.AuthTLSReady)
	}
	waitForEvents(t, process, serviceReadyEvents...)

	if cfg.Auth.Enabled && cfg.Databases.Enabled {
		waitForDatabases(t, process, cfg.Databases.Databases)
	}
	return process
}

func localListenerAddr() string {
	return fmt.Sprintf("localhost:%d", ports.PopInt())
}

func waitForEvents(t *testing.T, svc service.Supervisor, events ...string) {
	for _, event := range events {
		_, err := svc.WaitForEventTimeout(30*time.Second, event)
		require.NoError(t, err, "service server didn't receive %v event after 30s", event)
	}
}

func mustCreateAuthClientFormUserProfile(t *testing.T, tshHomePath, addr string) {
	ctx := context.Background()
	credentials := apiclient.LoadProfile(tshHomePath, "")
	c, err := apiclient.New(context.Background(), apiclient.Config{
		Addrs:                    []string{addr},
		Credentials:              []apiclient.Credentials{credentials},
		InsecureAddressDiscovery: true,
	})
	require.NoError(t, err)
	_, err = c.Ping(ctx)
	require.NoError(t, err)
}

// mustCloneTempDir is a test helper that clones a given directory recursively.
// Useful for parallelizing tests that rely on a ~/.tsh dir, since FSKeystore
// races with multiple tsh clients working in the same profile dir.
func mustCloneTempDir(t *testing.T, srcDir string) string {
	t.Helper()
	dstDir := t.TempDir()
	err := filepath.WalkDir(srcDir, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}

		if srcPath == srcDir {
			// special case: root of the walk. skip copying.
			return nil
		}

		// Construct the corresponding path in the destination directory.
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return trace.Wrap(err)
		}
		dstPath := filepath.Join(dstDir, relPath)

		info, err := d.Info()
		require.NoError(t, err)

		if d.IsDir() {
			// If the current item is a directory, create it in the destination directory.
			if err := os.Mkdir(dstPath, info.Mode().Perm()); err != nil {
				return trace.Wrap(err)
			}
		} else {
			// If the current item is a file, copy it to the destination directory.
			srcFile, err := os.Open(srcPath)
			if err != nil {
				return trace.Wrap(err)
			}
			defer srcFile.Close()

			dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
			if err != nil {
				return trace.Wrap(err)
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, srcFile); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	require.NoError(t, err)
	return dstDir
}

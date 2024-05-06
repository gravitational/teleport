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

package common

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
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
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Version: "v2",
		Global: config.Global{
			DataDir:  t.TempDir(),
			NodeName: "rootnode",
		},
		SSH: config.SSH{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.NodeSSHAddr,
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.ProxySSHAddr,
			},
			SSHPublicAddr: []string{dynAddr.ProxySSHAddr},
			WebAddr:       dynAddr.WebAddr,
			TunAddr:       dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
			ClusterName:      "root",
			SessionRecording: "node-sync",
		},
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Log = utils.NewLoggerForTests()
	err := config.ApplyFileConfig(fileConfig, cfg)
	require.NoError(t, err)
	cfg.FileDescriptors = dynAddr.Descriptors

	cfg.Proxy.DisableWebInterface = true
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	require.NoError(t, err)
	cfg.SetToken(staticToken)

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
}

func (s *suite) setupLeafCluster(t *testing.T, options testSuiteOptions) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Version: "v2",
		Global: config.Global{
			DataDir:  t.TempDir(),
			NodeName: "leafnode",
		},
		SSH: config.SSH{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.NodeSSHAddr,
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.ProxySSHAddr,
			},
			SSHPublicAddr: []string{dynAddr.ProxySSHAddr},
			WebAddr:       dynAddr.WebAddr,
			TunAddr:       dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: localListenerAddr(),
			},
			ClusterName:       "leaf1",
			ProxyListenerMode: types.ProxyListenerMode_Multiplex,
			SessionRecording:  "node-sync",
		},
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Log = utils.NewLoggerForTests()
	err := config.ApplyFileConfig(fileConfig, cfg)
	require.NoError(t, err)
	cfg.FileDescriptors = dynAddr.Descriptors

	user, err := user.Current()
	require.NoError(t, err)

	cfg.Proxy.DisableWebInterface = true
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	require.NoError(t, err)
	cfg.SetToken(staticToken)
	sshLoginRole, err := types.NewRole("ssh-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{user.Username},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	tunnelAddr := s.root.Config.Proxy.WebAddr.String()
	if s.root.Config.Auth.NetworkingConfig.GetProxyListenerMode() != types.ProxyListenerMode_Multiplex {
		tunnelAddr = s.root.Config.Proxy.ReverseTunnelListenAddr.String()
	}

	tc, err := types.NewTrustedCluster("root-cluster", types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                staticToken,
		ProxyAddress:         s.root.Config.Proxy.WebAddr.String(),
		ReverseTunnelAddress: tunnelAddr,
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

// deprecated: Use `tools/teleport/testenv.MakeTestServer` instead.
func newTestSuite(t *testing.T, opts ...testSuiteOptionFunc) *suite {
	var options testSuiteOptions
	for _, opt := range opts {
		opt(&options)
	}
	s := &suite{}

	s.setupRootCluster(t, options)

	if options.leafCluster || options.leafConfigFunc != nil {
		s.setupLeafCluster(t, options)
		require.Eventually(t, func() bool {
			rt, err := s.root.GetAuthServer().GetTunnelConnections(s.leaf.Config.Auth.ClusterName.GetClusterName())
			require.NoError(t, err)
			return len(rt) == 1
		}, time.Second*10, time.Second)
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

func mustMakeDynamicKubeCluster(t *testing.T, name, discoveredName string, labels map[string]string) types.KubeCluster {
	t.Helper()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[types.OriginLabel] = types.OriginDynamic
	if discoveredName != "" {
		// setup a fake "discovered" kube cluster by adding a discovered name label
		labels[types.DiscoveredNameLabel] = discoveredName
		labels[types.OriginLabel] = types.OriginCloud
	}
	kc, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   name,
			Labels: labels,
		},
		types.KubernetesClusterSpecV3{
			Kubeconfig: newKubeConfig(t, name),
		},
	)
	require.NoError(t, err)
	return kc
}

func mustRegisterKubeClusters(t *testing.T, ctx context.Context, authSrv *auth.Server, clusters ...types.KubeCluster) {
	t.Helper()
	if len(clusters) == 0 {
		return
	}

	wg, _ := errgroup.WithContext(ctx)
	wantNames := make([]string, 0, len(clusters))
	for _, kc := range clusters {
		kc := kc
		wg.Go(func() error {
			err := authSrv.CreateKubernetesCluster(ctx, kc)
			return trace.Wrap(err)
		})
		wantNames = append(wantNames, kc.GetName())
	}
	require.NoError(t, wg.Wait())

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		servers, err := authSrv.GetKubernetesServers(ctx)
		assert.NoError(c, err)
		gotNames := map[string]struct{}{}
		for _, ks := range servers {
			gotNames[ks.GetName()] = struct{}{}
		}
		for _, name := range wantNames {
			assert.Contains(c, gotNames, name, "missing kube cluster")
		}
	}, time.Second*10, time.Millisecond*500, "dynamically created kube clusters failed to register")
}

func setupWebAuthnChallengeSolver(device *mocku2f.Key, success bool) CliOption {
	return func(c *CLIConf) error {
		c.WebauthnLogin = func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
			car, err := device.SignAssertion(origin, assertion)
			if err != nil {
				return nil, "", err
			}

			carProto := wantypes.CredentialAssertionResponseToProto(car)
			if !success {
				carProto.Type = "NOT A VALID TYPE" // set to an invalid type so the ceremony fails
			}

			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: carProto,
				},
			}, "", nil
		}
		return nil
	}
}

func registerDeviceForUser(t *testing.T, authServer *auth.Server, device *mocku2f.Key, username string, origin string) {
	ctx := context.Background()
	token, err := authServer.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
		Name: username,
	})
	require.NoError(t, err)

	tokenID := token.GetName()
	res, err := authServer.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:     tokenID,
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)
	cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

	ccr, err := device.SignCredentialCreation(origin, cc)
	require.NoError(t, err)
	_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID: tokenID,
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err)
}

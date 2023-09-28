// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hsm

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	// Enable HSM feature.
	// This is safe to do here, as all tests in this package require HSM to be
	// enabled.
	modules.SetModules(&modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			HSM: true,
		},
	})

	os.Exit(m.Run())
}

func newHSMAuthConfig(t *testing.T, storageConfig *backend.Config, log utils.Logger) *servicecfg.Config {
	config := newAuthConfig(t, log)

	config.Auth.StorageConfig = *storageConfig

	if gcpKeyring := os.Getenv("TEST_GCP_KMS_KEYRING"); gcpKeyring != "" {
		config.Auth.KeyStore.GCPKMS.KeyRing = gcpKeyring
		config.Auth.KeyStore.GCPKMS.ProtectionLevel = "HSM"
	} else {
		config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	}

	return config
}

func etcdBackendConfig(t *testing.T) *backend.Config {
	prefix := uuid.NewString()
	cfg := &backend.Config{
		Type: "etcd",
		Params: backend.Params{
			"peers":         []string{etcdTestEndpoint()},
			"prefix":        prefix,
			"tls_key_file":  "../../examples/etcd/certs/client-key.pem",
			"tls_cert_file": "../../examples/etcd/certs/client-cert.pem",
			"tls_ca_file":   "../../examples/etcd/certs/ca-cert.pem",
		},
	}
	t.Cleanup(func() {
		bk, err := etcdbk.New(context.Background(), cfg.Params)
		require.NoError(t, err)
		require.NoError(t, bk.DeleteRange(context.Background(), []byte(prefix),
			backend.RangeEnd([]byte(prefix))),
			"failed to clean up etcd backend")
	})
	return cfg
}

// etcdTestEndpoint returns etcd host used in tests.
func etcdTestEndpoint() string {
	host := os.Getenv("TELEPORT_ETCD_TEST_ENDPOINT")
	if host != "" {
		return host
	}
	return "https://127.0.0.1:2379"
}

func liteBackendConfig(t *testing.T) *backend.Config {
	return &backend.Config{
		Type: lite.GetName(),
		Params: backend.Params{
			"path": t.TempDir(),
		},
	}
}

func requireHSMAvailable(t *testing.T) {
	if os.Getenv("SOFTHSM2_PATH") == "" && os.Getenv("TEST_GCP_KMS_KEYRING") == "" {
		t.Skip("Skipping test because neither SOFTHSM2_PATH or TEST_GCP_KMS_KEYRING are set")
	}
}

func requireETCDAvailable(t *testing.T) {
	if os.Getenv("TELEPORT_ETCD_TEST") == "" {
		t.Skip("Skipping test because TELEPORT_ETCD_TEST is not set")
	}
}

// Tests a single CA rotation with a single HSM auth server
func TestHSMRotation(t *testing.T) {
	requireHSMAvailable(t)

	// pick a conservative timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	t.Cleanup(cancel)
	log := utils.NewLoggerForTests()

	log.Debug("TestHSMRotation: starting auth server")
	authConfig := newHSMAuthConfig(t, liteBackendConfig(t), log)
	auth1 := newTeleportService(t, authConfig, "auth1")
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})
	allServices := teleportServices{auth1}

	log.Debug("TestHSMRotation: waiting for auth server to start")
	require.NoError(t, auth1.start(ctx))

	// start a proxy to make sure it can get creds at each stage of rotation
	log.Debug("TestHSMRotation: starting proxy")
	proxy := newTeleportService(t, newProxyConfig(t, auth1.authAddr(t), log), "proxy")
	require.NoError(t, proxy.start(ctx))
	allServices = append(allServices, proxy)

	log.Debug("TestHSMRotation: sending rotation request init")
	err := auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, allServices.waitForPhaseChange(ctx))

	log.Debug("TestHSMRotation: sending rotation request update_clients")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, allServices.waitForRestart(ctx))

	log.Debug("TestHSMRotation: sending rotation request update_servers")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, allServices.waitForRestart(ctx))

	log.Debug("TestHSMRotation: sending rotation request standby")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, allServices.waitForRestart(ctx))
}

// Tests multiple CA rotations and rollbacks with 2 HSM auth servers in an HA configuration
func TestHSMDualAuthRotation(t *testing.T) {
	// TODO(nklaassen): fix this test and re-enable it.
	// https://github.com/gravitational/teleport/issues/20217
	t.Skip("TestHSMDualAuthRotation is temporarily disabled due to flakiness")

	requireHSMAvailable(t)
	requireETCDAvailable(t)

	// pick a global timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	log := utils.NewLoggerForTests()
	storageConfig := etcdBackendConfig(t)

	// start a cluster with 1 auth server and a proxy
	log.Debug("TestHSMDualAuthRotation: Starting auth server 1")
	auth1Config := newHSMAuthConfig(t, storageConfig, log)
	auth1 := newTeleportService(t, auth1Config, "auth1")
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil),
			"failed to delete hsm keys during test cleanup")
	})
	authServices := teleportServices{auth1}
	allServices := append(teleportServices{}, authServices...)
	require.NoError(t, authServices.start(ctx), "auth service failed initial startup")

	log.Debug("TestHSMDualAuthRotation: Starting load balancer")
	hostName, err := os.Hostname()
	require.NoError(t, err)
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort(hostName, "0")),
		auth1.authAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	// start a proxy to make sure it can get creds at each stage of rotation
	log.Debug("TestHSMDualAuthRotation: Starting proxy")
	proxyConfig := newProxyConfig(t, utils.FromAddr(lb.Addr()), log)
	proxy := newTeleportService(t, proxyConfig, "proxy")
	require.NoError(t, proxy.start(ctx), "proxy failed initial startup")
	allServices = append(allServices, proxy)

	// add a new auth server
	log.Debug("TestHSMDualAuthRotation: Starting auth server 2")
	auth2Config := newHSMAuthConfig(t, storageConfig, log)
	auth2 := newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth2.start(ctx))
	t.Cleanup(func() {
		require.NoError(t, auth2.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})
	authServices = append(authServices, auth2)
	allServices = append(allServices, auth2)

	// make sure the admin identity used by tctl works
	getAdminClient := func() *auth.Client {
		identity, err := auth.ReadLocalIdentity(
			filepath.Join(auth2Config.DataDir, teleport.ComponentProcess),
			auth.IdentityID{Role: types.RoleAdmin, HostUUID: auth2Config.HostUUID})
		require.NoError(t, err)
		tlsConfig, err := identity.TLSConfig(nil)
		require.NoError(t, err)
		authAddrs := []utils.NetAddr{auth2.authAddr(t)}
		clt, err := auth.NewClient(client.Config{
			Addrs: utils.NetAddrsToStrings(authAddrs),
			Credentials: []client.Credentials{
				client.LoadTLS(tlsConfig),
			},
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
		})
		require.NoError(t, err)
		return clt
	}
	testClient := func(clt *auth.Client) error {
		_, err = clt.GetClusterName()
		return trace.Wrap(err)
	}
	clt := getAdminClient()
	require.NoError(t, testClient(clt))

	stages := []struct {
		targetPhase string
		verify      func(t *testing.T)
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
	}

	// do a full rotation
	for _, stage := range stages {
		log.Debugf("TestHSMDualAuthRotation: Sending rotate request %s", stage.targetPhase)
		require.NoError(t, auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: stage.targetPhase,
			Mode:        types.RotationModeManual,
		}))
		stage.verify(t)
	}

	// Safe to send traffic to new auth server now that a full rotation has been completed.
	lb.AddBackend(auth2.authAddr(t))

	// load balanced client shoud work with either backend
	getAdminClient = func() *auth.Client {
		identity, err := auth.ReadLocalIdentity(
			filepath.Join(auth2Config.DataDir, teleport.ComponentProcess),
			auth.IdentityID{Role: types.RoleAdmin, HostUUID: auth2Config.HostUUID})
		require.NoError(t, err)
		tlsConfig, err := identity.TLSConfig(nil)
		require.NoError(t, err)
		authAddrs := []string{lb.Addr().String()}
		clt, err := auth.NewClient(client.Config{
			Addrs: authAddrs,
			Credentials: []client.Credentials{
				client.LoadTLS(tlsConfig),
			},
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
		})
		require.NoError(t, err)
		return clt
	}
	testClient = func(clt *auth.Client) error {
		_, err1 := clt.GetClusterName()
		_, err2 := clt.GetClusterName()
		return trace.NewAggregate(err1, err2)
	}
	clt = getAdminClient()
	require.NoError(t, testClient(clt))

	// Do another full rotation from the new auth server
	for _, stage := range stages {
		log.Debugf("TestHSMDualAuthRotation: Sending rotate request %s", stage.targetPhase)
		require.NoError(t, auth2.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: stage.targetPhase,
			Mode:        types.RotationModeManual,
		}))
		stage.verify(t)
	}

	// test rollbacks
	stages = []struct {
		targetPhase string
		verify      func(t *testing.T)
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
	}
	for _, stage := range stages {
		log.Debugf("TestHSMDualAuthRotation: Sending rotate request %s", stage.targetPhase)
		require.NoError(t, auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: stage.targetPhase,
			Mode:        types.RotationModeManual,
		}))
		stage.verify(t)
	}
}

// Tests a dual-auth server migration from raw keys to HSM keys
func TestHSMMigrate(t *testing.T) {
	requireHSMAvailable(t)
	requireETCDAvailable(t)

	// pick a global timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	log := utils.NewLoggerForTests()
	storageConfig := etcdBackendConfig(t)

	// start a dual auth non-hsm cluster
	log.Debug("TestHSMMigrate: Starting auth server 1")
	auth1Config := newHSMAuthConfig(t, storageConfig, log)
	auth1Config.Auth.KeyStore = keystore.Config{}
	auth1 := newTeleportService(t, auth1Config, "auth1")
	auth2Config := newHSMAuthConfig(t, storageConfig, log)
	auth2Config.Auth.KeyStore = keystore.Config{}
	auth2 := newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth1.start(ctx))
	require.NoError(t, auth2.start(ctx))

	log.Debug("TestHSMMigrate: Starting load balancer")
	hostName, err := os.Hostname()
	require.NoError(t, err)
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort(hostName, "0")),
		auth1.authAddr(t),
		auth2.authAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	// start a proxy to make sure it can get creds at each stage of migration
	log.Debug("TestHSMMigrate: Starting proxy")
	proxyConfig := newProxyConfig(t, utils.FromAddr(lb.Addr()), log)
	proxy := newTeleportService(t, proxyConfig, "proxy")
	require.NoError(t, proxy.start(ctx))

	// make sure the admin identity used by tctl works
	getAdminClient := func() *auth.Client {
		identity, err := auth.ReadLocalIdentity(
			filepath.Join(auth2Config.DataDir, teleport.ComponentProcess),
			auth.IdentityID{Role: types.RoleAdmin, HostUUID: auth2Config.HostUUID})
		require.NoError(t, err)
		tlsConfig, err := identity.TLSConfig(nil)
		require.NoError(t, err)
		authAddrs := []utils.NetAddr{auth2.authAddr(t)}
		clt, err := auth.NewClient(client.Config{
			Addrs: utils.NetAddrsToStrings(authAddrs),
			Credentials: []client.Credentials{
				client.LoadTLS(tlsConfig),
			},
			CircuitBreakerConfig: breaker.NoopBreakerConfig(),
		})
		require.NoError(t, err)
		return clt
	}
	testClient := func(clt *auth.Client) error {
		_, err1 := clt.GetClusterName()
		_, err2 := clt.GetClusterName()
		return trace.NewAggregate(err1, err2)
	}
	clt := getAdminClient()
	require.NoError(t, testClient(clt))

	// Phase 1: migrate auth1 to HSM
	lb.RemoveBackend(auth1.authAddr(t))
	auth1.process.Close()
	require.NoError(t, auth1.waitForShutdown(ctx))
	auth1Config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	auth1 = newTeleportService(t, auth1Config, "auth1")
	require.NoError(t, auth1.start(ctx))

	clt = getAdminClient()
	require.NoError(t, testClient(clt))

	authServices := teleportServices{auth1, auth2}
	allServices := teleportServices{auth1, auth2, proxy}

	stages := []struct {
		targetPhase string
		verify      func(t *testing.T)
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, allServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
	}

	// do a full rotation
	for _, stage := range stages {
		log.Debugf("TestHSMMigrate: Sending rotate request %s", stage.targetPhase)
		require.NoError(t, auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: stage.targetPhase,
			Mode:        types.RotationModeManual,
		}))
		stage.verify(t)
	}

	// Safe to send traffic to new auth1 again
	lb.AddBackend(auth1.authAddr(t))

	// Phase 2: migrate auth2 to HSM
	lb.RemoveBackend(auth2.authAddr(t))
	auth2.process.Close()
	require.NoError(t, auth2.waitForShutdown(ctx))
	auth2Config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	auth2 = newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth2.start(ctx))

	authServices = teleportServices{auth1, auth2}
	allServices = teleportServices{auth1, auth2, proxy}

	clt = getAdminClient()
	require.NoError(t, testClient(clt))

	// do a full rotation
	for _, stage := range stages {
		log.Debugf("TestHSMMigrate: Sending rotate request %s", stage.targetPhase)
		require.NoError(t, auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
			Type:        types.HostCA,
			TargetPhase: stage.targetPhase,
			Mode:        types.RotationModeManual,
		}))
		stage.verify(t)
	}

	// Safe to send traffic to new auth2 again
	lb.AddBackend(auth2.authAddr(t))
	require.NoError(t, testClient(clt))
}

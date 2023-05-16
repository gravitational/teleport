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

package integration

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
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
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

type teleportService struct {
	name           string
	log            utils.Logger
	config         *servicecfg.Config
	process        *service.TeleportProcess
	serviceChannel chan *service.TeleportProcess
	errorChannel   chan error
}

func newTeleportService(t *testing.T, config *servicecfg.Config, name string) *teleportService {
	s := &teleportService{
		config:         config,
		name:           name,
		log:            config.Log,
		serviceChannel: make(chan *service.TeleportProcess, 1),
		errorChannel:   make(chan error, 1),
	}
	t.Cleanup(func() {
		require.NoError(t, s.Close(), "error while closing %s during test cleanup", name)
	})
	return s
}

func (t *teleportService) Close() error {
	if t.process == nil {
		return nil
	}
	if err := t.process.Close(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(t.process.Wait())
}

func (t *teleportService) start(ctx context.Context) {
	go func() {
		t.errorChannel <- service.Run(ctx, *t.config, func(cfg *servicecfg.Config) (service.Process, error) {
			t.log.Debugf("(Re)starting %s", t.name)
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				t.log.Debugf("started %s, writing to serviceChannel", t.name)
				t.serviceChannel <- svc
			}
			return svc, trace.Wrap(err)
		})
	}()
}

func (t *teleportService) waitForStart(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to start", t.name)
	t.start(ctx)
	select {
	case t.process = <-t.serviceChannel:
	case err := <-t.errorChannel:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "timed out waiting for %s to start", t.name)
	}
	t.log.Debugf("read %s from serviceChannel", t.name)
	return t.waitForReady(ctx)
}

func (t *teleportService) waitForReady(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to be ready", t.name)
	if _, err := t.process.WaitForEvent(ctx, service.TeleportReadyEvent); err != nil {
		return trace.Wrap(err, "timed out waiting for %s to be ready", t.name)
	}
	// also wait for AuthIdentityEvent so that we can read the admin credentials
	// and create a test client
	if t.process.GetAuthServer() != nil {
		if _, err := t.process.WaitForEvent(ctx, service.AuthIdentityEvent); err != nil {
			return trace.Wrap(err, "timed out waiting for %s auth identity event", t.name)
		}
		t.log.Debugf("%s is ready", t.name)
	}
	return nil
}

func (t *teleportService) waitForRestart(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to restart", t.name)
	// get the new process
	select {
	case t.process = <-t.serviceChannel:
	case err := <-t.errorChannel:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "timed out waiting for %s to restart", t.name)
	}

	// wait for the new process to be ready
	err := t.waitForReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	t.log.Debugf("%s successfully restarted", t.name)
	return nil
}

func (t *teleportService) waitForShutdown(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to shut down", t.name)
	select {
	case err := <-t.errorChannel:
		t.process = nil
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "timed out waiting for %s to shut down", t.name)
	}
}

func (t *teleportService) waitForLocalAdditionalKeys(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to have local additional keys", t.name)
	clusterName, err := t.process.GetAuthServer().GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	hostCAID := types.CertAuthID{DomainName: clusterName.GetClusterName(), Type: types.HostCA}
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "timed out waiting for %s to have local additional keys", t.name)
		case <-time.After(250 * time.Millisecond):
		}
		ca, err := t.process.GetAuthServer().GetCertAuthority(ctx, hostCAID, true)
		if err != nil {
			return trace.Wrap(err)
		}
		hasUsableKeys, err := t.process.GetAuthServer().GetKeyStore().HasUsableAdditionalKeys(ctx, ca)
		if err != nil {
			return trace.Wrap(err)
		}
		if hasUsableKeys {
			break
		}
	}
	t.log.Debugf("%s has local additional keys", t.name)
	return nil
}

func (t *teleportService) waitForPhaseChange(ctx context.Context) error {
	t.log.Debugf("Waiting for %s to change phase", t.name)
	if _, err := t.process.WaitForEvent(ctx, service.TeleportPhaseChangeEvent); err != nil {
		return trace.Wrap(err, "timed out waiting for %s to change phase", t.name)
	}
	t.log.Debugf("%s changed phase", t.name)
	return nil
}

func (t *teleportService) AuthAddr(testingT *testing.T) utils.NetAddr {
	addr, err := t.process.AuthAddr()
	require.NoError(testingT, err)

	return *addr
}

type TeleportServices []*teleportService

func (s TeleportServices) forEach(f func(t *teleportService) error) error {
	for i := range s {
		if err := f(s[i]); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s TeleportServices) waitForStart(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForStart(ctx) })
}

func (s TeleportServices) waitForRestart(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForRestart(ctx) })
}

func (s TeleportServices) waitForLocalAdditionalKeys(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForLocalAdditionalKeys(ctx) })
}

func (s TeleportServices) waitForPhaseChange(ctx context.Context) error {
	return s.forEach(func(t *teleportService) error { return t.waitForPhaseChange(ctx) })
}

func newHSMAuthConfig(ctx context.Context, t *testing.T, storageConfig *backend.Config, log utils.Logger) *servicecfg.Config {
	hostName, err := os.Hostname()
	require.NoError(t, err)

	config := servicecfg.MakeDefaultConfig()
	config.PollingPeriod = 1 * time.Second
	config.SSH.Enabled = false
	config.Proxy.Enabled = false
	config.ClientTimeout = time.Second
	config.ShutdownTimeout = time.Minute
	config.DataDir = t.TempDir()
	config.Auth.ListenAddr.Addr = net.JoinHostPort(hostName, "0")
	config.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        hostName,
		},
	}
	config.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "testcluster",
	})
	require.NoError(t, err)
	config.SetAuthServerAddress(config.Auth.ListenAddr)
	config.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{"Proxy", "Node"},
				Token: "foo",
			},
		},
	})
	require.NoError(t, err)
	config.Log = log
	if storageConfig != nil {
		config.Auth.StorageConfig = *storageConfig
	}
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	config.InstanceMetadataClient = cloud.NewDisabledIMDSClient()

	if gcpKeyring := os.Getenv("TEST_GCP_KMS_KEYRING"); gcpKeyring != "" {
		config.Auth.KeyStore.GCPKMS.KeyRing = gcpKeyring
		config.Auth.KeyStore.GCPKMS.ProtectionLevel = "HSM"
	} else {
		config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	}

	return config
}

func newProxyConfig(ctx context.Context, t *testing.T, authAddr utils.NetAddr, log utils.Logger) *servicecfg.Config {
	hostName, err := os.Hostname()
	require.NoError(t, err)

	config := servicecfg.MakeDefaultConfig()
	config.PollingPeriod = 1 * time.Second
	config.SetToken("foo")
	config.SSH.Enabled = false
	config.Auth.Enabled = false
	config.Proxy.Enabled = true
	config.Proxy.DisableWebInterface = true
	config.Proxy.DisableWebService = true
	config.Proxy.DisableReverseTunnel = true
	config.Proxy.SSHAddr.Addr = net.JoinHostPort(hostName, "0")
	config.Proxy.WebAddr.Addr = net.JoinHostPort(hostName, "0")
	config.CachePolicy.Enabled = true
	config.PollingPeriod = 1 * time.Second
	config.ShutdownTimeout = time.Minute
	config.DataDir = t.TempDir()
	config.SetAuthServerAddress(authAddr)
	config.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	config.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	config.Log = log
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
	authConfig := newHSMAuthConfig(ctx, t, liteBackendConfig(t), log)
	auth1 := newTeleportService(t, authConfig, "auth1")
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})
	teleportServices := TeleportServices{auth1}

	log.Debug("TestHSMRotation: waiting for auth server to start")
	require.NoError(t, auth1.waitForStart(ctx))

	// start a proxy to make sure it can get creds at each stage of rotation
	log.Debug("TestHSMRotation: starting proxy")
	proxy := newTeleportService(t, newProxyConfig(ctx, t, auth1.AuthAddr(t), log), "proxy")
	require.NoError(t, proxy.waitForStart(ctx))
	teleportServices = append(teleportServices, proxy)

	log.Debug("TestHSMRotation: sending rotation request init")
	err := auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, teleportServices.waitForPhaseChange(ctx))

	log.Debug("TestHSMRotation: sending rotation request update_clients")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, teleportServices.waitForRestart(ctx))

	log.Debug("TestHSMRotation: sending rotation request update_servers")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, teleportServices.waitForRestart(ctx))

	log.Debug("TestHSMRotation: sending rotation request standby")
	err = auth1.process.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
	require.NoError(t, teleportServices.waitForRestart(ctx))
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
	auth1Config := newHSMAuthConfig(ctx, t, storageConfig, log)
	auth1 := newTeleportService(t, auth1Config, "auth1")
	t.Cleanup(func() {
		require.NoError(t, auth1.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil),
			"failed to delete hsm keys during test cleanup")
	})
	authServices := TeleportServices{auth1}
	teleportServices := append(TeleportServices{}, authServices...)
	require.NoError(t, authServices.waitForStart(ctx), "auth service failed initial startup")

	log.Debug("TestHSMDualAuthRotation: Starting load balancer")
	hostName, err := os.Hostname()
	require.NoError(t, err)
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort(hostName, "0")),
		auth1.AuthAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	// start a proxy to make sure it can get creds at each stage of rotation
	log.Debug("TestHSMDualAuthRotation: Starting proxy")
	proxyConfig := newProxyConfig(ctx, t, utils.FromAddr(lb.Addr()), log)
	proxy := newTeleportService(t, proxyConfig, "proxy")
	require.NoError(t, proxy.waitForStart(ctx), "proxy failed initial startup")
	teleportServices = append(teleportServices, proxy)

	// add a new auth server
	log.Debug("TestHSMDualAuthRotation: Starting auth server 2")
	auth2Config := newHSMAuthConfig(ctx, t, storageConfig, log)
	auth2 := newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth2.waitForStart(ctx))
	t.Cleanup(func() {
		require.NoError(t, auth2.process.GetAuthServer().GetKeyStore().DeleteUnusedKeys(ctx, nil))
	})
	authServices = append(authServices, auth2)
	teleportServices = append(teleportServices, auth2)

	// make sure the admin identity used by tctl works
	getAdminClient := func() *auth.Client {
		identity, err := auth.ReadLocalIdentity(
			filepath.Join(auth2Config.DataDir, teleport.ComponentProcess),
			auth.IdentityID{Role: types.RoleAdmin, HostUUID: auth2Config.HostUUID})
		require.NoError(t, err)
		tlsConfig, err := identity.TLSConfig(nil)
		require.NoError(t, err)
		authAddrs := []utils.NetAddr{auth2.AuthAddr(t)}
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
				require.NoError(t, teleportServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
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
	lb.AddBackend(auth2.AuthAddr(t))

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
				require.NoError(t, teleportServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseRollback,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
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
	auth1Config := newHSMAuthConfig(ctx, t, storageConfig, log)
	auth1Config.Auth.KeyStore = keystore.Config{}
	auth1 := newTeleportService(t, auth1Config, "auth1")
	auth2Config := newHSMAuthConfig(ctx, t, storageConfig, log)
	auth2Config.Auth.KeyStore = keystore.Config{}
	auth2 := newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth1.waitForStart(ctx))
	require.NoError(t, auth2.waitForStart(ctx))

	log.Debug("TestHSMMigrate: Starting load balancer")
	hostName, err := os.Hostname()
	require.NoError(t, err)
	lb, err := utils.NewLoadBalancer(
		ctx,
		*utils.MustParseAddr(net.JoinHostPort(hostName, "0")),
		auth1.AuthAddr(t),
		auth2.AuthAddr(t),
	)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	t.Cleanup(func() { require.NoError(t, lb.Close()) })

	// start a proxy to make sure it can get creds at each stage of migration
	log.Debug("TestHSMMigrate: Starting proxy")
	proxyConfig := newProxyConfig(ctx, t, utils.FromAddr(lb.Addr()), log)
	proxy := newTeleportService(t, proxyConfig, "proxy")
	require.NoError(t, proxy.waitForStart(ctx))

	// make sure the admin identity used by tctl works
	getAdminClient := func() *auth.Client {
		identity, err := auth.ReadLocalIdentity(
			filepath.Join(auth2Config.DataDir, teleport.ComponentProcess),
			auth.IdentityID{Role: types.RoleAdmin, HostUUID: auth2Config.HostUUID})
		require.NoError(t, err)
		tlsConfig, err := identity.TLSConfig(nil)
		require.NoError(t, err)
		authAddrs := []utils.NetAddr{auth2.AuthAddr(t)}
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
	lb.RemoveBackend(auth1.AuthAddr(t))
	auth1.process.Close()
	require.NoError(t, auth1.waitForShutdown(ctx))
	auth1Config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	auth1 = newTeleportService(t, auth1Config, "auth1")
	require.NoError(t, auth1.waitForStart(ctx))

	clt = getAdminClient()
	require.NoError(t, testClient(clt))

	authServices := TeleportServices{auth1, auth2}
	teleportServices := TeleportServices{auth1, auth2, proxy}

	stages := []struct {
		targetPhase string
		verify      func(t *testing.T)
	}{
		{
			targetPhase: types.RotationPhaseInit,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForPhaseChange(ctx))
				require.NoError(t, authServices.waitForLocalAdditionalKeys(ctx))
				clt := getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateClients,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseUpdateServers,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
				clt = getAdminClient()
				require.NoError(t, testClient(clt))
			},
		},
		{
			targetPhase: types.RotationPhaseStandby,
			verify: func(t *testing.T) {
				require.NoError(t, teleportServices.waitForRestart(ctx))
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
	lb.AddBackend(auth1.AuthAddr(t))

	// Phase 2: migrate auth2 to HSM
	lb.RemoveBackend(auth2.AuthAddr(t))
	auth2.process.Close()
	require.NoError(t, auth2.waitForShutdown(ctx))
	auth2Config.Auth.KeyStore = keystore.SetupSoftHSMTest(t)
	auth2 = newTeleportService(t, auth2Config, "auth2")
	require.NoError(t, auth2.waitForStart(ctx))

	authServices = TeleportServices{auth1, auth2}
	teleportServices = TeleportServices{auth1, auth2, proxy}

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
	lb.AddBackend(auth2.AuthAddr(t))
	require.NoError(t, testClient(clt))
}

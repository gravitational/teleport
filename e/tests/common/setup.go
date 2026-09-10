package common

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common/tctl"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// SUT is a system under test.
type SUT struct {
	Teleport *helpers.TeleInstance
	Clock    clockwork.Clock

	DataDir          string
	ProxyAddr        string
	AuthListenerAddr string
}

func InitSUT(t *testing.T, opts ...Option) *SUT {
	options := &sutOptions{
		license: "../../fixtures/license-eub.pem",
		clock:   clockwork.NewRealClock(),
		modules: modulestest.OSSModules(),
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(options)
	}

	cfg := newInstanceConfig(t)
	cfg.Clock = options.clock
	if options.clusterName != "" {
		cfg.ClusterName = options.clusterName
		cfg.NodeName = options.clusterName + "-node"
	}
	cfg.Logger = options.logger
	cfg.Modules = options.modules
	teleport := helpers.NewInstance(t, cfg)

	teleport.ProcessProvider = &entProcessProvider{}

	serviceConfig := newTeleportConfig(t)
	serviceConfig.Modules = options.modules

	serviceConfig.InsecureMode = options.insecureMode
	serviceConfig.Auth.BootstrapResources = options.resources
	if options.license != "" {
		serviceConfig.Auth.LicenseFile = options.license
	}

	serviceConfig.Auth.HostedPlugins.Enabled = true
	serviceConfig.Clock = options.clock
	serviceConfig.Testing.HTTPTransport = options.HTTPTransport
	serviceConfig.Auth.Preference.SetSecondFactor(constants.SecondFactorOptional)
	serviceConfig.Auth.Preference.SetWebauthn(&types.Webauthn{RPID: "127.0.0.1"})
	serviceConfig.Apps = options.appConfig

	// Set user monitor intervals to be short to speed up tests that involve user state changes.
	// And avoid flakiness in tests where user state changes are expected to be detected within a short time frame.
	serviceConfig.UserMonitor.LockTTL = time.Second
	serviceConfig.UserMonitor.ReconcileInterval = time.Second

	serviceConfig.CachePolicy.Enabled = !options.disableCache

	err := teleport.CreateEx(t, nil, serviceConfig)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, teleport.StopAll())
	})

	auth := teleport.Process.GetAuthServer()
	_, err = auth.GetAccessLists(t.Context())
	require.NoError(t, err)

	sut := SUT{
		Teleport:         teleport,
		Clock:            options.clock,
		DataDir:          serviceConfig.DataDir,
		ProxyAddr:        serviceConfig.Proxy.WebAddr.String(),
		AuthListenerAddr: serviceConfig.Auth.ListenAddr.String(),
	}

	if options.samlConnector != "" {
		_, err := sut.Teleport.Process.GetAuthServer().CreateSAMLConnector(t.Context(), mustUnmarshalSAMLConnector(t, options.samlConnector))
		require.NoError(t, err)
	}
	return &sut
}

func (s *SUT) GetAuthServiceGRPCConn(t *testing.T, user string) *grpc.ClientConn {
	tc := s.GetClusterClientForUser(t, user)
	authClient, ok := tc.AuthClient.(*authclient.Client)
	require.True(t, ok)
	return authClient.APIClient.GetConnection()
}

func (s *SUT) GetOktaAuthClient(t *testing.T, user string) oktapb.OktaServiceClient {
	return oktapb.NewOktaServiceClient(s.GetAuthServiceGRPCConn(t, user))
}

// GetClusterClientForUser returns a client for the given user.
func (s *SUT) GetClusterClientForUser(t *testing.T, user string) *client.ClusterClient {
	t.Helper()
	var tc *client.ClusterClient
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		aliceClient, err := s.Teleport.NewClient(helpers.ClientConfig{
			TeleportUser: user,
			Cluster:      s.Teleport.Config.Auth.ClusterName.GetClusterName(),
			Host:         s.ProxyAddr,
		})
		require.NoError(collect, err)

		tc, err = aliceClient.ConnectToCluster(context.Background())
		require.NoError(collect, err)
		_, err = tc.AuthClient.Ping(context.Background())
		if err != nil {
			tc.Close()
		}
		require.NoError(collect, err)
		t.Cleanup(func() {
			tc.Close()
		})
	}, time.Second*20, time.Millisecond*100)
	_, err := tc.AuthClient.Ping(context.Background())
	require.NoError(t, err)

	return tc
}

func (s *SUT) CreateWebClientForUser(t *testing.T, user string) *helpers.WebClientPack {
	pass := uuid.NewString()
	require.NoError(t, s.Teleport.Process.GetAuthServer().UpsertPassword(user, []byte(pass)))
	return helpers.LoginWebClient(t, s.ProxyAddr, user, pass)
}

func (s *SUT) GetTCTL(t *testing.T) *tctl.CLI {
	tctlCLI, err := tctl.New(s.DataDir, s.AuthListenerAddr)
	require.NoError(t, err)
	t.Cleanup(func() { tctlCLI.Cleanup() })
	return tctlCLI
}

func newInstanceConfig(t *testing.T) helpers.InstanceConfig {
	// Create the CA authority that will be used in Auth.
	kg, err := testauthority.NewKeygen(modules.BuildEnterprise, time.Now)
	require.NoError(t, err)
	priv, pub, err := kg.GenerateKeyPair()
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
		Logger:      logtest.NewLogger(),
	}
}

func newTeleportConfig(t *testing.T) *servicecfg.Config {
	serviceConfig := servicecfg.MakeDefaultConfig()
	serviceConfig.DataDir = t.TempDir()
	// TODO: Propagate the Auth.StorageConfig values to the running cluster, as
	// it's currently blindly overwritten by the integration test setup code.
	serviceConfig.Auth.StorageConfig.Params["path"] = filepath.Join(serviceConfig.DataDir, defaults.BackendDir)
	serviceConfig.Proxy.DisableWebInterface = true
	serviceConfig.Proxy.DisableDatabaseProxy = true
	serviceConfig.SSH.Enabled = false
	serviceConfig.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	serviceConfig.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	serviceConfig.DebugService.Enabled = false
	serviceConfig.PollingPeriod = 500 * time.Millisecond
	serviceConfig.Testing.ClientTimeout = time.Second
	serviceConfig.Testing.ShutdownTimeout = 2 * serviceConfig.Testing.ClientTimeout
	return serviceConfig
}

func mustUnmarshalSAMLConnector(t *testing.T, input string) types.SAMLConnector {
	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw services.UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	connector, err := services.UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	return connector
}

func (s *SUT) NewResourceWatcher(t *testing.T, kind ...string) types.Watcher {
	t.Helper()
	watchKinds := make([]types.WatchKind, 0, len(kind))
	for _, k := range kind {
		watchKinds = append(watchKinds, types.WatchKind{Kind: k})
	}

	watcher, err := s.Teleport.Process.GetAuthServer().NewWatcher(t.Context(), types.Watch{
		Kinds: watchKinds,
	})
	require.NoError(t, err)
	t.Cleanup(func() { watcher.Close() })

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			t.Fatalf("expected initial event, got %s", event.Type)
		}
	case <-t.Context().Done():
		t.Fatal("timeout waiting for initial event")
	}
	return watcher
}

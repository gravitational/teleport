package common

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/constants"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// SUT is a system under test.
type SUT struct {
	Teleport *helpers.TeleInstance
	Clock    clockwork.Clock

	DataDir          string
	ProxyAddr        string
	AuthListenerAddr string
}

func InitSUT(t *testing.T, opts ...option) *SUT {
	options := &sutOptions{
		license: "../../fixtures/license-eub.pem",
		clock:   clockwork.NewRealClock(),
	}

	for _, opt := range opts {
		opt(options)
	}

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	})

	cfg := newInstanceConfig(t)
	cfg.Clock = options.clock
	teleport := helpers.NewInstance(t, cfg)

	teleport.ProcessProvider = &entProcessProvider{}

	serviceConfig := newTeleportConfig()

	serviceConfig.Auth.BootstrapResources = options.resources
	if options.license != "" {
		serviceConfig.Auth.LicenseFile = options.license
	}

	serviceConfig.Auth.HostedPlugins.Enabled = true
	serviceConfig.Clock = options.clock
	serviceConfig.Testing.HTTPTransport = options.HTTPTransport
	serviceConfig.Auth.Preference.SetSecondFactor(constants.SecondFactorOptional)
	serviceConfig.Auth.Preference.SetWebauthn(&types.Webauthn{RPID: "127.0.0.1"})

	err := teleport.CreateEx(t, nil, serviceConfig)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, teleport.StopAll())
	})

	auth := teleport.Process.GetAuthServer()
	_, err = auth.GetAccessLists(context.Background())
	require.NoError(t, err)

	sut := SUT{
		Teleport:         teleport,
		Clock:            options.clock,
		DataDir:          serviceConfig.DataDir,
		ProxyAddr:        serviceConfig.Proxy.WebAddr.String(),
		AuthListenerAddr: serviceConfig.Auth.ListenAddr.String(),
	}

	if options.samlConnector != "" {
		_, err := sut.Teleport.Process.GetAuthServer().CreateSAMLConnector(context.Background(), mustUnmarshalSAMLConnector(t, idp.SAMLConnector))
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
			Cluster:      helpers.Site,
			Host:         helpers.Host,
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
	}, time.Second*5, time.Millisecond*100)
	_, err := tc.AuthClient.Ping(context.Background())
	require.NoError(t, err)

	return tc
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
		Logger:      utils.NewSlogLoggerForTests(),
	}
}

func newTeleportConfig() *servicecfg.Config {
	serviceConfig := servicecfg.MakeDefaultConfig()
	// Replace the default auth and proxy listeners with the ones so we can
	// run multiple tests in parallel.
	serviceConfig.Proxy.DisableWebInterface = true
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

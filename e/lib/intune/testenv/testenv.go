package testenv

import (
	"cmp"
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtenv "github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/e/lib/intune/api"
	intunefake "github.com/gravitational/teleport/e/lib/intune/fake"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest" //nolint:depguard // This is a test package.
	"github.com/gravitational/teleport/lib/utils/log"
)

// DefaultApps are the app credentials added by default to the fake Intune API.
var DefaultApps = []*api.AppCredentials{
	{ClientID: "client-id", ClientSecret: "client-secret", Tenant: "example.onmicrosoft.com"},
}

// Env handles preparing a complete environment for the fake Intune API, including serving the API
// on an actual TLS server and returning an [http.Client] that can connect to the TLS server.
type Env struct {
	Clock  clockwork.Clock
	Logger *slog.Logger

	// server is the server that runs the fake API.
	server *httptest.Server
	API    *intunefake.API
	// HTTPClient is an [http.Client] with the correct TLS settings to access the Intune API.
	HTTPClient *http.Client

	deviceEnv     *dtenv.E
	DevicesClient devicepb.DeviceTrustServiceClient
}

// Config provides values needed by [Env].
type Config struct {
	Clock clockwork.Clock
	// DeviceTrustEnv makes [Env] prepare a test environment for Device Trust as well. This includes
	// changing the build type to Enterprise.
	DeviceTrustEnv bool
}

// MustNew sets up a new TLS server with the fake Intune API.
// Automatically cleans up [Env] when the test finishes.
func MustNew(t *testing.T, config *Config) *Env {
	t.Helper()

	if config.DeviceTrustEnv {
		// Set build type and features.
		modulestest.SetTestModules(t, modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DeviceTrust:            {Enabled: true},
					entitlements.MobileDeviceManagement: {Enabled: true},
				},
			},
		})
	}

	level := slog.LevelError + 1 // Silence logging by default.
	if testing.Verbose() {
		// The API client logs requests only in trace level.
		level = log.TraceLevel
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	config.Clock = cmp.Or(config.Clock, clockwork.NewRealClock())

	api := intunefake.New(intunefake.Config{
		Clock:  config.Clock,
		Logger: logger,
	})
	api.SetApps(DefaultApps)

	server := httptest.NewTLSServer(api.Handler())
	httpClient := server.Client()
	var d net.Dialer
	httpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		// Ignore the address and always direct all requests to the fake API server.
		// High-level level tests which run intuneInstanceFactory don't have control over intune.Client
		// initialization, but they can pass a custom http.Client. This allows them to connect to the
		// fake API server despite the intune.Client trying to reach the official endpoints.
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", server.Listener.Addr().String())
		},
	}

	e := &Env{
		Logger:     logger,
		Clock:      config.Clock,
		API:        api,
		HTTPClient: httpClient,
		server:     server,
	}

	if config.DeviceTrustEnv {
		var err error
		e.deviceEnv, err = dtenv.New()
		require.NoError(t, err)
		e.DevicesClient = e.deviceEnv.DevicesClient
	}

	t.Cleanup(func() {
		require.NoError(t, e.Close())
	})

	return e
}

// Close cleans up the TLS server once [Env] is no longer needed.
func (e *Env) Close() error {
	if e.HTTPClient != nil {
		e.HTTPClient.CloseIdleConnections()
	}
	if e.server != nil {
		e.server.Close()
	}

	if e.deviceEnv != nil {
		return e.deviceEnv.Close()
	}
	return nil
}

// MustNewClient returns a new Intune client that connects to the fake API served by [Env].
func (e *Env) MustNewClient(t *testing.T) *api.Client {
	client, err := api.NewClient(t.Context(), api.ClientConfig{
		APIConfig: api.Config{
			AppCredentials: api.AppCredentials{
				ClientID:     DefaultApps[0].ClientID,
				ClientSecret: DefaultApps[0].ClientSecret,
				Tenant:       DefaultApps[0].Tenant,
			},
		},
		Clock:      e.Clock,
		Logger:     e.Logger,
		HTTPClient: e.HTTPClient,
	})
	require.NoError(t, err)
	return client
}

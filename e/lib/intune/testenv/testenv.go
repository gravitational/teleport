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

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtenv "github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/e/lib/intune/api"
	intunefake "github.com/gravitational/teleport/e/lib/intune/fake"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
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
	// DeviceTrustEnv makes [Env] prepare a test environment for Device Trust as well.
	DeviceTrustEnv bool
	// InMemory serves the fake Intune API over an in-memory listener instead of a
	// real TCP socket. This keeps all I/O inside the process so the environment
	// can run inside a [testing/synctest] bubble.
	InMemory bool
}

// MustNew sets up a new TLS server with the fake Intune API.
// Automatically cleans up [Env] when the test finishes.
func MustNew(t *testing.T, config *Config) *Env {
	t.Helper()

	level := slog.LevelError + 1 // Silence logging by default.
	if testing.Verbose() {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	config.Clock = cmp.Or(config.Clock, clockwork.NewRealClock())

	api := intunefake.New(intunefake.Config{
		Clock:  config.Clock,
		Logger: logger.With(teleport.ComponentKey, "fakeintune"),
	})
	api.SetApps(DefaultApps)

	// Ignore the address and always direct all requests to the fake API server.
	// High-level level tests which run intuneInstanceFactory don't have control over intune.Client
	// initialization, but they can pass a custom http.Client. This allows them to connect to the
	// fake API server despite the intune.Client trying to reach the official endpoints.
	var server *httptest.Server
	var dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	if config.InMemory {
		lis := listenerutils.NewInMemoryListener()
		server = httptest.NewUnstartedServer(api.Handler())
		server.Listener = lis
		server.StartTLS()
		dialContext = lis.DialContext
	} else {
		server = httptest.NewTLSServer(api.Handler())
		var d net.Dialer
		dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", server.Listener.Addr().String())
		}
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: dialContext,
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

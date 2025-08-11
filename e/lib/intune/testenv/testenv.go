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
	"testing" //nolint:depguard // This is a test package.

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require" //nolint:depguard // This is a test package.

	"github.com/gravitational/teleport/e/lib/intune/api"
	intunefake "github.com/gravitational/teleport/e/lib/intune/fake"
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
}

// Config provides values needed by [Env].
type Config struct {
	Clock clockwork.Clock
}

// MustNew is like [New] but it fails the test on error.
// Automatically cleans up [Env] when the test finishes.
func MustNew(t *testing.T, config *Config) *Env {
	t.Helper()
	env, err := New(config)
	require.NoError(t, err)
	t.Cleanup(env.Close)
	return env
}

// New sets up a new TLS server with the fake Intune API.
// The caller is expected to call [Env.Close] once [Env] is no longer needed.
func New(config *Config) (*Env, error) {
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

	return &Env{
		Logger:     logger,
		Clock:      config.Clock,
		API:        api,
		HTTPClient: httpClient,
		server:     server,
	}, nil
}

// Close cleans up the TLS server once [Env] is no longer needed.
func (e *Env) Close() {
	if e.HTTPClient != nil {
		e.HTTPClient.CloseIdleConnections()
	}
	if e.server != nil {
		e.server.Close()
	}
}

// MustNewClient is like [NewClient] but it fails the test on error.
func (e *Env) MustNewClient(t *testing.T) *api.Client {
	t.Helper()
	client, err := e.NewClient(t.Context())
	require.NoError(t, err)
	return client
}

// NewClient returns a new Intune client that connects to the fake API served by [Env].
func (e *Env) NewClient(ctx context.Context) (*api.Client, error) {
	client, err := api.NewClient(ctx, api.ClientConfig{
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
	if err != nil {
		return nil, err
	}
	return client, err
}

package testenv

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jonboulle/clockwork"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtenv "github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/log"
)

// DefaultUsers are the users added by default to the fake Jamf API.
var DefaultUsers = []*jamffake.User{
	{Username: "admin", Password: "pa$$word"},
	{Username: "llama", Password: "secret!!1!"},
}

// E is an integrated test environment for Jamf.
type E struct {
	Clock  clockwork.Clock
	Logger *slog.Logger

	// APIEndpoint for the fake Jamf API.
	// Example: "https://localhost:12345/api".
	APIEndpoint string
	API         *jamffake.API

	// Client is a [jamf.Client] with the correct TLS settings to access
	// the Jamf API.
	Client *jamf.Client

	/// HTTPClient is a an [http.Client] with the correct TLS settings to access
	// the Jamf API.
	HTTPClient *http.Client

	DevicesClient devicepb.DeviceTrustServiceClient

	deviceEnv *dtenv.E
	server    *httptest.Server
}

// Close tears down the test environment.
func (e *E) Close() error {
	if e.HTTPClient != nil {
		e.HTTPClient.CloseIdleConnections()
	}
	// e.server owns e.lis, if it exists.
	if e.server != nil {
		e.server.Close()
	}
	if e.deviceEnv != nil {
		e.deviceEnv.Close()
	}
	return nil
}

// Opts are the creation options for [E].
type Opts struct {
	Clock clockwork.Clock

	// DeviceTrustEnv enables configuration of its namesake testenv.
	DeviceTrustEnv bool
	DeviceOpts     []dtenv.Opt
}

// MustNew creates a new [E] or panics.
// Prefer [NewUsingT] when enabling the Device Trust env, as it configures
// [modules.TestModules] automatically.
func MustNew(opts *Opts) *E {
	env, err := New(opts)
	if err != nil {
		panic(err)
	}
	return env
}

// NewUsingT creates a new [E], automatically fails on errors and automatically
// registers [E.Close] on cleanup.
// If opts.DeviceTrustEnv is set, [NewUsingT] sets the build type to Enterprise.
func NewUsingT(t *testing.T, opts *Opts) *E {
	// Configure device trust settings?
	if opts != nil && opts.DeviceTrustEnv {
		// Set build type and features.
		modules.SetTestModules(t, &modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DeviceTrust:            {Enabled: true},
					entitlements.MobileDeviceManagement: {Enabled: true},
				},
			},
		})
	}

	env, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Jamf testenv.E: %v", err)
	}
	t.Cleanup(func() { _ = env.Close() })
	return env
}

// New creates a new [E].
// All required servers are already started on success. The fake Jamf API is
// configured with [DefaultUsers] by default.
func New(opts *Opts) (*E, error) {
	if opts == nil {
		opts = &Opts{}
	}

	e := &E{}

	// Use a consistent clock for everything.
	e.Clock = opts.Clock
	if e.Clock == nil {
		e.Clock = clockwork.NewRealClock()
	}

	level := slog.LevelError + 1 // Silence logging by default.
	if testing.Verbose() {
		// The API client logs requests only in trace level.
		level = log.TraceLevel
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	e.Logger = logger

	// TODO(codingllama): Pass clock down to deviceEnv?
	if opts.DeviceTrustEnv {
		var err error
		e.deviceEnv, err = dtenv.New(opts.DeviceOpts...)
		if err != nil {
			return nil, err
		}
		e.DevicesClient = e.deviceEnv.DevicesClient
	}

	ok := false
	defer func() {
		if !ok {
			_ = e.Close()
		}
	}()

	e.API = jamffake.New(&jamffake.Opts{
		Clock: e.Clock,
	})
	e.API.SetUsers(DefaultUsers)

	const prefix = "/api"
	e.server = httptest.NewTLSServer(e.API.Handler(prefix))
	e.APIEndpoint = fmt.Sprintf("%v%v", e.server.URL, prefix)
	e.HTTPClient = e.server.Client()

	var err error
	e.Client, err = e.NewClient()
	if err != nil {
		return nil, fmt.Errorf("jamf client: %w", err)
	}

	ok = true
	return e, nil
}

// MustNewClient creates a new [jamf.Client] or panics.
func (e *E) MustNewClient() *jamf.Client {
	client, err := e.NewClient()
	if err != nil {
		panic(err)
	}
	return client
}

// NewClient creates a new [jamf.Client] ready to connect to the API, using
// credentials from [DefaultUsers].
func (e *E) NewClient() (*jamf.Client, error) {
	client, err := jamf.NewClient(context.Background(), jamf.ClientOpts{
		Clock:      e.Clock,
		Logger:     e.Logger,
		HTTPClient: e.HTTPClient,
		APIURL:     e.APIEndpoint,
		Username:   DefaultUsers[1].Username,
		Password:   DefaultUsers[1].Password,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

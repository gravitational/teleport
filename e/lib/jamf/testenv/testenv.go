package testenv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtenv "github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	"github.com/gravitational/teleport/lib/modules"
)

// DefaultUsers are the users added by default to the fake Jamf API.
var DefaultUsers = []*jamffake.User{
	{Username: "admin", Password: "pa$$word"},
	{Username: "llama", Password: "secret!!1!"},
}

// E is an integrated test environment for Jamf.
type E struct {
	Clock  clockwork.Clock
	Logger log.FieldLogger

	// APIEndpoint for the fake Jamf API.
	// Example: "http://localhost:12345/api".
	APIEndpoint string

	API        *jamffake.API
	Client     *jamf.Client
	HTTPClient *http.Client

	DevicesClient devicepb.DeviceTrustServiceClient

	deviceEnv *dtenv.E
	lis       net.Listener
	server    *http.Server
}

// Close tears down the test environment.
func (e *E) Close() error {
	// e.server owns e.lis, if it exists.
	if e.server != nil {
		_ = e.server.Shutdown(context.Background())
	} else if e.lis != nil {
		_ = e.lis.Close()
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
		// Set build type.
		modules.SetTestModules(t, &modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
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

	logger := log.New()
	logger.SetLevel(log.PanicLevel) // Mostly silent
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

	var err error
	e.lis, err = net.Listen("tcp", "localhost:")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	e.API = jamffake.New(&jamffake.Opts{
		Clock: e.Clock,
	})
	e.API.SetUsers(DefaultUsers)

	const prefix = "/api"
	e.server = &http.Server{
		Handler: e.API.Handler(prefix),
	}
	e.APIEndpoint = fmt.Sprintf("http://%v%v", e.lis.Addr().String(), prefix)
	go func() {
		if err := e.server.Serve(e.lis); !errors.Is(err, http.ErrServerClosed) {
			// TODO(codingllama): Be more subtle?
			panic(fmt.Sprintf("Serve returned unexpected error: %v", err))
		}
	}()

	e.HTTPClient = &http.Client{
		Timeout: 10 * time.Second, // This should be long enough for testing.
	}
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
		Clock:          e.Clock,
		Logger:         e.Logger,
		HTTPClient:     e.HTTPClient,
		APIURL:         e.APIEndpoint,
		Username:       DefaultUsers[1].Username,
		Password:       DefaultUsers[1].Password,
		AllowPlainHTTP: true,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

package intune

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Client represents a client for the Intune API. It authorizes with the API through OAuth (see
// [AppCredentials]). It automatically exchanges credentials for a fresh access token when needed.
type Client struct {
	config ClientConfig
}

// ClientConfig is the config used by [Client].
type ClientConfig struct {
	APIConfig  APIConfig
	Logger     *slog.Logger
	Clock      clockwork.Clock
	HTTPClient *http.Client
}

// APIConfig are parameters required by the Intune API itself.
type APIConfig struct {
	// AppCredentials are credentials used to authenticate with the API.
	AppCredentials AppCredentials
	// LoginEndpoint is the address used to access Microsoft identity platform in order to exchange
	// app credentials for an access token. Optional, defaults to "https://login.microsoftonline.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	LoginEndpoint string
	// GraphEndpoint is the address used to access Microsoft Graph with the access token obtained
	// from LoginEndpoint. Optional, defaults to "https://graph.microsoft.com".
	//
	// https://learn.microsoft.com/en-us/graph/deployments
	GraphEndpoint string
}

const (
	// DefaultLoginEndpoint is the endpoint under which Microsoft identity platform APIs are available.
	DefaultLoginEndpoint = "https://login.microsoftonline.com"
	// DefaultGraphEndpoint is the endpoint under which Microsoft Graph is available.
	DefaultGraphEndpoint = "https://graph.microsoft.com"
)

func NewClient(ctx context.Context, config ClientConfig) (*Client, error) {
	if err := ValidateAppCredentials(config.APIConfig.AppCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	if config.Logger == nil {
		return nil, trace.BadParameter("param Logger required")
	}

	config.Clock = cmp.Or(config.Clock, clockwork.NewRealClock())
	config.APIConfig.LoginEndpoint = cmp.Or(config.APIConfig.LoginEndpoint, DefaultLoginEndpoint)
	config.APIConfig.GraphEndpoint = cmp.Or(config.APIConfig.GraphEndpoint, DefaultGraphEndpoint)

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 1 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	c := &Client{
		config: config,
	}
	return c, nil
}

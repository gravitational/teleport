package oktaapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/patrickmn/go-cache"

	"github.com/gravitational/teleport/lib/defaults"
)

// APIClientProvider allows setting mocked Okta API client for testing.
var APIClientProvider = &apiClientProvider{}

// apiClientProvider provides Set() and get() method allowing setting a mocked [APIClient] for
// testing.
type apiClientProvider struct {
	mux            sync.Mutex
	newApiClientFn newAPIClientFunc
}

type newAPIClientFunc func(ctx context.Context, cfg ...okta.ConfigSetter) (APIClient, error)

// Set sets the Okta API client provider for testing.
func (p *apiClientProvider) Set(fn newAPIClientFunc) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.newApiClientFn = fn
}

// get is used when creating a default [Client] with [New].
func (p *apiClientProvider) get() newAPIClientFunc {
	p.mux.Lock()
	defer p.mux.Unlock()
	if p.newApiClientFn != nil {
		return p.newApiClientFn
	}
	return p.defaultNewClient
}

// defaultNewClient is the default Okta API client creator func used in production.
func (*apiClientProvider) defaultNewClient(ctx context.Context, cfg ...okta.ConfigSetter) (APIClient, error) {
	_, client, err := okta.NewClient(ctx, cfg...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := fetchAndSetClientScopes(client); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAPIClient(client), nil
}

// wrappedTransport  wraps the Okta client transport and captures the access token and extracts the
// scopes.
type wrappedTransport struct {
	http.RoundTripper
	accessToken okta.RequestAccessToken
}

// RoundTrip implements the http.RoundTripper interface.
func (t *wrappedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bodyBytes, &t.accessToken); err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return resp, nil
}

// fetchAndSetClientScopes fetches the access token and extracts the scopes configured on the Okta
// side for Okta credentials. If the Okta client is configured with the "PrivateKey" authorization
// mode, the scopes will be fetched from the access token and then set on the local Okta client.
// This ensures that the Okta client is configured with the correct scopes. If the Okta client
// attempts to use scopes that have not been granted, the Okta API will return an error at runtime
// when the client tries to access the API.
func fetchAndSetClientScopes(client *okta.Client) error {
	if client.GetConfig().Okta.Client.AuthorizationMode != "PrivateKey" {
		client.GetConfig().Okta.Client.Scopes = oktaAPIScopes
		return nil
	}
	// Instead of always wrapping DefaultTransport, use the clients' if set.
	rt := client.GetConfig().HttpClient.Transport
	if rt == nil {
		var err error
		rt, err = defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	tr := &wrappedTransport{
		RoundTripper: rt,
	}
	auth := okta.NewPrivateKeyAuth(okta.PrivateKeyAuthConfig{
		Req: &http.Request{
			Header: make(http.Header),
		},
		HttpClient: &http.Client{
			Transport: tr,
		},
		TokenCache:       cache.New(5*time.Minute, 10*time.Minute),
		PrivateKeySigner: client.GetConfig().PrivateKeySigner,
		ClientId:         client.GetConfig().Okta.Client.ClientId,
		OrgURL:           client.GetConfig().Okta.Client.OrgUrl,
		MaxRetries:       client.GetConfig().Okta.Client.RateLimit.MaxRetries,
		MaxBackoff:       client.GetConfig().Okta.Client.RateLimit.MaxBackoff,
		Scopes:           client.GetConfig().Okta.Client.Scopes,
	})
	if err := auth.Authorize(); err != nil {
		return trace.Wrap(err, "failed to authorize with scopes %v", client.GetConfig().Okta.Client.Scopes)
	}
	client.GetConfig().Okta.Client.Scopes = strings.Split(tr.accessToken.Scope, " ")
	return nil
}

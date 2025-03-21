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

var clientProviderMtx sync.Mutex

// SetClientProvider sets the Okta client provider for testing.
func SetClientProvider(fn clientProviderFunc) {
	clientProviderMtx.Lock()
	defer clientProviderMtx.Unlock()
	clientProvider = fn
}

func getClientProvider() clientProviderFunc {
	clientProviderMtx.Lock()
	defer clientProviderMtx.Unlock()
	return clientProvider
}

type clientProviderFunc func(ctx context.Context, cfg ...okta.ConfigSetter) (APIClient, error)

// clientProvider is an Okta client interface that can be mocked for testing.
var clientProvider = func(ctx context.Context, cfg ...okta.ConfigSetter) (APIClient, error) {
	_, client, err := okta.NewClient(ctx, cfg...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// fetchAndSetClientScopes fetches the access token and extracts the scopes configured on the Okta side for
	// Okta credentials.
	// If the Okta client is configured with the "PrivateKey" authorization mode, the scopes
	// will be fetched from the access token and then set on the local Okta client.
	// This ensures that the Okta client is configured with the correct scopes.
	// If the Okta client attempts to use scopes that have not been granted, the Okta API will return an error at runtime when the
	// client tries to access the API.
	if err := fetchAndSetClientScopes(client); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAPIClient(client, client.GetConfig().Okta.Client.Scopes...), nil
}

// wrappedTransprot  wraps the Okta client transport and
// captures the access token and extracts the scopes.
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

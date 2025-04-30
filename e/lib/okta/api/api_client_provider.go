package oktaapi

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
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
	return NewAPIClient(client), nil
}

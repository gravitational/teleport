package msgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// baseURL is the default value for [client.baseURL]. It is the address of MS Graph API v1.0.
const baseURL = "https://graph.microsoft.com/v1.0"

// defaultPageSize is the page size used when [Config.PageSize] is not specified.
const defaultPageSize = 500

// scopes defines OAuth scopes the client authenticates for.
var scopes = []string{"https://graph.microsoft.com/.default"}

type azureTokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// Config defines configuration options for [client].
type Config struct {
	// TokenProvider provides tokens to authorize to MS Graph API.
	// Concrete implementations of this are defined by [github.com/Azure/azure-sdk-for-go/sdk/azidentity].
	TokenProvider azureTokenProvider
	// HTTPClient is the HTTP client to use for calls to the API.
	// If not specified, [http.DefaultClient] is used.
	HTTPClient *http.Client
	// Clock is the clock to use for time operations (e.g. delay when retrying requests).
	Clock clockwork.Clock
	// RetryConfig specifies parameters for retrying failed requests.
	// Client will prefer to use the `Retry-After` header returned from the API,
	// and only use this retry config if the header is not provided.
	RetryConfig *retryutils.RetryV2Config
	// PageSize limits the number of objects to return in one batch when using paginated requests (via the `$top` parameter).
	PageSize int
}

// SetDefaults sets the default values for optional fields.
func (cfg *Config) SetDefaults() {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.RetryConfig == nil {
		cfg.RetryConfig = &retryutils.RetryV2Config{
			First:  1 * time.Second,
			Driver: retryutils.NewExponentialDriver(1 * time.Second),
			Max:    defaults.HighResPollingPeriod,
		}
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}
}

// Validate checks that required fields are set.
func (cfg *Config) Validate() error {
	if cfg.TokenProvider == nil {
		return trace.BadParameter("TokenProvider must be set")
	}
	return nil
}

type client struct {
	httpClient    *http.Client
	tokenProvider azureTokenProvider
	clock         clockwork.Clock
	retryConfig   retryutils.RetryV2Config
	baseURL       *url.URL
	pageSize      int
}

// NewClient returns a new client for the given config.
func NewClient(cfg Config) (Client, error) {
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	uri, err := url.Parse(baseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &client{
		httpClient:    cfg.HTTPClient,
		tokenProvider: cfg.TokenProvider,
		clock:         cfg.Clock,
		retryConfig:   *cfg.RetryConfig,
		baseURL:       uri,
		pageSize:      cfg.PageSize,
	}, nil
}

// request is the base function for HTTP API calls.
// It implements retry handling in case of API throttling, see [https://learn.microsoft.com/en-us/graph/throttling].
func (c *client) request(ctx context.Context, method string, uri string, payload []byte) (*http.Response, error) {
	var body io.ReadSeeker = nil
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	token, err := c.tokenProvider.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: scopes,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to get azure authentication token")
	}
	req.Header.Add("Authorization", "Bearer "+token.Token)

	const maxRetries = 5
	var retryAfter time.Duration

	// RetryV2 only used when the API does not return a Retry-After header.
	retry, err := retryutils.NewRetryV2(c.retryConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i != 0 {
			c.clock.Sleep(retryAfter)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, trace.Wrap(err) // hard I/O error, bail
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return resp, nil
		}

		graphError, err := readError(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err // error while reading the graph error, relay
		} else if graphError != nil {
			lastErr = trace.Wrap(graphError)
		} else {
			// API did not return a valid error structure, best-effort reporting.
			lastErr = trace.Errorf(resp.Status)
		}
		if !isRetriable(resp.StatusCode) {
			break
		}

		retryAfter = retry.Duration()
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if seconds, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(seconds) * time.Second
			}
		}
		retry.Inc()

		// prepare for the next request attempt by rewinding the body
		if body != nil {
			_, err := body.Seek(0, io.SeekStart)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return nil, trace.Wrap(lastErr, "%s %s", req.Method, req.URL.Path)
}

func (c *client) endpointURI(segments ...string) *url.URL {
	uri := *c.baseURL
	uri.Path = path.Join(append([]string{uri.Path}, segments...)...)
	return &uri
}

// roundtrip makes a request to the API,
// serializing `in` as a JSON body, and deserializing the response as the given type `T`.
// It is used for GET and POST requests, where a response body is expected.
func roundtrip[T any](ctx context.Context, c *client, method string, uri string, in any) (T, error) {
	var zero T
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return zero, trace.Wrap(err)
		}
	}
	resp, err := c.request(ctx, method, uri, body)
	if err != nil {
		return zero, trace.Wrap(err)
	}
	defer resp.Body.Close()

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, trace.Wrap(err)
	}
	return out, nil
}

// patch makes a PATCH request to the API, serializing `in` as a JSON body.
// It expects a 204 No Content response.
func (c *client) patch(ctx context.Context, uri string, in any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := c.request(ctx, http.MethodPatch, uri, body)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return trace.BadParameter("expected a 204 No Content response, got status code %v", resp.StatusCode)
	}
	return nil
}

// CreateFederatedIdentityCredential creates a new FederatedCredential.
// Ref: [https://learn.microsoft.com/en-us/graph/api/application-post-federatedidentitycredentials].
func (c *client) CreateFederatedIdentityCredential(ctx context.Context, appObjectID string, cred *FederatedIdentityCredential) (*FederatedIdentityCredential, error) {
	uri := c.endpointURI("applications", appObjectID, "federatedIdentityCredentials")
	out, err := roundtrip[*FederatedIdentityCredential](ctx, c, http.MethodPost, uri.String(), cred)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// CreateServicePrincipalTokenSigningCertificate generates a new token signing certificate for the given service principal.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-addtokensigningcertificate].
func (c *client) CreateServicePrincipalTokenSigningCertificate(ctx context.Context, spID string, displayName string) (*SelfSignedCertificate, error) {
	uri := c.endpointURI("servicePrincipals", spID, "addTokenSigningCertificate")
	in := map[string]string{"displayName": displayName}
	out, err := roundtrip[*SelfSignedCertificate](ctx, c, http.MethodPost, uri.String(), in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GetServicePrincipalByAppId returns the service principal associated with the given application.
// Note that appID here is the app the application "client ID" ([Application.AppID]), not "object ID" ([Application.ID]).
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-get].
func (c *client) GetServicePrincipalByAppId(ctx context.Context, appID string) (*ServicePrincipal, error) {
	uri := c.endpointURI(fmt.Sprintf("servicePrincipals(appId='%s')", appID))
	out, err := roundtrip[*ServicePrincipal](ctx, c, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GetServicePrincipalsByDisplayName returns the service principals that have the given display name.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list].
func (c *client) GetServicePrincipalsByDisplayName(ctx context.Context, displayName string) ([]*ServicePrincipal, error) {
	filter := fmt.Sprintf("displayName eq '%s'", displayName)
	uri := c.endpointURI("servicePrincipals")
	uri.RawQuery = url.Values{
		"$filter": {filter},
	}.Encode()
	out, err := roundtrip[oDataListResponse[*ServicePrincipal]](ctx, c, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out.Value, nil
}

// InstantiateApplicationTemplate instantiates an application from the Entra application Gallery,
// creating a pair of [Application] and [ServicePrincipal].
// Ref: [https://learn.microsoft.com/en-us/graph/api/applicationtemplate-instantiate].
func (c *client) InstantiateApplicationTemplate(ctx context.Context, appTemplateID string, displayName string) (*ApplicationServicePrincipal, error) {
	uri := c.endpointURI("applicationTemplates", appTemplateID, "instantiate")
	in := map[string]string{
		"displayName": displayName,
	}
	out, err := roundtrip[*ApplicationServicePrincipal](ctx, c, http.MethodPost, uri.String(), in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// UpdateApplication issues a partial update for an [Application].
// Note that appID here is the app the application  "object ID" ([Application.ID]), not "client ID" ([Application.AppID]).
// Ref: [https://learn.microsoft.com/en-us/graph/api/application-update].
func (c *client) UpdateApplication(ctx context.Context, appObjectID string, app *Application) error {
	uri := c.endpointURI("applications", appObjectID)
	return trace.Wrap(c.patch(ctx, uri.String(), app))
}

// UpdateServicePrincipal issues a partial update for a [ServicePrincipal].
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-update].
func (c *client) UpdateServicePrincipal(ctx context.Context, spID string, sp *ServicePrincipal) error {
	uri := c.endpointURI("servicePrincipals", spID)
	return trace.Wrap(c.patch(ctx, uri.String(), sp))
}

// isRetriable returns `true` when the given HTTP status code should be retried.
func isRetriable(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

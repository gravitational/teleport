// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package msgraph

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
)

// baseURL is the default value for [client.baseURL]. It is the address of MS Graph API v1.0.
const baseURL = "https://graph.microsoft.com/v1.0"

// defaultPageSize is the page size used when [Config.PageSize] is not specified.
const defaultPageSize = 500

// scopes defines OAuth scopes the client authenticates for.
var scopes = []string{"https://graph.microsoft.com/.default"}

// AzureTokenProvider defines a method to get an authorization token from the Entra STS.
// Concrete implementations of this are defined by [github.com/Azure/azure-sdk-for-go/sdk/azidentity].
type AzureTokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

func defaultHTTPClient() (*http.Client, error) {
	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	transport.ExpectContinueTimeout = apidefaults.DefaultIOTimeout
	transport.ResponseHeaderTimeout = apidefaults.DefaultIOTimeout
	transport.IdleConnTimeout = apidefaults.DefaultIdleTimeout

	return &http.Client{
		Transport: transport,
		Timeout:   apidefaults.DefaultIOTimeout,
	}, nil
}

// Config defines configuration options for [client].
type Config struct {
	// TokenProvider provides tokens to authorize to MS Graph API.
	TokenProvider AzureTokenProvider
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
	defaultHTTPClient, _ := defaultHTTPClient()

	cfg.HTTPClient = cmp.Or(cfg.HTTPClient, defaultHTTPClient)
	cfg.Clock = cmp.Or(cfg.Clock, clockwork.NewRealClock())
	cfg.RetryConfig = cmp.Or(cfg.RetryConfig, &retryutils.RetryV2Config{
		First:  1 * time.Second,
		Driver: retryutils.NewExponentialDriver(1 * time.Second),
		Max:    defaults.HighResPollingPeriod,
	})
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}
}

// Validate checks that required fields are set.
func (cfg *Config) Validate() error {
	if cfg.TokenProvider == nil {
		return trace.BadParameter("TokenProvider must be set")
	}
	if cfg.HTTPClient == nil {
		return trace.BadParameter("HTTPClient must be set")
	}
	return nil
}

type Client struct {
	httpClient    *http.Client
	tokenProvider AzureTokenProvider
	clock         clockwork.Clock
	retryConfig   retryutils.RetryV2Config
	baseURL       *url.URL
	pageSize      int
}

// NewClient returns a new client for the given config.
func NewClient(cfg Config) (*Client, error) {
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	uri, err := url.Parse(baseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{
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
func (c *Client) request(ctx context.Context, method string, uri string, payload []byte) (*http.Response, error) {
	var body io.ReadSeeker = nil
	if len(payload) > 0 {
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
		if retryAfter > 0 {
			select {
			case <-c.clock.After(retryAfter):
			case <-ctx.Done():
				return nil, trace.NewAggregate(ctx.Err(), trace.Wrap(lastErr, "%s %s", req.Method, req.URL.Path))
			}
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

func (c *Client) endpointURI(segments ...string) *url.URL {
	escapedSegments := make([]string, 0, cap(segments))
	for _, s := range segments {
		// Handling of slash vs escaped slash (%2F) in paths is ambiguous and inconsistent.
		// See e.g.: https://stackoverflow.com/questions/1957115/is-a-slash-equivalent-to-an-encoded-slash-2f-in-the-path-portion-of-a
		// We do not expect slashes to be needed within a single path segment,
		// so we just remove slashes from each segment.
		escapedSegments = append(escapedSegments, url.PathEscape(strings.ReplaceAll(s, "/", "")))
	}
	uri := c.baseURL
	uri = uri.JoinPath(escapedSegments...)
	return uri
}

// roundtrip makes a request to the API,
// serializing `in` as a JSON body, and deserializing the response as the given type `T`.
// It is used for GET and POST requests, where a response body is expected.
func roundtrip[T any](ctx context.Context, c *Client, method string, uri string, in any) (T, error) {
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
func (c *Client) patch(ctx context.Context, uri string, in any) error {
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
func (c *Client) CreateFederatedIdentityCredential(ctx context.Context, appObjectID string, cred *FederatedIdentityCredential) (*FederatedIdentityCredential, error) {
	uri := c.endpointURI("applications", appObjectID, "federatedIdentityCredentials")
	out, err := roundtrip[*FederatedIdentityCredential](ctx, c, http.MethodPost, uri.String(), cred)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// CreateServicePrincipalTokenSigningCertificate generates a new token signing certificate for the given service principal.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-addtokensigningcertificate].
func (c *Client) CreateServicePrincipalTokenSigningCertificate(ctx context.Context, spID string, displayName string) (*SelfSignedCertificate, error) {
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
func (c *Client) GetServicePrincipalByAppId(ctx context.Context, appID string) (*ServicePrincipal, error) {
	uri := c.endpointURI(fmt.Sprintf("servicePrincipals(appId='%s')", appID))
	out, err := roundtrip[*ServicePrincipal](ctx, c, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GetServicePrincipalsByDisplayName returns the service principals that have the given display name.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list].
func (c *Client) GetServicePrincipalsByDisplayName(ctx context.Context, displayName string) ([]*ServicePrincipal, error) {
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

// GetServicePrincipal returns the service principal for the given principal ID.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-get].
func (c *Client) GetServicePrincipal(ctx context.Context, principalId string) (*ServicePrincipal, error) {
	uri := c.endpointURI(fmt.Sprintf("servicePrincipals/%s", principalId))
	out, err := roundtrip[*ServicePrincipal](ctx, c, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GrantAppRoleToServicePrincipal grants the given app role to the specified Service Principal.
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-post-approleassignedto]
func (c *Client) GrantAppRoleToServicePrincipal(ctx context.Context, spID string, assignment *AppRoleAssignment) (*AppRoleAssignment, error) {
	uri := c.endpointURI("servicePrincipals", spID, "appRoleAssignedTo")
	out, err := roundtrip[*AppRoleAssignment](ctx, c, http.MethodPost, uri.String(), assignment)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// InstantiateApplicationTemplate instantiates an application from the Entra application Gallery,
// creating a pair of [Application] and [ServicePrincipal].
// Ref: [https://learn.microsoft.com/en-us/graph/api/applicationtemplate-instantiate].
func (c *Client) InstantiateApplicationTemplate(ctx context.Context, appTemplateID string, displayName string) (*ApplicationServicePrincipal, error) {
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
func (c *Client) UpdateApplication(ctx context.Context, appObjectID string, app *Application) error {
	uri := c.endpointURI("applications", appObjectID)
	return trace.Wrap(c.patch(ctx, uri.String(), app))
}

// GetApplication returns the application with the given app client ID.
// Note that appID here is the app the application "client ID" ([Application.AppID]) not  "object ID" ([Application.ID]).
// Ref: [https://learn.microsoft.com/en-us/graph/api/application-get].
func (c *Client) GetApplication(ctx context.Context, applicationID string) (*Application, error) {
	applicationIDFilter := fmt.Sprintf("applications(appId='%s')", applicationID)
	uri := c.endpointURI(applicationIDFilter)
	out, err := roundtrip[*Application](ctx, c, http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// UpdateServicePrincipal issues a partial update for a [ServicePrincipal].
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-update].
func (c *Client) UpdateServicePrincipal(ctx context.Context, spID string, sp *ServicePrincipal) error {
	uri := c.endpointURI("servicePrincipals", spID)
	return trace.Wrap(c.patch(ctx, uri.String(), sp))
}

// isRetriable returns `true` when the given HTTP status code should be retried.
func isRetriable(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

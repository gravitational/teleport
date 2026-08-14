package common

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/web"
	websession "github.com/gravitational/teleport/lib/web/session"
	ossui "github.com/gravitational/teleport/lib/web/ui"
)

// doer is an interface for making HTTP requests, implemented by helpers.WebClientPack.
type doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Roundtrip makes an HTTP request and decodes the JSON response into type R.
// Returns an error if the request fails or the response status is not 200 OK.
// This is a convenience wrapper around RoundtripWithResponse that discards the http.Response.
func Roundtrip[R any](ctx context.Context, rt doer, method string, endpoint string, in any) (R, error) {
	var out R
	resp, err := RoundtripWithResponse(ctx, rt, method, endpoint, in)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buff, err := io.ReadAll(resp.Body)
		if err != nil {
			return out, trace.Wrap(err)
		}
		return out, trace.ReadError(resp.StatusCode, buff)
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, trace.Wrap(err)
	}
	return out, err
}

// RoundtripWithResponse makes an HTTP request and returns the response
func RoundtripWithResponse(ctx context.Context, rt doer, method string, endpoint string, in any) (*http.Response, error) {
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if in != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := rt.Do(httpReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// unifiedResourceItem is a superset struct that can deserialize both App and Server
// items from the unified resources API response. Fields that don't apply to a
// given resource kind will be zero-valued.
type unifiedResourceItem struct {
	// Kind is the resource kind (e.g. "node", "app").
	Kind string `json:"kind"`
	// Name is the resource name.
	Name string `json:"name"`
	// RequiresRequest indicates the resource is only accessible via an access request.
	RequiresRequest bool `json:"requiresRequest,omitempty"`
	// AWSRoles is populated for AWS Console app resources.
	AWSRoles []aws.Role `json:"awsRoles,omitempty"`
	// SSHLogins is populated for SSH node resources.
	SSHLogins []string `json:"sshLogins,omitempty"`
	// SSHLoginDetails provides per-login metadata for SSH node resources.
	SSHLoginDetails []ossui.SSHLogin `json:"sshLoginDetails,omitempty"`
	// SupportedFeatureIDs contains ComponentFeatureIDs supported end-to-end for this resource.
	SupportedFeatureIDs []int `json:"supportedFeatureIds,omitempty"`
}

// UnifiedResourcesResponse represents the response from the unified resources API.
type UnifiedResourcesResponse struct {
	Items []unifiedResourceItem `json:"items"`
}

// listUnifedResourceOptions holds options for listing unified resources.
type listUnifedResourceOptions struct {
	searchAsRoles      bool
	includeRequestable bool
}

// ListUnifedResourcesOption is a functional option for configuring unified resource listing.
type ListUnifedResourcesOption func(*listUnifedResourceOptions)

// WithSearchAsRole enables searching for resources using the user's roles.
// This allows discovering resources available through role requests.
func WithSearchAsRole() ListUnifedResourcesOption {
	return func(o *listUnifedResourceOptions) {
		o.searchAsRoles = true
	}
}

// WithIncludeRequestable sets the included resource mode to 'all'.
// This includes requestable as well as granted resources in the response.
func WithIncludeRequestable() ListUnifedResourcesOption {
	return func(o *listUnifedResourceOptions) {
		o.includeRequestable = true
	}
}

// MustListUnifedResources searches for resources using the unified resources API.
// It fails the test if the request fails.
func MustListUnifedResources(t *testing.T, client *helpers.WebClientPack, opts ...ListUnifedResourcesOption) *UnifiedResourcesResponse {
	t.Helper()
	op := &listUnifedResourceOptions{}
	for _, opt := range opts {
		opt(op)
	}
	endpoint := client.Endpoint("webapi", "sites", "$site", "resources")
	u, err := url.Parse(endpoint)
	require.NoError(t, err)

	q := u.Query()
	if op.searchAsRoles {
		q.Add("searchAsRoles", "yes")
	}
	if op.includeRequestable {
		q.Add("includedResourceMode", web.IncludedResourceModeAll)
	}
	u.RawQuery = q.Encode()

	result, err := Roundtrip[UnifiedResourcesResponse](t.Context(), client, http.MethodGet, u.String(), nil)
	require.NoError(t, err)
	return &result
}

// CreateAccessRequest creates a new access request using the enterprise API.
func CreateAccessRequest(ctx context.Context, client *helpers.WebClientPack, params ui.AccessRequestParameters) (*ui.AccessRequest, error) {
	endpoint := client.Endpoint("enterprise", "accessrequest")
	result, err := Roundtrip[ui.AccessRequest](ctx, client, http.MethodPost, endpoint, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

// MustApproveAccessRequest approves an access request and fails the test on error.
// Returns the updated access request with APPROVED state.
func MustApproveAccessRequest(t *testing.T, client *helpers.WebClientPack, accessRequestID string) ui.AccessRequest {
	t.Helper()
	req := ui.AccessRequestParameters{
		ID:    accessRequestID,
		State: "APPROVED",
	}
	endpoint := client.Endpoint("enterprise/accessrequest")
	resp, err := Roundtrip[ui.AccessRequest](t.Context(), client, http.MethodPut, endpoint, req)
	require.NoError(t, err)

	require.Equal(t, "APPROVED", resp.State)
	return resp
}

// GetSuggestedAccessLists fetches the Access Lists suggested as promotion targets
// for the given access request, from the perspective of the calling (reviewer) user.
func GetSuggestedAccessLists(ctx context.Context, client *helpers.WebClientPack, accessRequestID string) (ui.SuggestedAccessLists, error) {
	endpoint := client.Endpoint("enterprise", "accessrequest", accessRequestID, "suggestions", "accesslist")
	return Roundtrip[ui.SuggestedAccessLists](ctx, client, http.MethodGet, endpoint, nil)
}

type renewSessionRequest struct {
	AccessRequestID string `json:"requestId"`
}

// AssumeAccessRequestWebClient renews a web session with an approved access request,
// creating a new client with updated bearer token and session cookie that includes
// that renew the session and allows to access rescues granted by access request.
func AssumeAccessRequestWebClient(ctx context.Context, client *helpers.WebClientPack, accessRequestID string) (*helpers.WebClientPack, error) {
	endpoint := client.Endpoint("webapi/sessions/web/renew")
	renewRequest := renewSessionRequest{
		AccessRequestID: accessRequestID,
	}

	resp, err := RoundtripWithResponse(ctx, client, http.MethodPost, endpoint, renewRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("expected 200 OK, got %v", resp.Status)
	}

	var webSession web.CreateSessionResponse
	if err = json.NewDecoder(resp.Body).Decode(&webSession); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(resp.Cookies()) == 0 {
		return nil, trace.BadParameter("no cookies returned from session renewal")
	}
	cookie := resp.Cookies()[0]
	if cookie.Name != websession.CookieName {
		return nil, trace.BadParameter("unexpected cookie name: got %q, expected %q", cookie.Name, websession.CookieName)
	}
	return client.WithNewCredentials(webSession.Token, cookie.Value), nil
}

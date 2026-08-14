package oktaapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/patrickmn/go-cache"

	"github.com/gravitational/teleport/lib/defaults"
)

// TODO(kopiczko) Consider removing this and requiring passing scopes every single time. Could be done in scope or after https://github.com/gravitational/teleport.e/pull/6410
var oktaAPIScopes = []string{
	ScopeUserRead,
	ScopeAppsManage,
	ScopeAppsRead,
	ScopeGroupsManage,
	ScopeGroupsRead,
}

const (
	// ScopeUserRead allows to read user information from Okta API.
	ScopeUserRead = "okta.users.read"
	// ScopeAppsManage allows to manage applications in Okta create (needed to auto-creation of SAML okta APP) or
	// manage Okta applications assignments.
	ScopeAppsManage = "okta.apps.manage"
	// ScopeAppsRead allows to read applications from Okta API.
	ScopeAppsRead = "okta.apps.read"
	// ScopeGroupsManage allows to manage groups in Okta
	ScopeGroupsManage = "okta.groups.manage"
	// ScopeGroupsRead allows to read groups from Okta API.
	ScopeGroupsRead = "okta.groups.read"
	// ScopeOrgsRead allows to read organization information from Okta API.
	ScopeOrgsRead = "okta.orgs.read"
	// ScopeOktaLogsRead allows to read Okta logs from Okta API.
	ScopeOktaLogsRead = "okta.logs.read"
	// ScopeOktaAPITokensRead allows to read Okta API tokens from Okta API.
	ScopeOktaAPITokensRead = "okta.apiTokens.read"
	// ScopeRolesRead allows to read Okta roles from Okta API.
	ScopeRolesRead = "okta.roles.read"
)

// getAuthorizedScopes fetches the access token and extracts the scopes configured on the Okta
// side for Okta credentials. If the Okta client is configured with the "PrivateKey" authorization
// mode, the scopes will be fetched from the access token and then set on the local Okta client.
// This ensures that the Okta client is configured with the correct scopes. If the Okta client
// attempts to use scopes that have not been granted, the Okta API will return an error at runtime
// when the client tries to access the API.
func getAuthorizedScopes(ctx context.Context, client *okta.Client) ([]string, error) {
	if client.GetConfig().Okta.Client.AuthorizationMode != "PrivateKey" {
		return oktaAPIScopes, nil
	}
	// Instead of always wrapping DefaultTransport, use the clients' if set.
	rt := client.GetConfig().HttpClient.Transport
	if rt == nil {
		var err error
		rt, err = defaults.Transport()
		if err != nil {
			return nil, trace.Wrap(err, "failed getting default HTTP transport")
		}
	}
	tr := &wrappedTransport{
		RoundTripper: rt,
	}
	auth := okta.NewPrivateKeyAuth(okta.PrivateKeyAuthConfig{
		Req: (&http.Request{
			Header: make(http.Header),
		}).WithContext(ctx),
		HttpClient: &http.Client{
			Transport: tr,
		},
		TokenCache:       cache.New(5*time.Minute, 0),
		PrivateKeySigner: client.GetConfig().PrivateKeySigner,
		ClientId:         client.GetConfig().Okta.Client.ClientId,
		OrgURL:           client.GetConfig().Okta.Client.OrgUrl,
		MaxRetries:       client.GetConfig().Okta.Client.RateLimit.MaxRetries,
		MaxBackoff:       client.GetConfig().Okta.Client.RateLimit.MaxBackoff,
		Scopes:           client.GetConfig().Okta.Client.Scopes,
	})
	if err := auth.Authorize(); err != nil {
		return nil, trace.Wrap(err, "authorizing Okta client")
	}
	return strings.Split(tr.accessToken.Scope, " "), nil
}

// wrappedTransport  wraps the Okta client transport and captures the access token, which then can
// be used to extracts the allowed scopes.
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

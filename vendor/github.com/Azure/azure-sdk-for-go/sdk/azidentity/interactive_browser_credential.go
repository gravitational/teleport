// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"math/rand"
	"net/url"
	"path"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/uuid"
	"github.com/pkg/browser"
)

// InteractiveBrowserCredentialOptions can be used when providing additional credential information, such as a client secret.
// Use these options to modify the default pipeline behavior if necessary.
// All zero-value fields will be initialized with their default values. Please note, that both the TenantID or ClientID fields should
// changed together if default values are not desired.
type InteractiveBrowserCredentialOptions struct {
	// The Azure Active Directory tenant (directory) ID of the service principal.
	// The default value is "organizations". If this value is changed, then also change ClientID to the corresponding value.
	TenantID string
	// The client (application) ID of the service principal.
	// The default value is the developer sign on ID for the corresponding "organizations" TenantID.
	ClientID string
	// The client secret that was generated for the App Registration used to authenticate the client. Only applies for web apps.
	ClientSecret string
	// The redirect URL used to request the authorization code. Must be the same URL that is configured for the App Registration.
	RedirectURL string
	// The localhost port for the local server that will be used to redirect back.
	// By default, a random port number will be selected.
	Port int
	// The host of the Azure Active Directory authority. The default is AzurePublicCloud.
	// Leave empty to allow overriding the value from the AZURE_AUTHORITY_HOST environment variable.
	AuthorityHost string
	// HTTPClient sets the transport for making HTTP requests
	// Leave this as nil to use the default HTTP transport
	HTTPClient policy.Transporter
	// Retry configures the built-in retry policy behavior
	Retry policy.RetryOptions
	// Telemetry configures the built-in telemetry policy behavior
	Telemetry policy.TelemetryOptions
	// Logging configures the built-in logging policy behavior.
	Logging policy.LogOptions
}

// init returns an instance of InteractiveBrowserCredentialOptions initialized with default values.
func (o *InteractiveBrowserCredentialOptions) init() {
	if o.TenantID == "" {
		o.TenantID = organizationsTenantID
	}
	if o.ClientID == "" {
		o.ClientID = developerSignOnClientID
	}
}

// InteractiveBrowserCredential enables authentication to Azure Active Directory using an interactive browser to log in.
type InteractiveBrowserCredential struct {
	client *aadIdentityClient
	// options contains data necessary to authenticate through an interactive browser window
	options InteractiveBrowserCredentialOptions
}

// NewInteractiveBrowserCredential constructs a new InteractiveBrowserCredential with the details needed to authenticate against Azure Active Directory through an interactive browser window.
// options: configure the management of the requests sent to Azure Active Directory, pass in nil or a zero-value options instance for default behavior.
func NewInteractiveBrowserCredential(options *InteractiveBrowserCredentialOptions) (*InteractiveBrowserCredential, error) {
	cp := InteractiveBrowserCredentialOptions{}
	if options != nil {
		cp = *options
	}
	cp.init()
	if !validTenantID(cp.TenantID) {
		return nil, &CredentialUnavailableError{credentialType: "Interactive Browser Credential", message: tenantIDValidationErr}
	}
	authorityHost, err := setAuthorityHost(cp.AuthorityHost)
	if err != nil {
		return nil, err
	}
	c, err := newAADIdentityClient(authorityHost, pipelineOptions{HTTPClient: cp.HTTPClient, Retry: cp.Retry, Telemetry: cp.Telemetry, Logging: cp.Logging})
	if err != nil {
		return nil, err
	}
	return &InteractiveBrowserCredential{options: cp, client: c}, nil
}

// GetToken obtains a token from Azure Active Directory using an interactive browser to authenticate.
// ctx: Context used to control the request lifetime.
// opts: TokenRequestOptions contains the list of scopes for which the token will have access.
// Returns an AccessToken which can be used to authenticate service client calls.
func (c *InteractiveBrowserCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (*azcore.AccessToken, error) {
	tk, err := c.client.authenticateInteractiveBrowser(ctx, &c.options, opts.Scopes)
	if err != nil {
		addGetTokenFailureLogs("Interactive Browser Credential", err, true)
		return nil, err
	}
	logGetTokenSuccess(c, opts)
	return tk, nil
}

// NewAuthenticationPolicy implements the azcore.Credential interface on InteractiveBrowserCredential.
func (c *InteractiveBrowserCredential) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return newBearerTokenPolicy(c, options)
}

var _ azcore.TokenCredential = (*InteractiveBrowserCredential)(nil)

// authCodeReceiver is used to allow for testing without opening an interactive browser window. Allows mocking a response authorization code and redirect URI.
var authCodeReceiver = func(ctx context.Context, authorityHost string, opts *InteractiveBrowserCredentialOptions, scopes []string) (*interactiveConfig, error) {
	return interactiveBrowserLogin(ctx, authorityHost, opts, scopes)
}

// interactiveBrowserLogin opens an interactive browser with the specified tenant and client IDs provided then returns the authorization code
// received or an error.
func interactiveBrowserLogin(ctx context.Context, authorityHost string, opts *InteractiveBrowserCredentialOptions, scopes []string) (*interactiveConfig, error) {
	// start local redirect server so login can call us back
	rs := newServer()
	state, err := uuid.New()
	if err != nil {
		return nil, err
	}
	redirectURL := opts.RedirectURL
	if redirectURL == "" {
		redirectURL = rs.Start(state.String(), opts.Port)
	}
	defer rs.Stop()
	u, err := url.Parse(authorityHost)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Add("response_type", "code")
	values.Add("response_mode", "query")
	values.Add("client_id", opts.ClientID)
	values.Add("redirect_uri", redirectURL)
	values.Add("state", state.String())
	values.Add("scope", strings.Join(scopes, " "))
	values.Add("prompt", "select_account")
	cv := ""
	// the code verifier is a random 32-byte sequence that's been base-64 encoded without padding.
	// it's used to prevent MitM attacks during auth code flow, see https://tools.ietf.org/html/rfc7636
	b := make([]byte, 32) // nolint:gosimple
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	cv = base64.RawURLEncoding.EncodeToString(b)
	// for PKCE, create a hash of the code verifier
	cvh := sha256.Sum256([]byte(cv))
	values.Add("code_challenge", base64.RawURLEncoding.EncodeToString(cvh[:]))
	values.Add("code_challenge_method", "S256")
	u.Path = runtime.JoinPaths(u.Path, opts.TenantID, path.Join(oauthPath(opts.TenantID), "/authorize"))
	u.RawQuery = values.Encode()
	// open browser window so user can select credentials
	if err = browser.OpenURL(u.String()); err != nil {
		return nil, err
	}
	// now wait until the logic calls us back
	if err := rs.WaitForCallback(ctx); err != nil {
		return nil, err
	}

	authCode, err := rs.AuthorizationCode()
	if err != nil {
		return nil, err
	}
	return &interactiveConfig{
		authCode:     authCode,
		codeVerifier: cv,
		redirectURI:  redirectURL,
	}, nil
}

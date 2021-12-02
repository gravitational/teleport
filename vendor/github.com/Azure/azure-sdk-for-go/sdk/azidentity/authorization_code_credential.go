// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// AuthorizationCodeCredentialOptions contain optional parameters that can be used to configure the AuthorizationCodeCredential.
// All zero-value fields will be initialized with their default values.
type AuthorizationCodeCredentialOptions struct {
	// Gets the client secret that was generated for the App Registration used to authenticate the client.
	ClientSecret string
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

// AuthorizationCodeCredential enables authentication to Azure Active Directory using an authorization code
// that was obtained through the authorization code flow, described in more detail in the Azure Active Directory
// documentation: https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-auth-code-flow.
type AuthorizationCodeCredential struct {
	client       *aadIdentityClient
	tenantID     string // Gets the Azure Active Directory tenant (directory) ID of the service principal
	clientID     string // Gets the client (application) ID of the service principal
	authCode     string // The authorization code received from the authorization code flow. The authorization code must not have been used to obtain another token.
	clientSecret string // Gets the client secret that was generated for the App Registration used to authenticate the client.
	redirectURI  string // The redirect URI that was used to request the authorization code. Must be the same URI that is configured for the App Registration.
}

// NewAuthorizationCodeCredential constructs a new AuthorizationCodeCredential with the details needed to authenticate against Azure Active Directory with an authorization code.
// tenantID: The Azure Active Directory tenant (directory) ID of the service principal.
// clientID: The client (application) ID of the service principal.
// authCode: The authorization code received from the authorization code flow. The authorization code must not have been used to obtain another token.
// redirectURL: The redirect URL that was used to request the authorization code. Must be the same URL that is configured for the App Registration.
// options: Manage the configuration of the requests sent to Azure Active Directory, they can also include a client secret for web app authentication.
func NewAuthorizationCodeCredential(tenantID string, clientID string, authCode string, redirectURL string, options *AuthorizationCodeCredentialOptions) (*AuthorizationCodeCredential, error) {
	if !validTenantID(tenantID) {
		return nil, &CredentialUnavailableError{credentialType: "Authorization Code Credential", message: tenantIDValidationErr}
	}
	if options == nil {
		options = &AuthorizationCodeCredentialOptions{}
	}
	authorityHost, err := setAuthorityHost(options.AuthorityHost)
	if err != nil {
		return nil, err
	}
	c, err := newAADIdentityClient(authorityHost, pipelineOptions{HTTPClient: options.HTTPClient, Retry: options.Retry, Telemetry: options.Telemetry, Logging: options.Logging})
	if err != nil {
		return nil, err
	}
	return &AuthorizationCodeCredential{tenantID: tenantID, clientID: clientID, authCode: authCode, clientSecret: options.ClientSecret, redirectURI: redirectURL, client: c}, nil
}

// GetToken obtains a token from Azure Active Directory, using the specified authorization code to authenticate.
// ctx: Context used to control the request lifetime.
// opts: TokenRequestOptions contains the list of scopes for which the token will have access.
// Returns an AccessToken which can be used to authenticate service client calls.
func (c *AuthorizationCodeCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (*azcore.AccessToken, error) {
	tk, err := c.client.authenticateAuthCode(ctx, c.tenantID, c.clientID, c.authCode, c.clientSecret, "", c.redirectURI, opts.Scopes)
	if err != nil {
		addGetTokenFailureLogs("Authorization Code Credential", err, true)
		return nil, err
	}
	logGetTokenSuccess(c, opts)
	return tk, nil
}

// NewAuthenticationPolicy implements the azcore.Credential interface on AuthorizationCodeCredential.
func (c *AuthorizationCodeCredential) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return newBearerTokenPolicy(c, options)
}

var _ azcore.TokenCredential = (*AuthorizationCodeCredential)(nil)

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

const (
	deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"
)

// DeviceCodeCredentialOptions provide options that can configure DeviceCodeCredential instead of using the default values.
// All zero-value fields will be initialized with their default values. Please note, that both the TenantID or ClientID fields should
// changed together if default values are not desired.
type DeviceCodeCredentialOptions struct {
	// Gets the Azure Active Directory tenant (directory) ID of the service principal
	// The default value is "organizations". If this value is changed, then also change ClientID to the corresponding value.
	TenantID string
	// Gets the client (application) ID of the service principal
	// The default value is the developer sign on ID for the corresponding "organizations" TenantID.
	ClientID string
	// The callback function used to send the login message back to the user
	// The default will print device code log in information to stdout.
	UserPrompt func(DeviceCodeMessage)
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

// init provides the default settings for DeviceCodeCredential.
// It will set the following default values:
// TenantID set to "organizations".
// ClientID set to the default developer sign on client ID "04b07795-8ddb-461a-bbee-02f9e1bf7b46".
// UserPrompt set to output login information for the user to stdout.
func (o *DeviceCodeCredentialOptions) init() {
	if o.TenantID == "" {
		o.TenantID = organizationsTenantID
	}
	if o.ClientID == "" {
		o.ClientID = developerSignOnClientID
	}
	if o.UserPrompt == nil {
		o.UserPrompt = func(dc DeviceCodeMessage) {
			fmt.Println(dc.Message)
		}
	}
}

// DeviceCodeMessage is used to store device code related information to help the user login and allow the device code flow to continue
// to request a token to authenticate a user.
type DeviceCodeMessage struct {
	// User code returned by the service.
	UserCode string `json:"user_code"`
	// Verification URL where the user must navigate to authenticate using the device code and credentials.
	VerificationURL string `json:"verification_uri"`
	// User friendly text response that can be used for display purposes.
	Message string `json:"message"`
}

// DeviceCodeCredential authenticates a user using the device code flow, and provides access tokens for that user account.
// For more information on the device code authentication flow see: https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-device-code.
type DeviceCodeCredential struct {
	client       *aadIdentityClient
	tenantID     string                  // Gets the Azure Active Directory tenant (directory) ID of the service principal
	clientID     string                  // Gets the client (application) ID of the service principal
	userPrompt   func(DeviceCodeMessage) // Sends the user a message with a verification URL and device code to sign in to the login server
	refreshToken string                  // Gets the refresh token sent from the service and will be used to retreive new access tokens after the initial request for a token. Thread safety for updates is handled in the NewAuthenticationPolicy since only one goroutine will be updating at a time
}

// NewDeviceCodeCredential constructs a new DeviceCodeCredential used to authenticate against Azure Active Directory with a device code.
// options: Options used to configure the management of the requests sent to Azure Active Directory, please see DeviceCodeCredentialOptions for a description of each field.
func NewDeviceCodeCredential(options *DeviceCodeCredentialOptions) (*DeviceCodeCredential, error) {
	cp := DeviceCodeCredentialOptions{}
	if options != nil {
		cp = *options
	}
	cp.init()
	if !validTenantID(cp.TenantID) {
		return nil, &CredentialUnavailableError{credentialType: "Device Code Credential", message: tenantIDValidationErr}
	}
	authorityHost, err := setAuthorityHost(cp.AuthorityHost)
	if err != nil {
		return nil, err
	}
	c, err := newAADIdentityClient(authorityHost, pipelineOptions{HTTPClient: cp.HTTPClient, Retry: cp.Retry, Telemetry: cp.Telemetry, Logging: cp.Logging})
	if err != nil {
		return nil, err
	}
	return &DeviceCodeCredential{tenantID: cp.TenantID, clientID: cp.ClientID, userPrompt: cp.UserPrompt, client: c}, nil
}

// GetToken obtains a token from Azure Active Directory, following the device code authentication
// flow. This function first requests a device code and requests that the user login before continuing to authenticate the device.
// This function will keep polling the service for a token until the user logs in.
// scopes: The list of scopes for which the token will have access. The "offline_access" scope is checked for and automatically added in case it isn't present to allow for silent token refresh.
// ctx: The context for controlling the request lifetime.
// Returns an AccessToken which can be used to authenticate service client calls.
func (c *DeviceCodeCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (*azcore.AccessToken, error) {
	for i, scope := range opts.Scopes {
		if scope == "offline_access" { // if we find that the opts.Scopes slice contains "offline_access" then we don't need to do anything and exit
			break
		}
		if i == len(opts.Scopes)-1 && scope != "offline_access" { // if we haven't found "offline_access" when reaching the last element in the slice then we append it
			opts.Scopes = append(opts.Scopes, "offline_access")
		}
	}
	if len(c.refreshToken) != 0 {
		tk, err := c.client.refreshAccessToken(ctx, c.tenantID, c.clientID, "", c.refreshToken, opts.Scopes)
		if err != nil {
			addGetTokenFailureLogs("Device Code Credential", err, true)
			return nil, err
		}
		// assign new refresh token to the credential for future use
		c.refreshToken = tk.refreshToken
		logGetTokenSuccess(c, opts)
		// passing the access token and/or error back up
		return tk.token, nil
	}
	// if there is no refreshToken, then begin the Device Code flow from the beginning
	// make initial request to the device code endpoint for a device code and instructions for authentication
	dc, err := c.client.requestNewDeviceCode(ctx, c.tenantID, c.clientID, opts.Scopes)
	if err != nil {
		addGetTokenFailureLogs("Device Code Credential", err, true)
		return nil, err // TODO check what error type to return here
	}
	// send authentication flow instructions back to the user to log in and authorize the device

	c.userPrompt(DeviceCodeMessage{
		UserCode:        dc.UserCode,
		VerificationURL: dc.VerificationURL,
		Message:         dc.Message})
	// poll the token endpoint until a valid access token is received or until authentication fails
	for {
		tk, err := c.client.authenticateDeviceCode(ctx, c.tenantID, c.clientID, dc.DeviceCode, opts.Scopes)
		// if there is no error, save the refresh token and return the token credential
		if err == nil {
			c.refreshToken = tk.refreshToken
			logGetTokenSuccess(c, opts)
			return tk.token, err
		}
		// if there is an error, check for an AADAuthenticationFailedError in order to check the status for token retrieval
		// if the error is not an AADAuthenticationFailedError, then fail here since something unexpected occurred
		if authRespErr := (*AADAuthenticationFailedError)(nil); errors.As(err, &authRespErr) && authRespErr.Message == "authorization_pending" {
			// wait for the interval specified from the initial device code endpoint and then poll for the token again
			time.Sleep(time.Duration(dc.Interval) * time.Second)
		} else {
			addGetTokenFailureLogs("Device Code Credential", err, true)
			// any other error should be returned
			return nil, err
		}
	}
}

// NewAuthenticationPolicy implements the azcore.Credential interface on DeviceCodeCredential.
func (c *DeviceCodeCredential) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return newBearerTokenPolicy(c, options)
}

// deviceCodeResult is used to store device code related information to help the user login and allow the device code flow to continue
// to request a token to authenticate a user
type deviceCodeResult struct {
	UserCode        string `json:"user_code"`        // User code returned by the service.
	DeviceCode      string `json:"device_code"`      // Device code returned by the service.
	VerificationURL string `json:"verification_uri"` // Verification URL where the user must navigate to authenticate using the device code and credentials.
	Interval        int64  `json:"interval"`         // Polling interval time to check for completion of authentication flow.
	Message         string `json:"message"`          // User friendly text response that can be used for display purposes.
}

var _ azcore.TokenCredential = (*DeviceCodeCredential)(nil)

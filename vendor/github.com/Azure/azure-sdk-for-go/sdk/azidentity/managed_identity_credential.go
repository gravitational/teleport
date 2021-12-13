// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azidentity

import (
	"context"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// ManagedIdentityIDKind is used to specify the type of identifier that is passed in for a user-assigned managed identity.
type ManagedIdentityIDKind int

const (
	// ClientID is the default identifier for a user-assigned managed identity.
	ClientID ManagedIdentityIDKind = 0
	// ResourceID is set when the resource ID of the user-assigned managed identity is to be used.
	ResourceID ManagedIdentityIDKind = 1
)

// ManagedIdentityCredentialOptions contains parameters that can be used to configure the pipeline used with Managed Identity Credential.
// All zero-value fields will be initialized with their default values.
type ManagedIdentityCredentialOptions struct {
	// ID is used to configure an alternate identifier for a user-assigned identity. The default is client ID.
	// Select the identifier to be used and pass the corresponding ID value in the string param in
	// NewManagedIdentityCredential().
	// Hint: Choose from the list of allowed ManagedIdentityIDKind values.
	ID ManagedIdentityIDKind

	// HTTPClient sets the transport for making HTTP requests.
	// Leave this as nil to use the default HTTP transport.
	HTTPClient policy.Transporter

	// Telemetry configures the built-in telemetry policy behavior.
	Telemetry policy.TelemetryOptions

	// Logging configures the built-in logging policy behavior.
	Logging policy.LogOptions
}

// ManagedIdentityCredential attempts authentication using a managed identity that has been assigned to the deployment environment. This authentication type works in several
// managed identity environments such as Azure VMs, App Service, Azure Functions, Azure CloudShell, among others. More information about configuring managed identities can be found here:
// https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview
type ManagedIdentityCredential struct {
	id     string
	client *managedIdentityClient
}

// NewManagedIdentityCredential creates an instance of the ManagedIdentityCredential capable of authenticating a resource that has a managed identity.
// id: The ID that corresponds to the user assigned managed identity. Defaults to the identity's client ID. To use another identifier,
// pass in the value for the identifier here AND choose the correct ID kind to be used in the request by setting ManagedIdentityIDKind in the options.
// options: ManagedIdentityCredentialOptions that configure the pipeline for requests sent to Azure Active Directory.
// More information on user assigned managed identities cam be found here:
// https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview#how-a-user-assigned-managed-identity-works-with-an-azure-vm
func NewManagedIdentityCredential(id string, options *ManagedIdentityCredentialOptions) (*ManagedIdentityCredential, error) {
	// Create a new Managed Identity Client with default options
	if options == nil {
		options = &ManagedIdentityCredentialOptions{}
	}
	client := newManagedIdentityClient(options)
	msiType, err := client.getMSIType()
	// If there is an error that means that the code is not running in a Managed Identity environment
	if err != nil {
		credErr := &CredentialUnavailableError{credentialType: "Managed Identity Credential", message: "Please make sure you are running in a managed identity environment, such as a VM, Azure Functions, Cloud Shell, etc..."}
		logCredentialError(credErr.credentialType, credErr)
		return nil, credErr
	}
	// Assign the msiType discovered onto the client
	client.msiType = msiType
	// check if no clientID is specified then check if it exists in an environment variable
	if len(id) == 0 {
		if options.ID == ResourceID {
			id = os.Getenv("AZURE_RESOURCE_ID")
		} else {
			id = os.Getenv("AZURE_CLIENT_ID")
		}
	}
	return &ManagedIdentityCredential{id: id, client: client}, nil
}

// GetToken obtains an AccessToken from the Managed Identity service if available.
// scopes: The list of scopes for which the token will have access.
// Returns an AccessToken which can be used to authenticate service client calls.
func (c *ManagedIdentityCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (*azcore.AccessToken, error) {
	if opts.Scopes == nil {
		err := &AuthenticationFailedError{msg: "must specify a resource in order to authenticate"}
		addGetTokenFailureLogs("Managed Identity Credential", err, true)
		return nil, err
	}
	if len(opts.Scopes) != 1 {
		err := &AuthenticationFailedError{msg: "can only specify one resource to authenticate with ManagedIdentityCredential"}
		addGetTokenFailureLogs("Managed Identity Credential", err, true)
		return nil, err
	}
	// managed identity endpoints require an AADv1 resource (i.e. token audience), not a v2 scope, so we remove "/.default" here
	scopes := []string{strings.TrimSuffix(opts.Scopes[0], defaultSuffix)}
	tk, err := c.client.authenticate(ctx, c.id, scopes)
	if err != nil {
		addGetTokenFailureLogs("Managed Identity Credential", err, true)
		return nil, err
	}
	logGetTokenSuccess(c, opts)
	logMSIEnv(c.client.msiType)
	return tk, err
}

// NewAuthenticationPolicy implements the azcore.Credential interface on ManagedIdentityCredential.
// NOTE: The TokenRequestOptions included in AuthenticationOptions must be a slice of resources in this case and not scopes.
func (c *ManagedIdentityCredential) NewAuthenticationPolicy(options runtime.AuthenticationOptions) policy.Policy {
	return newBearerTokenPolicy(c, options)
}

var _ azcore.TokenCredential = (*ManagedIdentityCredential)(nil)

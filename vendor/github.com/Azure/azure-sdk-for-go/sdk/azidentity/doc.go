//go:build go1.13
// +build go1.13

// Copyright 2017 Microsoft Corporation. All rights reserved.
// Use of this source code is governed by an MIT
// license that can be found in the LICENSE file.

/*
Package azidentity implements a set of credential types for use with
Azure SDK clients that support Azure Active Directory (AAD) token authentication.

The following credential types are included in this module:
	- AuthorizationCodeCredential
	- AzureCLICredential
	- ChainedTokenCredential
	- ClientCertificateCredential
	- ClientSecretCredential
	- DefaultAzureCredential
	- DeviceCodeCredential
	- EnvironmentCredential
	- InteractiveBrowserCredential
	- ManagedIdentityCredential
	- UsernamePasswordCredential

By default, the recommendation is that users call NewDefaultAzureCredential() which will
provide a default ChainedTokenCredential configuration composed of:
	- EnvironmentCredential
	- ManagedIdentityCredential
	- AzureCLICredential
Configuration options can be used to exclude any of the previous credentials from the
DefaultAzureCredential implementation.

Example call to NewDefaultAzureCredential():
	cred, err := NewDefaultAzureCredential(nil) // pass in nil to get default behavior for this credential
	if err != nil {
		// process error
	}
	// pass credential in to an Azure SDK client

Example call to NewDefaultAzureCredential() with options set:
	// these options will make sure the AzureCLICredential will not be added to the credential chain
	cred, err := NewDefaultAzureCredential(&DefaultAzureCredentialOptions{ExcludeAzureCLICredential: true})
	if err != nil {
		// process error
	}
	// pass credential in to an Azure SDK client

Additional configuration of each credential can be done through each credential's
options type. These options can also be used to modify the default pipeline for each
credential.

Example pattern for modifying credential options:
	cred, err := azidentity.NewClientCertificateCredential("<tenant ID>", "<client ID>", "<certificate path>", &ClientCertificateCredentialOptions{Password: "<certificate password>"})
	if err != nil {
		// process error
	}

CREDENTIAL AUTHORITY HOSTS

The default authority host for all credentials, except for ManagedIdentityCredential, is the
AzurePublicCloud host. This value can be changed through the credential options or by specifying
a different value in an environment variable called AZURE_AUTHORITY_HOST.
NOTE: An alternate value for authority host explicitly set through the code will take precedence over the
AZURE_AUTHORITY_HOST environment variable.

Example of setting an alternate Azure authority host through code:
	cred, err := azidentity.NewClientSecretCredential("<tenant ID>", "<client ID>", "<client secret>", &ClientSecretCredentialOptions{AuthorityHost: azidentity.AzureChina})
	if err != nil {
		// process error
	}
	// pass credential in to an Azure SDK client

Example of setting an alternate authority host value in the AZURE_AUTHORITY_HOST environment variable (in Powershell):
	$env:AZURE_AUTHORITY_HOST="https://contoso.com/auth/"


ERROR HANDLING

The credential types in azidentity will return one of the following error types, unless there was some other unexpected failure:
	- CredentialUnavailableError: 	This error signals that an essential component for using the credential is missing or that
									the credential is being instantiated in an environment that is incompatible with its
									functionality. These will generally be returned at credential creation after calling
									the constructor.
	- AuthenticationFailedError:	This error typically signals that a request has been made to the service and that
									authentication failed at the service level.
*/
package azidentity

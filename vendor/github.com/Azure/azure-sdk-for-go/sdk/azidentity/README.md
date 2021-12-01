# Azure Identity Client Module for Go

[![PkgGoDev](https://pkg.go.dev/badge/github.com/Azure/azure-sdk-for-go/sdk/azidentity)](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity)
[![Build Status](https://dev.azure.com/azure-sdk/public/_apis/build/status/go/go%20-%20azidentity?branchName=main)](https://dev.azure.com/azure-sdk/public/_build/latest?definitionId=1846&branchName=main)
[![Code Coverage](https://img.shields.io/azure-devops/coverage/azure-sdk/public/1846/main)](https://img.shields.io/azure-devops/coverage/azure-sdk/public/1846/main)

The `azidentity` module provides a set of credential types for use with
Azure SDK clients that support Azure Active Directory (AAD) token authentication.

[Source code](https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/azidentity)
| [Azure Active Directory documentation](https://docs.microsoft.com/azure/active-directory/)

# Getting started

## Install the package

This project uses [Go modules](https://github.com/golang/go/wiki/Modules) for versioning and dependency management.

Install the Azure Identity module:

```sh
go get -u github.com/Azure/azure-sdk-for-go/sdk/azidentity
```

## Prerequisites

- an [Azure subscription](https://azure.microsoft.com/free/)
- Go 1.13 or above

### Authenticating during local development

When debugging and executing code locally it is typical for developers to use
their own accounts for authenticating calls to Azure services. The `azidentity`
module supports authenticating through developer tools to simplify
local development.

#### Authenticating via the Azure CLI

`DefaultAzureCredential` and `AzureCLICredential` can authenticate as the user
signed in to the [Azure CLI][azure_cli]. To sign in to the Azure CLI, run
`az login`. On a system with a default web browser, the Azure CLI will launch
the browser to authenticate a user.

![Azure CLI Account Sign In](img/AzureCLILogin.png)

When no default browser is available, `az login` will use the device code
authentication flow. This can also be selected manually by running `az login --use-device-code`.

![Azure CLI Account Device Code Sign In](img/AzureCLILoginDeviceCode.png)

## Key concepts

### Credentials

A credential is a type which contains or can obtain the data needed for a
service client to authenticate requests. Service clients across the Azure SDK
accept a credential instance when they are constructed, and use that credential
to authenticate requests.

The `azidentity` module focuses on OAuth authentication with Azure Active
Directory (AAD). It offers a variety of credential types capable of acquiring
an AAD access token. See [Credential Types](#credential-types "Credential Types") below for a list of this module's credential types.

### DefaultAzureCredential
The `DefaultAzureCredential` is appropriate for most scenarios where the application is ultimately intended to run in Azure Cloud. This is because `DefaultAzureCredential` combines credentials commonly used to authenticate when deployed, with credentials used to authenticate in a development environment.

> Note: `DefaultAzureCredential` is intended to simplify getting started with the SDK by handling common scenarios with reasonable default behaviors. Developers who want more control or whose scenario isn't served by the default settings should use other credential types.

 The `DefaultAzureCredential` will attempt to authenticate via the following mechanisms in order.

![DefaultAzureCredential authentication flow](img/DAC_flow.PNG)

 - Environment - The `DefaultAzureCredential` will read account information specified via [environment variables](#environment-variables) and use it to authenticate.
 - Managed Identity - If the application is deployed to an Azure host with Managed Identity enabled, the `DefaultAzureCredential` will authenticate with that account.
 - Azure CLI - If the developer has authenticated an account via the Azure CLI `az login` command, the `DefaultAzureCredential` will authenticate with that account.


## Examples
You can find more examples of using various credentials in [Azure Identity Examples Wiki page](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples).

### Authenticating with `DefaultAzureCredential`
This example demonstrates authenticating the `ResourcesClient` from the [armresources][armresources_library] module using `DefaultAzureCredential`.

```go
// The default credential checks environment variables for configuration.
cred, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil {
  // handle error
}

// Azure SDK Azure Resource Management clients accept the credential as a parameter
client := armresources.NewResourcesClient(armcore.NewDefaultConnection(cred, nil), "<subscription ID>")
```

See more how to configure the `DefaultAzureCredential` on your workstation or Azure in [Configure DefaultAzureCredential](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#configure-defaultazurecredential).

### Authenticating a user assigned managed identity with `DefaultAzureCredential`
This example demonstrates authenticating the `ResourcesClient` from the [armresources][armresources_library] module using the `DefaultAzureCredential`, deployed to an Azure resource with a user assigned managed identity configured.

See more about how to configure a user assigned managed identity for an Azure resource in [Enable managed identity for Azure resources](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-managed-identity-for-azure-resources).

```go
// The default credential will use the user assigned managed identity with the specified client ID.
// The client_ID for the user assigned is set through an environment variable.
cred, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil {
  // handle error
}

// Azure SDK Azure Resource Management clients accept the credential as a parameter
client := armresources.NewResourcesClient(armcore.NewDefaultConnection(cred, nil), "<subscription ID>")
```

## Managed Identity Support
The [Managed identity authentication](https://docs.microsoft.com/azure/active-directory/managed-identities-azure-resources/overview) is supported via either the `DefaultAzureCredential` or the `ManagedIdentityCredential` directly for the following Azure Services:
* [Azure Virtual Machines](https://docs.microsoft.com/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token)
* [Azure App Service](https://docs.microsoft.com/azure/app-service/overview-managed-identity?tabs=dotnet)
* [Azure Kubernetes Service](https://docs.microsoft.com/azure/aks/use-managed-identity)
* [Azure Cloud Shell](https://docs.microsoft.com/azure/cloud-shell/msi-authorization)
* [Azure Arc](https://docs.microsoft.com/azure/azure-arc/servers/managed-identity-authentication)
* [Azure Service Fabric](https://docs.microsoft.com/azure/service-fabric/concepts-managed-identity)

### Examples
####  Authenticating in Azure with Managed Identity
This examples demonstrates authenticating the `ResourcesClient` from the [armresources][armresources_library] module using `ManagedIdentityCredential` in a virtual machine, app service, function app, cloud shell, or AKS environment on Azure, with system assigned, or user assigned managed identity enabled.

See more about how to configure your Azure resource for managed identity in [Enable managed identity for Azure resources](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-managed-identity-for-azure-resources)

```go
// Authenticate with a User Assigned Managed Identity.
cred, err := azidentity.NewManagedIdentityCredential("<USER ASSIGNED MANAGED IDENTITY CLIENT ID>", nil) // specify a client_ID for the user assigned identity
if err != nil {
  // handle error
}

// Azure SDK Azure Resource Management clients accept the credential as a parameter
client := armresources.NewResourcesClient(armcore.NewDefaultConnection(cred, nil), "<subscription ID>")
```

```go
// Authenticate with a System Assigned Managed Identity.
cred, err := azidentity.NewManagedIdentityCredential("", nil) // do not specify a client_ID to use the system assigned identity
if err != nil {
  // handle error
}

// Azure SDK Azure Resource Management clients accept the credential as a parameter
client := armresources.NewResourcesClient(armcore.NewDefaultConnection(cred, nil), "<subscription ID>")
```

## Credential Types

### Authenticating Azure Hosted Applications

|credential|usage|configuration|example
|-|-|-|-
|DefaultAzureCredential|simplified authentication experience to get started developing applications for the Azure cloud|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#configure--defaultazurecredential)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-with-defaultazurecredential)
|ChainedTokenCredential|define custom authentication flows composing multiple credentials||[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#chaining-credentials)
|EnvironmentCredential|authenticate a service principal or user configured by environment variables||
|ManagedIdentityCredential|authenticate the managed identity of an Azure resource|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-managed-identity-for-azure-resources)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-in-azure-with-managed-identity)

### Authenticating Service Principals

|credential|usage|configuration|example|reference
|-|-|-|-|-
|ClientSecretCredential|authenticate a service principal using a secret|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#creating-a-service-principal-with-the-azure-cli)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-service-principal-with-a-client-secret)|[Service principal authentication](https://docs.microsoft.com/azure/active-directory/develop/app-objects-and-service-principals)
|CertificateCredential|authenticate a service principal using a certificate|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#creating-a-service-principal-with-the-azure-cli)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-service-principal-with-a-client-certificate)|[Service principal authentication](https://docs.microsoft.com/azure/active-directory/develop/app-objects-and-service-principals)

### Authenticating Users

|credential|usage|configuration|example|reference
|-|-|-|-|-
|InteractiveBrowserCredential|interactively authenticate a user with the default web browser|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-applications-for-interactive-browser-oauth-2-flow)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-user-account-interactively-in-the-browser)|[OAuth2 authentication code](https://docs.microsoft.com/azure/active-directory/develop/v2-oauth2-auth-code-flow)
|DeviceCodeCredential|interactively authenticate a user on a device with limited UI|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-applications-for-device-code-flow)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-user-account-with-device-code-flow)|[Device code authentication](https://docs.microsoft.com/azure/active-directory/develop/v2-oauth2-device-code)
|UsernamePasswordCredential|authenticate a user with a username and password||[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-user-account-with-username-and-password)|[Username + password authentication](https://docs.microsoft.com/azure/active-directory/develop/v2-oauth-ropc)
|AuthorizationCodeCredential|authenticate a user with a previously obtained authorization code as part of an Oauth 2 flow|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#enable-applications-for-oauth-2-auth-code-flow)||[OAuth2 authentication code](https://docs.microsoft.com/azure/active-directory/develop/v2-oauth2-auth-code-flow)

### Authenticating via Development Tools

|credential|usage|configuration|example|reference
|-|-|-|-|-
|AzureCLICredential|authenticate as the user signed in to the Azure CLI|[configuration](https://github.com/Azure/azure-sdk-for-go/wiki/Set-up-Your-Environment-for-Authentication#sign-in-azure-cli-for-azureclicredential)|[example](https://github.com/Azure/azure-sdk-for-go/wiki/Azure-Identity-Examples#authenticating-a-user-account-with-azure-cli)|[Azure CLI authentication](https://docs.microsoft.com/cli/azure/authenticate-azure-cli)

## Environment Variables

DefaultAzureCredential] and
EnvironmentCredential can be configured with
environment variables. Each type of authentication requires values for specific
variables:

#### Service principal with secret
|variable name|value
|-|-
|`AZURE_CLIENT_ID`|id of an Azure Active Directory application
|`AZURE_TENANT_ID`|id of the application's Azure Active Directory tenant
|`AZURE_CLIENT_SECRET`|one of the application's client secrets

#### Service principal with certificate
|variable name|value
|-|-
|`AZURE_CLIENT_ID`|id of an Azure Active Directory application
|`AZURE_TENANT_ID`|id of the application's Azure Active Directory tenant
|`AZURE_CLIENT_CERTIFICATE_PATH`|path to a PEM-encoded certificate file including private key (without password protection)

#### Username and password
|variable name|value
|-|-
|`AZURE_CLIENT_ID`|id of an Azure Active Directory application
|`AZURE_USERNAME`|a username (usually an email address)
|`AZURE_PASSWORD`|that user's password

Configuration is attempted in the above order. For example, if values for a
client secret and certificate are both present, the client secret will be used.

## Troubleshooting

### Error Handling

Credentials return `CredentialUnavailableError` when they're unable to attempt
authentication because they lack required data or state. For example,
`NewEnvironmentCredential` will return an error when
[its configuration](#environment-variables "its configuration") is incomplete.

Credentials return `AuthenticationFailedError` when they fail
to authenticate. Call `Error()` on `AuthenticationFailedError` to see why authentication failed. When returned by
`DefaultAzureCredential` or `ChainedTokenCredential`,
the message collects error messages from each credential in the chain.

For more details on handling specific Azure Active Directory errors please refer to the
Azure Active Directory
[error code documentation](https://docs.microsoft.com/azure/active-directory/develop/reference-aadsts-error-codes).

### Logging

This module uses the classification based logging implementation in azcore. To turn on logging set `AZURE_SDK_GO_LOGGING` to `all`. If you only want to include logs for `azidentity`, you must create you own logger and set the log classification as `LogCredential`.
Credentials log basic information only, including `GetToken` success or failure and errors. These log entries do not contain authentication secrets but may contain sensitive information.

To obtain more detailed logging, including request/response bodies and header values, make sure to leave the logger as default or enable the `LogRequest` and/or `LogResponse` classificatons. A logger that only includes credential logs can be like the following:

```go
import azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
// Set log to output to the console
azlog.SetListener(func(cls LogClassification, s string) {
		fmt.Println(s) // printing log out to the console
  })

// Include only azidentity credential logs
azlog.SetClassifications(azidentity.LogCredential)
```

> CAUTION: logs from credentials contain sensitive information.
> These logs must be protected to avoid compromising account security.

## Next steps

The Go client libraries listed [here](https://azure.github.io/azure-sdk/releases/latest/go.html) support authenticating with `TokenCredential` and the Azure Identity library.  You can learn more about their use, and find additional documentation on use of these client libraries along samples with can be found in the links mentioned [here](https://azure.github.io/azure-sdk/releases/latest/go.html).

## Provide Feedback

If you encounter bugs or have suggestions, please
[open an issue](https://github.com/Azure/azure-sdk-for-go/issues) and assign the `Azure.Identity` label.

## Contributing

This project welcomes contributions and suggestions. Most contributions require
you to agree to a Contributor License Agreement (CLA) declaring that you have
the right to, and actually do, grant us the rights to use your contribution.
For details, visit [https://cla.microsoft.com](https://cla.microsoft.com).

When you submit a pull request, a CLA-bot will automatically determine whether
you need to provide a CLA and decorate the PR appropriately (e.g., label,
comment). Simply follow the instructions provided by the bot. You will only
need to do this once across all repos using our CLA.

This project has adopted the
[Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information, see the
[Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/)
or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any
additional questions or comments.

[azure_cli]: https://docs.microsoft.com/cli/azure
[armresources_library]: https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/resources/armresources
[azblob]: https://github.com/Azure/azure-sdk-for-go/tree/main/sdk

![Impressions](https://azure-sdk-impressions.azurewebsites.net/api/impressions/azure-sdk-for-go%2Fsdk%2Fidentity%2Fazure-identity%2FREADME.png)

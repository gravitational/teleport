# Release History

## v0.11.0 (2021-09-08)
### Breaking Changes
* Unexported `AzureCLICredentialOptions.TokenProvider` and its type,
  `AzureCLITokenProvider`

### Bug Fixes
* `ManagedIdentityCredential.GetToken` returns `CredentialUnavailableError`
  when IMDS has no assigned identity, signaling `DefaultAzureCredential` to
  try other credentials


## v0.10.0 (2021-08-30)
### Breaking Changes
* Update based on `azcore` refactor [#15383](https://github.com/Azure/azure-sdk-for-go/pull/15383)

## v0.9.3 (2021-08-20)

### Bugs Fixed
* `ManagedIdentityCredential.GetToken` no longer mutates its `opts.Scopes`

### Other Changes
* Bumps version of `azcore` to `v0.18.1`


## v0.9.2 (2021-07-23)
### Features Added
* Adding support for Service Fabric environment in `ManagedIdentityCredential`
* Adding an option for using a resource ID instead of client ID in `ManagedIdentityCredential`


## v0.9.1 (2021-05-24)
### Features Added
* Add LICENSE.txt and bump version information


## v0.9.0 (2021-05-21)
### Features Added
* Add support for authenticating in Azure Stack environments
* Enable user assigned identities for the IMDS scenario in `ManagedIdentityCredential`
* Add scope to resource conversion in `GetToken()` on `ManagedIdentityCredential`


## v0.8.0 (2021-01-20)
### Features Added
* Updating documentation


## v0.7.1 (2021-01-04)
### Features Added
* Adding port option to `InteractiveBrowserCredential`


## v0.7.0 (2020-12-11)
### Features Added
* Add `redirectURI` parameter back to authentication code flow


## v0.6.1 (2020-12-09)
### Features Added
* Updating query parameter in `ManagedIdentityCredential` and updating datetime string for parsing managed identity access tokens.


## v0.6.0 (2020-11-16)
### Features Added
* Remove `RedirectURL` parameter from auth code flow to align with the MSAL implementation which relies on the native client redirect URL.


## v0.5.0 (2020-10-30)
### Features Added
* Flattening credential options


## v0.4.3 (2020-10-21)
### Features Added
* Adding Azure Arc support in `ManagedIdentityCredential`


## v0.4.2 (2020-10-16)
### Features Added
* Typo fixes


## v0.4.1 (2020-10-16)
### Features Added
* Ensure authority hosts are only HTTPs


## v0.4.0 (2020-10-16)
### Features Added
* Adding options structs for credentials


## v0.3.0 (2020-10-09)
### Features Added
* Update `DeviceCodeCredential` callback


## v0.2.2 (2020-10-09)
### Features Added
* Add `AuthorizationCodeCredential`


## v0.2.1 (2020-10-06)
### Features Added
* Add `InteractiveBrowserCredential`


## v0.2.0 (2020-09-11)
### Features Added
* Refactor `azidentity` on top of `azcore` refactor
* Updated policies to conform to `policy.Policy` interface changes.
* Updated non-retriable errors to conform to `azcore.NonRetriableError`.
* Fixed calls to `Request.SetBody()` to include content type.
* Switched endpoints to string types and removed extra parsing code.


## v0.1.1 (2020-09-02)
### Features Added
* Add `AzureCLICredential` to `DefaultAzureCredential` chain


## v0.1.0 (2020-07-23)
### Features Added
* Initial Release. Azure Identity library that provides Azure Active Directory token authentication support for the SDK.

---
authors: Jake Ward (jacob.ward@goteleport.com)
state: draft
---

# RFD 0250 - Enable Entra ID SAML authentication for users in 150+ groups

## Required Approvers

* Engineering: @smallinsky && @flyinghermit
* Product: @r0mant

## What

Enable users authenticating using Entra ID SAML that are members of 150+ groups to login to Teleport.

This RFD proposes to align the behaviour of the SAML authentication flow with a previous change by @flyinghermit (https://github.com/gravitational/teleport/pull/58098) done for the OIDC authentication flow.

## Why

Users in 150+ groups get no group-based role mappings when authenticating using Entra ID SAML, resulting in them seeing the error: "You are not authorized, please contact your SSO administrator."

## Details

When a user authenticates to Teleport using Entra ID SAML, the SAML assertion contains a `groups` attribute[^1] that contains a list of all the groups the user is a member of. When the number of groups exceeds 150, the assertion instead contains a `groups.link` attribute[^2] that contains a Graph API URL[^3] to query to get a list of all the groups the user is a member of. See the Important callout in MS docs here: [Configure group claims for applications by using Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims).

Note that the URL provided in the assertion is actually for a dreprecated API, we will actually use the user's ID (from http://schemas.microsoft.com/identity/claims/objectidentifier) to call `transitiveMemberOf` directly.

Since a similar issue was already resolved for the Entra ID OIDC connector, the proposal is to mostly mirror that for the SAML connector and leverage as much of the existing implemention as possible (refactoring, if necessary).

An `EntraIDGroupsProvider` field will be added to `SAMLConnectorSpecV2` to store a user groups provider.

```proto
message SAMLConnectorSpecV2 {
    // ...omitted for brevity...

    EntraIDGroupsProvider entra_id_groups_provider = 23;
}
```

Two methods will be added to the `SAMLConnector` interface to determine if user groups provider is enabled on the connector and to get it.

```go
type SAMLConnector interface {
    // ...omitted for brevity...

    GetEntraIDGroupsProvider() *EntraIDGroupsProvider
    IsEntraIDGroupsProviderDisabled() bool 
}
```

The existing `entraIDGroupsProvider` struct will be renamed to `oidcEntraIDGroupsProvider` and a new `samlEntraIDGroupsProvider` struct will be created.

```go
// existing (renamed)
type oidcEntraIDGroupsProvider struct {
    connector  types.OIDCConnector
    idToken    *oidc.Tokens[*oidc.IDTokenClaims]
    logger     *slog.Logger
    httpClient *http.Client
}

// new
type samlEntraIDGroupsProvider struct {
    connector  types.SAMLConnector
    auth       *auth.Server
    logger     *slog.Logger
    httpClient *http.Client
}
```

The `getGraphEndpoint` and `getGroupType` functions will be updated to accept a `*EntraIDGroupsProvider` instead of a `OIDCConnector` so that they can be used for both SAML and OIDC connectors.

```go
func getGraphEndpoint(provider *EntraIDGroupsProvider) string
func getGroupType(provider *EntraIDGroupsProvider) string
```

Construction of credentials will be moved out of `newGraphClient` and the function updated to accept `azcore.TokenCredential` instead of `types.OIDCConnector` so that it can be used to construct clients for both SAML and OIDC connectors.

```go
func newGraphClient(
    tokenProvider azcore.TokenCredential,
    graphEndpoint string,
    httpClient *http.Client,
) (*msgraph.Client, error)
```

In the `validateSAMLResponse` function in `e/lib/auth/saml.go`, `maybeFetchSAMLEntraIDGroups` will be called to fetch the groups and add them to the groups on `assertionInfo.Values` before attributes are extracted from the assertion for role mapping.

```go
provider := &samlEntraIDGroupsProvider{connector, auth, logger, httpClient}
provider.maybeFetchSAMLEntraIDGroups(ctx, assertionInfo)
```

### Authentication

To enumerate the groups the user is a member of, Teleport will need to authenticate to the MS Graph API. The proposal is to use the Entra ID plugin and OIDC integration to do this.

An Entra ID SAML connector can be set up in one of two ways: 

#### Entra ID Plugin

When a SAML connector is created using the Entra ID plugin (either via the UI guided set up or the CLI using `tctl plugins install entraid`), an Entra ID plugin is created with an `sso_connector_id` field with the same name as the SAML connector, along with an OIDC integration.

An `azureoidc.sh` Bash script is then generated to run in Azure Cloud Shell which does the required configuration in Azure.

No additional configuration will be required to support Graph API authentication for SAML connectors created using the Entra ID plugin.

#### Standalone SAML Connector

When a SAML connector is created directly, no Entra plugin or OIDC integration is created. To support Graph API authentication for SAML connectors created directly, a new Entra ID plugin and OIDC integration will need to be created.

The `tctl plugins install entraid` command will be extended with a `--saml-connector` flag which will take the name of an existing SAML connector. This will create an Entra ID plugin with the same `sso_connector_id` as provided by the `--saml-connector` flag. 

The Bash script generated will do a subset of the steps from the full Entra ID plugin setup, namely: create the federated identity credential and grant the required Graph API permissions (skipping the SAML SSO configuration, which is already in place).

Example (assuming there is an existing SAML connector called `my-entra-connector`): 

```bash
tctl plugins install entraid \
    --saml-connector=my-entra-connector \ # new flag
    --default-owner=admin \
    --auth-server example.teleport.sh:443 
```

In the case of an error, a suitable error message will be returned. Notable error cases include: 

- Provided `--saml-connector` flag with mutually exclusive flags, e.g. `--name`
- No SAML connector matching the name provided by the `--saml-connector` flag
- The matching SAML connector isn't configured for Entra
- Already an existing Entra ID plugin matching the `--saml-connector` flag

#### Graph API Authentication

In both cases, the system will now have a SAML connector for Entra, along with an Entra ID plugin and OIDC integration in Teleport, and a federated identity with Graph API permissions in Azure.

1. When the groups overage scenario is detected, the Entra ID plugin will be looked up by matching the `sso_connector_id` to name of the SAML connector being used to authetnicate into Teleport
2. A credential will be constructed using the `tenant_id` and `client_id` from the OIDC integration with a callback to `azureoidc.GenerateEntraOIDCToken` to create the OIDC JWT on token refresh
3. This credential will be passed to `newGraphClient` to build the authenticated client

Example: 

```go
spec := integration.GetAzureOIDCIntegrationSpec()

credential, err := azidentity.NewClientAssertionCredential(
    spec.TenantID,
    spec.ClientID,
    func(ctx context.Context) (string, error) {
        return azureoidc.GenerateEntraOIDCToken(ctx, cache, keyStore, clock)
    },
)
if err != nil {
    return err
}

endpoint := getGraphEndpoint(connector.GetEntraIDGroupsProvider())

client, err := newGraphClient(credential, endpoint, httpClient)
if err != nil {
    return err
}
```

## Alternatives Considered

### Client ID and Secret on the SAML Connector

A `client_id` and `client_secret` could have been added to the `SAMLConnectorSpecV2` alongside the `entra_id_groups_provider` for authenticating directly to the Graph API. However, this would have meant storing an additional long-lived secret in Teleport, and would not make use of existing infrastructure and configuration already present for connectors created using the Entra ID plugin.

--- 

[^1]: Actual attribute name: http://schemas.microsoft.com/ws/2008/06/identity/claims/groups
[^2]: Actual attribute name: http://schemas.microsoft.com/claims/groups.link
[^3]: Example value: https://graph.windows.net/396f5d4f-bc5f-48aa-aca0-4ce432c060ed/users/877021a5-998f-4271-b8ae-c6170636ba45/getMemberObjects

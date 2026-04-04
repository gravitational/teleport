---
authors: Jake Ward (jacob.ward@goteleport.com)
state: draft
---

# RFD 0252 - Enable Entra ID SAML authentication for users in 150+ groups

## Required Approvers

* Engineering: @smallinsky && @flyinghermit
* Product: @r0mant

## What

Enable users authenticating using Entra ID SAML that are members of 150+ groups to login to Teleport.

This RFD proposes to align the behaviour of the SAML authentication flow with a previous change by @flyinghermit (https://github.com/gravitational/teleport/pull/58098) done for the OIDC authentication flow.

## Why

Users in 150+ groups get no group-based role mappings when authenticating using Entra ID SAML, resulting in them seeing the error: "You are not authorized, please contact your SSO administrator."

## Details

### UX

When the groups overage scenario is detected and Teleport is unable to retrieve the user's groups, the user will receive the following error: 

```
Your account is a member of more than 150 Entra ID groups. Please contact your SSO administrator to configure Graph API access on the Teleport SAML connector.
```

#### Case 1 - User added to 150+ groups on existing SAML connector

User is a member of 136 groups and is able to login. User is then added to another 15 groups.

##### Legacy connector

The groups overage is detected and user receives the error message above.

The admin must then create a client secret and grant the `GroupMember.Read.All` Graph API permission[^4] on the existing enterprise app in Azure, then add the credentials to the SAML connector in Teleport.

```yaml
kind: saml
metadata:
    name: my-entra-connector
spec:
    credentials:
        oauth:
            tenant_id: <tenant_id>
            client_id: <client_id>
            client_secret: <client_secret>
    entra_id_groups_provider:
        group_type: <group_type> # Optional, defaults to "security-groups".
        graph_endpoint: <graph_endpoint> # Optional, defaults to "https://graph.microsoft.com".

    # ...omitted for brevity.
```

##### Entra ID plugin

Groups overage is handled automatically, with no additional configuration required.

#### Case 2 - Fresh Entra ID plugin with 150+ groups

Admin installs the Entra ID plugin.

Admin is already a member of 150+ groups and tests the integration by logging in to Teleport. 

Groups overage is detected and handled automatically, with no additional configuration required.

#### Case 3 - Existing Entra ID plugin and SAML connector

Groups overage is detected and handled automatically, with no additional configuration required.

### Backward Compatibility

The change is backwards compatible. The new fields are optional, so existing SAML connectors will work as they currently do.

If the `client_secret` expires, or is rotated in Entra without being updated in Teleport, then users who are members of 150+ groups will be unable to login (receive the error message above). The SAML login flow for users in <= 150 groups will be unaffected, since the groups overage will not be triggered.

### Implementation

When a user authenticates to Teleport using Entra ID SAML, the SAML assertion contains a `groups` attribute[^1] that contains a list of all the groups the user is a member of. When the number of groups exceeds 150, the assertion instead contains a `groups.link` attribute[^2] that contains a Graph API URL[^3] to query to get a list of all the groups the user is a member of. See the Important callout in MS docs here: [Configure group claims for applications by using Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims). Note: the endpoint in `groups.link` is for a deprecated API and is not the one that is actually called[^5]. 

Since a similar issue was already resolved for the Entra ID OIDC connector, the proposal is to mostly mirror that for the SAML connector and leverage as much of the existing implementation as possible (refactoring, if necessary).

Two fields will be added to `SAMLConnectorSpecV2`. `entra_id_groups_provider` will use the existing `EntraIDGroupsProvider` type to store a user groups provider. `credentials` will use a new `SAMLConnectorCredentials` type to store credentials for authenticating to MS Graph API.

```proto
message SAMLConnectorCredentials {
    string tenant_id = 1;
    string client_id = 2;
    string client_secret = 3;
}

message SAMLConnectorSpecV2 {
    // ...omitted for brevity...

    EntraIDGroupsProvider entra_id_groups_provider = 23;
    SAMLConnectorCredentials credentials = 24;
}
```

`SAMLConnectorCredentials.ClientSecret` will be handled consistently with `OIDCConnectorV3.ClientSecret` and stripped from responses via `WithoutSecrets()`.

The `SAMLConnector` interface will be updated with methods to get these two fields, along with a helper to determine if the groups provider is disabled.

```go
type SAMLConnector interface {
    // ...omitted for brevity...

    GetEntraIDGraphCredentials() *EntraIDGraphCredentials
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

The `getGraphEndpoint` and `getGroupType` functions will be updated to accept an `*EntraIDGroupsProvider` instead of a `OIDCConnector` so that they can be used for both SAML and OIDC connectors.

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

#### Authentication

When a user authenticates and the groups overage scenario is detected, a credential will be constructed and passed to `newGraphClient` to build the authenticated client to query the Graph API.

##### SAML connector created with Entra ID plugin

For SAML connectors created using the Entra ID plugin (either via the UI guided setup or the CLI using `tctl plugins install entraid`), the plugin will be looked up by matching its `sso_connector_id` to the SAML connector name, and the `tenant_id` and `client_id` will be retrieved from the corresponding OIDC integration to construct the credential.

Example: 
```go
integration, err := auth.GetIntegration(ctx, entraPlugin.GetName())
if err != nil {
    return err
}

spec := integration.GetAzureOIDCIntegrationSpec()

credential, err := azidentity.NewClientAssertionCredential(
    spec.TenantID,
    spec.ClientID,
    func(ctx context.Context) (string, error) {
        return azureoidc.GenerateEntraOIDCToken(ctx, cache, keyStore, clock)
    },
    nil,
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

##### SAML connector created directly

For SAML connectors created directly (either via the UI YAML editor, `tctl create ...`, or IaC), the newly added `EntraIDGraphCredentials` will be used to construct the credential.

Example:
```go
creds := connector.GetEntraIDGraphCredentials()

credential, err := azidentity.NewClientSecretCredential(
    creds.TenantID,
    creds.ClientID,
    creds.ClientSecret,
    nil,
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

If neither an Entra ID plugin exists or `entra_id_graph_credentials` is configured, authentication will fail with error, as described in UX section above.

## Alternatives Considered

### OIDC integration only

An Entra ID plugin/OIDC integration with reduced scope could have been required for all Entra SAML connectors. However, this would require the Teleport proxy endpoint to be public, which is often not the case, and the Entra ID plugin (without further changes) would start syncing users and groups from Entra, which is likely unwanted.

### Client ID and Client Secret only

A `client_id` and `client_secret` could have been required for all Entra SAML connectors. However, this would require connectors that are already backed by an Entra ID plugin and OIDC integration to add additional configuration.

--- 

[^1]: Actual attribute name: http://schemas.microsoft.com/ws/2008/06/identity/claims/groups
[^2]: Actual attribute name: http://schemas.microsoft.com/claims/groups.link
[^3]: Example value: https://graph.windows.net/396f5d4f-bc5f-48aa-aca0-4ce432c060ed/users/877021a5-998f-4271-b8ae-c6170636ba45/getMemberObjects
[^4]: Required permissions for `transitiveMemberOf`: https://learn.microsoft.com/en-us/graph/api/user-list-transitivememberof?view=graph-rest-1.0&tabs=http
[^5]: `IterateUsersTransitiveMemberOf`: https://github.com/gravitational/teleport/blob/2aaf823b00d8364f80525e16ed2a089ea8954c8c/lib/msgraph/paginated.go#L281-L290

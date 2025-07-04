---
title: RFD0025e - OAuth client_credentials flow for Teleport Plugins
authors: Marek Smoliński (marek@goteleport.com)
state: draft
---

# Required approvers

- Product: @r0mant

# What

This RFD proposes the implementation of the OAuth 2.0 `client_credentials` flow for Teleport plugins.
This enhancement will allow third-party APIs that integrate with Teleport like  SCIM clients such as Sailpoint to authenticate using dynamic,
short-lived tokens issued via OAuth, instead of static bearer tokens.

This RFD proposes design and implementation of OAuth flow for plugins
where the third-party API needs to access Teleport.


# Why

Teleport acts as a SCIM Provider in integrations with SCIM-compatible platforms (e.g., Okta), where groups and users are pushed to Teleport.
Currently, these integrations authenticate using static, long-lived bearer tokens. This approach lacks security best practices around token management and rotation.

By implementing the OAuth 2.0 `client_credentials` flow, Teleport can act as an OAuth Provider, enabling integrations to:

- Authenticate using a `client_id` and `client_secret`
- Obtain time-bound access tokens
- Avoid long-lived, static credentials

# Non-goals

- Implementing the full OAuth 2.0 specification (only `client_credentials` grant type is supported)
- Introducing fine-grained permission scopes within plugin access tokens

## UX

## SCIM Plugin Enrollment via OAuth

When a user creates a new SCIM integration, the plugin will default to using the OAuth flow. During this process, the following credentials will be displayed:
- `client_id`
- `client_secret`
- OAuth token endpoint URL
- Plugin API URL

These credentials are required to configure the external SCIM client.

**Example UI – Plugin Credential Display:**
![Screenshot 2025-06-20 at 09 30 13](https://github.com/user-attachments/assets/6d1443f4-76b7-472c-9e7d-25e162e6c58e)

**Example UI – External SCIM Client Configuration:**

![Screenshot 2025-06-20 at 09 30 59](https://github.com/user-attachments/assets/7f9d11d0-706d-450d-ae58-76c6a697c041)


# Technical Design

## Credential Issuance

- During plugin creation, instead of issuing a static bearer token, generate:
  - `client_id` - utils.CryptoRandomHex(16)
  - `client_secret` utils.CryptoRandomHex(16)
  Where [utils.CryptoRandomHex](https://github.com/gravitational/teleport/blob/master/lib/utils/rand.go#L37) is crypto-strong pseudo-random generator used also for join tokens.


- These credentials are securely stored in Teleport’s backend within the plugin's static credential resource where
 the spec [PluginStaticCredentialsOAuthClientSecret](https://github.com/gravitational/teleport/blob/master/api/proto/teleport/legacy/types/types.proto#L7857) will be used.


## OAuth Token Endpoint

A new HTTP endpoint will be introduced:
`/plugin/:name/token`

This endpoint will implement the OAuth 2.0 `client_credentials` grant and delegate token generation via a new gRPC method:

```proto
// CreatePluginOauthToken issues a short-lived OAuth access token for the specified plugin.
//
// This endpoint supports the OAuth 2.0 "client_credentials" grant type, where the plugin
// authenticates using its client ID and client secret
rpc CreatePluginOauthToken(CreatePluginOauthTokenRequest) returns (CreatePluginOauthTokenResponse);

// CreatePluginOauthTokenRequest is the request type for creating an OAuth token for a plugin.
message CreatePluginOauthTokenRequest {
  // plugin_name is the name of the plugin for which the OAuth token is requested.
  string plugin_name = 1;
  // client_id is the OAuth client identifier issued to the plugin.
  string client_id = 2;
  // client_secret is the secret associated with the client_id.
  string client_secret = 3;
  // grant_type is the OAuth 2.0 grant type being used. Currently, only "client_credentials" is supported.
  string grant_type = 4;
}

// CreatePluginOauthTokenResponse is the response type for a successful OAuth token creation.
message CreatePluginOauthTokenResponse {
  // access_token is the generated token issued to the plugin.
  string access_token = 1;
  // token_type describes the type of the token issued
  string token_type = 2;
  // expires_in is the number of seconds until the token expires.
  int64 expires_in = 3;
}
```

## Token Generation

Upon successful validation of client_id and client_secret, the server will issue a signed JWT access token with the following claims:
  - **Audience**: `plugin:{plugin_id}` where the plugin_id is unique plugin identifier generated during plugin creation.
  - **Issuer**: Teleport Cluster Name
  - **Expiry**: TTL (e.g., 1 hour default)
  - The token will be signed using Teleport’s OIDC IDP CA private key.


## Token Validation

Teleport plugin–facing SCIM API endpoints handlers:
  - `GET /webapi/scim/:integration/:resourceType`
  - `POST /webapi/scim/:integration/:resourceType`
  - `PUT /webapi/scim/:integration/:resourceType/:resourceID`
  - `PATCH /webapi/scim/:integration/:resourceType/:resourceID`
  - `DELETE /webapi/scim/:integration/:resourceType/:resourceID`

Defined in  [registerSCIMHandlers](https://github.com/gravitational/teleport.e/blob/master/lib/web/scim.go#L31) will be updated to:
- Accept requests with `Authorization: JWT ...`, along with the legacy `Authorization: Bearer ...` for backward compatibility.
- Validate the JWT signature using the OIDC IDP CA public key.
- Verify that the issuer and audience match `plugin:{plugin_id}`.



# Security:

- `client_secret` must be protected as sensitive credentials and never exposed in logs or UI (apart from one time display during plugin enrolment flow)
- Token TTL should be short (e.g., 1 hour) to limit impact of leaks.
- Usage of the existing OIDC IDP CA with different JWT audience will allow to separate the plugin token flow from AWS OIDC discovery flow and EntraID integration flow.
- Rate limiting will be implemented for the web token endpoint.
- A compromised `client_secret` can be rotated via `tctl edit plugin command`


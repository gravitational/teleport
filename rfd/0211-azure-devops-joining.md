---
authors: Noah Stride (noah@goteleport.com)
state: implemented (v17.5.0)
---
<!--- cslint:disable -->
# RFD 211 - Azure DevOps Joining

## Required Approvals

- Engineering: TimB (@timothyb89) && DanU (@boxofrad)
- Product: DaveS (@thedevelopnik) 

## What

Allow Bots & Agents to authenticate to Teleport without the use of long-lived
secrets from Azure DevOps pipelines.

## Why

Azure DevOps is a popular CI/CD platform, and today, leveraging Teleport 
Machine & Workload Identity from Azure DevOps is not possible without
laborious workarounds.

An Azure Devops join method would allow Bots/Agents to authenticate to Teleport
without the use of long-lived secrets, and provide richer metadata for audit
logging & authorization decision purposes.

## Details

Goals:

- Allow authentication to Teleport from Azure DevOps pipelines without the use
  of long-lived secrets.
- It should be possible to scope this authentication to a specific Azure DevOps
  pipeline. 
- Mitigate common attacks such as token reuse.

This RFD will make reference to and rely on prior art from other OIDC join
methods e.g `github`, `gitlab`.

### Overview

The Azure DevOps join method will be named `azure_devops` and will be a 
delegated, non-renewable join method.

The Azure DevOps join method will leverage the OpenID Connect (OIDC) token that
is available to pipelines via a special internal API. A public OpenID
configuration document and JWKS are available to support validating these
tokens.

The ID Token JWT issued to the pipeline contains the following claims:

```json
{
  "jti": "90b75b0a-61b6-4b71-ba6f-11107d95f4c5",
  "sub": "p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
  "aud": "api://AzureADTokenExchange",
  "org_id": "0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "prj_id": "271ef6f7-5998-4b0f-86fb-4b54d9129990",
  "def_id": "1",
  "rpo_id": "strideynet/azure-devops-testing",
  "rpo_uri": "https://github.com/strideynet/azure-devops-testing.git",
  "rpo_ver": "c291ea713801eb300054d353d279e7b02331f671",
  "rpo_ref": "refs/heads/main",
  "run_id": "5",
  "iss": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "nbf": 1745839609,
  "exp": 1745840508,
  "iat": 1745840209
}
```

Of note:

- `sub` identifies the organization, project and pipeline by user-facing name.
- `org_id` and `prj_id` contains the UUIDs of the organization and project
  included within `sub`.
- `iss` is specific to the AZDO organization.
- The token is issued with a 5 minute TTL.
- The `aud` claim cannot be modified and contains a fixed string. This makes our
  typical re-use mitigation with a nonce-including `aud` infeasible.
- The `jti` claim is present and appears to be unique as per specification. 

### ID Token Source

Before joining, the Agent or TBot will need to fetch the OIDC ID Token from
Azure DevOps.

This is done by making a POST request to the `oidctoken` endpoint. The location
of this endpoint is exposed to the task via the `SYSTEM_OIDCREQUESTURI`
environment variable or the `System.OidcRequestUri` variable. This POST request
must be authenticated with a bearer token, which is available in the
`System.AccessToken` pipeline variable.

The `System.AccessToken` variable is not made available to the environment by
default, it must be explicitly mapped in, e.g

```yaml
steps:
- env:
    SYSTEM_ACCESSTOKEN: $(System.AccessToken)
  script: |
    OIDC_REQUEST_URL="${SYSTEM_OIDCREQUESTURI}?api-version=7.1"
    curl -s -H "Content-Length: 0" -H "Content-Type: application/json" -H "Authorization: Bearer $SYSTEM_ACCESSTOKEN" -X POST $OIDC_REQUEST_URL
```

**Therefore, we will require the user to explicitly map this environment variable
in to steps that invoke the `tbot` binary.**

Sample endpoint response:

```json
{"oidcToken":"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI
6Imt6UFh2cVJPMEN1UzRqU296REc4d21EM1RmcyIsImtpZCI6IjkzMzNE
N0JFQTQ0RUQwMkI5MkUyMzRBOENDMzFCQ0MyNjBGNzRERkIifQ.eyJqdG
kiOiIwNGM2ODQ4Ni1kY2ViLTQ3M2EtYTMxMC1mODMwMmZjY2FiODEiLCJ
zdWIiOiJwOi8vbm9haHN0cmlkZTAzMDQvdGVzdGluZy1henVyZS1kZXZv
cHMtam9pbi9zdHJpZGV5bmV0LmF6dXJlLWRldm9wcy10ZXN0aW5nIiwiY
XVkIjoiYXBpOi8vQXp1cmVBRFRva2VuRXhjaGFuZ2UiLCJvcmdfaWQiOi
IwY2EzZGRkOS1mMGIwLTQ2MzUtYTk4Yy01ODY2NTI2OTYxYjYiLCJwcmp
faWQiOiIyNzFlZjZmNy01OTk4LTRiMGYtODZmYi00YjU0ZDkxMjk5OTAi
LCJkZWZfaWQiOiIxIiwicnBvX2lkIjoic3RyaWRleW5ldC9henVyZS1kZ
XZvcHMtdGVzdGluZyIsInJwb191cmkiOiJodHRwczovL2dpdGh1Yi5jb2
0vc3RyaWRleW5ldC9henVyZS1kZXZvcHMtdGVzdGluZy5naXQiLCJycG9
fdmVyIjoiZTZiOWViMjlhMjg4YjI3YTNhODJjYzE5YzQ4YjlkOTRiODBh
ZmYzNiIsInJwb19yZWYiOiJyZWZzL2hlYWRzL21haW4iLCJydW5faWQiO
iIxNyIsImlzcyI6Imh0dHBzOi8vdnN0b2tlbi5kZXYuYXp1cmUuY29tLz
BjYTNkZGQ5LWYwYjAtNDYzNS1hOThjLTU4NjY1MjY5NjFiNiIsIm5iZiI
6MTc0NTg1MTAzOCwiZXhwIjoxNzQ1ODUxOTM4LCJpYXQiOjE3NDU4NTE2
Mzh9.xgH3aeRgs482lNlh2kQn_Dda_pvpyFhQ6pbZLMR81ozp7_7PIFOA
DiEJlqfHryt7bVQj03zdvikAzJLGkvUW_5WusGQSJtwy_Y2cdou0mMReI
SNNHpPS2Jvn93VjA3YFaw2vBo3Vjay_w7WElRU8WwEtKaQZmu915Zejb4
IMK5zVF-jdILhng2RV_t0xm5pUUBN0gt7u-QKJ2px9iBdzYBJEtdR5Q-F
ExwM7WDTZp4w2wDVslkaExAd_K3IoN4tqZk6FhKhsB3IRNMdsm9hGxwjt
Jg3QkTETr8PZN2IUAKP6p7lXD1sXZOtV8exLh1VzTMqUHMqbWlVsG0rUX
1WF7g"}
```

### ID Token Validation

The ID Token is issued from an organization-specific issuer (e.g contains an
`iss` claim that is specific to the organization).

The OpenID configuration document available at the well-known URL for this `iss`
is as follows:

```json
{
  "issuer": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "jwks_uri": "https://vstoken.dev.azure.com/.well-known/jwks",
  "subject_types_supported": [
    "public",
    "pairwise"
  ],
  "response_types_supported": [
    "id_token"
  ],
  "claims_supported": [
    "sub",
    "aud",
    "exp",
    "iat",
    "iss",
    "nbf"
  ],
  "id_token_signing_alg_values_supported": [
    "RS256"
  ],
  "scopes_supported": [
    "openid"
  ]
}
```

Of note is the fact that the JWKS URI is not specific to each organization,
this has a few ramifications:

- The JWKS values could be cached across join token resources.
- We must not assume that because an ID Token passes validation for a specific
  organization's well-known that it belongs to that specific organization.

Notably, the issued JWTs include the `kid` header and this `kid` field is also
present within the JWKS. This allows the correct JWK to be selected from the 
JWKS for validation.

In order to be able to fetch the correct OIDC configuration document, we will
require knowledge ahead of time of the organization UUID. **The user will
configure this as part of the ProvisionToken specification**.

During validation, we will expect the `aud` claim to be
`api://AzureADTokenExchange`.

For validation, we will leverage the `github.com/coreos/go-oidc/v3/oidc` package
as is used for other OIDC join methods (e.g `bitbucket`).

### Join RPC

The Azure DevOps join method can leverage the existing `RegisterUsingToken` gRPC
RPC since it does not involve a challenge and response and therefore does not
require bi-di streaming.

No additional fields will need to be added to the `RegisterUsingToken` RPC 
request or response.

When a join occurs via the Azure DevOps join method, the Join RPC shall:

1. Perform the standard ProvisionToken resource validation
  (e.g. ensure the ProvisionToken has not expired ).
2. Ensure the specified ProvisionToken is of type `azure_devops`.
3. Fetch the well-known and JWKS based on the OrganizationID configured within
   the ProvisionToken.
4. Validate the provided ID Token signature using the fetched JWKS.
5. Validate common fields within the JWT (e.g `exp`, `nbf`, `aud`).
6. Validate that the issuer of the ID Token matches the value we would expect
   given the configured OrganizationID.
7. Extract the claims from the ID Token.
8. Validate these claims against the configured allow rules in the
   ProvisionToken.

### UX

#### Provision Token

The ProvisionToken resource will be extended with new fields to support the 
configuration of the Azure Devops join method.

Like other join methods, a `spec.azure_devops` field will be introduced to hold
configuration specific to the Azure DevOps join method. This field will include:

- Configuration necessary to validate the ID Token:
  - The UUID of the organization.
- The allow rules that control which pipelines are permitted to authenticate.

```protobuf
// ProvisionTokenSpecV2AzureDevops contains the Azure Devops-specific
// configuration. 
message ProvisionTokenSpecV2AzureDevops {
  message Rule {
    // Sub also known as Subject is a string that roughly uniquely identifies
    // the workload. Example:
    // `p://my-organization/my-project/my-pipeline`
    // Mapped from the `sub` claim.
    string Sub = 1 [(gogoproto.jsontag) = "sub,omitempty"];
    // The name of the AZDO project. Example:
    // `my-project`.
    // Mapped out of the `sub` claim.
    string ProjectName = 2 [(gogoproto.jsontag) = "project_name,omitempty"];
    // The name of the AZDO pipeline. Example:
    // `my-pipeline`.
    // Mapped out of the `sub` claim.
    string PipelineName = 3 [(gogoproto.jsontag) = "pipeline_name,omitempty"];
    // The ID of the AZDO pipeline. Example:
    // `271ef6f7-0000-0000-0000-4b54d9129990`
    // Mapped from the `prj_id` claim.
    string ProjectID = 4 [(gogoproto.jsontag) = "project_id,omitempty"];
    // The ID of the AZDO pipeline definition. Example:
    // `1`
    // Mapped from the `def_id` claim.
    string DefinitionID = 5 [(gogoproto.jsontag) = "definition_id,omitempty"];
    // The URI of the repository the pipeline is using. Example:
    // `https://github.com/gravitational/teleport.git`.
    // Mapped from the `rpo_uri` claim.
    string RepositoryURI = 6 [(gogoproto.jsontag) = "repository_uri,omitempty"];
    // The individual commit of the repository the pipeline is using. Example:
    // `e6b9eb29a288b27a3a82cc19c48b9d94b80aff36`.
    // Mapped from the `rpo_ver` claim.
    string RepositoryVersion = 7 [(gogoproto.jsontag) = "repository_version,omitempty"];
    // The reference of the repository the pipeline is using. Example:
    // `refs/heads/main`.
    // Mapped from the `rpo_ref` claim.
    string RepositoryRef = 8 [(gogoproto.jsontag) = "repository_ref,omitempty"];

  }
  // Allow is a list of TokenRules, nodes using this token must match one
  // allow rule to use this token.
  repeated Rule Allow = 1 [(gogoproto.jsontag) = "allow,omitempty"];
  // OrganizationID specifies the UUID of the Azure DevOps organization that
  // this join token will grant access to. This is used to identify the correct
  // issuer verification of the ID token.
  // This is a required field.
  string OrganizationID = 2 [(gogoproto.jsontag) = "organization_id"];
}
```

The Join Token Create/Edit UI should be extended to support this new join
method.

#### `tbot` configuration

Within `tbot` itself, no further configuration is required or will be added 
beyond:

- The user must specify the `azure_devops` join method
- The user must specify the name of the ProvisionToken to use.

However, when using `tbot` within a pipeline step, the user will need to map
the `System.AccessToken` variable into the environment, e.g:

```yaml
steps:
- env:
    SYSTEM_ACCESSTOKEN: $(System.AccessToken)
  script: tbot start -c ./my-config.yaml
```

#### Infrastructure as Code

The ProvisionToken resource is already generated for the Terraform provider and
the Kubernetes Operator, therefore the new fields to support the Azure DevOps
join method will be included without any additional work.

### Security Considerations

#### Token Reuse

One challenge with all OIDC join methods is the potential for token reuse.

In other join methods, we leverage a challenge-response flow with a nonce within
the `aud` claim to ensure that a token is only valid for a single join. However,
the `aud` claim in the ID Token issued by Azure DevOps is not configurable.

The Tokens do include a `jti` claim, containing a unique identifier for each
token, which we could use to reject the reuse of a token that has already been
used to join that particular Teleport Cluster, however, this does not mitigate
attacks where the token was used against a third-party service
(see Confused Deputy).

To some extent, this risk is mitigated by the unusually short (5m) TTL of the 
ID Token issued by Azure DevOps.

#### `aud` Confused Deputy

One challenge with the ID Tokens issued by Azure DevOps is that we cannot
specify a custom `aud` claim for inclusion in the ID Token.

This presents the risk of an ID Token intended for another service (or another
Teleport Cluster) being reused against our Teleport Cluster if that other
service has been compromised.

To some extent, this risk is mitigated by the unusually short (5m) TTL of the 
ID Token issued by Azure DevOps.

This risk would be mitigated by the use of a Service Connection ID Token (see
Alternatives: Use Service Connection JWTs rather than Pipeline JWTs) as users
could be encouraged to create a Service Connection per Teleport Cluster.

#### Audit Logging

As with any Bot join, the `bot.join` audit log will be emitted. This should
be extended to include the following information extracted from the ID token:

- `jti`
- `sub`
- `org_id`
- `prj_id`
- `def_id`
- `rpo_id`
- `rpo_uri`
- `rpo_ver`
- `repo_ref`
- `run_id`

### Alternatives 

#### Use Service Connection JWTs rather than Pipeline JWTs

Azure DevOps supports the creation of Service Connections. These are typically
used to hold secrets (e.g username/password) or to federate with Azure ARM.

It is also possible to generate an ID Token JWT for a specific Service
Connection rather than the pipeline itself. This JWT is signed by the same
issuer as the pipeline ID token.

This ID token has different claims:

```json
{
  "jti": "53042db8-a477-44c2-aca4-720bab67ad33",
  "sub": "sc://noahstride0304/testing-azure-devops-join/test-generic-sc",
  "aud": "api://AzureADTokenExchange",
  "iss": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "nbf": 1745851036,
  "exp": 1745851936,
  "iat": 1745851636
}
```

Notably:

- The service connection token shares the same issuer as the pipeline id token.
- The `sub` of the token identifies the organization, project and service 
  connection - but does not identify the pipeline.
- The token lacks the additional claims that exist in the pipeline token.

If we were to use service connection JWTs instead of pipeline JWTs, then we
would lose key information about the CI/CD run (e.g which pipeline, commit etc)
and users would only be able to control access to Teleport by restricting which
pipelines can access the service connection within Azure DevOps itself.

Additionally, to access the service connection token, there must be a step
within the pipeline which is a task with an input referencing the service token.
This is awkward to configure in Azure DevOps, and we'd need to publish a task
(similar to a GitHub Action) to make this simpler. This would be an additional
artifact to build, maintain and document.

### Out of Scope

#### Publish a Teleport Task

In a similar way to how GitHub Actions has publishable Actions, which are 
effectively off-the-shelf scripts, which can be used in workflows, Azure
DevOps has "Tasks" which can be published and used in pipelines.

See: https://learn.microsoft.com/en-us/azure/devops/extend/develop/add-build-task?toc=%2Fazure%2Fdevops%2Fmarketplace-extensibility%2Ftoc.json&view=azure-devops

As with GitHub actions, these tasks can be implemented in Typescript. Unlike
Typescript, a task can be configured via the GUI and can provide a guided
form with validation.

At a later date, we may wish to publish an Azure DevOps task that wraps the 
process of downloading, configuring and executing `tbot`. This would serve to 
simplify usage in a similar way to our GitHub Actions.

The following would be good indicators that we should publish a task:

- A significant uptake of the Azure DevOps join method indicated via usage
  analytics.
- A disproportionate amount of support tickets in relation to the Azure DevOps
  join method, indicating that the configuration is too cumbersome/complex.

It would be a requirement of the "Service Connection JWT" alternative
implementation to publish an action.

It should be noted that publishing an Azure Devops task would involve ongoing
maintenance to ensure that the latest versions of SDKs are used and security
vulnerabilities are patched.

### Research

#### Requesting OIDC ID Tokens from Azure DevOps

Sources:

- Notes from release of Workload Identity Federation: https://learn.microsoft.com/en-us/azure/devops/release-notes/2024/sprint-240-update#pipelines-and-tasks-populate-variables-to-customize-workload-identity-federation-authentication
- API reference: https://learn.microsoft.com/en-us/rest/api/azure/devops/distributedtask/oidctoken/create?view=azure-devops-rest-7.2&preserve-view=true

Similar to other join methods, we can fetch an ID Token from Azure DevOps 
using a special API:

```
OIDC_REQUEST_URL="${SYSTEM_OIDCREQUESTURI}?api-version=7.1"
echo $OIDC_REQUEST_URL
curl -s -H "Content-Length: 0" -H "Content-Type: application/json" -H "Authorization: Bearer $(System.AccessToken)" -X POST $OIDC_REQUEST_URL
```

The SYSTEM_OIDCREQUESTURI is an automagic environment variable provided by Azure
DevOps.

This yields a token with the following claims:

```json
{
  "jti": "90b75b0a-61b6-4b71-ba6f-11107d95f4c5",
  "sub": "p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
  "aud": "api://AzureADTokenExchange",
  "org_id": "0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "prj_id": "271ef6f7-5998-4b0f-86fb-4b54d9129990",
  "def_id": "1",
  "rpo_id": "strideynet/azure-devops-testing",
  "rpo_uri": "https://github.com/strideynet/azure-devops-testing.git",
  "rpo_ver": "c291ea713801eb300054d353d279e7b02331f671",
  "rpo_ref": "refs/heads/main",
  "run_id": "5",
  "iss": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "nbf": 1745839609,
  "exp": 1745840508,
  "iat": 1745840209
}
```

Notably:

- `sub` identifies organization, project and pipeline.
- `aud` is not specific to the upstream service connection.
- JWT has a validity window of 5 minutes.

Taking the `iss` claim of this JWT, we can find the OpenID well-known 
configuration document:

```
curl https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6/.well-known/openid-configuration
{
  "issuer": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "jwks_uri": "https://vstoken.dev.azure.com/.well-known/jwks",
  "subject_types_supported": [
    "public",
    "pairwise"
  ],
  "response_types_supported": [
    "id_token"
  ],
  "claims_supported": [
    "sub",
    "aud",
    "exp",
    "iat",
    "iss",
    "nbf"
  ],
  "id_token_signing_alg_values_supported": [
    "RS256"
  ],
  "scopes_supported": [
    "openid"
  ]
 }
```

From the OpenID configuration document, we can determine the following URL 
for validating the signature on the JWT: https://vstoken.dev.azure.com/.well-known/jwks

#### OIDC ID Tokens linked to Service Connections

You can also provide a `serviceConnectionId` parameter to the `oidctoken`
endpoint. This takes the UUID of a service connection. It produces a different
ID Token that references this service connection.

In order for this parameter to be usable, the pipeline must have a step that has
a task with an input that  references this service connection. If there is no
input that references this service connection, then it will return a Not Found
error. You cannot just provide the service connection UUID.

Additional notable facts:

- The step with an input that references the service connection does not
  necessarily need to be the step that requests the ID token.
- Providing the name of the service connection rather than the UUID will
  not return an error, but will silently fall back to providing the pipeline 
  ID token. One presumes that the API acts as if the parameter value has not
  been provided if the format is not a UUID.

It is not easy to provide an input without creating a custom task. For the
purposes of my testing, I leveraged 
https://marketplace.visualstudio.com/items?itemName=cloudpup.authenticated-scripts
to ensure an input referencing the service connection was present. Nothing
additional is performed by the custom task, it would appear merely referencing 
the service connection is sufficient. We would need to publish a custom task in
order to leverage this functionality.

```
OIDC_REQUEST_URL="${SYSTEM_OIDCREQUESTURI}?api-version=7.1&serviceConnectionId=abcd2db8-aaaa-bbbb-cccc-720bab6abcd"
echo $OIDC_REQUEST_URL
curl -s -H "Content-Length: 0" -H "Content-Type: application/json" -H "Authorization: Bearer $(System.AccessToken)" -X POST $OIDC_REQUEST_URL
```

The type of the service connection appears to impact the type of ID token that
is returned.

When using a Service Connection of the Generic type, we instead get the
following claims:

```json
{
  "jti": "53042db8-a477-44c2-aca4-720bab67ad33",
  "sub": "sc://noahstride0304/testing-azure-devops-join/test-generic-sc",
  "aud": "api://AzureADTokenExchange",
  "iss": "https://vstoken.dev.azure.com/0ca3ddd9-f0b0-4635-a98c-5866526961b6",
  "nbf": 1745851036,
  "exp": 1745851936,
  "iat": 1745851636
}
```

Example config:

```yaml
- task: AuthenticatedBash@1  
  inputs:
    serviceConnection: 'test-generic-sc'
    targetType: 'inline'
    script: |
      OIDC_REQUEST_URL="${SYSTEM_OIDCREQUESTURI}?api-version=7.1&serviceConnectionId=f0cda5b5-1aff-46ac-a80b-054d5c4f9b8a"
      echo $OIDC_REQUEST_URL
      echo $(System.AccessToken) | base64
      curl -s -H "Content-Length: 0" -H "Content-Type: application/json" -H "Authorization: Bearer $(System.AccessToken)" -X POST $OIDC_REQUEST_URL | base64
  displayName: 'Generic Service Connection w/ ID'
```

When using a Service Connection of the ARM Workload Identity Federation type, 
we instead get the following claims:

```json
{
  "aud": "fb60f99c-7a34-4190-8149-302f77469936",
  "iss": "https://login.microsoftonline.com/ff882432-09b0-437b-bd22-ca13c0037ded/v2.0",
  "iat": 1745842874,
  "nbf": 1745842874,
  "exp": 1745929574,
  "aio": "k2RgYFhfymJ2bqHRXNnP7EUL3urKJkfIGcb/EQ56GHzymVNEx00A",
  "azp": "499b84ac-1321-427f-aa17-267ca6975798",
  "azpacr": "2",
  "idtyp": "app",
  "oid": "9246576b-c9b6-441d-a0ca-796721fd971e",
  "rh": "1.AQQAMiSI_7AJe0O9IsoTwAN97Zz5YPs0epBBgUkwL3dGmTYEAQAEAA.",
  "sub": "/eid1/c/pub/t/MiSI_7AJe0O9IsoTwAN97Q/a/rISbSSETf0KqFyZ8ppdXmA/sc/0ca3ddd9-f0b0-4635-a98c-5866526961b6/64171687-7a59-4fce-b2c8-823278dcf176",
  "tid": "ff882432-09b0-437b-bd22-ca13c0037ded",
  "uti": "A3HaoRGwyEWyU-iJcDEFAA",
  "ver": "2.0",
  "xms_ficinfo": "CAAQARgAIAA",
  "xms_idrel": "7 30"
}
```

Notably, this ID token produced by the ARM WIF service connection is issued by
a different issuer - `https://login.microsoftonline.com`!

The format of these claims and the issuer for this service connection type
appears to have been recently modified:
https://learn.microsoft.com/en-us/azure/devops/release-notes/2025/sprint-253-update#workload-identity-federation-uses-entra-issuer
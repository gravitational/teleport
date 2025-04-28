---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 211 - Azure DevOps Joining

## Required Approvals

- Engineering: TimB && DanU
- Product: DaveS 

## What

Allow Bots & Agents to secretlessly authenticate to Teleport from Azure DevOps
pipelines.

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

### Background on Azure DevOps & Azure authentication

#### Service Connections

##### Research

https://learn.microsoft.com/en-us/azure/devops/release-notes/2024/sprint-240-update#pipelines-and-tasks-populate-variables-to-customize-workload-identity-federation-authentication

```json
https://dev.azure.com/noahstride0304/271ef6f7-5998-4b0f-86fb-4b54d9129990/_apis/distributedtask/hubs/build/plans/c738a23e-2307-4a6c-a406-f1f0f729387c/jobs/12f1170f-54f2-53f3-20dd-22fc7dff55f9/oidctoken?api-version=7.1&serviceConnectionId=test-generic-sc
```
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
```

From the OpenID configuration document, we can determine the following URL 
for validating the signature on the JWT: https://vstoken.dev.azure.com/.well-known/jwks

You can also provide a `serviceConnectionId` parameter to the `oidtoken`
endpoint. This takes the UUID of a service connection. This service connection
MUST be referenced by an input of the step (which requires a custom action and
cannot be done purely with Bash/Powershell) or it will return a Not Found error.

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

The format of these claims appears to have been recently modified: https://learn.microsoft.com/en-us/azure/devops/release-notes/2025/sprint-253-update#workload-identity-federation-uses-entra-issuer
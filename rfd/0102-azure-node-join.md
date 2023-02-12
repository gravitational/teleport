---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 102 - Azure join method

## Required Approvers

Engineering: @jakule && @r0mant
Product: @klizhentas && @xinding33
Security: @reedloden

## What

Teleport nodes running on Azure virtual machines will be able to join a
Teleport cluster without the need to share any secret token with the node.
Instead, the node will present an
[attested data document](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#attested-data)
and an access token generated from the
[instance metadata managed identity endpoint](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#managed-identity)
to confirm the subscription the VM is running in. With a configured Azure join
token on the auth server, the node will be allowed to join the cluster based
on its Azure subscription.

This is the Azure equivalent of EC2/IAM join as described in
[RFD 41](https://github.com/gravitational/teleport/blob/master/rfd/0041-aws-node-join.md).

## Why

This will make provisioning nodes on Azure simpler and arguably more secure.
There is no need to configure static tokens, or to manage the provisioning and
deployment of dynamic tokens.

## Details

In place of a join token, nodes will present a JWT access token and an attested
data document to the auth server, both of which will be fetched on the nodes
via instance metadata. The attested data request can include a user-specified
nonce which will also be signed. Like the custom headers in IAM join, the nonce
allows Teleport to issue a challenge (crypto random string of bytes) that the
node must include in the signed request. We can use gRPC bidirectional streams
to implement the following flow at a new `RegisterWithAzureToken` gRPC endpoint:

1. The Node initiates a `RegisterWithAzureToken` request.
2. The auth server enforces that TLS is used for transport.
3. The auth server sends a base64 encoded 24 byte crypto-random challenge.

- We use 24 bytes instead of the 32 used by the IAM method because the nonce
  parameter is limited to 32 characters, and 24 bytes require 32 base64 characters.

4. The node requests a) the attested data document using the challenge as the
   nonce and b) an access token. Azure signs and returns both.
5. The node sends a join request to the auth server, including the signed
   document, token, and other parameters in a `RegisterUsingTokenRequest`.
6. The auth server checks that attested data is valid:

- The document's pkcs7 signature is valid.
- The returned nonce matches the issued challenge.

7. The auth server checks that the access token is valid:

- The access token's signature is valid. The key set will be fetched with the
  OpenID Connect discovery mechanism.
- The access token was not issued before the start of the
  `RegisterWithAzureToken` request and hasn't expired.
- The access token and attested data document were produced by the same VM
  (i.e. their subscription ID, resource group, name, and VM ID match).
- The requested Teleport token allows the node to join. Teleport will get the
  subscription ID and resource group from the access token's `xms_mirid` claim.

8. The auth server uses the access token to fetch info about the VM from
   [the Azure API](https://learn.microsoft.com/en-us/rest/api/compute/virtual-machines/get?tabs=HTTP)
   and use the VM ID to confirm that the access token and attested data are from
   the same VM.
9. The auth server sends credentials to join the cluster.

As with IAM join, the flow will occur within a single streaming gRPC request and
only 1 attempt with a given challenge will be allowed, with a 1 minute timeout.

The Azure VM will need either a system- or user-assigned
[managed identity](https://learn.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/qs-configure-portal-windows-vm)
to be able to request an access token. The identity will need the
`Microsoft.Compute/virtualMachines/read` permission. If a VM has more than one
assigned identity, the identity to use must be specified in the join parameters.

### Signature verification and CAs

When verifying the signature of attested data, Teleport should accept the
following common names in certificates:

- \*.metadata.azure.com # Public
- \*.metadata.azure.us # Government
- \*.metadata.azure.cn # China
- \*.metadata.microsoftazure.de # Germany

Additionally, Teleport will pin any needed intermediate certificates at build
time, as it already does for EC2 join. More information on Azure certificate
authorities can be found in the
[Azure documentation](https://learn.microsoft.com/en-us/azure/security/fundamentals/azure-ca-details).

### Teleport Configuration

The existing provision token type can be extended to support Azure
authentication, using new Azure-specific fields in the token rules section.

```yaml
kind: token
version: v2
metadata:
  name: example_azure_token
spec:
  roles: [Node, Kube, Db]
  azure:
    allow:
      # Subscription from which nodes can join. Required.
      - azure_subscription: '22222222'

        # Resource groups from which nodes can join. If empty or omitted, nodes
        # from any resource group are allowed.
        azure_resource_groups: ['rg1', 'rg2']
```

teleport.yaml on the nodes should be configured so that they will use the Azure
join token:

```yaml
teleport:
  join_params:
    token_name: 'example_azure_token'
    method: azure
    azure:
      # client ID of the managed identity to use. Required if the VM has more
      # than one assigned identity.
      client_id: <client_id>
```

## Appendix I - Example attested data document

```json
{
  "encoding": "pkcs7",
  "signature": "MIIEEgYJKoZIhvcNAQcCoIIEAzCCA/8CAQExDzANBgkqhkiG9w0BAQsFADCBugYJKoZIhvcNAQcBoIGsBIGpeyJub25jZSI6IjEyMzQ1NjY3NjYiLCJwbGFuIjp7Im5hbWUiOiIiLCJwcm9kdWN0IjoiIiwicHVibGlzaGVyIjoiIn0sInRpbWVTdGFtcCI6eyJjcmVhdGVkT24iOiIxMS8yMC8xOCAyMjowNzozOSAtMDAwMCIsImV4cGlyZXNPbiI6IjExLzIwLzE4IDIyOjA4OjI0IC0wMDAwIn0sInZtSWQiOiIifaCCAj8wggI7MIIBpKADAgECAhBnxW5Kh8dslEBA0E2mIBJ0MA0GCSqGSIb3DQEBBAUAMCsxKTAnBgNVBAMTIHRlc3RzdWJkb21haW4ubWV0YWRhdGEuYXp1cmUuY29tMB4XDTE4MTEyMDIxNTc1N1oXDTE4MTIyMDIxNTc1NlowKzEpMCcGA1UEAxMgdGVzdHN1YmRvbWFpbi5tZXRhZGF0YS5henVyZS5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAML/tBo86ENWPzmXZ0kPkX5dY5QZ150mA8lommszE71x2sCLonzv4/UWk4H+jMMWRRwIea2CuQ5RhdWAHvKq6if4okKNt66fxm+YTVz9z0CTfCLmLT+nsdfOAsG1xZppEapC0Cd9vD6NCKyE8aYI1pliaeOnFjG0WvMY04uWz2MdAgMBAAGjYDBeMFwGA1UdAQRVMFOAENnYkHLa04Ut4Mpt7TkJFfyhLTArMSkwJwYDVQQDEyB0ZXN0c3ViZG9tYWluLm1ldGFkYXRhLmF6dXJlLmNvbYIQZ8VuSofHbJRAQNBNpiASdDANBgkqhkiG9w0BAQQFAAOBgQCLSM6aX5Bs1KHCJp4VQtxZPzXF71rVKCocHy3N9PTJQ9Fpnd+bYw2vSpQHg/AiG82WuDFpPReJvr7Pa938mZqW9HUOGjQKK2FYDTg6fXD8pkPdyghlX5boGWAMMrf7bFkup+lsT+n2tRw2wbNknO1tQ0wICtqy2VqzWwLi45RBwTGB6DCB5QIBATA/MCsxKTAnBgNVBAMTIHRlc3RzdWJkb21haW4ubWV0YWRhdGEuYXp1cmUuY29tAhBnxW5Kh8dslEBA0E2mIBJ0MA0GCSqGSIb3DQEBCwUAMA0GCSqGSIb3DQEBAQUABIGAld1BM/yYIqqv8SDE4kjQo3Ul/IKAVR8ETKcve5BAdGSNkTUooUGVniTXeuvDj5NkmazOaKZp9fEtByqqPOyw/nlXaZgOO44HDGiPUJ90xVYmfeK6p9RpJBu6kiKhnnYTelUk5u75phe5ZbMZfBhuPhXmYAdjc7Nmw97nx8NnprQ="
}
```

Decoded contents of the attested data document:

```json
{
  "licenseType": "",
  "nonce": "Yi09ymh-yIl4_zkmA6kIki4mDPpUlVxK",
  "plan": {
    "name": "",
    "product": "",
    "publisher": ""
  },
  "sku": "20_04-lts-gen2",
  "subscriptionId": "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy",
  "timeStamp": {
    "createdOn": "12/06/22 21:01:35 -0000",
    "expiresOn": "12/07/22 03:01:35 -0000"
  },
  "vmId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

## Appendix II - Example access token payload

```json
{
  "aud": "https://management.azure.com/",
  "iss": "https://sts.windows.net/ff882432-09b0-437b-bd22-ca13c0037ded/",
  "iat": 1671237372,
  "nbf": 1671237372,
  "exp": 1671324072,
  "aio": "...",
  "appid": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "appidacr": "2",
  "idp": "https://sts.windows.net/ff882432-09b0-437b-bd22-ca13c0037ded/",
  "idtyp": "app",
  "oid": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "rh": "...",
  "sub": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "tid": "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX",
  "uti": "...",
  "ver": "1.0",
  "xms_mirid": "/subscriptions/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/resourcegroups/example_group/providers/Microsoft.Compute/virtualMachines/example_vm",
  "xms_tcdt": "1434650756"
}
```

---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD X - Azure join method

## What

Teleport nodes running on Azure virtual machines will be able to join a Teleport cluster without the need to share any secret token with the node. Instead, the node will present a signed [attested data document](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#attested-data) to confirm the subscription the VM is running in. With a configured Azure join token on the auth server, the node will be allowed to join the cluster based on its Azure subscription.

This is the Azure equivalent of EC2/IAM join as described in [RFD 41](https://github.com/gravitational/teleport/blob/master/rfd/0041-aws-node-join.md).

## Why

This will make provisioning nodes on Azure simpler and arguably more secure.
There is no need to configure static tokens, or to manage the provisioning and
deployment of dynamic tokens.

## Details

In place of a join token, nodes will present a signed attested data document to the auth server. The document will be fetched on the nodes via instance metadata. The request can include a user-specified nonce which will also be signed. Like the custom headers in IAM join, the nonce allows Teleport to isue a challenge (crypto random string of bytes) that the node must include in the signed request. We can use gRPC bidirectional streams to implement the following flow at a new `RegisterWithAzureToken` gRPC endpoint:

1. The Node initiates a `RegisterWithAzureToken` request.
2. The auth server enforces that TLS is used for transport.
3. The auth server sends a base64 encoded 24 byte crypto-random challenge.
  - We use 24 bytes instead of 32 because the nonce parameter is limited to 32 characters, and 24 bytes require 32 base64 characters.
4. The node requests the attested data document using the challenge as the nonce. Azure returns the signed document.
5. The node sends a join request to the auth server, including the signed document and other paramaters in a `RegisterUsingTokenRequest`.
  - `RegisterUsingTokenRequest` will need to be extended to include info not in the attested data document (notably the VM's resource group and region).
6. The auth server checks that signed document is valid:
  - The document's pkcs7 signature is valid.
  - The returned nonce matches the issued challenge.
  - The requested token allows the node to join.
7. The auth server sends credentials to join the cluster.

As with IAM join, the flow will occur within a single streaming gRPC request and only 1 attempt with a given challenge will be allowed, with a 1 minute timeout.

### Teleport Configuration

The existing provision token type can be extended to support Azure authentication, using new Azure-specific fields in the token rules section.

```yaml
kind: token
version: v2
metadata:
  name: example_azure_token
spec:
  roles: [Node,Kube,Db]
  allow:
    # new token fields below

    # Subscription from which nodes can join. Required.
    - azure_subscription: "22222222"

      # Resource groups from which nodes can join. If empty or omitted, nodes
      # from any resource group are allowed.
      azure_resource_groups: ["rg1", "rg2"]
      
      # Regions from which nodes can join. If empty or omitted, nodes from any
      # region are allowed.
      azure_regions: ["r1", "r2"]

  # The duration after the attested data created time that the document will
  # be accepted. The default is 5 minutes.
  azure_attested_data_ttl: 5m
```

teleport.yaml on the nodes should be configured so that they will use the Azure join token:
```yaml
teleport:
  join_params:
    token_name: "example_azure_token"
    method: azure
```

## Appendix I - Example attested data document

```json
{
    "encoding":"pkcs7",
    "signature":"MIIEEgYJKoZIhvcNAQcCoIIEAzCCA/8CAQExDzANBgkqhkiG9w0BAQsFADCBugYJKoZIhvcNAQcBoIGsBIGpeyJub25jZSI6IjEyMzQ1NjY3NjYiLCJwbGFuIjp7Im5hbWUiOiIiLCJwcm9kdWN0IjoiIiwicHVibGlzaGVyIjoiIn0sInRpbWVTdGFtcCI6eyJjcmVhdGVkT24iOiIxMS8yMC8xOCAyMjowNzozOSAtMDAwMCIsImV4cGlyZXNPbiI6IjExLzIwLzE4IDIyOjA4OjI0IC0wMDAwIn0sInZtSWQiOiIifaCCAj8wggI7MIIBpKADAgECAhBnxW5Kh8dslEBA0E2mIBJ0MA0GCSqGSIb3DQEBBAUAMCsxKTAnBgNVBAMTIHRlc3RzdWJkb21haW4ubWV0YWRhdGEuYXp1cmUuY29tMB4XDTE4MTEyMDIxNTc1N1oXDTE4MTIyMDIxNTc1NlowKzEpMCcGA1UEAxMgdGVzdHN1YmRvbWFpbi5tZXRhZGF0YS5henVyZS5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAML/tBo86ENWPzmXZ0kPkX5dY5QZ150mA8lommszE71x2sCLonzv4/UWk4H+jMMWRRwIea2CuQ5RhdWAHvKq6if4okKNt66fxm+YTVz9z0CTfCLmLT+nsdfOAsG1xZppEapC0Cd9vD6NCKyE8aYI1pliaeOnFjG0WvMY04uWz2MdAgMBAAGjYDBeMFwGA1UdAQRVMFOAENnYkHLa04Ut4Mpt7TkJFfyhLTArMSkwJwYDVQQDEyB0ZXN0c3ViZG9tYWluLm1ldGFkYXRhLmF6dXJlLmNvbYIQZ8VuSofHbJRAQNBNpiASdDANBgkqhkiG9w0BAQQFAAOBgQCLSM6aX5Bs1KHCJp4VQtxZPzXF71rVKCocHy3N9PTJQ9Fpnd+bYw2vSpQHg/AiG82WuDFpPReJvr7Pa938mZqW9HUOGjQKK2FYDTg6fXD8pkPdyghlX5boGWAMMrf7bFkup+lsT+n2tRw2wbNknO1tQ0wICtqy2VqzWwLi45RBwTGB6DCB5QIBATA/MCsxKTAnBgNVBAMTIHRlc3RzdWJkb21haW4ubWV0YWRhdGEuYXp1cmUuY29tAhBnxW5Kh8dslEBA0E2mIBJ0MA0GCSqGSIb3DQEBCwUAMA0GCSqGSIb3DQEBAQUABIGAld1BM/yYIqqv8SDE4kjQo3Ul/IKAVR8ETKcve5BAdGSNkTUooUGVniTXeuvDj5NkmazOaKZp9fEtByqqPOyw/nlXaZgOO44HDGiPUJ90xVYmfeK6p9RpJBu6kiKhnnYTelUk5u75phe5ZbMZfBhuPhXmYAdjc7Nmw97nx8NnprQ="
}
```

Decoded contents of the attested data document:

```json
{
  "licenseType":"",
  "nonce":"Yi09ymh-yIl4_zkmA6kIki4mDPpUlVxK",
  "plan":{
    "name":"",
    "product":"",
    "publisher":""
  },
  "sku":"20_04-lts-gen2","subscriptionId":"yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy",
  "timeStamp":{
    "createdOn":"12/06/22 21:01:35 -0000",
    "expiresOn":"12/07/22 03:01:35 -0000"
  },
  "vmId":"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```
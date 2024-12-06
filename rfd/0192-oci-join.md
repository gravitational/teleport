---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 192 - Oracle cloud join method

## Required Approvers

* Engineering: @nklassen && @strideynet

## What

Add the ability for Teleport agents running on Oracle Cloud instances to join
a cluster without a static token.

## Why

This feature removes the need to use a shared secret to establish trust between
a Teleport cluster and an Oracle Cloud compute instance.

## Details

### Glossary

- **OCI** - Oracle Cloud Infrastructure. Interchangeable with Oracle Cloud in this document.
- **OCID** - Oracle Cloud Identifier. Unique ID associated with all Oracle Cloud resources.
- **Tenancy** / **Tenant** - Oracle equivalent of an AWS account/Azure subscription/etc.
- **Compartment** - Logical grouping of resources. Equivalent to an Azure resource group.

### UX

Suppose Alice is a system administrator with a Teleport cluster, as well as an
Oracle Cloud compute instance that she would like to add to the cluster. She
would first create the file `token.yaml` with the following contents:

```yaml
# token.yaml
kind: token
version: v2
metadata:
  name: oci-token
spec:
  roles: [Node]
  oracle:
    allow:
      - tenancy: "ocid1.tenancy.oc1..<unique ID>"  # the OCID for Alice's tenancy
        # If needed, Alice can further restrict the compartments and regions
        # instances can join from.
```

She would then create the provision token:

```sh
$ tctl create token.yaml
```

Next, Alice would install Teleport on her instance, then configure it:

```sh
$ teleport node configure --token oci-token --join-method oracle --proxy example.com
```

Finally, Alice would start Teleport on the instance. She can confirm that the
node has joined either in the web UI or by running `tsh ls`.

### Implementation

#### Token spec

The provision token will be extended to include a new `oracle` section:

```yaml
kind: token
version: v2
metadata:
  name: example-oci-token
spec:
  roles: [Node, Kube, Db]
  oracle:
    allow:
        # OCID of the tenancy to allow instances to join from. Required.
      - tenancy: "ocid1.tenancy.oc1..<unique ID>"
        # Compartments to allow instances to join from. Only the direct parent
        # compartment applies; i.e. nested compartments are not taken into account.
        # May be specified by name or by OCID. If compartments is empty or a wildcard,
        # instances can join from any compartment in the tenancy.
        parent_compartments: ["my_compartment", "ocid1.compartment.oc1...<unique_ID>"]
        # Regions to allow instances to join from. Both full names ("us-phoenix-1")
        # and abbreviations ("phx") are allowed. If regions is empty or
        # a wildcard, instances can join from any region.
        regions: ["phx", "us-ashburn-1"]
        # Add more entries as necessary.
      - tenancy: "..."
        parent_compartments: ["foo", "bar"]
        regions: ["baz", "quux"]
        # ...
```

#### Join process

When a node initiates the Oracle join method:

- The node first fetches credentials for its
[instance principal](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm#concepts)
via the Oracle instance metadata service.
- With those credentials, the node will create a
[signed HTTP request](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/signingrequests.htm)
to the Oracle Cloud API to
[fetch the compartment the instance is in](https://docs.oracle.com/en-us/iaas/api/#/en/identity/20160918/Compartment/GetCompartment),
at `https://iaas.{region}.oraclecloud.com/{api version}/compartment/{compartment's OCID}`.
The instance's principal does not need any additional permissions to make this request.
- The node will then make a `RegisterUsingToken` request to the auth server and
sends the URL and headers of the signed request as parameters (the auth server
will be able to recreate the rest of the request).

When the auth server receives a `RegisterUsingToken` request for the Oracle join method:

- The auth server extracts the key ID from the provided Authorization header. The key
ID is a string that Oracle uses to identify the caller. For instance principals,
the key ID is a JWT prefixed by the string `ST$` (unfortunately I could not
find docs that back this up, I found this experimentally).
- The auth server decodes the JWT and maps the claims `opc-tenant`,
`opc-compartment`, and `opc-instance` to the instance's tenancy ID, compartment
ID, and instance ID respectively.
- The auth server verifies that the compartment ID and region in the provided
API URL match the compartment ID and region from the claims (the region can be
extracted from the instance ID).
- The auth server reconstructs and performs the API request; if Oracle accepts
it, the auth server validates the tenancy ID, compartment ID (or name, found in
the API response), and region against the Teleport provision token. If that
passes, the node is allowed to join the cluster.

#### Limitations

The Oracle provision tokens will not support nested compartments, i.e. if
compartment `foo` has a child compartment `bar` and the provision token has
`parent_compartments: ["foo"]`, this will not allow instances in container `bar` to
join. This is for simplicity's sake; Teleport would need to make several
requests to the Oracle Cloud API to walk up the compartment tree from the
compartment the instance is in, each of which would need to be signed. This
would require a complicated back-and-forth between the auth server and the
joining node to get signed requests for each compartment.

### Security

To mitigate SSRF, Teleport will sanitize the claims in the key ID JWT provided by the instance
to ensure that they are
[properly formed OCIDs](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/identifiers.htm),
including verifying that the region in the instance OCID is valid.

To mitigate replay attacks, Teleport will verify that the JWT is not expired
and was not issued in the future (with a leeway of 2 minutes, the same as the
other JWT-based join methods). The Oracle API will also verify that the Date
header in the signed request is
[within 5 minutes](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/usingapi.htm#clock)
of its own clock.

Teleport will not verify signature of the JWT. This is because the needed JWKs will always be behind a
[non-public API](https://docs.oracle.com/en-us/iaas/Content/APIGateway/Tasks/apigatewayusingjwttokens.htm#identityprovider).
Instead, Teleport will trust the response from the Oracle Cloud API to know if
it can trust the signature.

### Proto Specification

Extend `RegisterUsingTokenRequest` to accept parameters needed for the signed API request:

```proto
message RegisterUsingTokenRequest {
    // Existing fields...

    OracleParams OracleParams = 15;
}

message OracleParams {
  string URL = 1;
  map<string, string> Headers = 2;
}
```

Extend provision tokens to include roles for joining Oracle instances:

```proto
message ProvisionTokenSpecV2 {
    // Existing fields...

    ProvisionTokenSpecV2Oracle Oracle = 17;
}

message ProvisionTokenSpecV2Oracle {
    message Rule {
        string Tenancy = 1;
        repeated string ParentCompartments = 2;
        repeated string Regions = 3;
    }

    repeated Rule Allow = 1;
}
```

### Audit Events

Tokens created with the `oracle` join method and instances joining via Oracle
tokens will be captured by the existing `ProvisionTokenCreate` and `InstanceJoin`
events, respectively.

### Backwards Compatibility

Suppose Oracle join is released in Teleport version *X*. The expected behavior
of agents with mixed versions is as follows:

|  | Auth <X | Auth >=X |
|---|---|---|
| Node <X | Irrelevant | Node will not launch with unrecognized join method |
| Node >=X | Join will be rejected for unrecognized join method | Join works |

### Test Plan

Add an entry to the test plan to verify that the Oracle join method works as
described in the docs, just like the other join methods.

### Future work

Cluster admins with many Oracle Cloud compartments may wish to specify the
allowed compartments to join from by their tags, rather than having to
specify each by name/OCID. The `oracle` section of the provision token
spec could be expended with the `compartment_tags` field to allow filtering
by defined and/or freeform tags. Since Teleport would already fetch the compartment
from the Oracle API, no extra permissions would be required.

## Appendix A: Sample JWT claims

```json
{
  "sub": "ocid1.instance.oc1.phx.<random string>",
  "opc-certtype": "instance",
  "iss": "authService.oracle.com",
  "fprint": "<fingerprint>",
  "ptype": "instance",
  "aud": "oci",
  "opc-tag": "V3,ocid1.tenancy.oc1..<random string>,AAAAAQAAAAAAAACB,AAAAAQAAAAAAhy9d",
  "ttype": "x509",
  "opc-instance": "ocid1.instance.oc1.phx.<random string>",
  "exp": 1732738022,
  "opc-compartment": "ocid1.compartment.oc1..<random string>",
  "iat": 1732736822,
  "jti": "<jwt id>",
  "tenant": "ocid1.tenancy.oc1..<random string>",
  "jwk": "{\"kid\":\"<fingerprint>\",\"n\":\"0BOIi1uIrzoyQmNmfsew8aRv1DVNx979QqD6WoZ37QTDkFuNoGUPssk_mftatqQUGbkppKAtXutb9lXO1SsEnyOv2_tN1KxBhiahtMdRoha0wchla2GJQd7zxVxjSU70ousmuHfIAr29P6jdx3zQ15WYG-MMRcKfB8FtETzEcTBJH9ujjw00LkBmQ_CJsJIq2YFWjp4HW8DlX2YER_FYy7Apq98Rqno0Ze4lBBib-HeJP2x7q0mxJoHEJlsRBdMweMRKhsFL5oKJjWaul06TBp4wuEx7Czcr427d5RZJ-cSCYCDkf8bzMhZ4K5o2cpKV3gcqXEDuH81_B4odZ4-oLQ\",\"e\":\"AQAB\",\"kty\":\"RSA\",\"alg\":\"RS256\",\"use\":\"sig\"}",
  "opc-tenant": "ocid1.tenancy.oc1..<random string>"
}
```

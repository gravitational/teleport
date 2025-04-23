---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 192 - Oracle cloud join method

## Required Approvers

* Engineering: @nklaassen && @strideynet

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

Suppose Alice is a system administrator with a Teleport cluster, and she wants
to add some Oracle Cloud compute instances to it. She
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
        parent_compartments: ["ocid1.compartment.oc1..<unique ID>"] # the OCID for Alice's compartment
        # If needed, Alice can further restrict the compartments and regions
        # instances can join from.
```

She would then create the provision token:

```sh
$ tctl create token.yaml
```

Next, Alice would install, configure, and start Teleport on all of her instances.
If Alice has not yet created her instances, she can set `user_data` in each
instance's metadata to add an init script for
(cloud-init)[https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm#].
Otherwise, she can run the following script locally to install Teleport on her
existing instances:

```sh
$ INSTANCE_IDS=$(oci compute instance list | jq -r '.data | map(.id) | join(" ")') # filter instances as needed
$ for INSTANCE_ID in $(echo $INSTANCE_IDS)
do
  oci instance-agent command create \
  --compartment-id <compartment-id> \
  --content '{"source": {"sourceType": "TEXT", "text": "curl https://cdn.teleport.dev/install.sh | bash -s <Teleport version> && \
  teleport node configure --token oci-token --join-method oracle --proxy example.com && \
  sudo systemctl start teleport"}}' \
  --target '{"instanceId": "$INSTANCE_ID"}'
done
```

She can confirm that the nodes have joined either in the web UI or by running `tsh ls`.

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
        # OCIDs of compartments to allow instances to join from. Only the direct parent
        # compartment applies; i.e. nested compartments are not taken into account.
        # If compartments is empty,
        # instances can join from any compartment in the tenancy.
        parent_compartments: ["ocid1.compartment.oc1...<unique_ID>"]
        # Regions to allow instances to join from. Both full names ("us-phoenix-1")
        # and abbreviations ("phx") are allowed. If regions is empty, instances can join from any region.
        regions: ["phx", "us-ashburn-1"]
        # Add more entries as necessary.
      - tenancy: "..."
        parent_compartments: ["foo", "bar"]
        regions: ["baz", "quux"]
        # ...
```

#### Permissions

Before the join process can begin, the nodes joining need permission to authenticate clients.

- Create a [dynamic group](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingdynamicgroups.htm)
that matches all the instances that will join Teleport.
- Create the following policy: `Allow dynamic-group <dynamic-group-name> to inspect authentication in tenancy`

As long as the criteria for which instances can join doesn't change, the group
and policy do not need to be updated for each new instance. We will recommend that
users configure the dynamic group matching rules to match their provision token
to limit unnecessary permissions.

#### Join process

When a node initiates the Oracle join method:

- The node starts a `RegisterUsingOracleMethod` grpc request to the auth server.
- The auth server generates a 32 byte challenge string and sends it to the node.
- The node fetches credentials for its
[instance principal](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm#concepts)
via the Oracle instance metadata service. Instances are guaranteed to have a principal
and always have access to the instance metadata service to fetch their credentials.
- The node will create a [signed HTTP request](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/signingrequests.htm)
to `http://127.0.0.1`. The address doesn't matter as the node will only use the
signed headers and never make the request.
- The node will create a second signed request, this time to
`https://auth.{region}.oraclecloud.com/v1/authentication/authenticateClient`,
and include the signed headers from the previous request as the payload (the
authenticateClient route is not documented in the Oracle docs, but the
[request](https://docs.oracle.com/en-us/iaas/api/#/en/identity-dp/v1/datatypes/AuthenticateClientDetails)
and [response](https://docs.oracle.com/en-us/iaas/api/#/en/identity-dp/v1/datatypes/AuthenticateClientResult)
types are).
The node will [include and sign](https://github.com/oracle/oci-go-sdk/blob/c696c320af82270e0a2fc5324600c4902b907ecc/example/example_identity_test.go#L51-L59)
the challenge from the auth server under the header `x-teleport-challenge`.
- The node sends the signed headers and the common token request parameters
to the auth server.
- The auth server extracts the instance's region from the signed headers to
reconstruct the `authenticateClient` URL (found in the `keyID` field of the
`Authorization` header, formatted as a JWT) and forwards the request to the
Oracle API and verifies that the request succeeds.
- The auth server maps the claims `opc-tenant`, `opc-compartment`, and `opc-instance`
from the authenticateClient response to the instance's tenancy ID, compartment
ID, and instance ID respectively.
- The auth server validates/verifies several properties:
  - The tenancy ID, compartment ID, and instance ID are all valid Oracle OCIDs.
  - The tenancy ID, compartment ID, and region match the Teleport provision token. 
  - The signed challenge matches.
- If everything above succeeds, the node is allowed to join the cluster.

#### Throttling

If the auth server is ever
[throttled by Oracle](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/usingapi.htm#throttle),
the TooManyRequests error will be propagated back to the node, which will try
`RegisterUsingOracleMethod` again with the exponential backoff recommended by
Oracle (maximum of 60 seconds).

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

To mitigate SSRF, Teleport will verify that the region provided by the joining
node is valid.

On top of the signed challenge, both Teleport and the Oracle API will
verify that the `X-Date` header in the signed request is
[within 5 minutes](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/usingapi.htm#clock)
of their own clocks.

### Proto Specification

Add `RegisterUsingOracleMethod` rpc to the join service:

```proto
message RegisterUsingOracleMethodRequest { 
  types.RegisterUsingTokenRequest register_using_token_request = 1;
  map<string,string> headers = 2;
}

message RegisterUsingOracleMethodResponse {
  oneof Response {
    string challenge = 1;
    Certs certs = 2;
  }
}

service JoinService {
  // ...
  rpc RegisterUsingOracleMethod(stream RegisterUsingOracleMethodRequest) returns (stream RegisterUsingOracleMethodResponse);
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
specify each by OCID. The `oracle` section of the provision token
spec could be expended with the `compartment_tags` field to allow filtering
by defined and/or freeform tags. Since Teleport would already fetch the compartment
from the Oracle API, no extra permissions would be required.

## Appendix A: Sample keyID JWT claims

```json
/* spell-checker: disable */
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
/* spell-checker: enable */
```

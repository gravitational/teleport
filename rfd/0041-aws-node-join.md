---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 41 - Simplified Node Joining for AWS

## What

Teleport nodes running on EC2 instances will be able to join a teleport cluster
without the need to share any secret token with the node.

Instead, the node will provide either a signed
[EC2 Instance Identity Document](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html)
or a signed
[sts:GetCallerIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetCallerIdentity.html)
request that will be used to confirm the AWS account that the EC2 instance is
running in. With a configured AWS join token on the auth server, the node
will be allowed to join the cluster based on its AWS account.

## Why

This will make provisioning nodes on EC2 simpler and arguably more secure.
There is no need to configure static tokens, or to manage the provisioning and
deployment of dynamic tokens.

## Details

We are currently exploring two ways to implement this, both inspired by [similar
designs](https://www.vaultproject.io/docs/auth/aws) used by Vault.

The first is the "EC2 Method", which uses signed EC2 Instance Identity Documents
which EC2 instances can fetch from the AWS
[IMDSv2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html).

The second is the "IAM Method", in which the EC2 instance will create and sign
an HTTPS API request to Amazon's public `sts:GetCallerIdentity` endpoint and
pass it to the auth server.

Update 2022-01-24: The EC2 Method was implemented first and released in Teleport
7.3, the IAM method is currently being implemented and will support Teleport
Cloud.

### EC2 Method

In place of a join token, nodes will present an EC2 IID (Instance Identity
Document) to the auth server, with a corresponding PKCS7 signature.

The IID and signature will be fetched on the nodes from the IMDSv2.

This will be sent along with the other existing parameters in a
[RegisterUsingTokenRequest](
https://github.com/gravitational/teleport/blob/d4247cb150d720be97521347b74bf9c526ae869f/lib/auth/auth.go#L1538-L1563).

The auth server will then:
1. Check that the `PendingTime` of the IID, which is the time the instance was
   launched, is within the configured TTL.
   - if the node fails to join the cluster during this window, the user can
     stop and restart the EC2 instance to reset the `PendingTime` (and
     effectively create a new IID with a new signature)
2. Check that the AWS join token matches the AWS `account` and `region` in the
   IID, and the requested Teleport service role.
3. Use the AWS DSA public certificate to check that the PKCS7 signature for the
   IID is valid.
   - The `region` field of the IID will be used to select the correct public
     certificate
     (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/verify-pkcs7.html).
4. Check that this EC2 instance has not already joined the cluster.
   - The node name will be set to `<aws_account_id>-<aws_instance_id>` so that
     the auth server can efficiently check the backend to see whether this
     instance has already joined.
   - This is to prevent replay attacks with the IID, any attempt by the same
     instance to join the cluster more than once will be blocked and logged in
     detail.
5. Check that the EC2 instance is currently running. \*

\* Step 5 requires AWS credentials on the Auth server with permissions for
`ec2:DescribeInstances`. If you need Nodes to be able to join the teleport
cluster from different AWS accounts than the one in which the Teleport Auth
server is running, there are a few extra required steps:
1. Every AWS account in which Nodes should be able to join will need an IAM
   role which can be assumed from the account of the Auth server. This role
   needs permissions for `ec2:DescribeInstances`.
2. The IAM role of the Auth server needs permissions to `sts:AssumeRole` for
   the ARN of every foreign IAM role from step 1. These ARNs will need to be
   configured in the `aws_role` field of the AWS join token.

#### Cloud Support

The design described above is not ideal for Teleport Cloud, as the auth server
would need permissions to access customer AWS accounts to check that the
instance is currently running.

To make this work, we could create a new type of teleport agent that the client
would run on their own infrastructure, similar to app and db access. The
customer would need to configure the `ec2:DescribeInstances` permissions for
that agent only. In order to check that the joining instance is currently
running, the Auth server would delegate the `ec2:DescribeInstances` call to the
agent running in the customer's account.

### IAM Method

In place of a join token, nodes will present a signed `sts:GetCallerIdentity`
request to the auth server.

The form of this request is an HTTP `POST` request to `sts.amazonaws.com` with
`Action=GetCallerIdentity` in the body. The request can include arbitrary
headers which will also be signed using the
[Signature Version 4 signing process](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html).
The signature will then be included in the `Authorization` header of the request.
See Appendix I for an example.

The actual creation and signing of the request will be done on the node with
the AWS SDK for Go.

Because the signed request can include arbitrary headers, this allows us to
issue a challenge (a crypto random string of bytes) that the node must include
in the signed request. We can make use of gRPC bidirectional streams to
implement the following flow at a new `RegisterWithAWSToken` gRPC endpoint:

1. The node initiates a `RegisterWithAWSToken` request.
2. The auth server enforces that TLS is used for transport.
3. The auth server sends a base64 encoded 32 byte crypto-random challenge.
4. The node creates the `sts:GetCallerIdentity` request, adds the challenge in
   a header, and signs it with its AWS credentials.
5. The node sends a join request to the auth server, including the signed
   `sts:GetCallerIdentity` request and other parameters currently in
   [RegisterUsingTokenRequest](
   https://github.com/gravitational/teleport/blob/d4247cb150d720be97521347b74bf9c526ae869f/lib/auth/auth.go#L1538-L1563)
6. The auth server checks that the signed request is valid:
   - it is a `POST` request to `sts.amazonaws.com`
   - the body is `Action=GetCallerIdentity&Version=2011-06-15`
   - the `X-Teleport-Challenge` header value equals the issued challenge
   - the Authorization header includes `x-teleport-challenge` as one of the signed headers
   - the Authorization header includes `AWS4-HMAC-SHA256` as the signing algorithm
7. The auth server sends the signed request to `sts.amazonaws.com` over TLS and gets the response.
8. The auth server checks for a provision token that matches the AWS account in
   the `sts:GetCallerIdentity` response and the teleport service role of the
   join request.
9. The auth server sends credentials to join the cluster.

The above steps all occur within a single streaming gRPC request and only 1
attempt with a given challenge will be allowed. The complete request will have
a 1 minute timeout.

In order to create the signed requests, the node will need to have access to
AWS credentials. As the `sts:GetCallerIdentity` request does not require any
explicit permissions, it is sufficient to attach *any* IAM policy to the EC2
instance.

Note that cross-account `sts:AssumeRole` access should be restricted for all
IAM roles in the AWS account to prevent possible attackers in other accounts
from pretending they are in the configured AWS account. To mitigate this,
administrators can restrict access to only a (set of) IAM roles(s) which cannot
be assumed from other accounts, see #Teleport Configuration.

### Comparison of EC2 and IAM methods

|                                                              | EC2 | IAM | Dynamic Token |
| ------------------------------------------------------------ | --- | --- | ------------- |
| Requires a secret token to be shared with every Node         |     |     | ✅ |
| Requires attached IAM role on Nodes                          |     | ✅  | |
| Requires attached IAM role on Auth                           | ✅  |     |
| Requires IAM role in every Node account which can be assumed by Auth account   | ✅  |     | |
| Requires IAM role in Auth account which can assume roles in every Node account | ✅  |     | |
| Could work with Teleport Cloud                               | ✅\* | ✅  | ✅ |
| Nodes can sign a challenge to prevent replays                |     | ✅  | |
| Requires outbound access to public internet from Auth        | ✅  |  ✅ | |
| Requires outbound access to public internet from Nodes       |     |     | |

### Teleport Configuration

The existing provision token type can be extended to support aws
authentication. Token "rules" will be added to define which AWS accounts and
IAM roles to accept nodes from.

Support for `tctl create token.yaml`, `tctl get tokens`, and
`tctl rm tokens/example_token` will need to be added so that administrators can
create and modify tokens from a yaml definition.

```yaml
kind: token
version: v2
metadata:
  # `name` is currently used for the token value of dynamic tokens. When creating an
  # AWS token, administrators can set any value for the `name`. The `name` is safe
  # to share widely, as use of this token will be restricted by the `allow` rules if
  # any are present.
  name: example_aws_token
spec:
  roles: [Node,Kube,Db]

  # existing token fields above
  # new token fields below

  # `allow` is a list of token rules. If any exist, the joining node must match at least one of them.
  allow:
  # Each token rule must contain `aws_account`, specifying the account nodes can join from
  - aws_account: "111111111111"
    # If the EC2 method is chosen, `aws_role` is necessary and should be the ARN
    # of the role that the auth server will need to assume in order to call
    # `ec2:DescribeInstances` for this account. In a single AWS account setup,
    # this should be the role of the auth server (no other role needs to be
    # assumed.)
    # If the IAM method is chosen, `aws_role` would be optional but could be used
    # to restrict the role of joining nodes.
    aws_role: "arn:aws:iam::111111111111:role/example-role"
    # `aws_regions` is only applicable for the EC2 method, and restricts the
    # regions from which a node can join. An empty or omitted list will allow
    # any region
    aws_regions: ["us-west-2", "us-east-1"]
  # Multiple token rules can be listed
  - aws_account: "222222222222"
    aws_role: "arn:aws:iam::222222222222:role/example-role"
    aws_regions: ["us-gov-east-1"]
  - aws_account: "333333333333"
    aws_role: "arn:aws:iam::333333333333:role/other-example-role"

  # `aws_iid_ttl` is the duration after the IID PendingTime (the time at which
  # the EC2 instance launched) that the IID will be accepted. The default is
  # 5 minutes.
  aws_iid_ttl: 5m
```

teleport.yaml on the nodes should be configured so that they will use the new aws join token:
```yaml
teleport:
  # `join_params` should be used on nodes which will join the cluster with the
  # new aws join method, in place of `auth_token`. It is a dict rather than a
  # scalar so that it can be extended in the future (e.g. to choose EC2 or IAM
  # method if we ever implement both)
  join_params:
    token_name: "example_aws_token" # should match the name of the token on the auth server
    method: ec2
```

## Appendix I - Example Signed `sts:GetCallerIdentity` Request and Response

`sts:GetCallerIdentity` Request
```
POST / HTTP/1.1
Host: sts.amazonaws.com
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=XXXXXXXXXXXXXXXXXXXX/20210614/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date, Signature=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Length: 43
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20210614T014047Z

Action=GetCallerIdentity&Version=2011-06-15
```
Response:
```
{
    "GetCallerIdentityResponse":
        "GetCallerIdentityResult": {
            "Account":"111111111111",
            "Arn":"arn:aws:iam::111111111111:assumed-role/test-role/i-RRRRRRRRRRRRRRRRRR",
            "UserId":"AAAAAAAAAAAAAAAAAAAAA:i-RRRRRRRRRRRRRRRRRR"
        },
        "ResponseMetadata":{
            "RequestId":"4464f2b3-36ba-4dd5-b0a7-e9c4fbd7b568"
        }
    }
}
```

## Appendix II - Example Instance Identity Document

```json
{
  "accountId" : "111111111111",
  "architecture" : "x86_64",
  "availabilityZone" : "us-west-2a",
  "billingProducts" : null,
  "devpayProductCodes" : null,
  "marketplaceProductCodes" : null,
  "imageId" : "ami-00000000000000000",
  "instanceId" : "i-01111111111111111",
  "instanceType" : "t2.micro",
  "kernelId" : null,
  "pendingTime" : "2021-06-11T00:08:27Z",
  "privateIp" : "10.0.0.56",
  "ramdiskId" : null,
  "region" : "us-west-2",
  "version" : "2017-09-30"
}
```

The pkcs7 signature is seperate from the document and looks like:
```
MIAGCSqGSIb3DQEHAqCAMIACAQExCzAJBgUrDgMCGgUAMIAGCSqGSIb3DQEHAaCAJIAEggHZewog
ICJhY2NvdW50SWQiIDogIjI3ODU3NjIyMDQ1MyIsCiAgImFyY2hpdGVjdHVyZSIgOiAieDg2XzY0
IiwKICAiYXZhaWxhYmlsaXR5Wm9uZSIgOiAidXMtd2VzdC0yYSIsCiAgImJpbGxpbmdQcm9kdWN0
cyIgOiBudWxsLAogICJkZXZwYXlQcm9kdWN0Q29kZXMiIDogbnVsbCwKICAibWFya2V0cGxhY2VQ
cm9kdWN0Q29kZXMiIDogbnVsbCwKICAiaW1hZ2VJZCIgOiAiYW1pLTA4MDBmYzBmYTcxNWZkY2Zl
IiwKICAiaW5zdGFuY2VJZCIgOiAiaS0wMjg1Yjc2ZGJjOGY3NWNlNiIsCiAgImluc3RhbmNlVHlw
ZSIgOiAidDIubWljcm8iLAogICJrZXJuZWxJZCIgOiBudWxsLAogICJwZW5kaW5nVGltZSIgOiAi
MjAyMS0wNi0xMVQwMDowODoyN1oiLAogICJwcml2YXRlSXAiIDogIjEwLjAuMC41NiIsCiAgInJh
bWRpc2tJZCIgOiBudWxsLAogICJyZWdpb24iIDogInVzLXdlc3QtMiIsCiAgInZlcnNpb24iIDog
IjIwMTctMDktMzAiCn0AAAAAAAAxggEXMIIBEwIBATBpMFwxCzAJBgNVBAYTAlVTMRkwFwYDVQQI
ExBXYXNoaW5ndG9uIFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYDVQQKExdBbWF6b24gV2Vi
IFNlcnZpY2VzIExMQwIJAJa6SNnlXhpnMAkGBSsOAwIaBQCgXTAYBgkqhkiG9w0BCQMxCwYJKoZI
hvcNAQcBMBwGCSqGSIb3DQEJBTEPFw0yMTA2MTEwMDA4MzBaMCMGCSqGSIb3DQEJBDEWBBSj4yyW
kvOVpx656w8wuhjUyH9dWjAJBgcqhkjOOAQDBC4wLAIUBOSAnp6M57kAXXJoj2s8cb32AXwCFFHm
egSjvG+KBmmOgUyS15ZzFJJTAAAAAAAA
```

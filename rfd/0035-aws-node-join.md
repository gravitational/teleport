---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 35 - Simplified Node Joining for AWS

## What

EC2 instances will be able to join a teleport cluster without explicitly
provisioning a join token on the auth server.

In place of a join token, nodes will present one or more signed AWS API
requests to the auth server. The auth server will send these requests to the
public AWS API to confirm that:
1. Because the AWS API accepted the signed request, the node holds valid AWS
   credentials with permissions for the relevant API endpoints.
2. Based on the API response, the node's AWS credentials belong to the
   configured AWS Organization and/or Account.

### tl;dr

Configure auth server to allow nodes to join with the new aws join method:
```yaml
teleport:
  nodename: auth

auth_service:
  enabled: yes
  aws_join:
    allow:
    - organization: "o-1111111111"
```

Create ec2 instances in organization "o-1111111111" with a role with the
following attached policy:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "organizations:DescribeOrganization"
            ],
            "Resource": "*"
        }
    ]
}
```

Configure teleport nodes on ec2 instances:
```yaml
teleport:
  auth_servers:
    - auth
  aws_token:
    enabled: true

ssh_service:
  enabled: yes
```

Nodes will be authenticated with the new aws join method and be able to join
the cluster.

## Why

This will make provisioning nodes on ec2 simpler and arguably more secure.
There is no need to configure static tokens, or to manage the provisioning and
deployment of dynamic tokens.

## Details

Much inspiration is taken from Vault's [IAM auth method](https://www.vaultproject.io/docs/auth/aws#iam-auth-method).

### Authentication

In place of a join token, nodes will present two signed AWS API requests to the
auth server: `sts:GetCallerIdentity` and `organizations:DescribeOrganization`

These are signed HTTP requests which will be serialized and sent to the auth
server over GRPC.

The auth server will then:
1. Check that the request URL is a valid AWS API endpoint.
2. Send each request to the AWS API.
  - AWS will check the signature and return an error if it is invalid, in which case the node will be rejected.
3. Extract the Account from the `sts:GetCallerIdentity` response
4. Extract the Organization from the `organizations:DescribeOrganization` response
5. Check the Organization and Account against the configured deny rules. Reject the node if any match.
6. Check the Organization and Account against the configured allow rules. Reject the node if none match.
7. Possibly extra checks to prevent replay attacks, see the following section.

By presenting signed AWS API requests, the node proves that it has access to
AWS credentials. We rely on AWS to verify the signatures.

The API responses prove the Organization and Account of the credentials
used to sign the requests. We check that the request is a real AWS API
endpoint, and rely on AWS to return the real identity for the signed
requests.

Nodes never share their AWS credentials, only signed requests.

### Mitigating the Risk of Replay Attacks

Signed API requests will only be held in memory and never logged or written to
disk. All transport of signed API requests will be over TLS.

We can include arbitrary headers in the signed API request and AWS will verify
the signature over those headers. We can use this to include a timestamp on the
request and enforce a TTL. Amazon does this by default with a TTL of 15
minutes.  We could set a shorter TTL ourselves, or make it configurable.

We could also include a UUID header, and use this to verify that each signed
request is only used once by having the auth server store the UUID of requests
that have been used recently and still have a valid TTL. With multiple auth
servers this would require some backend coordination.

### Teleport Configuration

Support for new configuration options will be added to Teleport's configuration
file in order to enable and configure this feature.

```yaml
teleport:

  # This section should be used on nodes which will join the cluster with the
  # new aws join method, in place of auth_token.
  aws_token:
    enabled: true

auth_service:

  # This section should be used on auth servers which will allow nodes to
  # join the cluster with the new aws join method.
  aws_join:
    # Deny rules will be checked first. If any deny rule matches an incoming
    # node, it will be rejected.
    deny:
    - organization: "" # if organization is empty or omitted it matches any org
      account: "" # if account is empty or omitted it matches any account

    # Allow rules will be checked after deny rules. Incoming nodes will be
    # accepted if they match any allow rule.
    allow:
    - organization: "" # if organization is empty or omitted it matches any org
      account: "" # if accounts is empty or omitted it matches any account

    # Example:
    allow:
    - account: "2222222222" # allow any node from this account
    - organization: "o-1111111111" # allow any node in any account this org
    deny:
    - account: "3333333333" # this specific account in org "o-1111111111" should be rejected

    # In theory we could add support for more claims, like "role" or "AMI",
    # to be combined with the initially supported "organization" and "identity" claims
```

### AWS Configuration

In all AWS accounts where nodes using the new aws join method will be
deployed, you must create an IAM role with the following attached
policy. All EC2 instances which will run Teleport nodes using the `ec2`
credential source must be launched with this IAM role.
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sts:GetCallerIdentity",
                "organizations:DescribeOrganization"
            ],
            "Resource": "*"
        }
    ]
}
```

Notably, the credentials need only be accessible from the Teleport nodes, not
the auth server. All requests will be signed on the nodes and the auth server
will only forward them to public AWS endpoints. The auth server does not even
need to run on AWS.

### Alternatives

I originally looked into doing something like Vault's [EC2 auth method](https://www.vaultproject.io/docs/auth/aws#ec2-auth-method).

At a high level, instead of a join token or signed API requests, the node would
present it's signed AWS Instance Identity Document. We could check the
signature on this and the instance details to confirm the node's AWS account.

The drawback of this method is that there is no information about the AWS
Organization in the Identity Document, and being able to accept nodes from a
given Organization rather than explicitly listing every account is a
requirement for this feature.

It would be possible to combine the EC2 method with requests to Amazon's API to
get the Organization for a given Account, but the required endpoints
(`organizations:DescribeAccount` or `organizations:ListAccounts`) can only be
called from the organization's management account. At a minimum, this would
still require creating signed requests on the node and sending them to the auth
server as the current design does.

## Appendix I - Example Signed Requests and Responses

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

`organizations:DescribeOrganization` Request:
```
Host: organizations.us-east-1.amazonaws.com
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=XXXXXXXXXXXXXXXXXXXX/20210614/us-east-1/organizations/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-target, Signature=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Length: 2
Content-Type: application/x-amz-json-1.1
X-Amz-Date: 20210614T014047Z
X-Amz-Target: AWSOrganizationsV20161128.DescribeOrganization
```
Response:
```
{
    "Organization": {
        "Arn":"arn:aws:organizations::222222222222:organization/o-1111111111",
        "AvailablePolicyTypes":[
            {
                "Status":"ENABLED",
                "Type":"SERVICE_CONTROL_POLICY"
            }
        ],
        "FeatureSet":"ALL",
        "Id":"o-1111111111",
        "MasterAccountArn":"arn:aws:organizations::222222222222:account/o-1111111111/222222222222",
        "MasterAccountEmail":"ops@example.com",
        "MasterAccountId":"222222222222"
    }
}
```

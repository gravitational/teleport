---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 227 - EC2 Auto Discovery by AWS Organization

## Required Approvers

* Engineering: @r0mant &&
* Product: @r0mant

## What

Auto enroll EC2 instances from an AWS Organization without enumerating account ids.

## Why

Teleport auto discovers EC2 instances and installs `teleport` into them.
Instances are configured to join the cluster, and Teleport users can access them.

Setting up and configuring the auto discover mechanism requires an entry per AWS account.
For deployments with a high volatility of AWS Account IDs, this requires operators to constantly change the configuration.

There's also an UX scale issue, where the same user can have thousands of AWS Account IDs.

Being able to configure the auto discover once, and have it auto discover the account IDs will greatly benefit the UX of this feature.

## Details

Usually, all the accounts in the scope of a deployment are under the same [AWS Organization](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_introduction.html).

Using [AWS Organization API](https://docs.aws.amazon.com/organizations/latest/APIReference/API_ListAccounts.html), the discover flow will be able to enumerate all the AWS Account IDs and run the discover flow on all of them.

For the EC2 instances to join the cluster, they usually use an [IAM Join Token](https://goteleport.com/docs/enroll-resources/agents/aws-iam/).
Currently, it only supports the AWS Account ID parameter as validation.
This join method will be updated to allow onboarding EC2 instances which belong to a given AWS Organization.

### UX

#### User stories

**Self-hosted: Alice manages a multi-account AWS organization and wants to access all EC2 instances from Teleport**

Alice follows the [Server Auto-Discovery for Amazon EC2](https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/) guide to set up SSH access.

When creating the EC2 invite token (step 1), an AWS Organization is defined instead of enumerating the allowed account ids.
This will allow EC2 instances from that organization to join Teleport.

Optionally, an AWS Assumed Role ARN can also be specified to increase security.

```yaml
kind: token
version: v2
metadata:
  name: aws-discovery-iam-token
spec:
  roles: [Node]
  join_method: iam
  allow:
  - aws_organization_arn: "arn:aws:organizations::123456789012:organization/o-abcde12345"
    aws_arn: "arn:aws:sts::*:assumed-role/<role name that must exist in all AWS Accounts>/i-*" # optional
```

The IAM Role required for the Discovery Service is different and will only require access to assume other roles (in its own account and other accounts):

<details>
  <summary>IAM Role for Discovery Service</summary>

  Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "organizations:ListAccounts",
                    "sts:AssumeRole"
                ],
                "Resource": [
                    "*"
                ]
            }
        ]
    }
  ```
  
  Trust Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }
  ```
</details>

The user also needs to create an IAM Role in each Account, which will be assumed by the Role assigned to the Discovery Service (Role above):

<details>
  <summary>IAM Role to create in every Account</summary>

  Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "ec2:DescribeInstances",
                    "ssm:DescribeInstanceInformation",
                    "ssm:GetCommandInvocation",
                    "ssm:ListCommandInvocations",
                    "ssm:SendCommand"
                ],
                "Resource": [
                    "*"
                ]
            }
        ]
    }
  ```
  
  Trust Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "AWS": "<Role's ARN assigned to the Discovery Service>"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }
  ```
</details>

Still in step 2, the Alice is asked to add the `AmazonSSMManagedInstanceCore` IAM managed policy to the target EC2 instances.
Alongside this policy, the user must also include the following:
<details>
  <summary>IAM Role to create in each Account</summary>

  Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "organizations:DescribeOrganization"
                ],
                "Resource": [
                    "*"
                ]
            }
        ]
    }
  ```
  
  Trust Policy:
  ```json
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }
  ```
</details>

Going to step 3, Alice is now asked to create an SSM Document in every Account and every Region with target EC2 instances.

When configuring Teleport to discover EC2 instances, the `teleport.yaml` will include all the accessible accounts:
```yaml
version: v3
# ...
discovery_service:
  enabled: true
  aws:
   - types: ["ec2"]
     account_ids: ["*"] # or be explicit about the organization id instead (?)
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
        external_id: "<optional>" 
     # ...
```

The Discovery Service must run in the Organization's management account or in a member account that is the delegated administrator.
This limitation comes from the [`organization.ListAccounts` API](https://docs.aws.amazon.com/organizations/latest/APIReference/API_ListAccounts.html):
> This operation can be called only from the organization's management account or by a member account that is a delegated administrator.

EC2 instances that exist in the Accounts under the organization specified will join the cluster.

### Auto Discover EC2 instances in an AWS Organization

Two flows must be changed in order to achieve the RFD goal:
- EC2 server discovery must be able to enumerate all the account ids and install teleport into them
- IAM Join flow must accept instances from any account of a given organization

#### EC2 Server Discovery: install teleport in all the accounts of a given organization

When processing the enrollment rule, the Discovery Service must check whether 

#### IAM Join: allow instances from any account of a given AWS Organization

### Teleport IAM Join based on AWS Organization

In order for an instance to join Teleport using the IAM Join method, the instance signs an `sts.GetCallerIdentity` request and sends it to the Auth service.

The Auth then performs some validations, and issues the request itself against AWS APIs.
The response contains the Account ID, and that's how Teleport ensures only instances whose account id is the same as the IAM Join Token, are accepted.

This RFD aims to add a small change to this flow: allow validation to be based on the AWS Organization.

... ...

### Proto Specifications


#### New matcher field for EC2 Auto Discover

```proto
// AWSMatcher matches AWS EC2 instances and AWS Databases
message AWSMatcher {
  // Types are AWS database types to match, "ec2", "rds", "redshift", "elasticache",
  // or "memorydb".
  repeated string Types = 1 [(gogoproto.jsontag) = "types,omitempty"];
  // Regions are AWS regions to query for databases.
  repeated string Regions = 2 [(gogoproto.jsontag) = "regions,omitempty"];
  // AssumeRoleARN is the AWS role to assume for database discovery.
  AssumeRole AssumeRole = 3 [(gogoproto.jsontag) = "assume_role,omitempty"];
  // ... other existing fields
  // Accounts contain the list of AWS Account IDs to use when assuming an IAM Role.
  // AssumeRole must be present and have the account id part set to *.
  // Using an wildcard (*) as the account id will enumerate all the Account IDs within the AWS Organization.
  // Only EC2 type is supported.
  repeated string Accounts = 10 [(gogoproto.jsontag) = "accounts,omitempty"];
}
```

#### New field in IAM Join method

```proto
// RegisterUsingIAMMethodRequest is a request for registration via the IAM join
// method.
message RegisterUsingIAMMethodRequest {
  // RegisterUsingTokenRequest holds registration parameters common to all
  // join methods.
  types.RegisterUsingTokenRequest register_using_token_request = 1;
  // StsIdentityRequest is a signed HTTP request to the AWS
  // sts:GetCallerIdentity API endpoint used to prove the AWS identity of a
  // joining node. It must include the challenge string as a signed header.
  bytes sts_identity_request = 2;
  // OrganizationsDescribeOrganizationRequest is a signed HTTP request to the AWS
  // organizations:DescribeOrganization API endpoint used to prove the AWS organization of a
  // joining node. It must include the challenge string as a signed header.
  bytes organizations_describe_organization_request = 3;
}
```

### Security
The main concern is the new join method: IAM Join method using an Organization.

Validating the output of `sts.GetCallerIdentity` or the output of `organizations.DescribeOrganization` should have equivalent security implications.

The flow is similar, and the invariants are kept:
- EC2 instance builds another signed request using the ambient credentials
- sends both requests over gRPC to the Auth Server
- Auth Server sends them to the AWS API
- Auth Server will also validate the assumed role, if the IAM Join Token has the field set

### Backwards Compatibility

**Teleport version installed in target EC2 instance is old and does not send the DescribeOrganization request**

It might happen that the version that is installed in the EC2 instance is too old and does not yet have the logic to send the `DescribeOrganization` signed request.

In this case, the Auth Service will reject the join attempt because it cannot validate the organization.

**EC2 instance sends DescribeOrganization request, but Auth Service was not updated**

Auth Service will reject the join attempt because it 

### Audit Events

The `ssm.run` audit event must include the organization arn.

### Test Plan

Include a new testing item in EC2 Discovery section:
- discover all instances from the organization's management account

Include a new testing item in IAM Join method:
- join an instance using the organization in the allow rules

-----------
# Notes


### How can Auth decide whether the account id belongs to the organization?
#### `organizations.DescribeAccount`
`organizations.DescribeAccount(account_id:string) -> ... org ...`

Can only be called by the organization's management account or by a member account that is a delegated administrator.

#### `organizations.ListAccounts`
`organizations.ListAccounts -> ... account ids ...`

Can only be called by the organization's management account or by a member account that is a delegated administrator.

#### `organizations.ListAccountsForParent`
`organizations.ListAccountsForParent(id) -> ... account ids ...`

Can only be called by the organization's management account or by a member account that is a delegated administrator.

#### `organizations.ListParents`
`organizations.ListParents(child) -> ... account ids ...`

Can only be called by the organization's management account or by a member account that is a delegated administrator.


### Possible flows for org validation

#### Option A
agent sends the `sts.GetCallerIdentity` signed request
auth validates the caller identity
auth calls `organizations.DescribeAccount(account_id)` and ensures it is a valid org (validating against the iam token's aws org)

pro:
- no agent changes

cons:
- auth needs AWS credentials and must be able to access the org's management account
 - this either comes from the env creds for self-hosted deployments
 - or from an aws integration (for cloud tenants)

#### Option B
agent needs access to `organizations.DescribeOrganization` and will send a signed request to auth
auth performs the request against aws APIs and validates the organization arn against the one in the iam token

pro:
- no extra credentials in auth

cons:
- each EC2 instance must be able to call `organizations.DescribeOrganization`
  - for plain IAM Join: we now need a specific API, instead of the generic IAM Role which might not even have any Policy assigned
  - for the discover flow: we now need another API, besides the ones in the `AmazonSSMManagedInstanceCore` managed policy

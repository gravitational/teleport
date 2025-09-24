---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 227 - EC2 Auto Discovery by AWS Organization

## Required Approvers

* Engineering: @r0mant
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

When creating the EC2 invite token (step 1), an AWS Organization is defined instead of enumerating all the account ids.
This will allow any EC2 instance, from an account belonging to that organization, to join the cluster.

```yaml
kind: token
version: v2
metadata:
  name: aws-discovery-iam-token
spec:
  roles: [Node]
  join_method: iam
  allow:
  - aws_organization_id: o-abcde12345
```

The IAM Role required for the Discovery Service is different and will only require access to list accounts and assume other roles:

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
Alongside this policy, the user must also include the `organizations:DescribeOrganization` action:
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

Going to step 3, Alice is now asked to create an SSM Document in every Account and Region that have EC2 instances.

On Step 4, Alice must deploy a teleport Discovery Service in the organization's management account, with the following configuration:
```yaml
version: v3
# ...
discovery_service:
  enabled: true
  aws:
   - types: ["ec2"]
     account_ids: ["*"] # new field
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
        external_id: "<optional>" 
     # ...
```

The Discovery Service must run in the Organization's management account or in a member account that is the delegated administrator.
This limitation comes from the [`organization.ListAccounts` API](https://docs.aws.amazon.com/organizations/latest/APIReference/API_ListAccounts.html):
> This operation can be called only from the organization's management account or by a member account that is a delegated administrator.

EC2 instances that exist in the Accounts under the organization specified will join the cluster, and users can access them from Teleport.

### Access EC2 instances of a given AWS Organization

Two flows must be changed in order to achieve the RFD goal:
- EC2 server discovery must be able to enumerate all the account ids and install teleport into them
- IAM Join flow must accept instances from any account of a given organization

#### EC2 Server Discovery: install teleport in all the EC2 instances of a given organization

When discovering EC2 instances to install teleport into, the Discovery Service will call [`organizations.ListAccounts`](https://docs.aws.amazon.com/organizations/latest/APIReference/API_ListAccounts.html).
This API returns all the Accounts under the current organization.

Using the Assume Role feature, which already exists, the service will assume a role in each AWS Account and run the same process it has today.

```go
func (f *ec2InstanceFetcher) GetInstances(ctx context.Context, rotation bool) ([]Instances, error) {
  // ...
  var assumeRoles []*types.AssumeRole
  accountList, err := organizationsClient.ListAccounts(ctx, &organizations.ListAccountsInput{})
  for _, account := range accountList.Accounts {
    assumeRoleARNParts.AccountID = aws.ToString(account.Id)

    assumeRoles = append(assumeRoles, &types.AssumeRole{
      RoleARN:    assumeRoleARNParts.String(),
      ExternalID: f.AssumeRole.ExternalID,
    })
  }

  // ...
  for _, assumeRole := range assumeRoles {
    ec2Client, err := f.EC2Getter(ctx, f.Region, assumeRole, f.AWSClientsOpts...)
    if err != nil {
      return nil, trace.Wrap(err)
    }

    // use ec2Client to call ec2.DescribeInstances and submit instances
    // afterwards, ssm.SendCommand will be used to install teleport into them
  }
}
```

### Teleport IAM Join based on AWS Organization

In order for an instance to join Teleport using the IAM Join method, the instance signs an `sts.GetCallerIdentity` request and sends it to the Auth service.
The Auth executes the request on behalf of the instance.
The response contains the Account ID, and that's how Teleport ensures only instances whose account id is the same as the IAM Join Token, are accepted.

This RFD aims to add a small change to this flow: allow validation to be based on the AWS Organization.

When an EC2 instance tries to join the cluster using the IAM method, it will:
- create a signed request for `sts.GetCallerIdentity`, which already happens today
- create a signed request for `organizations.DescribeOrganization`
- send both signed requests

The Auth server might not execute both requests.
If the IAM Join token has any validation by account id or assumed role, then it will execute the `sts.GetCallerIdentity`.
If the token validates the AWS Organization ID, then it will execute the `organizations.DescribeOrganization`.

This prevents unnecessary API calls.

The `sts.GetCallerIdentity` API call should not fail, for well-behaved clients: there's not specific IAM permissions required, and clients only send it after doing some validation.

The `organizations.DescribeOrganization` API call might fail, even when the client sends it.
The client does not perform the API calls before signing them, so they might not know that they lack permissions.

Contrary to the `sts.GetCallerIdentity` - which doesn't require any specific permission -, the `organizations.DescribeOrganization` requires that permission to be allowed by the EC2's assigned IAM Role.

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

  // AccountIDs contain the list of AWS Account IDs to use when assuming an IAM Role.
  // AssumeRole must be present and have the account id part set to *.
  // Using an wildcard (*) as the account id will enumerate all the Account IDs within the AWS Organization.
  // Only EC2 type is supported.
  repeated string AccountIDs = 10 [(gogoproto.jsontag) = "account_ids,omitempty"];
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

Validating and calling the `sts.GetCallerIdentity` or `organizations.DescribeOrganization` should have equivalent security implications.

The flow is similar, and the invariants are kept:
- EC2 instance builds another signed request using the ambient credentials
- sends both requests over gRPC to the Auth Server
- Auth Server sends them to the AWS API

To improve security, AWS Assumed Role ARN is also validated, if present.

### Scale

#### UX
The user experience is improved because users no longer need to enumerate all the account ids, and can rely, instead on the organization.

#### Discover EC2 instances
When discovering EC2 instances, the Discovery Service will apply the same flow as today.

It will queue up all the discovered EC2 instances.

The flow will store in memory all the instances, which shouldn't be an issue given that it also stores all the teleport nodes in cache.
The number of EC2 instances should never be bigger than the number of teleport nodes, so we are not creating a new 


#### AWS Organization validation in IAM Join method
When an instance tries to join the cluster, it will call the Auth Server and pass the signed requests.

Today, only one request is executed (after validation).

After this RFD is implemented we might execute two API requests when the user filters by aws organization and either account id of assumed role ARN.
In this case, the Auth Server will issue two requests.

The join flow might consume more resources, but this only happens on agent onboarding, hopefully, once per EC2 instance.


### Backwards Compatibility

**Teleport version installed in target EC2 instance is old and does not send the DescribeOrganization request**

It might happen that the version that is installed in the EC2 instance is too old and does not yet have the logic to send the `organizations.DescribeOrganization` signed request.

If the IAM Join Token validates the Organization, Auth will reject the join attempt.

**EC2 instance sends DescribeOrganization request, but Auth Service was not updated**

Sending a signed `organizations.DescribeOrganization` request when attempting to join the cluster will not cause any issue because the signed request is ignored.

### Audit Events

No changes in audit events.

### Test Plan

Include a new testing item in EC2 Discovery section:
- discover ec2 instances in two aws accounts using the same matcher
- discover all instances from the organization's management account

Include a new testing item in IAM Join method:
- join an instance using the organization in the allow rules

-----------
# Notes

## Alternative configurations

### Discovery matcher
#### Alternative using accounts key
```yaml
  aws:
   - types: ["ec2"]
     account_ids: ["*"]
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
```

We can still discover multiple accounts without:
- requiring the organizations.ListAccounts
- which also means, we wouldn't need to deploy this in the organization management's account.

This option provides more flexibility:
- users can onboard from multiple accounts without using organizations
- instances can join using an IAM token which accepts account ids (instead of requiring )

#### Alternative using implicit assume_role wildcard
```yaml
  aws:
   - types: ["ec2"]
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
```

Setting the assume role's account id to `*`, means it will auto discover all accounts under the current organization

It's not explicit which might be confusing.

#### Being explicit about the organization
```yaml
  aws:
   - types: ["ec2"]
     organization: "o-123456"
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
```

This requires the user to be explicit about the org and the usage of `organizations.ListAccountsForParent` instead.

#### Discover everything under current organization
```yaml
  aws:
   - types: ["ec2"]
     all_accounts: true
     discover_accounts: true
     regions: ["us-east-1","us-west-1"]
     assume_role:
        role_arn: "arn:aws:iam::*:role/<role that exists in every account>"
```

---
authors: marco.dinis@goteleport.com
state: draft
---

# RFD 0204 - AWS CLI Access with IAM Roles Anywhere

## Required approvers

- Engineering: @r0mant
- Product: @klizhentas

## What

Provide access to AWS using IAM Roles Anywhere.

## Why

You can manage and access AWS by running [Teleport App Service with access to AWS credentials](https://goteleport.com/docs/enroll-resources/application-access/cloud-apis/aws-console/) or using AWS OIDC credentials.

This makes it possible to provide different levels of AWS access with RBAC and leveraging features like Access Requests, Access Lists and identity locking.

Existing methods require Teleport to proxy requests between the end user and AWS.
This has the advantage of auditing those requests, however it has some disadvantages:

**API requests go through Teleport**

All data to those APIs must go through Teleport, increasing resource consumption of the cluster which might create a poor experience because of increased latency, stricter limits imposed by Teleport and/or slower network paths.
Example: [#12922](https://gravitational.zendesk.com/agent/tickets/12922).

**Non-compliant with standard AWS tooling**

Some AWS and 3rd party tools expect specific environment variables (eg. `AWS_ACCESS_KEY`) or configuration files (eg. `~/.aws/config`) to be set.

That is not the case with current implementation by Teleport.
Some example of issues reporting AWS access problems:
- [Some AWS tools don't work with Teleport](https://github.com/gravitational/teleport/issues/10441)
- [Aqua Tool Manager fails when using Teleport for AWS access](https://github.com/gravitational/teleport/issues/42341)
- [Unable to initialize a terraform module from an S3 location](https://github.com/gravitational/teleport/issues/28025)
- [Access to EKS Clusters using `kubectl` doesn't work](https://github.com/gravitational/teleport/issues/24608)
- [Access to EKS Clusters using `aws eks` doesn't work](https://github.com/gravitational/teleport/issues/33583)

Workarounds for those exist, but require manual steps to accommodate the way Teleport works.

There's even some features that are implemented with workarounds because of teleport's incompatibility:
- [DynamoCB database access](https://github.com/gravitational/teleport/issues/17842)
- [Forwarding assumed-role-sessions to agent (to work terraform that assume other roles)](https://github.com/gravitational/teleport/pull/20568)
- [Athena ODBC, JDBC support](https://github.com/gravitational/teleport/issues/8281)
- [Fix `aws ssm` calls when KMS encryption is enabled](https://github.com/gravitational/teleport/pull/50402)

**Complex setup when using an App Service/agent** which must be installed and managed in an EC2 instance with a proper IAM Profile.

**Requires publicly accessible proxy when using AWS OIDC Integration** because AWS must consume the public keys exposed in Teleport Proxy.

**Using IAM Roles Anywhere**, Teleport acts solely as a X.509 certificate issuer, ensuring a transparent and seamless experience by not depending on any Teleport resources or networks paths for access.
It will be compatible with all AWS tooling by providing a [configuration profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html) in `~/.aws/config`.

## Non goals

**Revoking certificates** is supported by IAM Roles Anywhere by using [`rolesanywhere.ImportCrl`](https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ImportCrl.html) API), but it is not feature we are considering for this RFD.
See related issue: [#51178](https://github.com/gravitational/teleport/issues/51178).

**Auditing** API calls to AWS is not considered, only new sessions are audited.
This is because API calls will not go through Teleport.
This can be improved later on by pulling logs from AWS CloudTrail.

**AWS resources discovery (EC2, EKS, RDS, ...)** using AWS Roles Anywhere is not currently being considered.

**AWS Identity Center and AWS External Audit Storage** could use the Roles Anywhere integration (instead of OIDC IdP), however that is currently not being considered.

**AWS IAM Resource-based policies** is [not supported](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/security_iam_service-with-iam.html) by IAM Roles Anywhere.

## User stories

### End-user user stories

**As a Teleport end user, I want to list S3 buckets**

For new users, they need to login into teleport with `tsh login`.

They now need to fetch the credentials for accessing AWS, this is done using the following:
`tsh apps login --aws-role RoleRO-S3 my-awsra-integration`.

The user now runs `aws s3 ls` and they receive a list of buckets.

**As a Teleport end user, I have access to listing S3 buckets, but I want to elevate my permissions to be able to upload files**

To get access to another AWS IAM Role, user creates an Access Request.
They will request and assume an IAM Role which gives them the correct IAM permissions.

After assuming the new access, they are able to send files using:
`aws s3 cp my-large-file.bin s3://bucket/my-large-file.bin`.

**As a Teleport end user, I want to use terraform to load a module from an S3 bucket**

After logging in with `tsh login`, the user requests local credentials for their AWS Role with:
`tsh apps login --aws-role RoleRO-S3 my-awsra-integration`.

They have the following terraform file
```terraform
module "module" {
  source = "https://bucket.s3.eu-west-2.amazonaws.com/myobject"
}
```

Running `terraform init` works as expected.

**As a Teleport Administrator, I want to add access to another AWS IAM Role**
AWS Admin must create another IAM Roles Anywhere Profile, and associate the AWS IAM Roles they wish to provide access to.

Those IAM Roles must have their Trust Relationship accept the Teleport Trust Anchor.

After adding them, a new Application appears in Teleport which can be given access to users, using existing RBAC system.

**As a Teleport Administrator, I want to provide access to my users using Teleport, but I've never configured Roles Anywhere**
When setting up access to AWS using Teleport, the Administrator is informed that Teleport uses Roles Anywhere Profiles as resources.

After completing the set up, they are informed that there's no Profiles in their account.

Given they haven't used Roles Anywhere before, they must go to AWS Web Console and create the Profiles.
When creating the Profiles, they will add the existing IAM Roles.
As a pre-requirement for adding the IAM Roles into Profiles, they must change the Trust Policy of each IAM Role so that it can be used by Roles Anywhere.

After doing this, they get back to Teleport and after a couple of minutes the Profiles will appear as resources they can assign to users and can use to access their AWS account by assuming an IAM Role.

**As a Teleport Administrator, I want to give users of team "dev" read-only access to AWS Account, and ability to request read-write access to it**
There are two Profiles, each with only one IAM Role: ReadOnlyAccess and ReadWriteAccess.

When setting up the AWS Access, the administrator enables the Role Per Profile auto creation.
This creates as many Teleport Roles as Profiles, but each Teleport Role only allows access to a single Profile (using `app_labels` for RBAC checks on the application, and `aws_role_arn` rule for AWS IAM Roles allowed to assume).

They also create a new Teleport Role which allows access to requesting access to the ReadWriteAccess: RequestReadWriteAccess.
The administrator changes the "Dev" access list and adds the two roles: ReadOnlyAccess and RequestReadWriteAccess.

This way, "dev" members can now access the ReadOnlyAccess but can also request access to the ReadWriteAccess using the permissions inherited by the RequestReadWriteAccess role.

**As a user, I want to be able to see what AWS roles are available to me and request access to a role I don't have long standing access to**
There are two Profiles, each with only one IAM Role: ReadOnlyAccess and ReadWriteAccess.

When setting up the AWS Access, the administrator enables the Role Per Profile auto creation.
This creates as many Teleport Roles as Profiles, but each Teleport Role only allows access to a single Profile (using `app_labels` for RBAC checks on the application, and `aws_role_arn` rule for AWS IAM Roles allowed to assume).

The administrator assigns the ReadOnlyAccess to all their users, and creates another Role that allows users to request access to the ReadWriteAccess Role.

When users login, they can see that they can access the ReadOnlyAccess, but not ReadWriteAccess.
They can, however, ask for access to that role as well.

Leveraging Access Requests, users can now request access to ReadWriteAccess and, ultimately, assume the IAM Role.

## User experience

### Flow for: Enroll New Resource / AWS Access
We have the following AWS Integrations in Enroll New Integration screen:
- AWS External Audit Storage
- AWS Identity Center
- AWS OIDC Identity Provider

Adding another tile for "AWS Roles Anywhere" would create too many integrations and lead to users not knowing which one to chose.

Instead, the AWS OIDC Identity Provider should be removed, and only accessible from the Enroll New Resource tiles.

The Enroll New Resource/ AWS CLI/Web Console tile will only accept AWS Roles Anywhere integrations.

Other AWS related tiles (eg EC2 Auto Enrollment with SSM) will only be accessible using AWS OIDC integrations.

In the future we might add support for AWS Roles Anywhere for the remaining AWS related flows.

After clicking on adding a new AWS CLI/Web Console access, users can create a new AWS Roles Anywhere Integration.

#### Create new Roles Anywhere Integration screen

Setting up this integration consists on configuring AWS to trust Teleport.

Administrator goes into "Enroll new resource" page and selects the "AWS CLI/Web Console Access" tile.

A [summary of what IAM Roles Anywhere is](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html#first-time-user) and how Teleport uses it is displayed.

```
 Access flow                                                            
       ┌──────────────────────────┐             ┌────────────────┐      
       │ Teleport                 │             │  User          │      
       │                          │  App Login  │ ┌─────┐        │      
       │┌──────────┐Issues┌─────┐ ◄─────────────┼─┤ tsh │        │      
       ││ AWSRA-CA ┼──────►X.509│ │ Credentials │ │     │        │      
       │└────────▲─┘      └─────┘ ├─────────────┼─►     │        │      
       │         │                │             │ │     │        │      
       └─────────┼────────────────┘             │ └─▲──┬┘        │      
                 │                              │   │  │         │      
                 │                              │ ┌─┴──▼───────┐ │      
                 │                              │ │   aws cli  │ │      
                 │                              │ └──────┬─────┘ │      
                 │Trusts                        └────────┼───────┘      
                 │                                       │              
┌────────────────┼─────────────────────────────────┐     │              
│ AWS            │                                 │     │              
│ ┌──────────────┼───────────────────────────────┐ │     │Calls AWS APIs
│ │ AWS Account  │                               │ │     │              
│ │ ┌────────────┼──────────┐ ┌────────────────┐ │ │     │              
│ │ │ Region     │          │ │  IAM Roles     │ │ │     │              
│ │ │ ┌──────────┴───────┐  │ │                │ │ │     │              
│ │ │ │ RA Trust Anchor  │  │ │                │ │ ◄─────┘              
│ │ │ └──────────────────┘  │ │  ┌───────────┐ │ │ │                    
│ │ │                   ┌───┼─┼──►   Role1   │ │ │ │                    
│ │ │                   │   │ │  └───────────┘ │ │ │                    
│ │ │ ┌──────────────┐  │   │ │  ┌───────────┐ │ │ │                    
│ │ │ │ RA Profile1  ├──┴───┼─┼──►   Role2   │ │ │ │                    
│ │ │ └──────────────┘      │ │  └───────────┘ │ │ │                    
│ │ │                       │ │                │ │ │                    
│ │ │ ┌──────────────┐      │ │  ┌───────────┐ │ │ │                    
│ │ │ │ RA Profile2  ├──────┼─┼──►   Role3   │ │ │ │                    
│ │ │ └──────────────┘      │ │  └───────────┘ │ │ │                    
│ │ └───────────────────────┘ └────────────────┘ │ │                    
│ └──────────────────────────────────────────────┘ │                    
└──────────────────────────────────────────────────┘                    
                                                                        
 Sync process                                                           
┌────────────────────────────────────────────┐ ┌─────────────────────┐  
│ AWS                                        │ │ Teleport            │  
│ ┌────────────────────────────────────────┐ │ │                     │  
│ │ AWS Account                            │ │ │ ┌─────────────────┐ │  
│ │ ┌─────────────────┐ ┌────────────────┐ │ │ │ │App Service      │ │  
│ │ │ Region          │ │  IAM Roles     │ │ │ │ │ RA Profile1     │ │  
│ │ │┌─────────────┐  │ │ ┌───────────┐  │ │ │ │ │ Allowed Roles:  │ │  
│ │ ││ RA Profile1 ┼─┬┼─┼─►   Role1   │  │ │ │ │ │ - Role1         │ │  
│ │ │└─────────────┘ ││ │ └───────────┘  │ │ │ │ │ - Role2         │ │  
│ │ │                ││ │ ┌───────────┐  │ │ │ │ └─────────────────┘ │  
│ │ │                └┼─┼─►   Role2   │  │ │ │ │ ┌─────────────────┐ │  
│ │ │                 │ │ └───────────┘  │ │ │ │ │App Service      │ │  
│ │ │┌──────────────┐ │ │ ┌───────────┐  │ │ │ │ │ RA Profile2     │ │  
│ │ ││ RA Profile2  ┼─┼─┼─►   Role3   │  │ │ │ │ │ Allowed Roles:  │ │  
│ │ │└──────────────┘ │ │ └───────────┘  │ │ │ │ │ - Role3         │ │  
│ │ └─────────────────┘ └────────────────┘ │ │ │ └─────────────────┘ │  
│ └────────────────────────────────────────┘ │ │                     │  
└────────────────────────────────────────────┘ └─────────────────────┘  
```

##### How IAM Roles Anywhere work with Teleport

Teleport periodically syncs IAM Roles Anywhere Profiles as an Application resource into Teleport, which allows you to define RBAC policies on them.

##### RBAC
Re-using the Teleport Application resources gets us the following features out of the box:
- support for just-in-time Access Requests
- support for Access Lists

AWS Applications are created from a Profile and the Profile Tags are mapped into Application Labels.
To grant access to a Profile, users must be allowed to access the Application using `app_labels`.

When accessing AWS using a Profile/Application, users must also have access to the IAM Role they want to assume.
AWS validates that only Profile's allowed Roles are accessible and Teleport validates that only `aws_role_arn` (either in traits or explicit in Teleport Roles) can assume that IAM Role.

In order to assume a given Role, it must be present on both:
- Profile's allowed IAM Roles
- Teleport's `aws_role_arn` list for the User.

As an example, assuming the following IAM Roles Anywhere Profile:
```yaml
name: ProfileA
tags:
  - Team: ABC
  - Env: Prod
roles:
  - MyRoleA
  - MyRoleB
```

A Teleport Application will be created with the following metadata:
```yaml
kind: app_server
metadata:
  labels:
    Team: ABC
    Env: Prod
  name: ProfileA
spec:
  app:
    kind: app
    metadata:
      labels:
          Team: ABC
          Env: Prod
      name: ProfileA
    spec:
      aws:
        roles_anywhere:
          allowed_roles_arn:
          - arn:aws:iam::123456789012:role/MyRoleA
          - arn:aws:iam::123456789012:role/MyRoleB
          profile_arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/ac1f655b-aaaa-aaaa-aaaa-aaaaaaaaaaaa
      cloud: AWS
      integration: raa
      uri: https://console.aws.amazon.com
    version: v3
version: v3
```

And the following Teleport Role would grant access:
```yaml
kind: role
metadata:
  description: AWS Access for ProfileA
  name: access-profile-a
spec:
  allow:
    app_labels:
      Team: ABC
      Env: Prod
    aws_role_arns:
    - arn:aws:iam::123456789012:role/MyRoleA
    - arn:aws:iam::123456789012:role/MyRoleB
version: v7
```

##### New Resources
The following IAM resources must be be created in your AWS account.

**IAM Roles Anywhere Trust Anchor** that trusts Teleport's AWS Roles Anywhere Certificate Authority.
This ensures that IAM trusts Teleport as a Roles Anywhere certificate issues.\
This certificate can be found at `curl https://<proxy-url>/v1/webapi/auth/export?type=awsra` or `tctl auth export --type awsra`.

A new **IAM Role** which allows Teleport to query existing IAM Roles Anywhere Profiles, in order to sync them into Teleport.
This Role has a custom Trust Policy, as described in [AWS documentation](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/).

The following policy is required:
- `rolesanywhere:ListProfiles` - used to fetch existing IAM Roles Anywhere Profiles
- `rolesanywhere:ListTagsForResource` - used to fetch the Profile tags
- `iam:GetRole` - used to create UserTasks when a given IAM Role's trust policy does not accept the Trust Anchor

A new **IAM Roles Anywhere Profile** is created, containing the IAM Role above.

They are presented with default values for IAM Role, IAM Roles Anywhere Trust Anchor and Profile names to be used, which they can customize.

After accepting the names, they are asked to run a set up script in CloudShell.
```
> bash $(curl https://tenant.teleport.sh/scripts/integration-setup-awsra.sh)
```

Users must now copy the resources ARN into teleport and test the connection (ie is teleport able to list profiles?).

After this point, Teleport is now syncing IAM Roles Anywhere Profiles as Teleport AWS Application Access resources.

If the first sync process fails, then users are required to fix the issue and try again.

#### Set Up Access screen

In this screen users are asked to configure access to their own user using traits.

This includes two rules:
- which Profiles they can access, which relies on `app_labels` Role rule
- which IAM Roles they can assume, which relies on `aws_role_arn` Role rule and User traits

Similar to existing flows, users are asked to enter the Role ARNs they want to access.

This selection should be guided with the list of Role ARNs gathered from the synchronization process.

#### Test Connection

After completing the Set Up Access step, users land in the Test Connection step.

This step allows them to test the access they've just created: by logging in into AWS using one of the IAM Roles which was configured in the previous step.


### Integration Dashboard
Teleport has a status page for viewing the AWS OIDC Integration status.

The dashboard must also support this new Integration.

The stats page will report the number of synced IAM Profiles.

User Tasks might also be created by the sync process and the following issue types will be reported:
- IAM Roles Anywhere Trust Anchor was removed
- Sync process has an invalid Roles Anywhere Profile and/or Role
- IAM Role used for the sync process does not have the valid permissions
- IAM Role associated with a profile does not accept the Trust Anchor ARN defined in the integration

## Implementation

### AWS Roles Anywhere Integration

#### Set up
For setting up the integration, a script must be created that creates the AWS resources.

Here's a snippet of what it should look like.

```go
createTAResp, err := raClient.CreateTrustAnchor(ctx, &rolesanywhere.CreateTrustAnchorInput{
  Name: aws.String(fmt.Sprintf("Trust anchor for %s", clusterName)),
  Source: &rolesanywheretypes.Source{
    SourceData: &rolesanywheretypes.SourceDataMemberX509CertificateData{
      Value: "<AWSRA-CA>",
    },
    SourceType: rolesanywheretypes.TrustAnchorTypeCertificateBundle,
  },
})

trustPolicyJSON := trustPolicyForRolesAnywhereTrustAnchor(createTAResp)
_, err = iamClient.CreateRole(ctx, &iam.CreateRoleInput{
  RoleName:                 &roleName,
  AssumeRolePolicyDocument: &trustPolicyJSON,
})
// iam.PutRolePolicy with `rolesanywhere:ListProfiles` and `iam:GetRole`

_, err = raClient.CreateProfile(ctx, &rolesanywhere.CreateProfileInput{})
raClient.CreateProfile(ctx, &rolesanywhere.CreateProfileInput{
  Name:     aws.String(fmt.Sprintf("Sync Profiles to Teleport %s", clusterName)),
  RoleArns: []string{roleName},
})
```

#### Integration
A new integration resource must be created, using `aws-ra` subkind, and includes the Trust Anchor ARN which trusts Teleport CA and a synchronization section that contains:
- IAM Roles Anywhere Profile ARN and IAM Role ARN used to fetch existing AWS Roles Anywhere Profiles
- label matchers which are used to selectively sync profiles (defaults to sync)

Format for the new integration subkind:
```yaml
kind: integration
sub_kind: aws-ra
version: v1
metadata:
  name: <integration-name>
spec:
  aws_ra:
    trust_anchor_arn: <trust anchor arn>
    sync:
      enabled: <true|false>
      profile_arn: <profile arn>
      role_arn: <role arn>
      profile_filter:
        <list of profile ARNs or regex to apply on the profile name>
      create_role_per_profile: <true|false>
status:
  aws_ra:
    sync:
      state: <running | error | disabled>
      last_sync: <timestamp>
      profiles_synced: <number of profiles synced into teleport>
      error_message: <error message when state is error>
```

An example of the full resource:
```yaml
kind: integration
sub_kind: aws-ra
version: v1
metadata:
  name: my-awsra-integration
spec:
  aws_ra:
    trust_anchor_arn: arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/edffbaaa-6900-4524-b043-17c9b869f84d
    sync:
      enabled: true
      profile_arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/6778b17c-bb31-4c06-8c77-b773496094a3
      role_arn: arn:aws:iam::123456789012:role/role-for-rolesanywhere-listprofiles
      profile_filter:
        - arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/6778b17c-bb31-4c06-8c77-b773496094a4
        - arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/6778b17c-bb31-4c06-8c77-b773496094a5
        - name_regex: ^TeleportProfiles.*
      create_role_per_profile: true
status:
  aws_ra:
    sync:
      state: running
      last_sync: "2025-02-25T00:00:00Z"
      profiles_synced: 3
      error_message: null
```

#### Sync
The sync process fetches AWS IAM Roles Anywhere Profiles and converts them into Application Servers.
Example of an App Server:
```yaml
kind: app_server
metadata:
  name: awsra-my-profile
  labels:
    env: prod
spec:
  app:
    kind: app
    metadata:
      name: awsra-my-profile
      labels:
        env: prod
    spec:
      cloud: AWS
      integration: awsra-my-integration
      aws:
        roles_anywhere:
          profile_arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/6778b17c-bb31-4c06-8c77-b773496094a3
          allowed_role_arns:
          - arn:aws:iam::123456789012:role/my-custom-role1
          - arn:aws:iam::123456789012:role/my-custom-role2
          - arn:aws:iam::123456789012:role/my-custom-role3
```

Teleport must check if the Profile Roles are compatible with the Integration's Trust Anchor.
This is done using the `iam:get-role` and checking its trust policy property:
```json
        "Condition": {
          "ArnEquals": {
            "aws:SourceArn": [
              "<TrustAnchorARN>"
            ]
          }
        }
```

### AWS Roles Anywhere Certificate Authority

A new Certificate Authority will be created in Teleport to issue X.509 certificates, consumed by IAM Roles Anywhere.

This new CA is backed by a ECDSA key, according to our current recommended crypto suites (see RFD0136).

#### CA certificate
When setting the trust anchor in AWS IAM Roles Anywhere, we'll add the new CA certificate.

This certificate must comply with the following:
- must be a X.509v3 certificate
- Basic Constraints must include `CA: true`
- the signing algorithm must include SHA256 or stronger
- key usage must include Certificate Sign, CRL Sign and Digital Signature

Including the CRL Sign is not required at this point, but this allows us to implement certificate revocation later on without the need to change the Trust Anchor.

[See more](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/trust-model.html#signature-verification)

#### End entity certificates - X.509
When issuing X.509 certificates for users to access AWS, they must comply with the following:
- must be a X.509v3 certificate
- Basic Constraints must not include `CA` or its value must be false
- the signing algorithm must include SHA256 or stronger
- key usage must include Digital Signature

[See more](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/trust-model.html#signature-verification)

Sessions generated from IAM Roles Anywhere certificates have their [Source Identity](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_control-access_monitor.html) set to the common name (CN) of the certificate's Subject.
Given its importance for auditing, this should contain the Teleport User name that requested the certificate.

During IAM policy evaluation, the following certificate fields are exposed and can be used to allow or deny the request:
- Subject fields as `PrincipalTag: aws:PrincipalTag/x509Subject/<field>` (eg CN maps to `PrincipalTag: aws:PrincipalTag/x509Subject/CN`)
- Issuer fields as `PrincipalTag: aws:PrincipalTag/x509Issuer/<field>`
- Subject Alternative Name (SAN) as `PrincipalTag: aws:PrincipalTag/x509SAN/<field>`

Of the above, only some of them will be set by Teleport:
- Subject Common Name: Teleport user that requested the access
- Issuer Common Name: Teleport cluster name
- Not After: this will be set based on the user's current session expiration

### AWS Session creation
After generating the certificate, Teleport will call `rolesanywhere.CreateSession` and exchange the certificate for AWS credentials.

This call will not be explicit, but handled by the [rolesanywhere-credential-helper](https://github.com/aws/rolesanywhere-credential-helper) tool from AWS.

The `rolesanywhere.CreateSession` call accepts a `durationSeconds` param which accepts values from 15 minutes up to 12 hours.
Its value must be set using the current's Teleport User session, up to 12 hours if it exceeds that.

### AWS Roles requirements for usage with IAM Roles Anywhere
AWS IAM Roles must be accessible from Roles Anywhere service, which requires a [custom Trust Policy](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/trust-model.html#trust-policy).

For IAM Roles to be accessible from IAM Roles Anywhere, they must have the following trust policy:
```json
{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Principal": {
          "Service": "rolesanywhere.amazonaws.com"
        },
        "Action": [
          "sts:AssumeRole",
          "sts:TagSession",
          "sts:SetSourceIdentity"
        ],
        "Condition": {
          "ArnEquals": {
            "aws:SourceArn": [
              "arn:aws:rolesanywhere:<region>:<account>:trust-anchor/<trust-anchor-id>"
            ]
          }
        }
      }
    ]
  }
```

Note: The `Condition` section is optional, and will ensure only the mentioned Trust Anchor can use this IAM Role.

### Integration with end user tools
Users will login using `tsh apps login --aws-role <Role ARN> <App Name>`.
This command does two things:
- fetches and stores the credentials into the application certificate stored under `~/.tsh/`
- modifies the `~/.aws/config` file to create/update the AWS configuration profile

The  [`~/.aws/config`](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)` contains instructions on how to access AWS credentials locally, and is used by every aws cli and other aws-sdk-based tools.

The following entry is added (if it not exists yet):
```conf
[profile <App Name>]
credential_process = tsh apps config <App Name> --format aws-credential-process
```

Using `tsh` as the [`credential_process`](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html) ensures AWS API calls will call `tsh` for getting the credentials.

`tsh` must return a json document containing the credentials.
Its format should match the following specification:
```json
{
  "Version": 1,
  "AccessKeyId": "an AWS access key",
  "SecretAccessKey": "your AWS secret access key",
  "SessionToken": "the AWS session token for temporary credentials", 
  "Expiration": "ISO8601 timestamp when the credentials expire"
}
```

#### AWS configuration profiles
Users are required to pass `--profile <profile>` or set the `AWS_PROFILE` environment variable to access AWS, which can be tedious.

Instead, users can use set teleport as the default profile either editing the `~/.aws/config` or by passing the `--set-as-default-profile` when doing `tsh apps login`:
```conf
[default]
credential_process = tsh apps config <App Name> --format aws-credential-process
```

Doing this, users no longer need to set the profile and will always use this AWS IAM Role and Roles Anywhere Profile protected by Teleport.

#### Credentials expiration
When local credentials to access Teleport expire, `tsh` will try to re-login the user.
However, `credential_process` will not output anything while the process is still running, making this re-login process stuck on asking for user input (ie, password).

In this case, `tsh` must immediately exit with a clear error message asking the user to re-login.

See more information about [`credential_process`](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html).

### Proto Specification
#### Generate AWS credentials
AWS Credentials will be returned to the user after they log in into an App (same flow as exists today), embedded in the certificate.
The following field will be added to the certificate:

```proto
// RouteToApp contains parameters for application access certificate requests.
message RouteToApp {
  // Name is the application name certificate is being requested for.
  string Name = 1 [(gogoproto.jsontag) = "name"];
  // AWSRoleARN is the AWS role to assume when accessing AWS API.
  string AWSRoleARN = 5 [(gogoproto.jsontag) = "aws_role_arn,omitempty"];
  // ...

  // AWSCredentialProcessCredentials contains the credentials to access AWS APIs.
  // This is a JSON string that conforms with credential_process format.
  string AWSCredentialProcessCredentials = 10 [(gogoproto.jsontag) = "aws_credentialprocess_credentials,omitempty"];
}
```

#### Changes to `App` resource
```proto
// AppSpecV3 is the AppV3 resource spec.
message AppSpecV3 {
  // AWS contains additional options for AWS applications.
  AppAWS AWS = 6;
  // ...
}

// AppAWS contains additional options for AWS applications.
message AppAWS {
  // ExternalID is the AWS External ID used when assuming roles in this app.
  string ExternalID = 1;

  // RolesAnywhere contains the IAM Roles Anywhere fields associated with this Application.
  AppAWSRolesAnywhere RolesAnywhere = 2;
}

// AppAWSRolesAnywhere contains the fields that represent an AWS Roles Anywhere Profile.
message AppAWSRolesAnywhere {
  // ProfileARN is the IAM Roles Anywhere Profile ARN that originated this AWS App.
  string ProfileARN = 1;

  // The list of allowed Role ARNs associated with the Profile.
  repeated string AllowedRolesARN = 2;
}
```

#### Changes to `Integration` resource
```proto
// IntegrationSpecV1 contains properties of all the supported integrations.
message IntegrationSpecV1 {
  oneof SubKindSpec {
    // AWSRA contains the specific fields to handle the AWS Roles Anywhere Integration subkind
    AWSRAIntegrationSpecV1 AWSRA = 1;
  }
}

// AWSRAIntegrationSpecV1 contains the spec properties for the AWS Roles Anywhere Integration subkind.
message AWSRAIntegrationSpecV1 {
  // TrustAnchorARN contains the AWS Roles Anywhere Trust Anchor ARN used to set up the Integration.
  string TrustAnchorARN = 1;

  // Sync is the configuration for syncing Roles Anywhere Profiles to Applications.
  AWSRASyncConfiguration Sync = 2;
}

// AWSRASyncConfiguration contains the configuration used to sync AWS Roles Anywhere Profiles as Applications.
message AWSRASyncConfiguration {
  // Enabled indicates whether the synchronization is enabled.
  bool Enabled = 1;

  // ProfileARN is the AWS Roles Anywhere Profile to be used to access AWS APIs.
  string ProfileARN = 2;

  // RoleARN is the AWS IAM Role to be used to access AWS APIs.
  string RoleARN = 3;

  // ProfileFilter contains filters to be applied to Profiles.
  // Only matching Profiles will be synced.
  // Logical OR is applied when multiple filters are present.
  // Empty list of filters ensures all Profiles are synchronized.
  repeated AWSRAProfileFilter ProfileFilter = 4;

  // CreateRolePerProfile indicates whether a Teleport Role should be created by profile.
  // If enabled, a Role is created by each Profile.
  // This Role only allows access to this specific Profile and their AWS Role ARNs.
  bool CreateRolePerProfile = 5;
}

message AWSRAProfileFilter {
  // Include describes the AWS Resource filter to apply
  oneof include {
    // ARN indicates that the resource should be filtered by ARN.
    string ARN = 1;

    // NameRegex indicates that the resource should be included when its name matches
    // the supplied regex.
    string name_regex = 2;
  }
}
```

### Audit Events
The following events must be created:
- emit event when the sync process runs
- emit event when issuing a X.509 certificate

## Product Usage
The following metrics should be collected:
- number of integrations created
- number of successful/failed syncs
- number of X.509 certificates issued grouped by integration
- number of API calls

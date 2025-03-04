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
This has the advantage of auditing those requests, however it has two main disadvantages:

**API requests go through Teleport**

All data to those APIs must go through Teleport, increasing resource consumption of the cluster which might create a poor experience because of increased latency, stricter limits imposed by Teleport and/or slower network paths. Example: [#12922](https://gravitational.zendesk.com/agent/tickets/12922).

**Non-compliant with standard AWS tooling**

Some AWS and 3rd party tools expect specific environment variables (eg. `AWS_ACCESS_KEY`) or configuration files (eg. `~/.aws/`) to be set.

That is not the case with current implementation by Teleport. Some example of issues reporting AWS access problems:
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

**Using IAM Roles Anywhere**, Teleport acts solely as a X.509 certificate issuer, ensuring a transparent and seamless experience by not depending on any Teleport resources or networks paths for access.
It will be compatible with all AWS tooling by providing a [configuration profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html) in `~/.aws/`.

## Non goals

**Revoking certificates** is supported by IAM Roles Anywhere by using [`rolesanywhere.ImportCrl`](https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_ImportCrl.html) API), but it is not feature we are considering for this RFD.
See related issue: [#51178](https://github.com/gravitational/teleport/issues/51178).

**Access to Web Console** is not considered to narrow the focus to CLI/SDK UX, which is where we see more issues with using existing flows.

**Auditing** API calls to AWS is not considered, only new sessions are audited.
This is because API calls will not go through Teleport.
This can be improved later on by pulling logs from AWS CloudTrail.

**AWS resources discovery (EC2, EKS, RDS, ...)** using AWS Roles Anywhere is not currently being considered.

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

## User experience

### Integration setup flow

Setting up this integration consists on configuring AWS to trust Teleport.

Administrator goes into "Enroll new resource" page and selects the "AWS Access using Roles Anywhere" tile.

A [summary of what IAM Roles Anywhere is](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html#first-time-user) and how Teleport uses it is displayed.

```
                                                ┌─────────────────────────┐
     ┌─────────────────────┐                    │  User                   │
     │                     │                    │ ┌───────────┐           │
     │ Teleport            │                    │ │ tsh       │           │
     │    ┌───────────┐    │            Issues  │ │   ┌─────┐ │           │
     │    │ AWSRA-CA  ┼────┼────────────────────┼─┼───►X.509│ │           │
     │    └──────▲────┘    │                    │ │   └─────┘ │           │
     │           │         │                    │ └──────▲────┘           │
     └───────────┼─────────┘                    │        │Get Credentials │
                 │                              │ ┌──────┼─────┐          │
                 │                              │ │   aws cli  │          │
                 │                              │ └──────┬─────┘          │
                 │                              └────────┼────────────────┘
                 │Trusts                                 │                 
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
│ │ │                       │ │                │ │ │                       
│ │ │                       │ │                │ │ │                       
│ │ └───────────────────────┘ └────────────────┘ │ │                       
│ └──────────────────────────────────────────────┘ │                       
└──────────────────────────────────────────────────┘                       
```

#### How IAM Roles Anywhere work with Teleport

Teleport syncs IAM Roles Anywhere Profiles as an Application resource into Teleport, which allows you to define RBAC policies on them.

##### RBAC considerations
Users must be granted access to App Labels (`app_labels`) to ensure they have access to the IAM Roles Anywhere Profiles.
They also must be granted access to a set of AWS Role ARNs (`aws_role_arn`) either using the Role or using the traits in their User information.

For IAM Roles to be used by this flow, they must allow the `rolesanywhere.amazonaws.com` Service Principal and `sts:AssumeRole,sts:TagSession,sts:SetSourceIdentity` actions Trust Policy.
More detailed information will be presented in implementation section.

##### New Resources
The following IAM resources must be be created in your AWS account.

**IAM Roles Anywhere Trust Anchor** that trusts Teleport's AWS Roles Anywhere Certificate Authority.
This ensures that IAM trusts Teleport as a Roles Anywhere certificate issues.\
This certificate can be found at `curl https://<proxy-url>/v1/webapi/auth/export?type=awsra` or `tctl auth export --type awsra`.

A new **IAM Role** which allows Teleport to query existing IAM Roles Anywhere Profiles, in order to sync them into Teleport.
This Role has a custom Trust Policy, as described in [AWS documentation](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/).

The following policy is required:
- `rolesanywhere:ListProfiles` (only enabled Profiles with `teleport.dev/sync: true` tag are synced)

A new **IAM Roles Anywhere Profile** is created, containing the IAM Role above.
This profile only allows access to the role created above an has the `teleport.dev/sync: true`.
</details>

They are presented with default values for IAM Role, IAM Roles Anywhere Trust Anchor and Profile names to be used, which they can customize.

After accepting the names, they are asked to run a set up script in CloudShell.
```
bash $(curl https://tenant.teleport.sh/scripts/integration-setup-awsra.sh)
```

Users must now copy the resources ARN into teleport and test the connection (ie is teleport able to list profiles?).

After this point, Teleport is now syncing IAM Roles Anywhere Profiles as Teleport AWS Application Access resources.

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
// iam.PutRolePolicy with `rolesanywhere:ListProfiles`

_, err = raClient.CreateProfile(ctx, &rolesanywhere.CreateProfileInput{})
raClient.CreateProfile(ctx, &rolesanywhere.CreateProfileInput{
  Name:     aws.String(fmt.Sprintf("Sync Profiles to Teleport %s", clusterName)),
  RoleArns: []string{roleName},
})

```

#### Integration
A new integration resource must be created which supports the synchronization process.
Its subkind is `aws-ra` and will include the following spec:

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
      profile_arn: <profile arn>
      role_arn: <role arn>
status:
  aws_ra:
    sync:
      state: <running | error>
      last_sync: <timestamp>
      profiles_synced: <number of profiles synced into teleport>
      error_message: <truncated error message when state is error>
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
      profile_arn: arn:aws:rolesanywhere:eu-west-2:123456789012:profile/6778b17c-bb31-4c06-8c77-b773496094a3
      role_arn: arn:aws:iam::123456789012:role/role-for-rolesanywhere-listprofiles
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
spec:
  app:
    kind: app
    metadata:
      name: awsra-my-profile
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

Its usage will be described below on AWS Role requirements.

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
          "StringEquals": {
            "aws:PrincipalTag/x509Issuer/CN": "<teleport cluster name>"
          },
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

### Integration with end user tools
Users will login using `tsh apps login --aws-role <Role Name> <App Name>`.

This will update the default AWS configuration on the user's operating system by changing the [`~/.aws/config`](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html) file.
This is used by all AWS CLI tools and SDKs.

The following entry is added (if it not exists yet):
```conf
[profile teleport]
credential_process = tsh apps sign --aws-role <Role Name> <App Name>
```

Using `tsh` as the [`credential_process`](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html) ensures AWS API calls will call `tsh` for getting the credentials.

`tsh` must return a json document containing the credentials. Its format should match the following specification:
```
{
  "Version": 1,
  "AccessKeyId": "an AWS access key",
  "SecretAccessKey": "your AWS secret access key",
  "SessionToken": "the AWS session token for temporary credentials", 
  "Expiration": "ISO8601 timestamp when the credentials expire"
}
```

AWS provides a helper tool which converts a X.509 into the above document: [rolesanywhere-credential-helper](https://github.com/aws/rolesanywhere-credential-helper).
This will be imported as a go module in `tsh` and used to output the expected format.

#### AWS configuration profiles
Users can create custom profiles by passing the `--profile my-custom-profile` to `tsh apps login` command.
This changes the profile name in `~/.aws/config`:
```conf
[profile my-custom-profile]
credential_process = tsh apps sign --aws-role <Role Name> <App Name>
```

Using a named profile is useful when users have multiple profiles.
However, if they only have one profile it can be tedious to have to always set the `--profile my-profile` in every cli call, set the profile configuration parameter for SDK, or by setting the `AWS_PROFILE` env var.

The `tsh apps login` command supports another flag - `--set-as-default-profile` - which overwrites the default AWS profile:
```conf
[default]
credential_process = tsh apps sign --aws-role <Role Name> <App Name>
```
Doing this, users no longer need to set the profile and will always use this AWS IAM Role and Roles Anywhere Profile.

### Issue X.509 certificate from Roles Anywhere CA
Issuing X.509 certificate for a user will use the already existing `rpc GenerateUserCerts(UserCertsRequest) returns (Certs);` gRPC API.

### Proto Specification
No new resources are expected, but the following ones must be changed

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
  string RoleARN = 1;

  // Sync is the configuration for syncing Roles Anywhere Profiles to Applications.
  AWSRASyncConfiguration sync = 2;
}

// AWSRASyncConfiguration contains the configuration used to sync AWS Roles Anywhere Profiles as Applications.
message AWSRASyncConfiguration {
  // ProfileARN is the AWS Roles Anywhere Profile to be used to access AWS APIs.
  string ProfileARN = 1;

  // RoleARN is the AWS IAM Role to be used to access AWS APIs.
  string RoleARN = 2;
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

---
authors: Roman Tkachenko (roman@goteleport.com)
state: implemented
---

# RFD 38 - Automatic registration and discovery of databases on AWS

## What

Proposes a way for the database service agent to automatically discover and
register AWS-hosted RDS, Aurora and Redshift databases.

## Why

Using database access with AWS-hosted databases is currently somewhat painful
for a couple of reasons:

- Users have to manually enable IAM authentication and configure/attach IAM
  policies for each database which is very error and typo-prone, or create
  overly permissive IAM policies which is insecure.
- Users who have large numbers of AWS-hosted databases (we've had prospects with
  hundreds) or create/delete them on demand have no good way to automatically
  plug them into the cluster.

Teleport database agent should be able to automatically do both of these.

## Scope

This RFD focuses on AWS-hosted databases. Similar ideas will be explored for
GCP Cloud SQL and Azure-hosted databases in future.

## IAM

Currently, to allow IAM authentication with RDS instances, users have to enable
IAM authentication on the instance and create an IAM policy like this:

```json
{
   "Version": "2012-10-17",
   "Statement": [
      {
         "Effect": "Allow",
         "Action": [
           "rds-db:connect"
         ],
         "Resource": [
           "arn:aws:rds-db:us-east-2:1234567890:dbuser:db-ABCDEFGHIJKL01234/*"
         ]
      }
   ]
}
```

This policy needs to exist for each RDS (or similar one for Redshift) instance
that is proxied by database access and be attached to the IAM identity the
database agent is using.

Instead of making the user enable IAM auth and create/attach the policy manually
(or build an automation for it), the database agent can do it itself.

### Required permissions

To support this, the database agent must have certain IAM permissions.

- `rds:ModifyDBCluster` and `rds:ModifyDBInstance` to be able to enable IAM
  authentication. For Redshift clusters IAM auth is always enabled.
- `iam:Put*Policy`, `iam:Get*Policy` and `iam:Delete*Policy` to be able to
  manage inline policies for IAM RDS/Redshift authentication.

To know how to attach the policy to itself, the agent will use `sts:GetCallerIdentity`[1]
call to determine IAM identity it's running as. This operation does not require
any permissions.

### Privilege escalation

Database agent being able to create and attach policies to itself raises
privilege escalation concerns. Can somebody who has access to the database
agent instance give itself unlimited IAM permissions?

To prevent privilege escalation users can use IAM [permission
boundaries](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_boundaries.html).
Permission boundary is an IAM policy that does not grant any privileges by
itself but defines the maximum privileges that can be granted to a user/role.
For example, if permission boundary is set and doesn't explicitly allow to
create EC2 instances, the user won't be able to create them even if their
attached IAM policies allow it. The effective permissions set of an IAM identity
is the intersection between their attached IAM policy and permission boundary.

Applied to the database agent scenario, users will create a permission boundary
that only allows the agent to create/attach IAM policies and connect to an RDS
instance:

```json
{
   "Version": "2012-10-17",
   "Statement": [
      {
         "Effect": "Allow",
         "Action": [
           "iam:CreatePolicy",
           "iam:DeletePolicy",
           "iam:Attach*",
           "iam:Detach*"
         ],
         "Resource": "*",
      },
      {
         "Effect": "Allow",
         "Action": [
           "rds-db:connect"
         ],
         "Resource": [
           "arn:aws:rds-db:*:1234567890:dbuser:*/*"
         ]
      }
   ]
}
```

With this permission boundary, whoever has access to the database agent instance
will be able to create any policy but not use it for anything besides connecting
to an RDS instance.

## Discovery

Database agent can auto-discover RDS and Aurora instances in the AWS account
it has credentials for by matching resource tags. Auto-discovery is disabled
by default.

It can be enabled by providing the following matcher to the database agent
configuration:

```yaml
db_service:
  enabled: "yes"
  aws:
  - types: ["rds"]
    regions: ["us-west-1"]
    tags:
      "env": "prod"
  - types: ["redshift"]
    regions: ["us-east-1", "us-east-2"]
    tags:
      "env": "stage"
```

The agent will use RDS' `DescribeDBClusters`[2] and `DescribeDBInstances`[3]
APIs and Redshift's `DescribeClusters`[4] API for discovery. It will require the
agent's IAM policy and permission boundary to include `rds:DescribeDBClusters`,
`rds:DescribeDBInstances` and `redshift:DescribeClusters` permissions.

Initially, the database agent will scan the resources on a regular interval to
see if any new instances matching the criteria have appeared, or any existing
instances have been updated or deleted. In future, the agent may implement a
more sophisticated resource watching on top of AWS Event Bridge (former Cloud
Watch Events) and SNS/SNQ APIs, which will require the agent to have more AWS
permissions and is a significantly more complicated approach.

The agent will register and unregister database servers based on the detected
RDS and Redshift instances. A registered database resource will have the same
name as RDS instance or Redshift cluster it represents and static labels taken
from the instance's AWS tags and instance information.

Example:

```yaml
kind: db
version: v3
metadata:
  name: postgres-rds
  description: RDS instance in us-west-1
  labels:
    account-id: "1234567890"
    engine: postgres
    engine-version: "13.3"
    region: us-west-1
    teleport.dev/origin: cloud
spec:
  protocol: postgres
  uri: postgres-rds.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432
  aws:
    account_id: "1234567890"
    region: us-west-1
    rds:
      iam_auth: true
      instance_id: postgres-rds
      resource_id: db-ABCDEFGHIJKLMNOP
```

Monitoring behavior:

- Agent will start proxying new databases matching tags selector.
- Agent will update tags on the already proxied databases.
  - If a database no longer matches, agent will stop proxying it.
- Agent will stop proxying databases that were deleted.

## Footnotes

[1] https://docs.aws.amazon.com/sdk-for-go/api/service/sts/#STS.GetCallerIdentity
[2] https://docs.aws.amazon.com/sdk-for-go/api/service/rds/#RDS.DescribeDBClusters
[3] https://docs.aws.amazon.com/sdk-for-go/api/service/rds/#RDS.DescribeDBInstances
[4] https://docs.aws.amazon.com/sdk-for-go/api/service/redshift/#Redshift.DescribeClusters

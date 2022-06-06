---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 73 - Teleport Discover

## Required approvers

@klizhentas && @xinding33 && @jimbishopp

## What

Proposes the `teleport discover` command that simplifies the UX for the
first-time users who are connecting their cloud resources to Teleport.

## Related RFDs

- [RFD 38: AWS databases auto-discovery](./0038-database-access-aws-discovery.md)
- [RFD 57: AWS EC2 nodes auto-discovery](https://github.com/gravitational/teleport/pull/12410)

## Why

The proposal is aimed at providing an easy way for Teleport administrators to
connect their cloud-hosted resources (EC2 instances, RDS databases, EKS clusters
an so on) to a Teleport cluster.

Over the past few releases Teleport has been adding automatic discovery
capabilities allowing it to find and register AWS databases, EC2 instances
and (WIP as of this writing) EKS clusters. Despite providing an improved UX
compared to registering the resources manually, connecting resources to cluster
and setting up auto-discovery remains cumbersome with multiple different
`teleport configure` commands and does not provide good visibility into the
discovered resources.

The `teleport discover` approach aims to take advantage of the existing auto
discovery mechanisms Teleport has and provide a unified approach for users to
enroll their cloud resources.

## Scope

- Works with both self-hosted edition and Teleport Cloud.
- Covers adding cloud resources such as EC2 instances, RDS databases and
  EKS clusters, which Teleport can automatically discover.
- Focuses on AWS as other clouds auto-discovery is not implemented yet.

## Prerequisites

In order to use `teleport discover` the user will need:

- Running Auth and Proxy, or a Cloud account.
- A node to run `teleport discover` command from. The node must have IAM
  identity allowing it to perform high-privilege operations in AWS account.
- When connecting databases, at least one EC2 instance is required. The
  instance will be used to run database agent.

## Command

Eventually `teleport discover` will support multiple cloud providers. Given
that different cloud providers often have different concepts and different
names for similar things, it may be hard to unify them all under a single
`teleport discover` command. Hence, the proposal is to have a family of
subcommands for different cloud providers:

```
$ teleport discover aws ...
$ teleport discover gcp ...
```

This RFD will focus on `teleport discover aws`.

## UX

Administrator setting up Teleport will use `teleport discover aws` command on a
node that has appropriate IAM permissions for the cloud account containing
resources they're intending to connect (we'll detail those permissions later).
It can be a laptop configured with appropriate credentials (e.g.
`~/.aws/credentials`) or a cloud instance with an IAM role.

Using AWS as an example, the command will:

1. Try to locate cloud credentials on the node it's running and understand the
   cloud identity it's running as.
2. Using the detected credentials, search the cloud account for the supported
   resources, matching some filters user can specify on the CLI.
3. Display the discovered resources to the user and ask for the confirmation if
   they look correct.
4. Once confirmed, the command will execute the following depending on the
   resource type.

For EC2 nodes:

- Using AWS SSM, will install a Teleport SSH agent on each EC2 instance using
  the same approach as used for auto-discovery described in RFD 57.
- Optionally, if auto-discovery is enabled via a CLI flag:
  - Enable it in the config of one of deployed agents. The instance that runs
    discovery can be specified explictly, otherwise will be picked at random.
  - Update IAM permissions for the IAM role attached to the instance, allowing
    it to run auto-discovery.

For RDS and other AWS databases:

- Will register discovered databases in Teleport using dynamic resource
  registration. Will require appropriate Teleport permissions.
- Using AWS SSM, will install a Teleport database agent on one of the EC2 nodes.
  User can designate a specific instance for it, or the command can pick one at
  random if there are matching EC2 nodes.
- Optionally, if auto-discovery is enabled via a CLI flag:
  - Enable it in the config of the deployed database agent.
  - Update IAM permissions for the database agent IAM role so it can run the
    discovery.

For EKS:

No details yet as it's not been implemented.

Usage example:

```sh
$ tsh login --proxy=proxy.example.com
$ tctl tokens add --type=node,db
$ teleport discover aws --proxy=proxy.example.com --token=xxx
🔑 Found AWS credentials, account 1234567890, user alice
🔍 Looking for EC2, RDS, Redshift and EKS resources in us-east-1, us-east-2 regions
🔍 Hint: use --types and --regions flags to narrow down the search
🔍 Found the following matching resources:

Type          Name/ID            Region      Tags
---------------------------------------------------------------------
AWS EC2       node-1/i-12345     us-east-1   env:prod,os:ubuntu
AWS EC2       node-2/i-67890     us-east-1   env:prod,os:centos
AWS EC2       node-1/i-54321     us-west-2   env:test
AWS RDS       mysql-prod         us-east-1   env:prod,engine:mysql
AWS RDS       postgres           us-east-1   env:test,engine:postgres
AWS Redshift  redshift-1         us-west-2   team:warehouse

❓ Import everything? yes/no
❓ Confirm the instance that will run the database service [node-1/i-12345]: <Enter>
🔨 Updating IAM permissions for auto-discovery on SSH service node [node-1/i-12345]... ✅
🔨 Updating IAM permissions for auto-discovery on database service node [node-1/i-12345]... ✅
🚜 Installing Teleport SSH and database service on [node-1/i-12345] in us-east-1... ✅
🚜 Installing Teleport SSH service on [node-2/i-67890] in us-east-1... ✅
🚜 Installing Teleport SSH service on [node-1/i-54321] in us-west-2... ✅
🛢 Registering RDS MySQL database [mysql-prod] from us-east-1... ✅
🛢 Registering Aurora PostgreSQL database [postgres] from us-east-1... ✅
🛢 Registering Redshift database [redshift-1] from us-west-2... ✅
🎉 Done!
```

Optional command flags:

Filter flags
------------

`--regions`     | Cloud regions to search in. Defaults to US regions.
`--types`       | Resource types to consider. Defaults to `ec2`, `rds`, `redshift`, `eks` (when implemented).
`--labels`      | Labels (tags in AWS) resources should match. Defaults to `*: *`.
`--ec2`         | Selectors for EC2 nodes e.g. `us-east-1:env:prod` or `us-west-2:*:*`.
`--rds`         | Selectors for RDS databases.
`--redshift`    | Selectors for Redshift clusters.
`--elasticache` | Selectors for Elasticache clusters.
`--eks`         | Selectors for EKS clusters (when implemented).

Database flags
--------------

`--database-node` | Instance that will run database agent. If not provided, will be picked from one of EC2 nodes.

Auto-discovery flags
--------------------

`--enable-discovery`   | Enables auto-discovery on the agents with the same filters the command is run with.
`--ssh-discovery-node` | Instance that will run SSH auto-discovery. If not provided, will be picked from one of EC2 nodes.

## Security

The node where the user runs `teleport discover` will need to have the following
AWS IAM permissions:

* List/describe permissions for EC2, RDS and other resources user wants to connect.
* SSM permissions to be able to run commands on EC2 instances to install agents.
* IAM permissions to be able to setup IAM permissions for auto-discovery.

The discover command will never ask the user to enter any credentials and instead
rely on standard cloud provider credential chain.

## Database access

To connect database resources, `teleport discover` will install a database
agent.

If there are matching EC2 instances, the command can pick one of the instances
at random. This will assume that the selected instance is able to access the
databases. A user can select an instance to run database agent explicitly via
a `--database-node` flag.

## Auto-discovery

When auto-discovery is requested, in addition to installing SSH agents and a
database agent, `teleport discover` will enable auto-discovery in the agents'
configuration.

For EC2 auto-discovery, Teleport will pick one of the instances at random, or
a user can specify one explicitly using `--ssh-discovery-node` flag and include
the following in its config:

```yaml
ssh_service:
  enabled: "yes"
  aws:
  - types: ["ec2"]
    regions: ["us-east-1"]
    tags:
      "*": "*"
```

For database auto-discovery, it will be enabled in the configuration of the
deployed database agent (as describe above):

```yaml
db_service:
  enabled: "yes"
  aws:
  - types: ["rds", "redshift", ...]
    regions: ["us-east-1"]
    tags:
      "*": "*"
```

### IAM

Auto-discovery requires specific IAM permissions on the node that runs an agent
performing discovery.

Once `teleport discover` picks a node that will run auto-discovery (either by
user-provided CLI flag or at random), it will determine the IAM role attached
to it using `IamInstanceProfile` instance property and then querying that
instance profile:

```sh
$ aws ec2 describe-instances --region=us-east-1
$ aws iam get-instance-profile --instance-profile-name XXX --region=us-east-1
```

Once the IAM role name is determined, `teleport discover` will use the same
mechanism for attaching appropriate IAM roles used by `teleport db configure`
and `teleport ssh configure` commands.

## Teleport Cloud / UI

`teleport discover` works with both self-hosted and Cloud versions of Teleport
as it only requires Auth and Proxy.

The discover flow is CLI-driven and due to the security constraints (i.e. no
access to cloud account credentials) Teleport Web UI cannot run the discovery
for the user.

Web UI can be updated to include a wizard-like interface guiding the user
through the steps necessary to connect resources to the cluster:

Step 1. Select the cloud provider to connect resources from.
Step 2. Select resource types to connect from the supported list.
Step 3. Display IAM policy required to successfully execute discovery.
Step 4. Display `teleport discover` command for user to run.

## Scenarios

### Default behavior

User runs default discover command which discovers all EC2 instances, databases
and EKS clusters in US regions.

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx
```

### Database node

User wants to select the node that runs database agent explicitly.

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx \
    --database-node=i-12345
```

### Auto-discovery

User wants to select resources matching particular labels and enable discovery.

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx \
    --labels=teleport:true \
    --enable-discovery
```

### Multiple accounts

User wants to connect resources from multiple accounts and runs discovery twice.

```
$ AWS_PROFILE=account-a teleport discover aws --proxy=proxy.example.com --token=xxx \
    --regions=us-east-1,us-east-2

$ AWS_PROFILE=account-b teleport discover aws --proxy=proxy.example.com --token=xxx \
    --regions=us-west-1,us-west-2
```

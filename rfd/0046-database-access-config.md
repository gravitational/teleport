---
authors: Roman Tkachenko (roman@goteleport.com), Gabriel Corado (gabriel.oliveira@goteleport.com)
state: implemented
---

# RFD 46 - Database access configurator

## What

This RFD proposes UX for Teleport's database access configurator CLI tool.

The tool will help users bootstrap their database agents with appropriate cloud
permissions and configure self-hosted databases to work with database access.

## Why

The main purpose of the tool is to improve "time-to-first-value" for database
access users.

[RFD38](./0038-database-access-aws-discovery.md) implemented automatic discovery
and IAM configuration for AWS hosted databases, however database agents still
need to be configured with proper IAM permissions in order to take advantage
of these features. Currently, Teleport does not provide any tools that simplify
this initial IAM bootstrap besides documentation guides.

The same applies to self-hosted databases and other clouds. Instructions vary
greatly between different database deployment models (self-hosted, cloud) and
authentication types (X.509, IAM, AD) and having a tool that can guide users
through the initial setup would significantly improve the getting started
experience.

## Scope

The initial implementation will focus on:

1. Generate database agent configuration with samples;
2. Bootstrap the agent's AWS IAM policies but we'll make the effort to design
   the CLI interface in a way which would allow for extensibility to support
   other scenarios (self-hosted, other clouds and auth types);

## Prior art

Kubernetes cluster API provider includes a set of `clusterawsadm bootstrap iam`
commands we could take some inspiration from:

https://cluster-api-aws.sigs.k8s.io/clusterawsadm/clusterawsadm_bootstrap_iam.html

For instance, it has commands for bootstrapping IAM policies using CloudFormation
stack:

```bash
$ clusterawsadm bootstrap iam create-cloudformation-stack
```

To use this utility, credentials of the administrative AWS account must be
loaded in the environment the command is run.

It also has commands for displaying necessary policies for users to create
themselves:

```bash
$ clusterawsadm bootstrap iam print-policy --document AWSIAMManagedPolicyControllers
```

## UX

Teleport has a `teleport configure` command that generates a sample config file.

Database agent bootstrap commands will reside under teleport db configure family
of subcommands. This aligns with our strategy of placing commands related to a
particular service under its subcommand namespace. It will provide subcommands
for generating configuration files and bootstrapping database agents’ necessary
configurations, for example, setting up IAM permissions for agents to proxy
AWS-hosted databases.

### Create config subcommand

Similar to `teleport configure` but focused on configurations for Database
agents. This command will generate a configuration file. It can also provide
some examples of configuration, for instance, self-hosted databases setup.

There will be flags to enable features like Aurora/RDS auto-discovery and
dynamic registration. Each sample will show the required options with some
explanation on how to fill them and documentation links.

It will also provide flags to configure a specific database (similar to the
flags present on the `teleport db start` command). When executed with these
flags, the samples will not be included, only the static configuration of a
single database.

```bash
teleport db configure create [--proxy] [--token=] [--discovery-rds=] [--discovery-redshift] [--output=] [--ca-pin=] [--name=] [--protocol=] [--uri=] [--labels=]
```

| Flag                            | Description |
| ------------------------------- | ----------- |
| `--proxy`                       | Teleport proxy address to connect to [0.0.0.0:3080]. |
| `--token`                       | Invitation token to register with an auth server [none]. |
| `--discovery-rds`               | List of AWS regions the agent will discover for RDS/Aurora instances. |
| `--discovery-redshift`          | List of AWS regions the agent will discover for Redshift instances. |
| `--output`                      | Write to stdout with -o=stdout, default config file with -o=file or custom path with -o=file:///path |
| `--ca-pin`                      | CA pin to validate the auth server (can be repeated for multiple pins). |
| `--name`                        | Name of the proxied database. |
| `--protocol`                    | Proxied database protocol. Supported are: [postgres mysql mongodb cockroachdb]. |
| `--uri`                         | Address the proxied database is reachable at. |
| `--description`                 | Description of the proxied database. Default: "" |
| `--labels`                      | Comma-separated list of labels for the database, for example env=dev,dept=it |

* When configuring a single database, `name`, `protocol` and `uri` are required;

**Examples:**

None of the flags are required. Running the command without flags should output
a configuration with samples.

```bash
# will generate a configuration file for the database agent.
$ teleport db configure create

#
# Teleport database agent configuration file.
# Configuration reference: https://goteleport.com/docs/database-access/reference/configuration/
#
version: v2
teleport:
  nodename: localhost
  data_dir: /var/lib/teleport
  auth_token: /tmp/token
  auth_servers:
  - 127.0.0.1:3025
db_service:
  enabled: "yes"
  # Matchers for database resources created with "tctl create" command.
  # For more information: https://goteleport.com/docs/database-access/guides/dynamic-registration/
  resources:
  - labels:
      "*": "*"
  # Lists statically registered databases proxied by this agent.
  # databases:
  # # RDS database static configuration.
  # # RDS/Aurora databases Auto-discovery reference: https://goteleport.com/docs/database-access/guides/rds/
  # - name: rds
  #   description: AWS RDS/Aurora instance configuration example.
  #   # Supported protocols for RDS/Aurora: "postgres" or "mysql"
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database Service.
  #   uri: rds-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # RDS/Aurora specific configuration.
  #     rds:
  #       # RDS Instance ID. Only present on RDS databases.
  #       instance_id: rds-instance-1
  # # Aurora database static configuration.
  # # RDS/Aurora databases Auto-discovery reference: https://goteleport.com/docs/database-access/guides/rds/
  # - name: aurora
  #   description: AWS Aurora cluster configuration example.
  #   # Supported protocols for RDS/Aurora: "postgres" or "mysql"
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database Service.
  #   uri: aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # RDS/Aurora specific configuration.
  #     rds:
  #       # Aurora Cluster ID. Only present on Aurora databases.
  #       cluster_id: aurora-cluster-1
  # # Redshift database static configuration.
  # # For more information: https://goteleport.com/docs/database-access/guides/postgres-redshift/
  # - name: redshift
  #   description: AWS Redshift cluster configuration example.
  #   # Supported protocols for Redshift: "postgres".
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: redshift-cluster-example-1.abcdefghijklmnop.us-west-1.redshift.amazonaws.com:5439
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # Redshift specific configuration.
  #     redshift:
  #       # Redshift Cluster ID.
  #       cluster_id: redshift-cluster-example-1
  # # Self-hosted static configuration.
  # - name: self-hosted
  #   description: Self-hosted database configuration.
  #   # Supported protocols for self-hosted: postgres, mysql, mongodb, cockroachdb.
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: database.example.com:5432
auth_service:
  enabled: "no"
ssh_service:
  enabled: "no"
proxy_service:
  enabled: "no"
```

Similarly to configure command, it will also support writing the configuration
directly to a file.

```bash
# write to the default file location.
$ teleport db configure create --output file

Wrote config to file "/etc/teleport.yaml". Now you can start the server. Happy Teleporting!
```

```bash
# write to the provided file location.
$ teleport db configure create --output file:///teleport.yaml

Wrote config to file "/teleport.yaml". Now you can start the server. Happy Teleporting!
```

Configure a single database.

```bash
# generates a configuration for a Postgres database.
$ teleport db configure create \
   --token=/tmp/token \
   --auth-server=localhost:3025 \
   --name=sample-db \
   --protocol=postgres \
   --uri=postgres://localhost:5432
```

### Bootstrap permissions command

Subcommand will bootstrap the necessary configuration for the database agent. It
reads the provided agent configuration to determine what will be bootstrapped.

```bash
$ teleport db configure bootstrap [-c=,--config=] [--manual] [--policy-name=] [--attach-to-user=] [--attach-to-role=] [--confirm]
```

| Flag               | Description |
| ------------------ | ----------- |
| `-c, --config`     | Path to the database agent configuration file. Default: "/etc/teleport.yaml". |
| `--manual`         | When executed in "manual" mode, it will print the instructions to complete the configuration instead of applying them directly. |
| `--policy-name`    | Name of the Teleport Database agent policy. Default: "DatabaseAccess". |
| `--attach-to-role` | Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.|
| `--attach-to-user` | User name to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials. |
| `--confirm`        | Do not prompt the user and auto-confirm all actions. Defaults to false. |

**Examples:**

```bash
# When the database configuration has RDS/Aurora or Redshift auto-discovery.
# Configure AWS provider attaching the created policies to “SampleUser” when confirmation is given.
$ teleport db configure bootstrap --attach-to-user SampleUser
Reading configuration at "/etc/teleport.yaml"...

AWS
1. Create IAM policy "DatabaseAccess":
{ ...policy document... }

2. Create IAM boundary policy "DatabaseAccessBoundary":
{ ...policy document... }

3. Attach policy and boundary to "SampleUser".

Confirm actions? [y/N] y
✅[AWS] Creating IAM Policy "DatabaseAccess"... done.
✅[AWS] Creating IAM policy boundary "DatabaseAcessBoundary"... done.
✅[AWS] Attaching IAM policy and boundary to IAM user "SampleUser"... done.
```

```bash
# When the database configuration has RDS/Aurora or Redshift auto-discovery.
# Configure AWS provider attaching the created policies to “SampleUser”.
$ teleport db configure bootstrap --attach-to-user SampleUser --confirm
Reading configuration at "/etc/teleport.yaml"...

AWS
1. Create IAM policy "DatabaseAccess":
{ ...policy document... }

2. Create IAM boundary policy "DatabaseAccessBoundary":
{ ...policy document... }

3. Attach policy and boundary to "SampleUser".

✅[AWS] Creating IAM Policy "DatabaseAccess"... done.
✅[AWS] Creating IAM policy boundary "DatabaseAcessBoundary"... done.
✅[AWS] Attaching IAM policy and boundary to IAM user "SampleUser"... done.
```

```bash
# When the database configuration has RDS/Aurora or Redshift auto-discovery.
# Show instructions on how to configure AWS.
$ teleport db configure bootstrap --manual --attach-to-user SampleUser
Reading configuration at "/etc/teleport.yaml"...
Running in "manual" mode, there will be presented the instructions to bootstrap the agent.

AWS
1. Create IAM policy "DatabaseAccess":
{ ...policy document... }

2. Create IAM boundary policy "DatabaseAccessBoundary":
{ ...policy document... }

3. Attach policy and boundary to "username".
```

```bash
# When the database agent has a configuration that requires no bootstrapping.
$ teleport db configure bootstrap
Reading configuration at "/etc/teleport.yaml"...
Nothing to be bootstrapped.
```

#### Error handling:

No external calls will be issued when running with `--mode=manual`, meaning the
command won't fail. In automatic mode, however, if the command fails, the error
will be presented.

**Examples:**

```bash
# Fails to attach the created policy into the user.
$ teleport db configure bootstrap --attach-to-user SampleUser
Reading configuration at "/etc/teleport.yaml"...

AWS
1. Create IAM policy "DatabaseAccess":
{ ...policy document... }

2. Create IAM boundary policy "DatabaseAccessBoundary":
{ ...policy document... }

3. Attach policy and boundary to "SampleUser".

Confirm actions? [y/N] y
✅[AWS] Creating IAM Policy "DatabaseAccess"... done.
✅[AWS] Creating IAM policy boundary "DatabaseAcessBoundary"... done.
❌[AWS] Attaching IAM policy and boundary to IAM user "SampleUser"... failed.
Failure reason: unable to find user "SampleUser".
```

### AWS subcommand

```bash
$ teleport db configure aws ...
```

Similar to `clusterawsadm` mentioned above, `teleport db configure aws` will
have two modes of operation:

- Automatic, in which it will attempt to auto-configure IAM for agent itself:

```bash
$ teleport db configure aws create-iam
```

- Manual, in which it will display required IAM policies for user to create:

```bash
$ teleport db configure aws print-iam
```

#### Automatic mode

To use automatic mode, user needs to run `teleport db configure aws` command
on the machine with permissions to create and attach IAM policies.

**Note:** The command **will not** ask the user for their AWS credentials.
Instead, it will rely on default AWS credential provider chain used by all our
AWS clients (env vars -> credentials file -> IAM role).

The command will:

1. Create IAM policy with requested permissions.
2. Create IAM policy boundary with requested permissions.
3. Attach policy and boundary to the specified IAM identity.

```bash
$ teleport db configure aws create-iam [--name=] [--types=] [--attach] [--role=|--user=] [--confirm]
```

| Flag        | Description |
| ----------- | ----------- |
| `--name`    | Created policy name. Defaults to empty. Will be auto-generated if not provided. |
| `--types`   | Permissions to include in the policy. Any of `rds`, `aurora`, `redshift`. Defaults to none. |
| `--attach`  | Try to attach the policy to the IAM identity. Defaults to `true`. |
| `--role`    | IAM role name to attach policy to. Mutually exclusive with `--user`. Defaults to empty. |
| `--user`    | IAM user name to attach policy to. Mutually exclusive with `--role`. Defaults to empty. |
| `--confirm` | Do not prompt user and auto-confirm all actions. Defaults to `false`. |

* At least one type must be specified via `--types` flag.
* If neither `--role` nor `--user` is specified, tries to attach the policy to the identity of current AWS credentials.
* Unless `--confirm` flag is provided, command will prompt user before creating and attaching policies.

**Examples:**

Create and attach policy for the database agent that runs as an IAM user and is
proxying Aurora databases:

```bash
# will create policy/boundary and attach to "alice" IAM user
$ teleport db configure aws create-iam --types=aurora --user=alice
```

Example output:

```
Will create IAM policy DatabaseAccess:

{ // policy document }

Confirm? yes
✅ Creating IAM policy DatabaseAccess... done

Will create IAM policy boundary "DatabaseAccessBoundary":

{ // policy boundary document }

Confirm? yes
✅ Creating IAM policy boundary DatabaseAccessBoundary... done

Will attach IAM policy and boundary to IAM user "alice". Confirm? yes
✅ Attaching IAM policy and boundary to IAM user "alice"... done
```

Running configure command on the instance where database agent will be running
(instance must have IAM role attached with administrative permissions):

```bash
# will create policy/boundary and attach to the current IAM role of the instance with auto-confirmation
$ teleport db configure aws create-iam --types=rds,aurora,redshift --confirm
```

Example output:

```
✅ Creating IAM policy DatabaseAccess... done
✅ Creating IAM policy boundary DatabaseAccessBoundary... done
✅ Attaching IAM policy and boundary to DatabaseAgentRole... done
```

##### Error handling

In case the automatic mode encounters a permission or any other kind of error
(e.g. policy doesn't allow creating or attaching other policies), it falls back
to manual mode where it prints:

- IAM permissions it requires to be able to create and attach these policies.
- Policy and boundary in case user wants to create them themselves.

#### Manual mode

In manual mode configurator only prints the policy and boundary. User is
responsible for creating and attaching them to IAM identity themselves.

By default the command will print instructions with the policy and boundary
to create and identity to attach them to. Flags are provided to only print
respective policy/boundary documents, suitable for automation tools.

```bash
$ teleport db configure aws print-iam [--types=] [--role=|--user=] [--policy|--boundary]
```

| Flag         | Description |
| ------------ | ----------- |
| `--types`    | Permissions to include in the policy. Any of `rds`, `aurora`, `redshift`. Defaults to none. |
| `--role`     | IAM role name to attach policy to. Mutually exclusive with `--user`. Defaults to empty. |
| `--user`     | IAM user name to attach policy to. Mutually exclusive with `--role`. Defaults to empty. |
| `--policy`   | Only print IAM policy document. Defaults to `false`. |
| `--boundary` | Only print IAM boundary policy document. Defaults to `false`. |

**Examples:**

Print policy and boundary for the IAM role database agent is running as:

```bash
$ teleport db configure aws print-iam --types=rds,aurora
```

Example output:

```
1. Create the following IAM policy named "DatabaseAccess":

{ // policy document }

2. Create the following IAM policy named "DatabaseAccessBoundary":

{ // policy boundary document }

3. Attach policy "DatabaseAccess" and boundary "DatabaseAccessBoundary" to IAM role "example".
```

Print only the policy document:

```bash
$ teleport db configure aws print-iam --policy
{ // policy document }
```

Print only the policy boundary document:

```bash
$ teleport db configure aws print-iam --boundary
{ // policy boundary document }
```

## Future work

In future `teleport db configure` commands can be extended to support bootstrapping
agents and databases in other configurations, for example:

```bash
# GCP-related configurations
$ teleport db configure gcp ...

# Azure-related configurations
$ teleport db configure azure ...

# configurations for self-hosted databases
$ teleport db configure postgres ...
$ teleport db configure mysql ...
```

## Security

The bootstrap command doesn’t communicate with Teleport instances. However, it
makes calls to the AWS API. As mentioned earlier, the command won’t request the
AWS credentials, and it relies on the [default AWS credential provider
chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).

### Necessary permissions
When executed in "automatic" mode, the bootstrap command will require the
current identity to have the following permissions:

**Policy-related:**
- `iam:GetPolicy` wildcard ("*") or the policy to be retrieved: Used to identify
  if a policy already exists to update it.
- `iam.ListPolicyVersions`: wildcard ("*") or the policy to be retrieved: Used
  to identify if the policy exceeds the policy versions limit;
- `iam:CreatePolicy` wildcard ("*") or policy that will be created: Used when
  the policy doesn’t exist.
- `iam:DeletePolicyVersion` wildcard ("*") or policy that will be created: Used
  when the policy exceeds the version’s limit. It will then delete the oldest
  non-default version.
- `iam:CreatePolicyVersion` wildcard ("*") or policy that will be created: Used
  when the policy already exists and will be updated;

**Identity-related:**
- `iam:AttachUserPolicy` wildcard ("*") or provided user identity: Needed to
  attach the policy to the user. Only necessary when attaching policies to a
  user;
- `iam:AttachRolePolicy` wildcard ("*") or provided role identity: Needed to
  attach the policy to the role. Only necessary when attaching policies to a
  role;
- `iam:PutUserPermissionsBoundary` wildcard ("*") or provided user identity:
  Needed to attach boundary policy to the user. Only necessary when attaching
  policies to a user;
- `iam:PutRolePermissionsBoundary` wildcard ("*") or provided role identity:
  Needed to attach boundary policy to the role. Only necessary when attaching
  policies to a role;

### IAM policies
The IAM policies created by the bootstrap command will follow the AWS
recommendation of [least privilege](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#grant-least-privilege),
meaning that the Teleport database access policies will be granted permissions
based on the agent configuration. For example, the policy won’t include
RDS/Aurora permissions if the agent only has Redshift databases configured.

#### RDS databases and Aurora clusters
If the configuration has any static RDS/Aurora database or auto-discovery
enabled, the following permissions will be added:

- `rds:DescribeDBInstances`: Is a list operation, only supporting wildcard resources;
- `rds:ModifyDBInstance`: Since the databases that will be modified are only
  known during the discovery process or configured using dynamic registration,
  we need wildcard access;
- `rds:DescribeDBClusters`: Is a list operation, only supporting wildcard
  resources;
- `rds:ModifyDBCluster`: Since the clusters that will be modified are only known
  during the discovery process or configured using dynamic registration, we
  need wildcard access;

##### Boundary
- `rds-db:connect`: The list of database/clusters the Teleport agent will
  connect to is only known after the discovery; we need wildcard access;

#### Redshift clusters
If the configuration has any static Redshift cluster or auto-discovery enabled,
the following permissions will be added:

- `redshift:DescribeClusters`: Is a list operation, only supporting wildcard
  resources.

##### Boundary
- `redshift:GetClusterCredentials`: The list of clusters the Teleport agent will
  get credentials to is only known after the discovery or configured using
  dynamic registration; we need wildcard access;

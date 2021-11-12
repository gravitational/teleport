---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
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
through the initial setup would significanly improve the getting started
experience.

## Scope

The initial implementation will focus on bootstrapping the agent's AWS IAM
policies but we'll make the effort to design the CLI interface in a way which
would allow for extensibility to support other scenarios (self-hosted, other
clouds and auth types).

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

Database agent bootstrap commands will reside under `teleport db configure`
family of subcommands. This aligns with our strategy of placing commands related
to a particular service under its own subcommand namespace.

The "configure" command will provide subcommands for configuring database agents
in different environments. This RFD focuses on `aws` subcommands that deal with
configuring IAM for agents that proxy AWS-hosted databases (RDS, Aurora,
Redshift).

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

### Automatic mode

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

| Flag          | Description |
| ------------- | ----------- |
| `--name`      | Created policy name. Defaults to empty. Will be auto-generated if not provided. |
| `--types`     | Permissions to include in the policy. Any of `rds`, `aurora`, `redshift`. Defaults to none. |
| `--attach`    | Try to attach the policy to the IAM identity. Defaults to `true`. |
| `--role`      | IAM role name to attach policy to. Mutually exclusive with `--user`. Defaults to empty. |
| `--user`      | IAM user name to attach policy to. Mutually exclusive with `--role`. Defaults to empty. |
| `--confirm`   | Do not prompt user and auto-confirm all actions. Defaults to `false`. |

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

#### Error handling

In case the automatic mode encounters a permission or any other kind of error
(e.g. policy doesn't allow creating or attaching other policies), it falls back
to manual mode where it prints:

- IAM permissions it requires to be able to create and attach these policies.
- Policy and boundary in case user wants to create them themselves.

### Manual mode

In manual mode configurator only prints the policy and boundary. User is
responsible for creating and attaching them to IAM identity themselves.

By default the command will print instructions with the policy and boundary
to create and identity to attach them to. Flags are provided to only print
respective policy/boundary documents, suitable for automation tools.

```bash
$ teleport db configure aws print-iam [--types=] [--role=|--user=] [--policy|--boundary]
```

| Flag          | Description |
| ------------- | ----------- |
| `--types`     | Permissions to include in the policy. Any of `rds`, `aurora`, `redshift`. Defaults to none. |
| `--role`      | IAM role name to attach policy to. Mutually exclusive with `--user`. Defaults to empty. |
| `--user`      | IAM user name to attach policy to. Mutually exclusive with `--role`. Defaults to empty. |
| `--policy`    | Only print IAM policy document. Defaults to `false`. |
| `--boundary`  | Only print IAM boundary policy document. Defaults to `false`. |

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

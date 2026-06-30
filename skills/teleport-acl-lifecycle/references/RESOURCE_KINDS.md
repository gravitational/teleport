# Resource Kinds

Do not choose commands from memory. Use this file to answer:

- which resource kind the user means
- which access-list flag grants it
- how to list or preview matching resources before create/update

## Kind Map

| User says | Resource kind | Grant flag | Principal / identity flags |
| --- | --- | --- | --- |
| servers, nodes, SSH | SSH server | `--node-labels` | `--logins` |
| databases, DBs | Database | `--db-labels` | `--db-users`, `--db-names` |
| kube, Kubernetes, k8s, clusters | Kubernetes cluster | `--kubernetes-labels` | `--kubernetes-users`, `--kubernetes-groups` |
| apps, applications, web apps, TCP apps, cloud apps, MCP apps | Application | `--app-labels` | see Application Identities |
| Windows, desktops | Windows desktop | `--windows-labels` | `--windows-logins` |
| AWS IC, AWS Identity Center, permission sets | AWS Identity Center assignment | `--aws-ic-assignments` | encoded in assignment |
| git, GitHub | Git server / GitHub org | `--github-orgs` | none |

SSH servers, databases, Kubernetes clusters, applications, and Windows desktops
are granted by label selector. AWS IC and Git server are granted by exact
identifier.

## Resource Offer List

When asking what access should be granted, offer these choices in this order:

```text
- SSH servers
- Databases
- Kubernetes clusters
- Applications (web, TCP, cloud, MCP)
- Windows desktops
- AWS Identity Center accounts or permission sets
- Git servers / GitHub orgs
```

## Listing Resources

Use this section only to answer "what is available in my cluster?" or to look up
candidate resources before the user chooses access scope. Listing is read-only.
Do not turn a listing result into a grant until the user picks labels or exact
identifiers.

### 1. Simple Listing

Use these when the user asks for a kind without a filter.

| Kind | Command | Columns to show |
| --- | --- | --- |
| SSH server | `$TCTL nodes ls --format=json` | Hostname, Address, Labels |
| Database | `$TCTL db ls --format=json` | Name, Protocol, Labels |
| Kubernetes | `$TCTL kube ls --format=json` | Name, Labels |
| Application | `$TCTL apps ls --format=json` | Name, Type, Public Address, Labels |
| Windows desktop | `$TCTL get windows_desktop --format=json` | Name, Address, Labels |
| AWS IC | `$TCTL integrations awsic accounts ls --format=json` | Account Name, Account ID, Permission Set Name, Permission Set ARN |
| Git server | `$TCTL get git_server --format=json` | GitHub Org |

### 2. Server-Side Search

Use `--search` when the command supports it and the user gives a free-text term.

| Kind | Command |
| --- | --- |
| SSH server | `$TCTL nodes ls --search='<term>' --format=json` |
| Database | `$TCTL db ls --search='<term>' --format=json` |
| Kubernetes | `$TCTL kube ls --search='<term>' --format=json` |
| Application | `$TCTL apps ls --search='<term>' --format=json` |

Do not invent `--search` for AWS IC, Windows desktops, or Git servers.

### 3. Client-Side Filtering

Use client-side filtering when the command has no `--search` or `--query`.
Fetch the normal listing, then filter in a read-only parser.

| Kind | Filter by |
| --- | --- |
| Windows desktop | `metadata.name`, `metadata.labels`, `spec.addr` |
| Git server | `spec.github.organization`, `spec.github.integration` |
| AWS IC | account name, account ID, permission set name, permission set ARN |

AWS IC has no `--search` or `--query`, so fetch JSON and filter the assignment
records client-side. Each JSON item is one account/permission-set assignment:
use `spec.account_id`, `spec.account_name`, `spec.permission_set.name`, and
`spec.permission_set.arn`.

For every matching assignment record, build and retain the exact assignment:

```text
accountID^permissionSetARN
```

If the user asks for "every permission set", "all permission sets", or "every
permission set in every account", expand the request to every returned
assignment record. Do not ask the user to re-pick permission sets that they
already selected with "every". The response or draft must include the exact
assignment strings, not only account or permission-set display names.

## Application Search

Applications include plain web/TCP apps, cloud apps, and MCP apps. AWS IC has a
dedicated listing command; do not look it up with `tctl apps ls`.

If the user asks for an application subtype, use a predicate query instead of
broad text search.

| User asks for | Command |
| --- | --- |
| AWS console apps / AWS apps | `$TCTL apps ls --query='resource.spec.cloud == "AWS"' --format=json` |
| Azure apps | `$TCTL apps ls --query='resource.spec.cloud == "Azure"' --format=json` |
| GCP apps | `$TCTL apps ls --query='resource.spec.cloud == "GCP"' --format=json` |
| MCP apps / MCP servers | `$TCTL apps ls --query='resource.sub_kind == "mcp"' --format=json` |
| AWS IC accounts / permission sets | `$TCTL integrations awsic accounts ls --format=json` |
| regular apps excluding AWS IC | `$TCTL apps ls --query='labels["teleport.dev/origin"] != "aws-identity-center"' --format=json` |

Inside `--query`, app fields are evaluated against the unwrapped app:
`resource.spec.cloud`, `resource.spec.uri`, `resource.sub_kind`.

In `apps ls --format=json` output, the returned records are app servers, so the
same fields are nested under `spec.app.spec.*` and `spec.app.sub_kind`.

## Selection And Preview

There are two selection models.

### Label-Selected Resources

SSH servers, databases, Kubernetes clusters, applications, and Windows desktops
are selected by labels. Once the user states or settles on labels, show a
preview of the label selector before asking for write approval.

Use `--query` for kinds that support predicate queries:

| Kind | Preview command |
| --- | --- |
| SSH server | `$TCTL nodes ls --query='<predicate>' --format=json` |
| Database | `$TCTL db ls --query='<predicate>' --format=json` |
| Kubernetes | `$TCTL kube ls --query='<predicate>' --format=json` |
| Application | `$TCTL apps ls --query='<predicate>' --format=json` |

Windows desktops do not support `--query`; fetch all desktops and filter labels
client-side:

```text
$TCTL get windows_desktop --format=json
```

Preview warning:

```text
This preview only shows resources your own roles let you see, so treat the count as a lower bound. Members may be able to reach more.
```

The grant is still the label flag, not the predicate:

```text
--node-labels="env=staging"
```

### Name-Selected Resources

AWS IC and Git server are not selected by labels for access-list grants.

- AWS IC grants use exact `accountID^permissionSetARN` assignments.
- Git grants use exact GitHub org names through `--github-orgs`.

After the user chooses one of these from a listing, no scoped preview is needed.
Confirm the exact selected value in the draft instead.

## Label And Predicate Syntax

Access-list label flags use comma-separated `key=value` pairs. Repeated key
means OR; different keys mean AND.

```text
--node-labels="env=staging,team=backend"   # env=staging AND team=backend
--node-labels="env=staging,env=dev"        # env is staging OR dev
```

`env=*` means the key exists. `*=*` matches everything. Treat broad selectors as
risky and include the warning in the final write approval.

Predicate syntax for preview/search uses resource expressions:

| Desired selector | Predicate |
| --- | --- |
| `env=staging` | `labels["env"] == "staging"` |
| `env=staging` AND `team=backend` | `labels["env"] == "staging" && labels["team"] == "backend"` |
| `env=staging` OR `env=prod` | `(labels["env"] == "staging" \|\| labels["env"] == "prod")` |
| key exists | `exists(labels["env"])` |
| match everything | omit `--query` |

Supported predicate commands: `tctl nodes ls`, `tctl db ls`, `tctl kube ls`, and
`tctl apps ls`.

## Application Identities

After previewing app matches, inspect the JSON and ask only for identity fields
that apply.

| Matched `apps ls --format=json` field | Ask for | Flag |
| --- | --- | --- |
| `spec.app.spec.cloud == "AWS"` or AWS console URI | AWS role ARNs | `--aws-role-arns` |
| `spec.app.spec.cloud == "Azure"` | Azure identities | `--azure-identities` |
| `spec.app.spec.cloud == "GCP"` | GCP service accounts | `--gcp-service-accounts` |
| `spec.app.sub_kind == "mcp"` or `spec.app.spec.mcp` is present | MCP tools | `--mcp-tools` |

Plain web/TCP apps do not need cloud or MCP identity flags. If matches are
mixed, ask for each relevant identity type.

## Rendering

Render from parsed JSON, not from truncated terminal text. `tctl` may return far
more rows than the command runner shows.

Client-page listings and previews at 10 rows unless the user explicitly asks for
all rows. State the shown range and true total, for example:

```text
Showing 1-10 of 200 matching nodes (page 1 of 20).
```

After a resource listing, ask what to do next:

- choose a specific resource
- give a label selector
- give a search/filter term
- see the next page, if more pages exist

For AWS IC and Git server, ask the user to choose exact assignments/orgs rather
than labels.

## AWS Identity Center

AWS IC grants use assignment strings:

```text
--aws-ic-assignments="123456789012^arn:aws:sso:::permissionSet/ssoins-XXXX/ps-YYYY"
```

The account may be `*`; the permission set must be a specific ARN. Do not grant
AWS IC with `--app-labels`, and do not set `teleport.dev/origin` yourself.

Never use a wildcard permission set such as `accountID^*` or `*^*`. If the user
asks for every permission set, list AWS IC accounts with `--format=json` and
expand to exact `accountID^permissionSetARN` assignments instead.

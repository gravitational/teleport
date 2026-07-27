# Resource Kinds

Do not choose commands from memory. Use this file to answer:

- which resource kind the user means
- which access-list flag grants it
- how to list or preview matching resources before create/update

## Contents

- Kind Map: map user language to resource kinds, grant flags, and identity flags.
- Resource Offer List: canonical choices when asking what access is needed.
- Listing Resources: read-only list/search/filter commands, display columns, and next actions.
- Application Classification: derive app Type from `tctl apps ls` JSON.
- Application Search: app subtype queries for cloud, MCP, AWS IC, and regular apps.
- Selection And Preview: label-selected vs exact-name grants and preview commands.
- Label And Predicate Syntax: access-list label flags and preview query syntax.
- Deriving Selectors From Listed Labels: quote or block untrusted label values.
- Application Identities: when to ask for AWS/Azure/GCP/MCP identity fields.
- Rendering: paginate parsed JSON output.
- AWS Identity Center: exact assignment string rules and wildcard restrictions.

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

When asking what access should be granted, offer these choices in this order.
Use this list for any create route whose access target is missing, including
generic prompts like "create an access list".

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
candidate resources before the user chooses a grant target. Listing is read-only.
Do not turn a listing result into a grant until the user picks labels or exact
identifiers. Do not include the lower-bound preview warning here, even though
the same visibility limit technically applies — that warning is specific to
Selection And Preview, once a selector is settled and a real grant decision is
imminent, not to browsing what's available.

For a listing-only route, render this menu after the table, then stop:

- give users access
- search/filter this listing
- see the next page, if more pages exist

A candidate table inside a create/update route ends with that route's current
selection question instead.

When listing candidates for a create route that started from plain-language
access wording or a post-listing next action, follow CREATE.md's
plain-language access bridge before rendering this listing.

The command tables show only the `tctl` portion. Run listing/getter commands
according to SECURITY.md's Read Capture Discipline before inspecting results.

Columns named for a kind, anywhere in this file, are exact — do not
substitute, reorder, or drop one for readability. Labels is what the user
needs to pick a label selector; Application's Type is what the user needs to
tell app subtypes apart. Show a different column set only if the user
explicitly asks.

### 1. Simple Listing

Use these when the user asks for a kind without a filter.

| Kind | tctl command portion | Columns to show |
| --- | --- | --- |
| SSH server | `$TCTL nodes ls --format=json` | Hostname, Address, Labels |
| Database | `$TCTL db ls --format=json` | Name, Protocol, URI, Labels |
| Kubernetes | `$TCTL kube ls --format=json` | Name, Labels |
| Application | `$TCTL apps ls --format=json` | Name, Type, Public Address, URI, Labels |
| Windows desktop | `$TCTL get windows_desktop --format=json` | Name, Address, Labels |
| AWS IC | `$TCTL integrations awsic accounts ls --format=json` | Account Name, Account ID, Permission Set Name, Permission Set ARN |
| Git server | `$TCTL get git_server --format=json` | GitHub Org |

Use this effective label set whenever displaying, filtering, or deriving a
label selector from listed resources:

| Kind | Static labels | Dynamic-label result |
| --- | --- | --- |
| SSH server | `metadata.labels` | `spec.cmd_labels.<key>.result`; `spec.immutable_labels` wins over both |
| Database | `spec.database.metadata.labels` | `spec.database.spec.dynamic_labels.<key>.result` |
| Kubernetes | `spec.cluster.metadata.labels` | `spec.cluster.spec.dynamic_labels.<key>.result` |
| Application | `spec.app.metadata.labels` | `spec.app.spec.dynamic_labels.<key>.result` |
| Windows desktop | `metadata.labels` | none |

For SSH, database, Kubernetes, and application labels, a dynamic result
overrides a static value with the same key. AWS IC and Git server are not
label-selected.

Read the non-label columns from these paths:

| Kind | Column paths |
| --- | --- |
| SSH server | hostname `spec.hostname`, address `spec.addr` |
| Database | name `spec.database.metadata.name`, protocol `spec.database.spec.protocol`, URI `spec.database.spec.uri` |
| Kubernetes | name `spec.cluster.metadata.name` |
| Application | name `spec.app.metadata.name`, public address `spec.app.spec.public_addr`, URI `spec.app.spec.uri` |
| Windows desktop | name `metadata.name`, address `spec.addr` |

URI and public-address keys are absent when unset. Application Type is derived,
not a literal field: use Application Classification.

### 2. Server-Side Search

Use `--search` when the command supports it and the user gives a free-text term.

| Kind | tctl command portion |
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
`spec.permission_set.arn` to build the exact `accountID^permissionSetARN`
assignment string for each matching record. See AWS Identity Center below for
wildcard restrictions and how to expand an "every permission set" request.

## Application Classification

Use this section to render Application's Type column and to tell web, TCP,
cloud, and MCP apps apart. `tctl apps ls --format=json` wraps each app in a
server record, so inspect the wrapped app fields.

Classify in this order:

| Type | Wrapped `apps ls --format=json` field |
| --- | --- |
| AWS app | `spec.app.spec.cloud == "AWS"` or AWS console URI |
| Azure app | `spec.app.spec.cloud == "Azure"` |
| GCP app | `spec.app.spec.cloud == "GCP"` |
| MCP app | `spec.app.sub_kind == "mcp"` or `spec.app.spec.mcp` is present |
| TCP app | `spec.app.spec.uri` starts with `tcp://` or `tls://` |
| Web app | `spec.app.spec.uri` starts with `http://` or `https://`, after the checks above do not match |

## Application Search

Applications include plain web/TCP apps, cloud apps, and MCP apps. AWS IC has a
dedicated listing command; do not look it up with `tctl apps ls`.

If the user asks for an application subtype, use a predicate query instead of
broad text search.

| User asks for | tctl command portion |
| --- | --- |
| AWS console apps / AWS apps | `$TCTL apps ls --query='resource.spec.cloud == "AWS"' --format=json` |
| Azure apps | `$TCTL apps ls --query='resource.spec.cloud == "Azure"' --format=json` |
| GCP apps | `$TCTL apps ls --query='resource.spec.cloud == "GCP"' --format=json` |
| MCP apps / MCP servers | `$TCTL apps ls --query='resource.sub_kind == "mcp"' --format=json` |
| AWS IC accounts / permission sets | `$TCTL integrations awsic accounts ls --format=json` |
| regular apps excluding AWS IC | `$TCTL apps ls --query='labels["teleport.dev/origin"] != "aws-identity-center"' --format=json` |

Inside `--query`, app fields are evaluated against the unwrapped app:
`resource.spec.cloud`, `resource.spec.uri`, `resource.sub_kind`. Application
Classification and Application Identities cover the wrapped paths for reading
matched results.

## Selection And Preview

There are two selection models.

### Label-Selected Resources

SSH servers, databases, Kubernetes clusters, applications, and Windows desktops
are selected by labels. Once the user states or settles on a previewable
selector, show a preview before asking for write approval.

If the user chooses one or more listed label-selected resources by name or row,
do not treat the resource names as exact access-list grants. Build the proposed
label selector from the selected row labels per Deriving Selectors From Listed
Labels below. Show the resulting grant flag and preview the selector before
moving on. If those labels may match more resources than the chosen rows, say
that in the preview warning and let the user narrow or confirm.

### Explicit Pattern Selectors

A selector value containing `*` other than the standalone `*` wildcard, or a
raw regex value that starts with `^` and ends with `$`, is a pattern.
The standalone wildcard forms `key=*` and `*=*` remain previewable with the
predicate mappings below. Do not derive a pattern from a listed row; Deriving
Selectors From Listed Labels blocks that case.

When the user explicitly supplies a pattern, show the resulting grant flag, then
say in user language: `<selector>` is a pattern, not an exact label. It may
grant access to multiple `<resource kind plural>`. Ask, "Do you
want to continue with this pattern?", then stop. Treat the
user's answer as the selection confirmation before asking any remaining Stage 1
details.

Use `--query` for kinds that support predicate queries:

| Kind | tctl preview command portion |
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

Preview warning — include every time a label-selected preview is shown,
regardless of match count (zero, few, or many):

```text
This preview only shows resources your own roles let you see, so treat the count as a lower bound. Members may be able to reach more.
```

The grant is still the label flag, not the predicate:

```text
--node-labels='env=staging'
```

### Name-Selected Resources

AWS IC and Git server are not selected by labels for access-list grants.

- AWS IC grants use exact `accountID^permissionSetARN` assignments.
- Git grants use exact GitHub org names through `--github-orgs`.

After the user chooses one of these from a listing, no label-selector preview is
needed.
Confirm the exact selected value in the draft instead.

## Label And Predicate Syntax

Access-list label flags use comma-separated `key=value` pairs. Repeated key
means OR; different keys mean AND.

```text
--node-labels='env=staging,team=backend'   # env=staging AND team=backend
--node-labels='env=staging,env=dev'        # env is staging OR dev
```

These unquoted examples show selectors the user supplied. A selector derived
from a listed row always quotes each value — see Deriving Selectors From Listed
Labels.

`env=*` means the key exists. `*=*` matches everything. Treat broad selectors as
risky and include the warning in the final write approval.

Predicate syntax for preview/search uses resource expressions and can preview
literal values and the standalone wildcard forms only:

| Desired selector | Predicate |
| --- | --- |
| `env=staging` | `labels["env"] == "staging"` |
| `env=staging` AND `team=backend` | `labels["env"] == "staging" && labels["team"] == "backend"` |
| `env=staging` OR `env=prod` | `(labels["env"] == "staging" \|\| labels["env"] == "prod")` |
| key exists | `exists(labels["env"])` |
| match everything | omit `--query` |

Supported predicate commands: `tctl nodes ls`, `tctl db ls`, `tctl kube ls`, and
`tctl apps ls`.

## Deriving Selectors From Listed Labels

For each label value from a selected row's effective label set above:

- Stop and ask the user to choose a safe label subset if it contains `*`, is a
  raw regex (starts with `^` and ends with `$`), has leading/trailing
  whitespace, or contains `"`. Teleport treats `*` as a glob and anchored
  values as regexes, so neither can make an exact row-derived grant. Name the
  rejected key; never omit it yourself.
- Otherwise emit `key="value"`: keep keys unquoted and always quote values.
  The double quotes are literal selector characters, even when the value has no
  separator.
- Build the preview from JSON-string-encoded key and value, as
  `labels["key"] == "value"`, joining pairs with `&&`. Then shell-escape the
  complete selector or predicate argument per SECURITY.md.

## Application Identities

After previewing app matches, inspect the wrapped JSON fields and ask only for
identity fields that apply.

| Matched `apps ls --format=json` field | Ask for | Flag |
| --- | --- | --- |
| `spec.app.spec.cloud == "AWS"` or AWS console URI | AWS role ARNs | `--aws-role-arns` |
| `spec.app.spec.cloud == "Azure"` | Azure identities | `--azure-identities` |
| `spec.app.spec.cloud == "GCP"` | GCP service accounts | `--gcp-service-accounts` |
| `spec.app.sub_kind == "mcp"` or `spec.app.spec.mcp` is present | MCP tools | `--mcp-tools` |

Plain web/TCP apps do not need cloud or MCP identity flags. If matches are
mixed, ask for each relevant identity type.

`apps ls` does not report which tools an MCP server exposes — `spec.app.spec.mcp`
holds only `command`, `args`, and `run_as_host_user`. `--mcp-tools` is a policy
value the user supplies, and each entry may be a literal name, a glob such as
`prefix_*`, or a regex anchored with `^` and `$`. Ask the user for it; never
offer to look the tools up, and never guess them from the app name.

## Rendering

Render from parsed JSON, not from truncated terminal text. `tctl` may return far
more rows than the command runner shows.

Client-page listings and previews at 10 rows unless the user explicitly asks for
all rows. Per SECURITY.md's Read Capture Discipline, slice the retained result
for each page. Follow that discipline for all capture reuse and refresh rules;
never re-fetch just to show the next page, since the set can change between
calls. State the shown range and true total, for example:

```text
Showing 1-10 of 200 matching nodes (page 1 of 20).
```

## AWS Identity Center

AWS IC grants use assignment strings:

```text
--aws-ic-assignments='123456789012^arn:aws:sso:::permissionSet/ssoins-XXXX/ps-YYYY'
```

The account may be `*`; the permission set must be a specific ARN. Do not grant
AWS IC with `--app-labels`, and do not set `teleport.dev/origin` yourself.

Never use a wildcard permission set such as `accountID^*` or `*^*`. If the user
asks for "every permission set", "all permission sets", or "every permission
set in every account", list AWS IC accounts with `--format=json` and expand to
every returned assignment record's exact `accountID^permissionSetARN` string —
do not settle for account or permission-set display names alone. For an
unambiguous request, use the expanded records as the target: do not ask the user
to narrow or re-pick them, and show any applicable production, admin, or
broad-access warning.

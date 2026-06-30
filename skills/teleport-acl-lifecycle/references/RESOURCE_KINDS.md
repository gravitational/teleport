# Resource Kinds

Leaf reference for resource kinds, flags, listing commands, scoped preview
commands, predicates, and identity fields. Do not substitute commands from memory.

## Kind Map

| User says | Kind | Visibility/grant flag | Principal or identity flags |
| --- | --- | --- | --- |
| servers, nodes, SSH | SSH server | `--node-labels` | `--logins` |
| databases, DBs | database | `--db-labels` | `--db-users`, `--db-names` |
| kube, Kubernetes, k8s, clusters | Kubernetes | `--kubernetes-labels` | `--kubernetes-users`, `--kubernetes-groups` |
| apps, applications, web apps, TCP apps, cloud apps, MCP apps | application | `--app-labels` | see Application identities |
| AWS IC, AWS Identity Center, permission sets | AWS IC app | `--aws-ic-assignments` | encoded in assignment |
| Windows, desktops | Windows desktop | `--windows-labels` | `--windows-logins` |
| git, GitHub | Git server | `--github-orgs` | none |

## Resource Offer List

Canonical user-facing list of what an access list can grant. Offer this whenever
you ask the user what to grant access to, on create or update. Present all seven
kinds, in this order, exactly as written. Do not drop any (Git servers / GitHub
orgs is real and easy to forget), reorder them, or replace them with invented
per-kind descriptions.

```text
- SSH servers
- Databases
- Kubernetes clusters
- Applications (web, TCP, cloud, MCP)
- Windows desktops
- AWS Identity Center accounts or permission sets
- Git servers / GitHub orgs
```

## Exact Resource Commands

Use the listing column when the user wants to see what exists or choose labels.
Use the scoped preview column after the access scope is known. These command
shapes are exact.

| Kind | Listing | Scoped preview |
| --- | --- | --- |
| SSH server | `$TCTL nodes ls --format=json` | `$TCTL nodes ls --query='<predicate>' --format=json` |
| Database | `$TCTL db ls --format=json` | `$TCTL db ls --query='<predicate>' --format=json` |
| Kubernetes | `$TCTL kube ls --format=json` | `$TCTL kube ls --query='<predicate>' --format=json` |
| Application | `$TCTL apps ls --format=json` | `$TCTL apps ls --query='<predicate>' --format=json` |
| AWS IC app | `$TCTL awsic ls --format=json` | None. The grant is the exact `accountID:permissionSetARN` the user picked from the listing, not a label selector, so there is nothing to re-query. |
| Windows desktop | `$TCTL get windows_desktop --format=json` then filter client-side | same |
| Git server | `$TCTL get git_server --format=json` then filter client-side by org | None. The grant is the exact GitHub org the user picked from the listing, not a label selector, so there is nothing to re-query. |

## Rendering Listing And Preview

Render from JSON, not truncated text tables. Lead with the true total (count at
the shell level when the listing is large; listing output may be truncated).

**Always client-page at 10 rows per page.** Show at most 10 resources at a time,
no matter how many the listing returns. Do not dump the full set, even when it is
only slightly more than 10, and even if it seems convenient. The only exception is
when the user explicitly asks to see all/everything/the full list; then show them
all.

When the total exceeds the 10 you show, make the partial view explicit so the user
knows they are seeing a slice, not the whole set. State the shown range against the
true total, for example "Showing 1-10 of 200 matching nodes (page 1 of 20)". Never
imply the displayed rows are everything, and prompt the user to narrow with a
label/filter or ask for the next page when the list is long.

When previewing add warning:

```text
⚠️ This preview only shows resources your own roles let you see, so treat the count as a lower bound — members may be able to reach more.
```

For listings, include these columns:

| Kind | Columns |
| --- | --- |
| SSH server | Hostname, Address, Labels |
| Database | Name, Protocol, Labels |
| Kubernetes | Name, Labels |
| Application | Name, Type, Public Address, Labels |
| AWS IC | Account Name, Account ID, Permission Set Name, Permission Set ARN |
| Windows desktop | Name, Address, Labels |
| Git server | GitHub Org |

When the user asks to see resources while creating or updating access, show the
resource rows first using the columns above.

After a listing, ask the user how to proceed:

- choose a specific resource
- give a label selector
- give a search/filter term
- see the next page (only when more pages exist)

Only offer "see the next page" when there are actually more pages (the true total
exceeds the rows shown so far). If everything fits on one page, drop that bullet
entirely. Paging is client-side:
you already fetched the full JSON, so showing the next page means displaying the
next rows you have, not re-running the command or looking for a pagination flag.
Keep the same "Showing X-Y of TOTAL" framing on each page. Do not draft a
create/update command from a common label unless the user explicitly says they
want all matching resources.

## Label Syntax

Label flags are comma-separated `key=value` pairs. Repeated key means OR;
different keys mean AND.

```text
--node-labels="env=staging,team=backend"   # env=staging AND team=backend
--node-labels="env=staging,env=dev"        # env is staging OR dev
```

`env=*` means key exists. `*=*` matches everything. Treat broad selectors as
risky and include the warning in the final write approval.

## Predicate Syntax

Supported for nodes, databases, Kubernetes, and apps: `==`, `!=`, `&&`, `||`,
`exists()`, `search()`.

| Scope | Predicate |
| --- | --- |
| `env=staging` | `labels["env"] == "staging"` |
| `env=staging` AND `team=backend` | `labels["env"] == "staging" && labels["team"] == "backend"` |
| `env=staging` OR `env=prod` | `(labels["env"] == "staging" \|\| labels["env"] == "prod")` |
| key exists | `exists(labels["env"])` |
| match everything | omit `--query` |
| AWS IC apps | `resource.sub_kind == "aws_ic_account"` |
| exclude AWS IC from regular app query | append `&& labels["teleport.dev/origin"] != "aws-identity-center"` |

## Application Identities

After previewing app matches, inspect the JSON and ask only for identity fields
that apply to the matched app types.

| Matched `tctl` JSON | Ask for | Flag |
| --- | --- | --- |
| `spec.cloud == "AWS"` or `spec.uri` starts with an AWS console URL | AWS role ARNs | `--aws-role-arns` |
| `spec.cloud == "Azure"` | Azure identities | `--azure-identities` |
| `spec.cloud == "GCP"` | GCP service accounts | `--gcp-service-accounts` |
| `sub_kind == "mcp"` or `spec.mcp` is present | MCP tools | `--mcp-tools` |

If matched apps are mixed, ask for each relevant identity type. Plain web/TCP apps
do not need cloud or MCP identity flags.

## AWS Identity Center

AWS IC listing must show account apps and their
`identity_center.permission_sets`. Let the user choose exact assignments; do not
infer them.

Assignment syntax:

```text
--aws-ic-assignments="123456789012:arn:aws:sso:::permissionSet/ssoins-XXXX/ps-YYYY"
```

The account may be `*`; the permission set must be a specific ARN. AWS IC
assignments may appear alongside regular app flags, but do not set
`teleport.dev/origin` yourself and do not use `--app-labels` to grant AWS IC.

# Create Access List

Create has two jobs: decide what the list grants, then ask for one final write
approval. A user request to create a list starts the route; it is not permission
to run `acl create`.

## Flow

1. Run `$TCTL acl create --help`.
2. Choose flavor: access type or custom.
3. Collect only missing decisions, in the staged order below.
4. Preview or verify the grant when one is defined.
5. Present the final draft and exact command intent for approval, then stop.
6. After explicit approval, run one `$TCTL acl create ... --format=json`.
7. Relay `tctl` output, the UUID, created roles when present, and the web URL.

If the user provided every required field up front, still do steps 4 and 5. No
blockers means ready to ask for approval, not ready to submit.

## Flavor

Use access type when the user asks for standing/request-based access, describes
Teleport resources, or wants an access-type list whose resource access will be
defined later. Teleport creates supporting roles for these lists.

Use custom only when the user names existing roles/traits to grant, explicitly
asks for custom/plain/no access type, or wants a grantless container/nested list
without standing/request-based semantics. Custom lists have no resource preview.

## Question Staging

Ask in stages so the user settles the grant before list metadata. If the user
already supplied a later-stage value, record it and do not ask again; the staging
order only controls missing questions.

### Access-Type Lists

**Stage 1 - Resource scope.** Settle what members can reach: resource kind,
labels/selectors, and resource-specific values like SSH logins or principals.
Ask these together. If the user explicitly wants to define resource access later,
record `resource scope: none yet` and skip preview. Otherwise preview
label-selected resources before moving on. Do not ask for access type, owners,
title, or description in this stage unless the user already gave them.

**Stage 2 - Access type.** Ask standing vs request-based as its own question
unless PRESETS.md gives a clear default. Mark any default as guessed in the final
draft.

**Stage 3 - List framing.** Collect owners, title, description, and members in
one batch. Owners and title are required. Members may be deferred only if the
user explicitly wants an empty list for now.

When a field does not obviously fit a stage: scope changes what resources are
reached, access type changes how the list grants access, and framing changes who
manages or belongs to the list.

### Custom Lists

Custom lists have no resource scope and no standing/request-based choice.

**Stage 1 - Grants.** Settle the custom grants, including the option of no
grants. If the user wants a grantless container/nested list, record
`custom grants: none` and skip role checks. If the user names roles, check them
with `$TCTL get roles --format=json`; do not check traits this way. If a named
role is not found, do not silently keep it in the command. Ask whether to correct
the role name or remove it from the draft. Once no missing named roles remain,
continue; an empty grant set is valid.

**Stage 2 - List framing.** Collect owners, title, description, and members in
one batch.

## Vague Access Prompt

When the user says only that people need access, such as "I need to give access
to Alice and Bob", assume they may be new to Teleport. Start with the access
target, not owners or access-list terminology:

```text
I can help with that. What should Alice and Bob be able to access?

<the seven kinds from RESOURCE_KINDS.md "Resource Offer List", verbatim>

If you already know this should use an existing access list, send its title or
UUID. Otherwise, pick the resource type and I will help narrow it down.
```

Do not ask for owners in this first response unless the user already provided
the access target. After the target is clear, collect owners with a short
explanation: "Owners manage/review this access list."

## Common Fields

- Title: infer from the use case if obvious, otherwise mark `(need one)`.
- Description: optional. In the draft, either show the supplied value or
  `description skipped`; do not block on a description alone.
- Owners: required. Values can be users or nested access-list UUIDs.
- Members: values can be users or nested access-list UUIDs. May be deferred only
  when the user says so.
- Requirements: optional `--member-required-*` / `--owner-required-*`; include
  only when the user asks for membership or owner eligibility gates.
- Audit: `--audit-frequency` (1, 3, 6, or 12 months) and `--audit-day` (1, 15,
  or 31) use `tctl` defaults unless the user specifies them.

Nested access-list owner/member flags take UUIDs, not display titles.

Do not validate users with `tctl get users` unless the user requests it; valid
SSO users may be absent.

If the resource kind is missing, ask with the Resource Offer List from
RESOURCE_KINDS.md.

## Access-Type Create

Required:

- `--access-type=<standing|request-based>`
- `--title=<...>`
- at least one owner flag

Resource scope:

- Guess resource kind if obvious; kind is only a category.
- Do not guess labels, logins, identities, GitHub orgs, or AWS IC assignments.
- Resource scope is optional when the user explicitly wants to define access
  later. In that case, omit all resource flags, skip preview, and show
  `resource scope: none yet` in the draft.
- If scope is missing and the user asks what resources exist, list resources
  using the table columns in RESOURCE_KINDS.md; do not replace the listing with a
  common-label summary.
- Treat labels shared by most or all resources as broad. Do not suggest an
  all-match label as the default scope unless the user explicitly asks for all
  resources of that kind.
- Preview label-selected resources once the selector is known; block on zero
  matches.
- AWS IC and Git grants use exact selected identifiers, so confirm the selected
  values in the draft instead of running a scoped preview.
- For AWS IC, show the exact `accountID^permissionSetARN` assignment strings in
  the draft and eventual `--aws-ic-assignments` value.
- Ask for principals/identities with examples, but allow blank when intentional.
- Bundle scope warnings into the final approval.

Command shape:

```bash
$TCTL acl create \
  --access-type=standing \
  --title="Backend Staging" \
  --description="Standing SSH access for backend engineers in staging" \
  --owners="carol@example.com" \
  --members="alice@example.com,bob@example.com" \
  --node-labels="env=staging" \
  --logins="ubuntu,ec2-user" \
  --format=json
```

## Custom Create

Required:

- `--title=<...>`
- at least one owner flag

Grants are optional. Use any exact grants the user gave:

- `--member-grant-roles`
- `--member-grant-traits`

Optional owner grants:

- `--owner-grant-roles`
- `--owner-grant-traits`

For a grantless custom list, omit all grant flags and show `custom grants: none`
in the approval draft.

Command shape:

```bash
$TCTL acl create \
  --title="Auditors" \
  --description="Audit team role grants for quarterly reviews" \
  --owners="carol@example.com" \
  --members="alice@example.com,bob@example.com" \
  --member-grant-roles="auditor,db-read" \
  --format=json
```

## Blockers

Do not ask for approval until these are resolved:

- Missing title.
- Missing owners.
- No members, unless the user explicitly wants to add members later.
- Access type: missing or ambiguous resource choices, unless the user explicitly
  wants to define resource access later. If scope was provided, zero-match
  preview blocks submit.
- Custom: the draft still contains a named role that role validation did not
  find.
- Any unresolved guessed value.

## Approval Gate

Use one final approval request. Include title, optional description or
`description skipped`, type, owners, members, resource scope (`none yet` is
valid for access-type lists), custom grants, preview count or exact selected
identifiers, and warnings.

The approval request should make the next action obvious, for example:

```text
If this looks right, approve and I will run:
<single acl create command>
```

Do not run `acl create` in the same turn that first produced this draft unless
the user already gave explicit conditional approval for this exact command intent
and all stated conditions are met.

## After Approved Submit

Submit `acl create` with `--format=json`. Parse `access_list.metadata.name` for
the UUID and, for access-type lists, `created_roles`; do not scrape text output.
Build the web URL from the parsed name:

```text
https://<proxy-host>/web/accesslists/<access-list-name>
```

Find proxy host via `tsh status` or `$TCTL status`.

## On Failure

Create can partially fail after the list itself already exists. Relay the exact
recovery command `tctl` prints; do not invent a cleanup script.

- **Role-build failure**: the list was created but its supporting roles weren't.
  `tctl` may suggest `tctl acl rm <name>`. Run that only after explicit
  confirmation; it can surface the leftover-roles flow in LEFTOVER_ROLES.md.
- **Member-setup failure**: the list and roles were created, but adding members
  failed. `tctl` may print a retry command such as
  `tctl acl update <name> --members="apple,banana"`. Run it only after explicit
  confirmation, scoped to the members it named.

If `tctl` cannot confirm whether the list was created, relay that uncertainty
and its suggested command. If nothing was created, it may print only the raw
error.

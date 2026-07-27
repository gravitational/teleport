# Create Access List

Create has two jobs: decide what the list grants, then ask for one final write
approval. A user request to create a list starts the route; it is not permission
to run `acl create`.

## Contents

- Flow: required create sequence from help check through JSON submit.
- Flavor: choose access-type vs custom list semantics.
- Question Staging: collect grant target, access type, and framing in order.
- Generic Missing Access Target Prompt: first response for "create an access list".
- Vague Access Prompt: first response when users only name people.
- Common Fields: title, owners, members, nested lists, requirements, audit.
- Access-Type Create: required flags and shared grant-target rules.
- Custom Create: role/trait grants and grantless custom lists.
- Approval Gate: final draft fields and write approval rule.
- After Approved Submit: parse UUID, roles, and web URL.
- On Failure: partial-create recovery and leftover-role handoff.

## Flow

1. Run `$TCTL acl create --help`.
2. Choose flavor: access type or custom.
3. Collect only missing decisions, in the staged order below.
4. Preview or verify the grant when one is defined.
5. Present the final draft and exact command intent for approval, then stop.
6. After explicit approval, run one `$TCTL acl create ... --format=json`.
7. Parse `tctl` output and build the web URL, clean captures per SECURITY.md,
   then relay the UUID, created roles when present, and the web URL.

If the user provided every required field up front, still do steps 4 and 5. No
blockers means ready to ask for approval, not ready to submit.

## Flavor

Use access type when the user asks for standing access, an Access Request
workflow, describes Teleport resources, or wants an access-type list whose
resource access will be defined later. Teleport creates supporting roles for
these lists.

Use custom only when the user names existing roles/traits to grant, explicitly
asks for custom/plain/no access type, or wants a grantless container/nested list
without standing or Access Request semantics. Custom lists have no resource preview.

## Question Staging

Ask in stages so the user settles the grant before list metadata. If the user
already supplied a later-stage value, record it and do not ask again; the staging
order controls only missing questions. On each response, ask only the earliest
incomplete stage and stop before advancing. For how to present a stage's
questions, see SKILL.md's Presentation section. SECURITY.md's general
"ask once for missing choices" rule does not cross create-stage boundaries.

For access-type lists, Stage 1 is complete only when one of these is true:

- The user explicitly wants to define resource access later, so the grant target
  is `none yet`.
- A name-selected grant target (AWS IC or Git server) has exact identifiers.
- A label-selected grant target has a resource kind, a proposed label selector,
  its selection confirmation per RESOURCE_KINDS.md (a preview with the
  lower-bound warning when available, or an explicit pattern acknowledgement),
  and every required principal or identity value from RESOURCE_KINDS.md.

For access-type lists, a response that lists candidates (including expanded AWS
IC assignments), previews a selector, asks for confirmation, or asks for
resource-specific principals or identities is Stage 1 output: end with only its
remaining Stage 1 question, not Stage 2 or framing. When Stage 1 has nothing
left to settle, state the warnings plus one line saying you will proceed as
asked unless the user says otherwise, then stop. Do not re-ask a selection the
user stated explicitly: every warning is repeated in the final write approval,
which is the one gate for that decision.

Plain-language access bridge: if the create route started from a request like
"give app access to alice" or from a post-listing next action, show this bridge
exactly once before doing anything else in Stage 1: before any resource listing,
grant-target question, selector prompt, preview, or exact-target confirmation.

```text
The recommended way to give access in Teleport is to create an access list:
members of the list receive access, and owners manage or review the list.
```

Use the bridge only after the access target category is clear. If the resource
kind is still missing, ask the appropriate missing-target prompt below and show
the bridge after the user picks a target. If the user explicitly asked to create
an access list, skip the bridge unless they seem unfamiliar with the terms.

### Access-Type Lists

**Stage 1 - Grant target.** Settle what members can reach.

- If the resource kind is missing, ask the appropriate missing-target prompt
  below and stop.
- If the kind is known but labels, selector, or exact identifiers are missing,
  first show the plain-language access bridge when it applies, then list
  candidates per RESOURCE_KINDS.md and stop.
- If the user chooses a listed label-selected resource by name or row, keep the
  flow in Stage 1: derive the proposed label selector from the selected row per
  RESOURCE_KINDS.md, preview that selector with the lower-bound warning, then
  ask only any remaining Stage 1 follow-up questions.
- If the user explicitly wants to define resource access later, record
  `grant target: none yet` and skip the rest of this stage.
- For a settled label selector or exact identifiers, preview or confirm the
  selection per RESOURCE_KINDS.md.
- If selection needs confirmation, narrowing, adjustment, or risk
  acknowledgement, ask only that, then stop before asking for principals or
  identities.
- After selection is confirmed, ask only resource-specific principal or identity
  values required by RESOURCE_KINDS.md's Kind Map and Application Identities.

**Stage 2 - Access type.** Use PRESETS.md to recommend standing or Access
Request when the surrounding context points clearly one way. Ask only standing
vs Access Request, then stop.

**Stage 3 - List framing.** Collect owners, title, description, and members in
one batch. Owners and title are required. Members may be deferred only if the
user explicitly wants an empty list for now. When collecting owners, explain
briefly: "Owners manage/review this access list."

When a field does not obviously fit a stage: grant target changes what
resources are reached, access type changes how the list grants access, and
framing changes who manages or belongs to the list.

### Custom Lists

Custom lists have no resource grant target and no standing or Access Request
choice.

**Stage 1 - Grants.** Settle the custom grants, including the option of no
grants. If the user wants a grantless container/nested list, record
`custom grants: none` and skip role checks. If the user names roles, check them
with `$TCTL get roles --format=json`; do not check traits this way. If a named
role is not found, do not silently keep it in the command. Ask whether to correct
the role name or remove it from the draft. Once no missing named roles remain,
continue; an empty grant set is valid.

**Stage 2 - List framing.** Collect owners, title, description, and members in
one batch.

## Generic Missing Access Target Prompt

When the user asks to create an access list but gives no access target, such as
"create an access list", start with the grant target, not owners or list
metadata:

```text
What should this access list grant access to?

<RESOURCE_KINDS.md "Resource Offer List">
```

## Vague Access Prompt

When the user says only that people need access, such as "I need to give access
to Alice and Bob", assume they may be new to Teleport. Start with the access
target, not owners or list metadata:

```text
What should Alice and Bob be able to access?

<RESOURCE_KINDS.md "Resource Offer List">

If you already know this should use an existing access list, send its title or
identifier, such as a UUID or scope-qualified name. Otherwise, pick the resource
type and I will help narrow it down.
```

## Common Fields

- Title: infer from the use case if obvious, otherwise mark `(need one)`.
- Description: optional. In the draft, either show the supplied value or
  `description skipped`; do not block on a description alone.
- Owners: required. Values can be users or nested access-list identifiers (see
  SCOPES.md for the nesting hierarchy rule).
- Members: values can be users or nested access-list identifiers, same
  hierarchy rule as owners. May be deferred only when the user says so.
- Requirements: optional `--member-required-*` / `--owner-required-*`; include
  only when the user asks for membership or owner eligibility gates.
- Audit: `--audit-frequency` (1, 3, 6, or 12 months) and `--audit-day` (1, 15,
  or 31) use `tctl` defaults unless the user specifies them.

Nested access-list owner/member flags take identifiers, not display titles. User
owner/member values follow SECURITY.md's opaque-principal rule: do not
automatically validate or offer to validate them.

## Access-Type Create

Required:

- `--access-type=<standing|access-request>`
- `--title=<...>`
- at least one owner flag

Grant target:

- Guess the resource kind only when obvious; kind is only a category.
- Do not guess labels, logins, identities, GitHub orgs, or AWS IC assignments.
- Grant target is optional when the user explicitly wants to define access later.
  In that case, omit all resource flags, skip preview, and show
  `grant target: none yet` in the draft.
- For previewable selectors, show preview count in the draft. If the preview
  returns zero matches, say so and ask the user to confirm the selector or
  correct it before drafting — a label that matches nothing is usually a typo,
  and a draft would imply the grant is ready. If they confirm it, proceed and
  note in the draft that the list will grant access once matching resources exist.
- Use RESOURCE_KINDS.md for resource listing, selection, preview, label syntax,
  AWS IC assignment strings, GitHub orgs, and app identity flags.
- Use SECURITY.md for broad-selector warnings, preview warnings, and blockers
  for guessed principals or identities.

Command shape:

```bash
$TCTL acl create \
  --access-type=standing \
  --title='Backend Staging' \
  --description='Standing SSH access for backend engineers in staging' \
  --owners='carol@example.com' \
  --members='alice@example.com,bob@example.com' \
  --node-labels='env=staging' \
  --logins='ubuntu,ec2-user' \
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
  --title='Auditors' \
  --description='Audit team role grants for quarterly reviews' \
  --owners='carol@example.com' \
  --members='alice@example.com,bob@example.com' \
  --member-grant-roles='auditor,db-read' \
  --format=json
```

## Approval Gate

Do not ask for approval while any guessed value in the draft is unresolved.

Use one final approval request. Include title, optional description or
`description skipped`, type, owners, members, grant target (`none yet` is valid
for access-type lists), custom grants, preview count, exact selected
identifiers, or an acknowledged no-preview pattern, and warnings.

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
  confirmation; if that surfaces unused supporting roles, handle them with the
  route-loaded leftover-role guidance.
- **Member-setup failure**: the list and roles were created, but adding members
  failed. `tctl` may print a retry command naming members to add. Run it only
  after explicit confirmation, limited to the members it named, and re-escape
  values per SECURITY.md before execution.

If `tctl` cannot confirm whether the list was created, relay that uncertainty
and its suggested command. If nothing was created, it may print only the raw
error.

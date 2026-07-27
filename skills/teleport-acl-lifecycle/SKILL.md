---
name: teleport-acl-lifecycle
description: Use for Teleport access list work with tctl; listing available resources (servers, databases, apps, etc.) before choosing an access grant target, creating new access lists, updating existing lists, or deleting/retiring lists. Trigger for requests like "give alice access", "create an access list", "standing access", "Access Request", "custom access list", "show/list apps", "show AWS IC permission sets", "add AWS IC to this list", "change owners/members/access", "remove access", "delete/retire/tear down an access list". Handles access-type lists where Teleport creates supporting roles, and custom lists that grant existing roles/traits.
---

# Teleport Access List Lifecycle

Use `tctl` to list resources, create access lists, update existing access
lists, and delete access lists. Route first, then read only the referenced leaf
files for that route.

## Route First

Choose exactly one route before running commands:

| User intent | Examples | Do this |
| --- | --- | --- |
| **Resource listing only** | "list AWS IC", "show apps", "what labels do databases have?", "what resources are in the cluster?" | Read [RESOURCE_KINDS.md](references/RESOURCE_KINDS.md) and [SECURITY.md](references/SECURITY.md). List resources/labels/assignments only; do not draft, create, or update yet. |
| **Create new list** | "give alice access to apps", "create Prod SSH access", "bob needs AWS IC" | Read [CREATE.md](references/CREATE.md), [RESOURCE_KINDS.md](references/RESOURCE_KINDS.md), [PRESETS.md](references/PRESETS.md), [SCOPES.md](references/SCOPES.md), and [SECURITY.md](references/SECURITY.md). If create partially succeeds and cleanup/unused-role handling is needed, also read [LEFTOVER_ROLES.md](references/LEFTOVER_ROLES.md). |
| **Update existing list** | "add bob to Prod Apps", "add AWS IC to the existing list", "rename the list", "remove app access" | Read [UPDATE.md](references/UPDATE.md), [RESOURCE_KINDS.md](references/RESOURCE_KINDS.md), [PRESETS.md](references/PRESETS.md), [LEFTOVER_ROLES.md](references/LEFTOVER_ROLES.md), [SCOPES.md](references/SCOPES.md), and [SECURITY.md](references/SECURITY.md). |
| **Delete existing list** | "delete", "remove this list", "retire", "tear down" | Read [DELETE.md](references/DELETE.md), [PRESETS.md](references/PRESETS.md), [LEFTOVER_ROLES.md](references/LEFTOVER_ROLES.md), [SCOPES.md](references/SCOPES.md), and [SECURITY.md](references/SECURITY.md). |

If the route is unclear, ask whether the user wants to inspect available
resources, create a new access list, update an existing access list, or delete an
existing access list.

If the user asks to create an access list but gives no access target, such as
"create an access list", choose the create route and show CREATE.md's Generic
Missing Access Target Prompt.

If the user names people who need access but gives no target, such as "give
Alice and Bob access", choose the create route and ask a first-time-friendly
resource question. Do not lead with "access list", "grant target", "UUID",
or owners. Show CREATE.md's Vague Access Prompt.

If the user names people and a target but does not explicitly ask to create an
access list, such as "give app access to claudia", choose the create route and
follow CREATE.md's plain-language access bridge.

## Critical Gotchas

- **Resource listing is not create/update.** If the user asks to list, show, or
  inspect resources, do not draft, create, or update an access list.
- **Do not choose resource commands from memory.** Use the exact command matrix in
  [RESOURCE_KINDS.md](references/RESOURCE_KINDS.md).
- **AWS IC is special.** Its listing command has no `--query` flag; use
  `$TCTL integrations awsic accounts ls --format=json` and build exact
  `accountID^permissionSetARN` assignments from each returned record. AWS IC
  grants use `--aws-ic-assignments`, not `--app-labels`.
- **Display title is not the CLI identifier.** Existing-list update/delete needs
  the access-list identifier, and titles are not unique — resolve per
  SECURITY.md's duplicate-title rule before any `acl get`, `acl update`, or
  `acl rm`.
- **Delete parent detaches are separate writes.** A user's permanent-delete
  confirmation does not approve parent updates discovered later — see DELETE.md's
  Parent Nesting.
- **Access type is immutable.** A list created as standing, Access Request
  (`access-request`), or custom cannot be converted in place; recreate it.
  Detect it from the `access-list-preset` label, not `spec.type` — see
  PRESETS.md's Existing Lists section.
- **List kind controls legal flags.** Access-type lists use resource flags and
  reject grant flags; custom lists use grant flags and reject resource flags and
  `--remove-access` — see UPDATE.md's Legal Changes.
- **Update member/owner flags replace sets, not merge.** Use `tctl acl users
  add`/`rm` for single member changes instead — see UPDATE.md's Replace Traps.
- **Preserve identifier casing exactly.** Usernames, logins, principals, role
  names, and traits are case-sensitive. Pass them into `tctl` flags exactly as
  the user typed them, even if surrounding prose capitalizes the name for
  readability — "give alice access" means the member value is `alice`, not
  `Alice`.
- **Never eyeball raw `tctl` output.** It can be huge and may be truncated —
  capture complete JSON first, then inspect it through filters, per
  SECURITY.md's Read Capture Discipline.

Use these markers in drafts: `(need one)`, `← default`, `← guessed for
approval`, and `(optional, ask)`.

## Presentation

Render structured output with markdown structure, never as one long paragraph.
Both `tctl` JSON and your own drafts must be reshaped into something scannable.

- **Resource listings:** use a markdown table with one row per resource. Use
  the route-loaded columns for that resource kind. Do not stream resources
  inside a sentence. Do not include selector-preview warnings in a listing.
- **Selection confirmation:** for a previewable label-selected grant, show a
  markdown table with preview matches and route-loaded warnings. For a
  user-explicit pattern selector, follow RESOURCE_KINDS.md's no-preview
  acknowledgement; do not render a table as its match preview.
- **Drafts, deltas, and approvals:** use a short markdown labeled list or a
  two-column markdown `field | value` table. Include applicable fields such as
  target, type, owners, members, grant target, preview, warnings, removals,
  and command intent, one field per line. When there is more than one command
  intent, do not cram them into a single table cell — never use `<br>` or
  other HTML to fake a line break inside a cell. List each command on its own
  line in a fenced bash code block below the table instead.
- **Questions and choices:** follow the route's stages. Ask only the current
  stage's missing question(s), then stop for the user's answer. Do not preview
  or narrate any other unanswered question before the user answers the current
  one.

  Named reference menus are exact markdown menus owned by their reference
  files, not structured question tool choices. Render them in full as plain
  markdown, without truncating, paging, merging, omitting, rewording, or
  compressing them into a picker. This includes RESOURCE_KINDS.md's Resource
  Offer List whenever asking for an access target/resource kind, and
  RESOURCE_KINDS.md's post-listing next-action menu. If you are about to show
  `☐ Access target`, render the Resource Offer List as plain markdown instead.

  Use a structured question tool for small stage-local choices or field
  batches when available, such as standing vs Access Request, or
  owners/title/description/members. Use one question per field or choice.

  If no structured question tool is available, use plain markdown with each
  field or choice on its own line. Never string several asks together in one
  prose sentence.

  Bad: "Who should own this, what title and description do you want, and who are
  the members?"

  Good (plain markdown form):
  - Owners (who manages/reviews this list)?
  - Title?
  - Description (optional)?
  - Members (can defer)?
- Put a blank line between sections and use a short bold header or `##` when a
  message has more than one part.
- Keep prose for explanation and warnings only; put data in markdown tables or
  lists.

## Setup

Find `tctl` (`which tctl`, `/usr/local/bin/tctl`, `/opt/homebrew/bin/tctl`,
`~/go/bin/tctl`) and set `TCTL=<path>`. From a workstation, `tctl` uses the
current `tsh login`; on an auth server it uses local admin identity. If `tctl`
returns access denied, relay the error as data and name the likely permission
class: `access_list` write, plus `role` access for access-type role work.

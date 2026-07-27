# Delete Access List

Delete has two possible write phases: parent detaches, then the access-list
delete. Approval for the delete does not approve parent updates discovered later.

## Contents

- Flow: resolve target, check parent nesting, delete, and handle leftover roles.
- Parent Nesting: why parent detaches need separate approval.
- Detach Commands: remove target from member/owner parents, including scoped parents.
- Approval Shape: required delete and parent-detach approval fields.

## Flow

1. Resolve to the exact target per SECURITY.md's Core Rules.
2. Establish delete permission for the unique target. Only an explicit
   confirmation, such as "I confirm, go ahead," may serve as conditional delete
   approval; "delete <target>" is a request, not approval. Do not carry
   confirmation across a target choice. Otherwise ask for approval naming title,
   identifier, access type, and description.
3. Load `$TCTL acl get <identifier> --format=json` for the target and inspect
   the unwrapped access list's `status.member_of`, `status.owner_of`,
   `status.scoped_member_of`, and `status.scoped_owner_of`.
4. If all four fields are empty and delete permission is approved, run
   `$TCTL acl rm <identifier>`. If delete permission is missing, ask for it and
   stop.
5. If parent nesting exists, load each parent with
   `$TCTL acl get <parent-identifier> --format=json` (UUID or scope-qualified
   name, matching the field it came from), draft the detach operations, and
   stop for parent-detach plan approval.
6. After all parent detaches have been approved and applied, run
   `$TCTL acl rm <identifier>` without asking for delete permission again.
7. Relay leftover role output exactly and follow route-loaded
   LEFTOVER_ROLES.md. Delete roles only after separate per-role confirmation.

## Parent Nesting

Parent nesting blocks `acl rm` until detached. The user's delete confirmation
does not approve these writes because the parent changes are not known until
after `acl get`.

For each parent, show a separate detach item with:

- parent title and identifier
- target title and identifier being detached
- current -> new nested-list membership or ownership
- the exact command intent

The user may approve the whole parent-detach plan in one response after seeing
all detach items. Do not bundle parent detaches with the final delete; the delete
permission is separate from the parent-update approval. Do not run any parent
detach in the same turn that first reveals the parent nesting unless the user
already approved the fully enumerated detach plan after seeing the before/after
for every parent.

For large parent counts, do not ask for one approval per parent. Page the detach
plan into readable sections if needed, but collect one approval for the complete
enumerated plan. State the total parent count and make clear that approval covers
every listed parent update.

## Detach Commands

### Target Is A Nested Member

For each UUID in `status.member_of`, the target is nested as a member of that
parent:

```bash
$TCTL acl users rm <parent-uuid> <target-uuid>
```

Show the parent member nesting before and after:

| Field | Value |
| --- | --- |
| Parent | Team Leads (9999...) |
| Members | Junior Devs (8888...) removed |
| Command | `acl users rm 9999... 8888...` |

### Target Is A Nested Owner

For each UUID in `status.owner_of`, the target is nested as an owner of that
parent. There is no owner equivalent of `acl users rm`; update the parent's
nested owner set with `--owner-access-lists`.

Load the parent, keep all direct user owners unchanged, and preserve every other
nested-list owner. Remove only the target UUID from the nested owner list:

```bash
$TCTL acl update <parent-uuid> --owner-access-lists='<remaining-nested-owner-uuids>'
```

Show the parent owners before and after the detach. If that plan shows the
target is the parent's only owner, pause before asking for approval: the parent
needs a replacement owner, or the user can ask for a separate plan to delete
the parent.

When the user supplies a replacement owner in response, stay in this delete
workflow. The parent is already resolved and loaded, so use its identifier and
current owners to draft the replacement plus detach; do not restart through the
update route or run `acl ls` again.

If direct user owners remain but no nested-list owners remain, pass an empty
value:

```bash
$TCTL acl update <parent-uuid> --owner-access-lists=''
```

Show owner before/after:

| Field | Value |
| --- | --- |
| Parent | Platform Review Board (aaaa...) |
| Command | `acl update aaaa... --owner-access-lists=''` |

| Field | Before | After |
| --- | --- | --- |
| Direct owners | carol | carol |
| Nested owners | Junior Devs (8888...) | none |

### Target Is A Scoped Nested Member Or Owner

Use route-loaded SCOPES.md for the identifier format.
`scoped_member_of`/`scoped_owner_of` entries are already the parent's
scope-qualified name — no separate UUID lookup step. Use the same commands
above, substituting scope-qualified names for parent and/or target wherever
either side is scoped:

```bash
$TCTL acl users rm <parent-scope>::<parent-name> <target-identifier>
$TCTL acl update <parent-scope>::<parent-name> --owner-access-lists='<remaining-nested-owner-identifiers>'
```

Do not assume a UUID exists for a scoped parent or target.

## Approval Shape

Delete permission:

| Field | Value |
| --- | --- |
| Delete target | Junior Devs (8888...) |
| Access type | custom |
| Description | Custom grants for junior engineers, nested under Team Leads. |
| Effect | permanently delete this access list after required parent detaches |

Parent-detach plan approval:

**Detach 1:**

| Field | Value |
| --- | --- |
| Parent | Team Leads (9999...) |
| Members | Junior Devs (8888...) removed |
| Command | `acl users rm 9999... 8888...` |

**Detach 2:**

| Field | Value |
| --- | --- |
| Parent | Platform Review Board (aaaa...) |
| Command | `acl update aaaa... --owner-access-lists=''` |

| Field | Before | After |
| --- | --- | --- |
| Direct owners | carol | carol |
| Nested owners | Junior Devs (8888...) | none |

Approve this parent-detach plan to apply both parent updates.

The approval request must include the full detach plan in the same response.
Do not say only "approve the plan shown above"; repeat the parent count, every
detach item, and the command intent for each parent.

When parent detaches are pending, end the turn after the detach-plan approval
request. Do not run parent updates or `acl rm` until the detach plan is approved
and the parent writes have succeeded.

# Delete Access List

Delete has two possible write phases: parent detaches, then the access-list
delete. Approval for the delete does not approve parent updates discovered later.

## Flow

1. Resolve the target with `$TCTL acl ls --format=json` piped through a filter.
   Match the user's title/description wording case-insensitively.
2. If more than one list matches, show every candidate with title, UUID,
   description, and grants; stop until the user picks the exact UUID.
3. Establish delete permission for the unique target. If the user already said
   the deletion is permanent and to go ahead, treat that as conditional delete
   approval once the unique target is resolved. Otherwise ask for approval naming
   title, UUID, and description.
4. Load `$TCTL acl get <uuid> --format=json` for the target and inspect:
   `status.member_of` and `status.owner_of`.
5. If no parent nesting exists and delete permission is approved, run
   `$TCTL acl rm <uuid>`. If delete permission is missing, ask for it and stop.
6. If parent nesting exists, load each parent with
   `$TCTL acl get <parent-uuid> --format=json`, draft the detach operations, and
   stop for parent-detach plan approval.
7. After all parent detaches have been approved and applied, run
   `$TCTL acl rm <uuid>` without asking for delete permission again.
8. Relay leftover role output exactly and follow LEFTOVER_ROLES.md. Delete roles
   only after separate per-role confirmation.

## Parent Nesting

Parent nesting blocks `acl rm` until detached. The user's delete confirmation
does not approve these writes because the parent changes are not known until
after `acl get`.

For each parent, show a separate detach item with:

- parent title and UUID
- target title and UUID being detached
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

```text
Parent  | Team Leads (9999...)
Members | Junior Devs (8888...) -> removed
Command | acl users rm 9999... 8888...
```

### Target Is A Nested Owner

For each UUID in `status.owner_of`, the target is nested as an owner of that
parent. There is no owner equivalent of `acl users rm`; update the parent's
nested owner set with `--owner-access-lists`.

Load the parent, keep all direct user owners unchanged, and preserve every other
nested-list owner. Remove only the target UUID from the nested owner list:

```bash
$TCTL acl update <parent-uuid> --owner-access-lists="<remaining-nested-owner-uuids>"
```

If no nested-list owners remain, pass an empty value:

```bash
$TCTL acl update <parent-uuid> --owner-access-lists=""
```

Show owner before/after:

```text
Parent        | Platform Review Board (aaaa...)
Direct owners | carol -> carol
Nested owners | Junior Devs (8888...) -> none
Command       | acl update aaaa... --owner-access-lists=""
```

## Approval Shape

Delete permission:

```text
Delete target | Junior Devs (8888...)
Description   | Custom grants for junior engineers, nested under Team Leads.
Effect        | permanently delete this access list after required parent detaches
```

Parent-detach plan approval:

```text
Detach 1:
Parent  | Team Leads (9999...)
Change  | nested member Junior Devs (8888...) -> removed
Command | acl users rm 9999... 8888...

Detach 2:
Parent        | Platform Review Board (aaaa...)
Direct owners | carol -> carol
Nested owners | Junior Devs (8888...) -> none
Command       | acl update aaaa... --owner-access-lists=""

Approve this parent-detach plan to apply both parent updates.
```

The approval request must include the full detach plan in the same response.
Do not say only "approve the plan shown above"; repeat the parent count, every
detach item, and the command intent for each parent.

When parent detaches are pending, end the turn after the detach-plan approval
request. Do not run parent updates or `acl rm` until the detach plan is approved
and the parent writes have succeeded.

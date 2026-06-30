# Delete Access List

Leaf reference for deleting access lists and cleaning up leftover access-type
roles.

## Flow

1. Always start with `$TCTL acl ls --format=json` to enumerate every access
   list, even when the user names a specific list. Match their title/description
   against the results.
2. Surface duplicates. Titles are not unique, so more than one list can share
   the name the user gave. Surface every match and have the user pick the exact
   UUID; never delete when the title resolves to more than one list without an
   explicit choice.
3. Record the exact target for the final approval: title, UUID, and what it
   grants.
4. Load `$TCTL acl get <uuid> --format=json`.
5. Check parent nesting:
   - `status.member_of`
   - `status.owner_of`
6. If parent nesting exists, detach from each parent with a separate approved
   parent update before deleting.
7. Get one explicit confirmation that deletion is permanent and includes the
   target title, UUID, and granted access.
8. Run `$TCTL acl rm <uuid>`.
9. Relay leftover role output and follow the route's leftover-role guidance.
   Delete roles only if the user confirms each printed role.

## Parent Detach

Member nesting:

```bash
$TCTL acl users rm <parent-uuid> <uuid>
```

Owner nesting: update the parent `--owner-access-lists` value without this UUID.
Show the before/after set and approve that parent update first.

## Approval Shape

For a unique match, do not ask for a separate "is this the right UUID?"
confirmation before read-only inspection. Bundle target confirmation into the
final delete approval. Parent detach updates are separate writes, so approve each
parent before changing it; after all detaches are done, ask once for the final
delete.

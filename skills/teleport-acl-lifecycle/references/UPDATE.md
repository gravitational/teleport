# Update Access List

Leaf reference for updating existing access lists.

## Flow

1. Run `$TCTL acl update --help`.
2. Always start with `$TCTL acl ls --format=json` to enumerate every access list,
   even when the user names a specific list. Match their title/description/wording
   against the results.
3. Surface duplicates. Titles are not unique, so more than one list can share
   the name the user gave. Surface every match and have the user pick the exact
   UUID; do not proceed until they do.
4. Load current state with `$TCTL acl get <uuid> --format=json`.
5. Identify list kind.
6. Draft a delta and preview any scope changes.
7. Apply only changed flags after one approval that includes title + UUID.

## Identify Kind

Inspect labels from `acl get`:

- `teleport.internal/access-list-preset` present: access-type list.
- `teleport.internal/access-list-preset == long-term`: standing.
- `teleport.internal/access-list-preset == short-term`: request-based.
- Label absent or empty: custom list.
- `teleport.internal/access-list-preset-roles`: auto-created supporting roles.

Access type is immutable. To change standing/request-based/custom, create a new
list and migrate/delete the old one.

## Legal Changes

Both kinds accept metadata, owners/members, requirements, and audit changes.

Access-type lists accept resource flags and `--remove-access`. They reject grant
flags.

Custom lists accept grant flags. They reject resource flags and `--remove-access`.

## Replace Traps

- `--members` replaces only the user member set.
- `--owners` replaces only the user owner set.
- `--member-access-lists` replaces only nested-list members.
- `--owner-access-lists` replaces only nested-list owners.
- Show removals before applying.
- Use `$TCTL acl users add` / `$TCTL acl users rm` for single user member
  changes.
- Use `$TCTL acl users add --kind=list` to add a nested access-list member.
- A list must keep at least one owner.
- Title cannot be unset.

When drafting the delta, show only changed fields as current -> new. Surface
removals caused by replacement flags. Include target title + UUID and bundle any
risk warnings into the same approval request.

```text
Target  | Prod Apps (4e2c...)
Owners  | carol, dave -> carol, dave, erin
Members | alice, bob -> alice        removes bob
Scope   | app labels env=prod -> app labels env=prod,region=eu
Preview | 8 apps match what your account can see
```

## Resource Re-Scope

For access-type lists, re-scope like create:

- When offering or adding a new resource kind, use the Resource Offer List from
  RESOURCE_KINDS.md (same canonical list create uses).
- Do not guess labels, principals, identities, GitHub orgs, or AWS IC assignments.
- Use exact listing/preview commands.
- Show preview count in the delta.
- Block on zero matches.
- Bundle broad-selector and large-preview warnings into the final update
  approval instead of asking separately.

Re-scope can leave old supporting roles unused; handle any roles `tctl acl
update` prints with the route's leftover-role guidance.

## Remove Access

`--remove-access` is update-only and access-type-only. It detaches resource access
roles and leaves the list, members, and owners intact.

Do not combine `--remove-access` with resource flags. It may leave supporting
roles unused; handle them with the route's leftover-role guidance.

## Approval Shape

Use one approval request for the final update. Include target title + UUID,
current -> new values, removals, preview count if scope changes, and warnings.

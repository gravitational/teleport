# Access List Scopes

## Identifier Format

- Unscoped list: identifier is `.metadata.name`, a generated UUID.
- Scoped list: identifier is `<scope>::<name>` (for example `/team-a::shared`
  or `/team-a/child::shared`). `<scope>` always starts with `/`; nested scope
  segments are also separated by `/`.
- The `.scope` field in `acl ls`/`acl get --format=json` output tells you
  whether a given list is scoped.
- Names are not globally unique. Scoped lists can reuse the same
  `.metadata.name` across different scopes, so scope must always be shown
  alongside name when disambiguating candidates (see SECURITY.md's Core
  Rules).
- Use the `<scope>::<name>` form wherever a command takes a scoped target:
  `acl get`, `acl update`, `acl rm`, `acl users add`/`rm`, and
  `--owner-access-lists`/`--member-access-lists` values that reference a
  scoped nested list.

## Nesting Hierarchy Rule

- An access list can only be added as an owner or member of another access
  list if its own scope is equal to or an ancestor of the target list's
  scope. The Auth Service rejects a descendant or sibling scope.
- A scoped list can never be an owner or member of an unscoped list,
  regardless of hierarchy. If `acl create --help` shows no `--scope` flag,
  every newly created list is unscoped, so it can never take a scoped nested
  owner or member — only an unscoped nested list works during Create.
- An unscoped list can always be an owner or member of a scoped list —
  unscoped satisfies the ancestor rule at any scope.
- Do not guess around a rejected scope-hierarchy write. Relay the exact error
  and ask the user how to proceed rather than retrying with a different
  scope.

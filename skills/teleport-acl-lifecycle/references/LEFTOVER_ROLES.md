# Leftover Roles

Both `tctl acl rm` and `tctl acl update` can print this block when an access-type
list stops using a supporting role:

```
The following roles are no longer used by this access list:
  - <role>

These roles may still be assigned to users or referenced by other roles.
Verify each role is unused before deleting it with:
  tctl rm roles/<name>
```

Treat it as a warning, not a task to complete.

- **Relay the tctl block verbatim**, including the "may still be assigned to users
  or referenced by other roles" and "verify each role is unused before deleting
  it" lines. Do not soften, shorten, or rephrase the warning into a casual prompt.
- **Deletion is opt-in and per role.** For each role the user explicitly wants
  gone: name the role, restate that deleting a role still assigned to users or
  referenced by other roles can lock people out, confirm the user has verified it
  is unused, and only then run `tctl rm roles/<name>` for that one role.
- **Never delete a role tctl did not print**, and only delete ones matching
  `*-acl-preset-<uuid>`.

# Access Types

Leaf reference for choosing user-facing access type. Internally, an access-type
list is also called a preset because Teleport creates supporting roles.

Do not use internal terms with users:

- Say `standing`, not `long-term`.
- Say `request-based`, not `short-term`.
- Avoid saying `preset`.

## Standing

Flag: `--access-type=standing`

Members get access automatically on login. Use for ongoing, low-friction access:
staging, internal tools, read-only dashboards, or base team access.

## Request-Based

Flag: `--access-type=request-based`

Members file an access request and owners approve before access is granted for
the request TTL. Use for production, admin, sensitive, incident, JIT, approval,
or short-lived access.

## Ambiguous Requests

If the user gives resources but not the access type:

- Default to request-based for prod, admin, sensitive, incident, JIT, or approval
  language.
- Default to standing for staging, internal tools, read-only, or base team access.
- Mark the choice as guessed and include it in the final approval.

When asking the user to choose, use:

```text
Standing - members get access automatically on login (good for staging, internal tools, read-only)
Request-based - members must request access and owners approve each time (good for production, admin, sensitive)
```

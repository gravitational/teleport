---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 211 - JSONPath Interpolation

## What

Introduce a new interpolation function, `jsonpath`, which will use the
[JSONPath query language](https://www.rfc-editor.org/rfc/rfc9535.html)
to transform a json blob into a single value or list of values.

Initially, `jsonpath` will be supported for:

- [Role templates](https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/#interpolation-rules),
  where the user trait value being evaluated is a json blob.
- OIDC `claims_to_roles` mapping, where the claim value being evaluated is a json blob.

## Why

In some cases, OIDC claims may not be a string or list of strings (string value)
as is currently expected by Teleport, making these claims unusable for both OIDC
role mapping and role templates. `jsonpath` interpolation is primarily aimed at
solving this use case.

There may also be some local administrative use cases where `jsonpath`
interpolation is useful, especially given some of the more powerful features
of JSONPath parsing like filtering.

## Details

### Traits

Traits will maintain the same key/value pair structure, where the value is either
a string or list of strings. If the string value is a JSON blob, then it can be
interpolated in role templates with `jsonpath`.

For example:

```yaml
kind: user
metadata:
  name: alice
spec:
  ...
  traits:
    server_perms: '{
      "env": "staging",
      "logins": "alice"
    }'
---
kind: role
...
spec:
  ...
  allow:
    logins: "{{ jsonpath(external.server_perms, '$.logins]') }}"
    node_labels:
      "env": "{{ jsonpath(external.server_perms, '$.env]') }}"
---
```

### OIDC Claims

During OIDC login, a user's OIDC claims serve two purposes:

- Determine what Teleport roles to give the user by mapping on `connector.spec.claims_to_roles`.
- Set claims as Teleport user traits.

Currently, Teleport expects these claims to be a string or list of strings and
ignores any other value types.

```json
{
   "email": "alice@example.com",
   "groups": ["admin", "viewer"]
}
```

However, some custom OIDC providers allow claims with custom JSON structures.

For example:

```json
{
   "email": "alice@example.com",
   "groups": {
      "staging": ["admin", "viewer"],
      "prod": ["viewer"]
   }
}
```

Custom JSON structures will be supported for both setting user traits and
mapping claims.

#### Claims to Traits

If a claim value does not parse as a string or list of strings, Teleport will
store the JSON blob itself as the user trait value:

```yaml
kind: user
...
spec:
  ...
  traits:
    email: alice@example.com
    groups:
    - '{"staging": "admin", "prod": "viewer"}'
```

#### Claims to Roles - `claim_expression`

Teleport will support interpolating a claim value, from its original, unparsed
JSON form (including strings and lists), for use in claim mapping. This will
be supported with a new `claim_expression` field in `connector.spec.claims_to_roles`:

```yaml
kind: oidc 
...
spec:
  ...
  claims_to_roles:
    - claim: "groups"
      claim_expression: "jsonpath('$.[*]')"
      value: "viewer"
      roles: "viewer"
    - claim: "groups"
      claim_expression: "jsonpath('$.[*]')"
      value: "admin"
      roles: "admin"
```

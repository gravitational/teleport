---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 78 - Role Variables

## What

Teleport admins should be able to define variables within roles which can extend
claims coming from internal and external user traits.

Requirements:

1. Variable definitions should support some form of pattern matching so that
   traits can be easily and powerfully extended.
2. A variable defined in one role should be usable in many other roles.

Non-requirements:

1. Variables can depend on other variables.

## Why

Variables will be useful to specify additional traits that are not available in
OIDC/SAML claims.

Teleport admins don't always have control over their org's OIDC/SAML provider.

Role variables will simplify role evaluation, keep logic in one place, and
minimize repetition.

## Details

Example role spec:

```yaml
kind: role
version: v5
metadata:
  name: node-users
spec:
  vars:
    - name: "logins"
      # "select" is a string value which will be used as the input to all
      # possible expansion matchers. It can (and usually will) contain a trait
      # which will be expanded before passing as input.
      select: "{{external.email}}"
      expansions:
        # nic should be able to login as root
        - match: '^nic@goteleport\.com$'
          out: [root]
        # alice should be able to login as root and admin
        - match: '^alice@goteleport\.com$'
          out: [root, admin]
        # all teleport employees should be able to login as the local part of their email
        - match: '@goteleport\.com$'
          # "out" values can contain transforms and any traits
          out: ["{{email.local(external.email)}}"]
    - name: "allow_env"
      # If the trait in a "select" expands to a list, all values will be checked
      # for matches and the outputs will be combined.
      select: "{{external.groups}}"
      expansions:
        # matchers can contain regexp capture groups, which can be expanded in
        # "out" values
        - match: '^env-(\w+)$'
          out: ['$1']
        - match: '^admins$'
          out: ["prod"]
  allow:
    logins:
      - "{{vars.logins}}"
    node_labels:
      env: ["{{vars.allow_env}}"]
```

These variables could be used in another role without redefinition:

```yaml
kind: role
version: v5
metadata:
  name: app-users
spec:
  allow:
    app_labels:
      env: ["{{vars.allow_env}}"]
```

### Should we select the first matching expansion, or all matching expansions?

For the example

```yaml
vars:
  - name: "logins"
    select: "{{external.username}}"
    expansions:
      - match: "nic"
        out: [root]
      - match: "bob"
        out: [readonly]
      - match: "(.*)"
        out: ["$1"]
```

we could either choose to expand the first match or all matches, the results for
which are shown in this table:

username | first match | all matches
---------|-------------|------------
nic      | [root]      | [root, nic]
bob      | []          | [bob]
alice    | [alice]     | [alice]

In favor of expanding all matches, it is useful to be able to provide a default
which applies to all possible values.

In favor of selecting only the first match, it is useful to be able to exclude
some value from matching any other expansions.
This is especially true given Go's lack of support for negative-lookahead
regular expressions.

I'm not quite sure which is the best route to take here and am quite open to
suggestions from reviewers.

### When to compute variable values

Variables will be computed during `FetchRoles`, before traits normally are
expanded in the role definition.
This first occurs during login, and again when building an `AccessChecker` on
the proxy or on teleport Nodes/services before evaluating whether the user can
access a resource.

If it is desirable to avoid computing the variables more than once on login, the
expanded values could be stored in the user's traits, which would be encoded in
their certificates.
However, this would make it difficult to detect variable redefinition and would
make root cluster variables override alternate definitions in leaf clusters.

### What happens when the same variable name is defined in two different roles?

This is fine, until a user tries to login with both roles, in which case the
variable definition will conflict.

When fetching roles, we will detect if a variable is being redefined, and
return an error.
This will block login with two roles which define the same variable.

### Can variable definitions depend on other variables?

A simple implementation iterates all roles held by the current user and computes
the variables, then iterates all roles again and expands variables/traits in the
rest of the role.

This would allow variables defined in a single role to depend on variables
defined earlier in the same role spec, but would not guarantee that variables in
separate roles could depend on each other (it would depend on which role is
processed first).

To prevent confusion about variables intermittently being defined, we can
enforce no variable definition can depend on variables defined in a different
role. But I think it's simple and useful enough to allow variables to depend on
variables defined earlier in the same role.

### Trusted clusters

Variables defined in the root cluster will not be usable in leaf clusters.
It is expected that the variable will be redefined in the leaf cluster.

It's common for deployments to copy the exact same role definitions to all
clusters, so this seems like the most logical choice.
As long as the role which defines the variable is present in both clusters and
it is mapped to its leaf equivalent, you can seamlessly use the variable in both
clusters.

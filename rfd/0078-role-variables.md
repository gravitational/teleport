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
2. Variables should be easily referenced anywhere you can reference a trait.

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
  name: node_users
spec:
  vars:
    # Each variable has a name it will be referenced by.
    - name: "logins"
      # values hold a list of possible values which will all be appended to
      # form the complete list of values for the variable.
      values:
        # The simplest value is a list of static strings.
        - out: [ubuntu]

        # Values can also contain traits and transforms.
        - out: ['{{regexp.replace(external.username, "-", "_")}}']

        # Values can also take an input, which will usually be a trait.
        - input: "{{external.email}}"
          # match is a regular expression which must match the input in order
          # for this value to be included.
          match: "^nic@goteleport.com$"
          out: [root]

        - input: "{{external.email}}"
          # match may contain contain capture groups which can be expanded in
          # the output.
          #
          # Here "alice@goteleport.com" maps to "alice", but "foo@example.com"
          # will not be included because it does not match the regex.
          match: "^(.*)@goteleport.com$"
          out: ["$1"]

    - name: "allow-env"
      values:
        # An input will expand to a list of N inputs if the trait is a list of size N.
        # Any input value which matches the `match` regex will contribute one
        # output value, which may contain regex captures from that specific input.
        #
        # Here [devs, env-staging, env-prod] maps to [staging, prod].
        - input: "{{external.groups}}"
          match: '^env-(\w+)$'
          out: ['$1']

        # If any of the N input values match the `not_match` regex, none of the
        # values will be used.
        #
        # Here [devs] maps to [dev], but [devs, contractors] maps to []
        - input: "{{external.groups}}"
          match: '^devs$'
          not_match: 'contractors'
          out: ["dev"]
  allow:
    logins:
      # Variables can be referenced as `vars.<variable_name>` or
      # `vars["<variable_name"]`.
      - "{{vars.logins}}"
    node_labels:
      env: ["{{vars["allow-env"]}}"]
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
      env: ["{{vars["allow-env"]}}"]
```

### How to reference variables

Variables can be referenced similar to traits, by enclosing their name in
`{{}}`.

Variable names are prefixed by that `vars` namespace to avoid name collisions
with external traits.

You can use the selector syntax `{{vars.variable_name}}` or the index syntax
`{{vars["variable-name"]}}`.

It is necessary to use the index syntax and enclose the variable name in quotes
if it is not a valid
[Go identifier](https://go.dev/ref/spec#Identifiers),
or else the parser will not be able to handle it.
The same is true for traits today.

Variables can only be referenced in the role which defines them.
The idea of allowing variables to be defined and referenced accross different
roles was explored, but it has some downsides:

1. Scope of the variable is not clear.
2. Variable dependencies could create cycles.
3. Today each role is self-sufficient and can be linted and checked on its own,
   depending on variables in other roles would break that and make things much
   more complicated.

### Where variables can be used

Variables can be used anywhere traits can be used in a role spec, including
allow or deny rules such as `logins` or `node_labels`, impersonation conditions,
etc.

### When to compute variable values

Variables will be computed during `FetchRoles`, before traits normally are
expanded in the role definition.
This first occurs during login, and again when building an `AccessChecker` on
the Proxy or on Teleport Nodes/services before evaluating whether the user can
access a resource.

If it is desirable to avoid computing the variables more than once on login, the
expanded values could be stored in the user's traits, which would be encoded in
their certificates.
However, this would make it difficult to detect variable redefinition and would
make root cluster variables override alternate definitions in leaf clusters.
Computing variable values will not be much more intensive than what we already
do to expand traits, and I don't expect it will noticably slow anything down.
The performance impact can be further explored during implementation.

### What happens when the same variable name is defined in two different roles?

Variables are local to the role which defines them only, variables defined in
other roles will not be visible or effect the current role in any way.

### Can variable definitions depend on other variables?

Yes, within the variable definition block variables may depend on other
variables only if they are defined *earlier*, or higher-up, in the variable
list.

It would be technically feasible not to enforce this ordering by sorting the
variables topologically based on their dependencies, but requiring the ordering
forces admins to carefully consider their dependencies and automatically avoids
cycles, which is arguably for the best.

Example:

```yaml
kind: role
version: v5
metadata:
  name: example
spec:
  vars:
    - name: var_a
      values:
          out: ["foo"]
    - name: var_b
      values:
          out:
            # var_b can depend on var_a because it is defined above.
            - "{{vars.var_a}}"
            # Error: cannot depend on a variable defined later in the list.
            # - "{{vars.var_c}}"
    - name: var_c
      values:
          # Variables can be used as inputs.
          input: "{{var_a}}"
          match: "^foo$"
          out:
            - "bar"
            - "{{vars.var_b}}"
```

### Trusted clusters

Variables defined in the root cluster will not be usable in leaf clusters.
It is expected that the variable will be redefined in the leaf cluster.

It's common for deployments to copy the exact same role definitions to all
clusters, so this seems like the most logical choice.
As long as the role which defines the variable is present in both clusters and
it is mapped to its leaf equivalent, you can seamlessly use the variable in both
clusters.

## Examples

### Defined allow labels for each group

If the Teleport admin cannot create custom traits in their IDP, then it is
extremely cumbersome and repetitive (borderline impossible) to define a custom
mapping per group.

Solution today:

```yaml
kind: role
version: v5
metadata:
  name: users
spec:
  allow:
    node_labels:
      env:
        - regexp.replace(external.groups, "dev", "dev")
        - regexp.replace(external.groups, "dev", "staging")
        - regexp.replace(external.groups, "qa", "qa")
        - regexp.replace(external.groups, "qa", "staging")
    app_labels:
      env:
        - regexp.replace(external.groups, "dev", "dev")
        - regexp.replace(external.groups, "dev", "staging")
        - regexp.replace(external.groups, "qa", "qa")
        - regexp.replace(external.groups, "qa", "staging")
```

The full list would have to be repeated for every value in every role. I don't
think anyone even knows you can do this, and I hope no-one actually does,
because it's ridiculous.

With role variables it becomes trivial:

```yaml
kind: role
version: v5
metadata:
  name: users
spec:
  vars:
    - name: allow_envs
      values:
        - input: "{{external.groups}}"
          match: "^(qa|devs)$"
          out:
            - "$1"
            - "staging"
  allow:
    node_labels:
      env: "{{vars.allow_envs}}"
    app_labels:
      env: "{{vars.allow_envs}}"
```

The same variable can easily be defined once, and used many times within this
role and other roles.

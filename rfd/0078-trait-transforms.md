---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 78 - Trait Transforms

## What

A new feature which allows admins to filter, transform, and extend incoming
SAML assertions and OIDC claims from their identity provider before they are
embedded in the traits of their Teleport user's certificates.

## Why

Teleport admins often do not have control over their company's SSO provider and
which claims will be provided to Teleport.
All of these claims will be embedded in each user's Teleport certificate as
`traits`.

SSO providers can include hundreds to thousands of claims which are not
necessary or used by Teleport at all, unecessarily bloating the certificate size
of all users.
This can adversly impact latency and throughput of Teleport operations, and in
an extreme case could reach a limit which would render Teleport unusable with
that identity provider.
Trait Transforms will allow the Teleport admin to filter out claims which are
unecessary before they make it into the Teleport certificate.

Admins often wish to extend an incoming claim with extra values which make sense
for their Teleport deployment.
For example, users within the `splunk` group should also be added to the `dbs`
group within Teleport so that they can access databases.
Trait Transforms will allow the Teleport admins to modify existing traits and
add new ones to solve these problems.

If teleport admins cannot add or change the claims provided by their SSO
provider, making simple rules like this can be extremely cumbersome or
impossible within teleport.
There are currently teleport users with deployments which automatically generate
hundreds to thousands of roles to get around these limitations.

## Details

Example trait transform spec:

```yaml
kind: trait_transform
version: v1
metadata:
  name: example
spec:
  # priority can be used to order the evaluation of multiple trait transforms in
  # a cluster. Lower priorities will be evaluated first. Trait transforms with
  # the same priority will be ordered by a lexicographical sort by their names. 
  priority: 0

  # filter is a predicate expression which will be applied to each incoming
  # trait. `trait.key` and `trait.values` will be available to use in the
  # expression. If the expression returns `false` for a given trait, it will be
  # filtered out and excluded from further steps in the trait transform and from
  # the user's certificate.
  #
  # In this example, only the "groups", "username", and "email" traits will be
  # included. More powerful expressions are possible and will be explored later.
  filter: >
    contains(list("groups", "username", "email"), trait.key)

  # override holds a map of trait keys to predicate expressions which should
  # return the desired value for that trait. This can be used to override
  # existing traits, or add new ones.
  #
  # All incoming traits are available for use in the expression by
  # `external.<trait_name>` or `external["<trait_name>"]`.
  #
  # Various helper functions are provided, which will be explained later in the
  # RFD.
  override:
    # groups will override the existing groups trait by appending the "dbs"
    # group if the user's current groups includes "splunk".
    groups: >
      match(contains(external.groups, "splunk"),
        option(true, list(external.groups, "dbs")),
        option(false, external.groups))

    # logins will be a new trait added to the cert.
    logins: >
      list(
        "ubuntu",

        regexp.replace(external.username, "-", "_"),

        match(external.email,
          option("nic@goteleport.com", "root")),

        match(regexp.replace(external.email, "^.*@goteleport.com$", "teleporter"),
          option("teleporter", email.local(external.email)),
          default_option("contractor")))
```

### Predicate Helper Functions

A set of helper functions to be used in the predicate expressions will be
included.
Part of the reasoning for using the
[predicate language](https://github.com/vulcand/predicate)
is so that it will be extensible.
As the need arises, we can always add new helper function.

The predicate parser is based on the `Go` language parser, so `Go` keywords
which may be better names (such as `select`, `default`) must unfortunately be
avoided.

#### `list(...items)`

`list()` returns a flattened and deduplicated list of all of its arguments. `list("a", list("b",
"c"), list(), list("a"))` will return ["a", "b", "c"]


#### `contains(list, value)`

`contains(list, value)` returns `true` if any item in the list is an exact match
for `value`, else it returns false. `contains(list("a", "b"), "b")` returns
true.

#### `match(value, ...options)`

`match(value, ...options)` returns the value of the first `option` which is a
match for `value`.

`match("c", option("a", "foo"), option("b", "bar"), option("c", "baz"))` returns
`"baz"`

`match(true, option(false, list("a", "b")), option(true, list("c", "d")))`
returns `["c", "d"]`

`default_option(value)` will match for any input value.

`match("xyz", option("abc", "go"), default_option("stop"))` returns `"stop"`.

We could easily add `regexp_option` which would match based on a regular
expression, and/or `match_all` which would return the concatenation of all
matching options.

#### `email.local(email)`

Has the same meaning it currently does in
[role templates](https://goteleport.com/docs/access-controls/guides/role-templates/#interpolation-rules).

Returns the local part of an email field.

#### `regexp.replace(variable, expression, replacement)`

Has the same meaning it currently does in
[role templates](https://goteleport.com/docs/access-controls/guides/role-templates/#interpolation-rules).

Finds all matches of expression and replaces them with replacement. This
supports expansion, e.g.
`regexp.replace(external.email, "^(.*)@example.com$", "$1")`

### Using transformed traits in roles

Trait transforms transparently filter, override, and extend incoming traits
during login.
To the rest of the cluster, including role templates, they will appear identical
to normal traits which are typically reference by `{{external.<trait_name>}}`.

### When trait transforms will be parsed and evaluated

Trait transforms will be evaluated during each user login.
This will occur after the SSO provider has returned its assertions/claims, and
before Teleport maps these to internal Teleport roles via `attributes_to_roles` or
`claims_to_roles` so that transformed traits can be used for this mapping.

The predicate expression should only need to be parsed a single time during the
first user login, the parsed value can be permanently cached in memory.
The parsed expression will be evaluated during login with the context of the
unique user's traits.

During login, the auth server will load all `trait_transform` resources in the
cluster, sort them by `priority` and `name`, and apply all of them in order.

### Trusted clusters

Since trait transforms will be evaluated during login, they only need to be
created in the root cluster and the transformed traits will be visible and
usable in all leaf clusters.

### Creating and modifying trait transforms

Trait transforms will be a new backend resource.

They should be written in a YAML file and created by the usual means of
`tctl create resource.yaml`. `tctl get trait_transforms/example` can be used to
fetch the current resource, and `tctl rm trait_transforms/example` can be used
to delete the resource.

### Validating and testing trait transforms

A new `tctl` command will be introduced, which can be used to experiment with
trait transforms and test what their effect will be before deploying them to
your cluster.

```bash
$ tctl test trait_transform \
  --load transform1.yaml \
  --load transform2.yaml \
  --load-from-cluster \
  --input_traits '{"groups": ["splunk"], "email": "nic@goteleport.com", "username": "nklaassen"}'
```

You can load multiple yaml files with `--load resource.yaml`, optionally load
existing trait transforms from the cluster with `--load-from-cluster`, and
provide a set of input traits to test with.

The command will report any syntax errors, and will print the output traits for
the given input.

### Future Work

The existing `claims_to_roles` and `attributes_to_roles` offer only a simple
static mapping.
This could also be factored out of the SAML and OIDC connecter specifications
into a new resource which could leverage the predicate language to create more
powerful expressions to determine which roles a user should receive.
This will give us a complete system for solving the problem of incomplete data
coming from identity providers

## Extra Examples

### Static list defined per group

```yaml
kind: trait_transform
version: v1
metadata:
  name: example
spec:
  priority: 0
  override:
    # For a single-valued external.group, the following will work:
    allow_env: >
      match(external.group,
        option(list("dev"), list("dev", "staging")),
        option(list("qa"), list("qa", "staging")),
        option(list("admin"), list("dev", "qa", "staging", "prod")))

    # If external.groups is a list, you can match on each one
    allow_env: >
      list(
        match(true, option(contains(external.groups, "dev"), list("dev", "staging"))),
        match(true, option(contains(external.groups, "qa"), list("qa", "staging"))),
        match(true, option(contains(external.groups, "admin"), list("dev", "qa", "staging", "prod"))))
```

--- 

## Previous design discussed below, no longer relevant
## RFD 78 - Role Variables

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

With more roles and more complex rules, this would be even more difficult. The
full list would have to be repeated for every value in every role. I don't think
anyone even knows you can do this, and I hope no-one actually does, because it's
ridiculous.

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
role.

## Alternatives

### Trait transformations

Instead of defining variables in the role spec, we could create a new `trait`
resource which can define new traits which will be computed during login and
embedded in the user's cert.

These traits would be reusable across roles and trusted clusters.

The syntax could be similar to the proposed role variable syntax:

```yaml
kind: trait
version: v1
metadata:
  name: groups
spec:
  values:
    # An input will expand to a list of N inputs if the trait is a list of N.
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
```

Will likely switch to this trait method, possibly with different syntax.

### Predicate language for variable assignment

Instead of the proposed regex-based construct, variables (or traits) could be
defined using the
[`vulcand/predicate`](https://github.com/vulcand/predicate)
language.
`vulcand/predicate` is already used
elsewhere in teleport, in search and condition evaluation.

`predicate` is perfectly capable of manipulating lists of strings with the help
of a few new functions that can easily be added as extensions.
I've written a simple sandbox to experiment at
https://github.com/nklaassen/predicate-sandbox.

Rewritten to use the predicate language, the first example in this RFD could
look like:

```yaml
kind: role
version: v5
metadata:
  name: node_users
spec:
  vars:
    - name: "logins"
      # values should be an expression which returns a list of strings
      values: >
        // The concat helper returns the concatenation of 0 or more strings or
        // lists of strings
        concat(
          "ubuntu",

          // The transform helper applies the transform function to each item in
          // the list. The replace helper does a regex replacement on its input.
          transform(external.username, replace("-", "_")),

          // ifelse evaluates the condition in its first argument. If true it
          // returns the second argument, else it returns the third argument.
          ifelse(contains(external.email, "nic@goteleport.com"), "root", concat()),

          // filter returns a filtered list containing only elements of its
          // first argument which match the filter in the second argument.
          // The replace helper support regex capture group replacements.
          transform(filter(external.email, matches("@goteleport.com")), replace("^(.*)@goteleport.com", "$1")))

    - name: "allow-env"
      values: >
        concat(
          transform(
            filter(external.groups, matches(`^env-\w+$`)),
            replace(`^env-(\w+)$`, "$1")),
          ifelse(
            contains(external.groups, "contractors"),
            // concat with no args returns an empty list
            concat(),
            ifelse(contains(external.groups, "devs"), "dev", concat())))

  allow:
    logins:
      - "{{vars.logins}}"
    node_labels:
      env: ["{{vars["allow-env"]}}"]
```

Pros:

- more powerful expressions
- more fun to write

Cons:

- arguably more complex
- the usual result of a `vulcand/predicate` expression is a boolean predicate,
  the UX for building lists of strings is not great

### Predicate language for filtering only

Instead of using predicate language to build and assign variables, we could use
a syntax more similar to the proposed regex syntax and only use predicates for
matching.

```yaml
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
      - input: "external.email"
        # filter is a predicate expression which must evaluate `true` in order
        # for this value to be included. `item` will be used to refer to the
        # current item of the input which is being checked.
        filter: 'item == "nic@goteleport.com"'
        out: [root]

      - input: "external.email"
        # `item` can be used in `out` as well.
        filter: 'matches("@goteleport.com$", item)'
        out: ['regexp.replace(item, "^(.*)@goteleport.com$", $1)']

  - name: "allow-env"
    values:
      # An input will expand to a list of N inputs if the trait is a list of size N.
      # Any input value which matches the `filter` expression will contribute one
      # output value.
      #
      # `filter` and `out` will be evalauated for each item in the list, and
      # `item` can be used to refer to that list item.
      #
      # Here [devs, env-staging, env-prod] maps to [staging, prod].
      - input: "{{external.groups}}"
        out: ['regexp.replace(item, "^env-(\w+)$", "$1")']

      # Full traits are available in the filter.
      # Here [devs] maps to [dev], but [devs, contractors] maps to []
      - input: "{{external.groups}}"
        filter: 'matches("^devs$") && !contains(external.groups, "contractors")'
        out: ["dev"]
```

Pros:

- predicate expressions are used as just that: predicates (rather than as they
  are in alternative "Predicate language for variable assignment")
- predicate expressions are more powerful than just regex's

Cons:

- may need to duplicate regex in `filter` and `out` to use capture groups

### Common Expression Language

Edit: decided against this to avoid adding another configuration language to
Teleport, especially one so similar to `vulcand/predicate` which is already
used.

Another option for defining role variables (or traits) would be to use an
expression language rather than the proposed regular expression based
construction.

Rewritten to use
[CEL](https://github.com/google/cel-go)
the first example in this RFD could look like:

```yaml
kind: role
version: v5
metadata:
  name: node_users
spec:
  vars:
    - name: "logins"
      values: >
        ['ubuntu'] +
        external.username.map(username, username.replace('-', '_')) +
        ('nic@goteleport.com' in external.email ? ['root'] : []) +
        external.email.map(email, email.matches('^[^@]+@goteleport.com$'), email.replace('@goteleport.com', '', 1))

    - name: "allow-env"
      values: >
        external.groups.map(group, group.matches('^env-\\w+$'), group.replace('env-', '', 1)) +
        ('contractors' in external.groups ? [] : 'devs' in external.groups ? ['dev'] : [])

  allow:
    logins:
      - "{{vars.logins}}"
    node_labels:
      env: ["{{vars["allow-env"]}}"]
```

For this example, I couldn't find any implementation of regex replacement with
capture groups, so worked without it. We could relatively easily implement a
custom CEL extension to support regex capture groups.

Pros:

- more powerful
- already well-defined and documented language

Cons:

- the usual result of a CEL expression is a
  [boolean](https://github.com/google/cel-spec/blob/6040c0a6df9601751e628405706bac18948b8eb3/README.md?plain=1#L46),
  the UX for building lists of strings is not great
- one more language users need to learn to configure teleport (yaml, custom
  `{{external.trait}}` syntax with transforms and clauses, predicate language)

I've written a simple sandbox to experiment with CEL for this usecase at
https://github.com/nklaassen/cel-sandbox

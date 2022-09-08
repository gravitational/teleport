---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 78 - Login Rules

## What

A new feature which allows admins to filter, transform, and extend incoming
SAML assertions and OIDC claims from their identity provider during user login
before they are embedded in the traits of their Teleport user's certificates.

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
Login rules will allow the Teleport admin to filter out claims which are
unecessary before they make it into the Teleport certificate.

Admins often wish to extend an incoming claim with extra values which make sense
for their Teleport deployment.
For example, users within the `splunk` group should also be added to the `dbs`
group within Teleport so that they can access databases.
Login rules will allow the Teleport admins to modify existing traits and
add new ones to solve these problems.

If Teleport admins cannot add or change the claims provided by their SSO
provider, making simple rules like this can be extremely cumbersome or
impossible within teleport.
There are currently teleport users with deployments which automatically generate
hundreds to thousands of roles to get around these limitations.

## Details

Example login rule spec:

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  # priority can be used to order the evaluation of multiple login rules within
  # a cluster. Lower priorities will be evaluated first. Login rules with the
  # same priority will be ordered by a lexicographical sort by their names. 
  priority: 0

  # traits is a single predicate expression which must return a dict which will
  # set the user's traits during login
  traits: >
    dict(
      pair("groups",
        ifelse(external.groups.contains("splunk"),
          external.groups.add("dbs"),
          external.groups)),
      pair("logins",
        set(
          "ubuntu",
          ifelse(external.username.contains("nic@goteleport.com"), "root", ""),
          ifelse(external.organization.contains("teleport"), "teleporters", "external"))))

  # eventually login_rules could also return the desired teleport roles for the
  # user to allow custom logic, but this will be out of scope for the initial
  # implementation
  # roles: >
  #   set("ssh-users", "db-users")
```

### Predicate Helper Functions

A set of helper functions to be used in the predicate expressions will be
included.
Part of the reasoning for using the
[predicate language](https://github.com/vulcand/predicate)
is so that it will be extensible.
As the need arises, we can always add new helper function.

The predicate parser is based on the `Go` language parser, so `Go` keywords
which may be better names (such as `if`) must unfortunately be avoided.

#### `set(...items)`

`set()` returns a set including all its arguments.
Duplicates are not possible and will be filtered out on creation of the set if
they are passed in.
All items must be strings.
As a special case, the empty string will not be included in the set.

#### `set.contains(value)`

`set.contains(value)` returns `true` if any item in the set is an exact match
for `value`, else it returns `false`.
`set("a", "b").contains("b")` returns `true`.

#### `set.add(value)`

`set.add(value)` returns a copy of the set with `value` added.
`set("a", "b").add("c")` returns `("a", "b", "c")`.

#### `set.remove(value)`

`set.add(value)` returns a copy of the set with `value` removed.
`set("a", "b", "c").remove("c")` returns `("a", "b")`.

#### `dict(...pairs)`

`dict()` returns a dictionary with keys of type `string` and values of type `set`.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "brocolli")),
)
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("asparagus", "brocolli"),
}
```

Arguments must be pairs, the first element will be the key,
and the second element will be the value.

#### `dict.add_values(key, ...values)`

`dict.add_values(key, ...values)` returns a copy of the dict with the given values
added to the set at `dict[key]`.
If `dict[key]` is empty or it does not exist, it will be added with `values` as
its only elements.

```
dict(
  pair("fruits", set("apple")),
).add("fruits", "banana").add("vegetables", "asparagus", "brocolli")
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("asparagus", "brocolli"),
}
```

#### `dict.remove_keys(...keys)`

`dict.remove_keys(...keys)` returns a copy of the dict with the given keys
removed.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "brocolli")),
).remove_keys("vegetables")
```

returns

```
{
  "fruits": ("apple", "banana"),
}
```

#### `dict.overwrite(key, values)`

`dict.overwrite(key, values)` returns a copy of the dict with the given `key` set
to `values`.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "brocolli")),
).overwrite("vegetables", set("carrot")).overwrite("trees", set("aspen"))
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("carrot"),
  "trees": ("aspen"),
}
```

#### `ifelse(cond, value_if_true, value_if_false)`

`ifelse(cond, value_if_true, value_if_false)` returns `value_if_true` if `cond`
evaluates to `true`, else it returns `value_if_false`.
`ifelse(set("a").contains("a"), set("b", "c"), set())`
returns
`("b", "c")`.

#### `match(value, ...options)`

`match(value, ...options)` returns the value of the first `option` which is a
match for `value`.

`match("c", option("a", "foo"), option("b", "bar"), option("c", "baz"))` returns
`"baz"`

`match(true, option(false, set("a", "b")), option(true, set("c", "d")))`
returns `("c", "d")`

`default_option(value)` will match for any input value.

`match("xyz", option("abc", "go"), default_option("stop"))` returns `"stop"`.

We could add `regexp_option` which would match based on a regular expression,
and/or `match_all` which would return the concatenation of all matching options.

#### `strings.replace(input, match, replacement)`

Finds all matches of `match` in `input`, and replaces them with `replacement`.
`strings.replace("user-nic", "-", "_")` returns `"user_nic"`.
`input` can be a string or a set of strings, in which case the replacement will
be applied to all strings in the set.

### Using transformed traits in roles

Login rules transparently set the user's traits during login.
To the rest of the cluster, including role templates, they will appear identical
to normal traits which are typically reference by `{{external.<trait_name>}}`.

### When login rules will be parsed and evaluated

Login rules will be evaluated during each user login.
This will occur after the SSO provider has returned its assertions/claims, and
before Teleport maps these to internal Teleport roles via `attributes_to_roles` or
`claims_to_roles` so that transformed traits can be used for this mapping.

An eventual extension to login rules could also return the desired set of
Teleport roles which the user should have.

The predicate expression should only need to be parsed a single time during the
first user login, the parsed value can be permanently cached in memory (keyed by
the full plaintext predicate expression).
The parsed expression will be evaluated during login with the context of the
unique user's SAML/OIDC assertions/claims.

During login, the auth server will load all `login_rule` resources in the
cluster, sort them by `priority` and `name`, and apply all of them in order.

### Trusted clusters

Since login rules will be evaluated during login and the resulting traits will
be embedded in the user's certificates, they only need to be created in the root
cluster and the transformed traits will be visible and usable in all leaf
clusters.

### Creating and modifying login rules

Login rules will be a new backend resource `login_rule`.

They should be written in a YAML file and created by the usual means of
`tctl create resource.yaml`. `tctl get login_rule/example` can be used to
fetch the current resource, and `tctl rm login_rule/example` can be used
to delete the resource.

### Validating and testing login rules

A new `tctl` command will be introduced, which can be used to experiment with
login rules and test what their effect will be before deploying them to
your cluster.

```bash
$ tctl test login_rule \
  --load login_rule1.yaml \
  --load login_rule2.yaml \
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
into the login rule in order to leverage the predicate language to create more
powerful expressions to determine which roles a user should receive.
This will give us a complete system for solving the problem of incomplete data
coming from identity providers

## Extra Examples

### Set a trait to a static list of values defined per group

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits: >
    external.overwrite("allow-env",
      match(external.group,
        option(set("dev"), set("dev", "staging")),
        option(set("qa"), set("qa", "staging")),
        option(set("admin"), set("dev", "qa", "staging", "prod")))
```

### Use only specific traits provided by the OIDC/SAML provider

To only keep the `groups` and `email` traits, with their original values:

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits: >
    dict(
      pair("groups", external.groups),
      pair("email", external.email))
```

### Remove a specific trait

To remove a specific trait and keep the rest:

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits: >
    external.remove_keys("big-trait")
```

### Extend a specific trait with an extra value

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits: >
    external.add_values("logins", "ec2-user")
```

### Use the output of 1 login rule in another rule

```yaml
kind: login_rule
version: v1
metadata:
  name: set_groups
spec:
  priority: 0
  traits: >
    external.add_values("groups",
      ifelse(external.groups.contains("admins"),
        "superusers",
        ""))
---
kind: login_rule
version: v1
metadata:
  name: set_logins
spec:
  priority: 1
  traits: >
    external.add_values("logins",
      ifelse(external.groups.contains("superusers"),
        "root",
        ""))
```

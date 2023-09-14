---
authors: Nic Klaassen (nic@goteleport.com)
state: implemented
---

# RFD 78 - Login Rules

## Required Approvers

- @klizhentas

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
necessary or used by Teleport at all, unnecessarily bloating the certificate size
of all users.
This can adversely impact latency and throughput of Teleport operations, and in
an extreme case could reach a limit which would render Teleport unusable with
that identity provider.
Login rules will allow the Teleport admin to filter out claims which are
unnecessary before they make it into the Teleport certificate.

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
  # a cluster.
  #
  # Login rules with lower numbered priorities will be applied first, followed
  # by rules with priorities in increasing order. In case of a tie, login rules
  # with the same priority will be ordered by a lexicographical sort of their
  # names.
  priority: 0

  # traits_map will determine the traits of all users who log in to this cluster.
  #
  # This is a YAML map where the key must be a static string which will be the
  # final trait key, and the value is a list of predicate expressions which each
  # must return a set of strings. The final trait will be set to the union of
  # the resulting string sets of all predicate expressions for that trait key.
  #
  # traits_map must contain the complete set of desired traits, any external
  # traits not found here will not be included in the user's certificates.
  traits_map:
    # keep an external trait, unmodified
    logins:
      - external.logins

    # rename an external trait
    db_logins:
      - external.Database_Usernames

    # merge external traits
    kube_groups:
      - external.groups
      - external.kubernetes_groups

    # transform an external trait
    apps:
      - 'lower(external.apps)'

    # extend an external trait
    windows_logins:
      - external.windows_logins
      - "bill"

    # conditionally extend an external trait
    groups:
      - 'ifelse(external.groups.contains("splunk"), external.groups.add("dbs"), external.groups)'

    # static string values
    tags:
      # if the "expression" is a static string or returns a single strings, this
      # will be automatically treated as a set of size 1.
      - "teleport"
      - "access"

  # traits_expression is a single predicate expression which must return a dict which will
  # set the user's traits during login.
  #
  # The dict must always have keys of type string and values of type set of
  # strings, or an error will be returned when the expression is evaluated.
  #
  # traits_expression is an alternative to traits_map, only one or the other
  # can be specified, never both in the same login_rule.
  #
  # traits_expression: >
  #   external.remove("irrelevant", "internal", "tags")
  #     .put("groups", 
  #       ifelse(external.groups.contains("splunk"),
  #         external.groups.add("dbs"),
  #         external.groups))
  #     .put("logins",
  #       set(
  #         "ubuntu",
  #         ifelse(external.groups.contains("admins"), "root", ""),
  #         ifelse(external.organization.contains("teleport"), "teleporters", "external"))
  #       .remove(""))

  # eventually login_rules could also return the desired teleport roles for the
  # user to allow custom logic, but this will be out of scope for the initial
  # RFD and implementation.
  # roles: >
  #   set("ssh-users", "db-users")
```

### Why both `traits_map` and `traits_expression`

`traits_expression` is a way for users with complex requirements to express
their desired traits.
It has the ability to modify the original external traits and operate on the
keys with logical expressions, rather than only have a map of desired traits
with static keys.

The `traits_expression` could also be generated by an external tool such as an
upcoming project where traits could be modelled in an external language with
support for theorem proving and formal verification.

The `traits_map` is a more approachable syntax for Teleport admins.
It is mostly YAML with some reference to external traits which should be
familiar if they have written templated Teleport roles.
There are some predicate helper functions available like `ifelse` and `lower`
which can be used to write reasonably powerful expressions without requiring the
user to grok dict and set manipulation in predicate.

```yaml
traits_map:
  key1: 
    - expr1
    - expr2
  key2:
    - expr3
```

Is really just another way of writing

```yaml
traits_expression: >
  dict(
    pair(key1, union(expr1, expr2)),
    pair(key2, union(expr3)))
```

### Context available in predicate expressions

For all predicate expressions in the login rule (values of `traits_map`, or the
`traits_expression`) the full dict of the users incoming external traits are
available under the `external` identifier.
This is similar to how traits are accessed in role templates, you can get trait
`example` with either `external["example"]` or `external.example`.

If there is only a single login rule in the cluster, the traits available will
be those provided by the SSO connector used to log in, and are the traits the
user would otherwise have been given if the login rule were not in place.

If the cluster includes multiple login rules, they will be sorted in increasing
order of their `priority` field (ties will be deterministically broken by a
secondary sort by the rule `name`).
The traits input to each successive login rule will be the output of the
previous login rule.

### Predicate Helper Functions

A set of helper functions to be used in the predicate expressions will be
included.
Part of the reasoning for using the
[predicate language](https://github.com/vulcand/predicate)
is so that it will be extensible.
As the need arises, we can always add new helper function.

The predicate parser is based on the `Go` language parser, so `Go` keywords
which may be better names (such as `if` or `select`) must unfortunately be avoided.

#### `ifelse(cond, value_if_true, value_if_false)`

`ifelse(cond, value_if_true, value_if_false)` returns `value_if_true` if `cond`
evaluates to `true`, else it returns `value_if_false`.
`ifelse(set("a").contains("a"), set("b", "c"), set())`
returns
`("b", "c")`.

Note: this would ideally be called just `if` but `ast.ParseExpr` used by our
parser does not accept Go keywords which are normally part of statements rather
than expressions.

#### `choose(...options)`

`choose(...options)` returns the value of the first `option` for which the
condition evaluates to `true`.

`choose(option(false, set("a", "b")), option(true, set("c", "d")))`
returns `("c", "d")`

`choose(option(set("a").contains("b"), "foo"), option(set("a").contains("a"), "bar"))`
returns `"bar"`.

Use an option with the condition hardcoded to `true` to set a default value.

`choose(option(set("a").contains("b"), "foo"), option(true, "default"))`
returns `"default"`.

The same could always be accomplished with a series of `ifelse` expressions, but
with the function syntax this would require deep nesting when there are many
options.

Note: this would ideally be called `select(...case)` but `ast.ParseExpr` used by
our parser does not accept Go keywords which are normally part of statements
rather than expressions.

#### `strings.replaceall(input, match, replacement)`

Finds all literal string matches of `match` in `input`, and replaces them with
`replacement`.
`strings.replaceall("user-nic", "-", "_")` returns `"user_nic"`.
`input` can be a string or a set of strings, in which case the replacement will
be applied to all strings in the set.

#### `strings.upper(input)`

`strings.upper(input)` returns a copy of the input string converted to uppercase.
`strings.upper("ExAmPlE")` returns `"EXAMPLE"`.
`input` can be a string or a set of strings, in which case all strings in the
set will be converted to uppercase.

#### `strings.lower(input)`

`strings.lower(input)` returns a copy of the input string converted to lowercase.
`strings.lower("ExAmPlE")` returns `"example"`.
`input` can be a string or a set of strings, in which case all strings in the
set will be converted to lowercase.

#### `set(...items)`

`set()` returns a set including all its arguments.
Duplicates are not possible and will be filtered out on creation of the set if
they are passed in.
All items must be strings.

#### `set.contains(value)`

`set.contains(value)` returns `true` if any item in the set is an exact match
for `value`, else it returns `false`.
`set("a", "b").contains("b")` returns `true`.

#### `set.add(...values)`

`set.add(...values)` returns a copy of the set with `values` added.
All values must be strings.
`set("a", "b").add("c").add("d", "e")` returns `("a", "b", "c", "d", "e")`.

#### `set.remove(...values)`

`set.remove(...values)` returns a copy of the set with `values` removed.
`set("a", "b", "c", "d").remove("d").remove("c", "b")` returns `("a")`.

#### `union(...sets)`

`union(...sets)` returns a new set which holds the union of all given sets.

`union(set("a", b"), set("c"))` returns `("a", "b", "c")`

#### `dict(...pairs)`

`dict()` returns a dictionary with keys of type `string` and values of type `set`.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "broccoli")),
)
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("asparagus", "broccoli"),
}
```

Arguments must be pairs, the first element will be the key, and the second
element will be the value.
The key must be a string and the value must be a set (of strings).

#### `dict.add_values(key, ...values)`

`dict.add_values(key, ...values)` returns a copy of the dict with the given
values added to the set at `dict[key]`.
If `dict[key]` is empty or it does not exist, it will be added with `values` as
its only elements.

```
dict(
  pair("fruits", set("apple")),
).add_values("fruits", "banana").add_values("vegetables", "asparagus", "broccoli")
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("asparagus", "broccoli"),
}
```

#### `dict.remove(...keys)`

`dict.remove(...keys)` returns a copy of the dict with the given keys
removed.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "broccoli")),
).remove("vegetables")
```

returns

```
{
  "fruits": ("apple", "banana"),
}
```

#### `dict.put(key, value)`

`dict.put(key, value)` returns a copy of the dict with the given `key` set
to `value`.
Dictionary keys are always strings and the value must always be a set, else an
error will be returned when the expression is evaluated.

```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "broccoli")),
).put("vegetables", set("carrot")).put("trees", set("aspen"))
```

returns

```
{
  "fruits": ("apple", "banana"),
  "vegetables": ("carrot"),
  "trees": ("aspen"),
}
```

### Modifications to predicate

The predicate language currently does not support "methods" on objects such as
`dict.remove` or `set.contains` as described above.
We will need to add support for these in our
[gravitational/predicate](https://github.com/gravitational/predicate)
fork.

^ Update: method support has been implemented
[here](https://github.com/gravitational/predicate/pull/4), remaining changes
can all happen in teleport.

### Using transformed traits in roles

Login rules transparently set the user's traits during login.
To the rest of the cluster, including role templates, they will appear identical
to normal traits which are typically reference by `{{external.<trait_name>}}`.

### When login rules will be parsed and evaluated

Login rules will be parsed and evaluated during each SSO user login.
This will occur after the SSO provider has returned its assertions/claims, and
before Teleport maps these to internal Teleport roles via `attributes_to_roles` or
`claims_to_roles` so that transformed traits can be used for this mapping.

An eventual extension to login rules could also return the desired set of
Teleport roles which the user should have.

During login, the auth server will load all `login_rule` resources in the
cluster, sort them by `priority` and `name`, and apply all of them in order.

### Local users

Login rules will not apply to local users for the initial release of login
rules.
One technical reason for this is that the user's static traits are held in its
`User` resource, and these are sometimes accessed by teleport subsystems to
determine the traits of various users.
If these were different than the dynamic traits the user would get on login it
would create inconsistencies.

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
  --resource-file login_rule1.yaml \
  --resource-file login_rule2.yaml \
  --load-from-cluster \
  <<< '{"groups": ["splunk"], "email": "nic@goteleport.com", "username": "nklaassen"}'
```

You can load multiple yaml files with `--resource-file resource.yaml`, optionally load
existing login rules from the cluster with `--load-from-cluster`, and
provide a set of input traits to test with.

The command will report any syntax errors, and will print the output traits for
the given input.

### Protobuf Definitions

The `login_rule` resource and associated CRUD RPCs will be added to a new
package `teleport/loginrule/v1`.

```
// loginrule.proto
...

// LoginRule is a resource to configure rules and logic which should run during
// Teleport user login.
message LoginRule {
  // Metadata is resource metadata.
  types.Metadata metadata = 1;

  // Version is the resource version of this login rule. Initially "v1" is
  // supported.
  string version = 2;

  // Priority is the priority of the login rule relative to other login rules
  // in the same cluster. Login rules with a lower numbered priority will be
  // evaluated first.
  int32 priority = 3;

  // TraitsMap is a map of trait keys to lists of predicate expressions which
  // should evaluate to the desired values for that trait.
  map<string, wrappers.StringValues> traits_map = 4;

  // TraitsExpression is a predicate expression which should return the
  // desired traits for the user upon login.
  string traits_expression = 5;
}
```


```
// loginrule_service.proto
...


// LoginRuleService provides CRUD methods for the LoginRule resource.
service LoginRuleService {
  // CreateLoginRule creates a login rule if one with the same name does not
  // already exist, else it returns an error.
  // (RFD note) Used for: tctl create rule.yaml
  rpc CreateLoginRule(CreateLoginRuleRequest) returns (LoginRule);

  // UpsertLoginRule creates a login rule if one with the same name does not
  // already exist, else it replaces the existing login rule.
  // (RFD note) Used for: tctl create -f rule.yaml
  rpc UpsertLoginRule(UpsertLoginRuleRequest) returns (LoginRule);

  // GetLoginRule retrieves a login rule described by the given request.
  rpc GetLoginRule(GetLoginRuleRequest) returns (LoginRule);

  // ListLoginRules lists all login rules.
  rpc ListLoginRules(ListLoginRulesRequest) returns (ListLoginRulesResponse);

  // DeleteLoginRule deletes an existing login rule.
  rpc DeleteLoginRule(DeleteLoginRuleRequest) returns (google.protobuf.Empty);
}

// CreateLoginRuleRequest is a request to create a login rule.
message CreateLoginRuleRequest {
  // LoginRule is the login rule to be created.
  LoginRule login_rule = 1;
}

// UpsertLoginRuleRequest is a request to upsert a login rule.
message UpsertLoginRuleRequest {
  // LoginRule is the login rule to be created.
  LoginRule login_rule = 1;
}

// GetLoginRuleRequest is a request to get a single login rule.
message GetLoginRuleRequest {
  // Name is the name of the login rule to get.
  string name = 1;
}

// ListLoginRulesRequest is a paginated request to list all login rules.
message ListLoginRulesRequest {
  // PageSize is The maximum number of login rules to return in a single
  // response.
  int32 page_size = 1;

  // PageToken is the NextPageToken value returned from a previous
  // ListLoginRules request, if any.
  string page_token = 2;
}

// ListLoginRulesResponse is a paginated response to a ListLoginRulesRequest.
message ListLoginRulesResponse {
  // LoginRules is the list of login rules.
  repeated LoginRule login_rules = 1;

  // NextPageToken is a token to retrieve the next page of results, or empty
  // if there are no more results.
  string next_page_token = 2;
}

// DeleteLoginRuleRequest is a request to delete a login rule.
message DeleteLoginRuleRequest {
  // Name is the name of the login rule to delete.
  string name = 1;
}
```

Note: there will be a separate Go struct type wrapping the LoginRule type which
can be marshalled to and from YAML by `tctl` and includes the requisite
[ResourceHeader](https://github.com/gravitational/teleport/blob/991651e872a46c3918eb280ed977147c9e9bf1ab/api/proto/teleport/legacy/types/types.proto#L145).

### Resource RBAC

The new resource `login_rule` will support the standard RBAC verbs
`list`, `create`, `read`, `update`, and `delete`.
These will all be added to the preset `editor` role for new and existing
clusters.

These can be defined in a role like
```
kind: role
version: v5
metadata:
  name: example
spec:
  allow:
    rules:
      - resources: [login_rule]
        verbs: [list, create, read, update, delete]
```

### Future Work

The existing `claims_to_roles` and `attributes_to_roles` in our SAML and OIDC
connectors offer only a simple static mapping of traits to roles.
This could also be factored out of the connecter specifications into the login
rule in order to leverage the predicate language to create more powerful
expressions to determine which roles a user should receive.
This will give us a complete system for solving the problem of incomplete data
coming from identity providers.

This could use a new field in the currently proposed login rule yaml spec.
We would need to figure out how to merge roles coming from multiple login rules
and from SAML and OIDC connectors before adding this.

## Extra Examples

### Set a trait to a static list of values defined per group

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits_expression: >
    external.put("allow-env",
      choose(
        option(external.group.contains("dev"), set("dev", "staging")),
        option(external.group.contains("qa"), set("qa", "staging")),
        option(external.group.contains("admin"), set("dev", "qa", "staging", "prod")),
        option(true, set()))
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
  traits_expression: >
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
  traits_expression: >
    external.remove("big-trait")
```

### Extend a specific trait with extra values

```yaml
kind: login_rule
version: v1
metadata:
  name: example
spec:
  priority: 0
  traits_expression: >
    external.add_values("logins", "ubuntu", "ec2-user")
```

### Use the output of 1 login rule in another rule

```yaml
kind: login_rule
version: v1
metadata:
  name: set_groups
spec:
  priority: 0
  traits_expression: >
    external.put("groups",
      ifelse(external.groups.contains("admins"),
        external["groups"].add("superusers"),
        external["groups"]))
---
kind: login_rule
version: v1
metadata:
  name: set_logins
spec:
  priority: 1
  traits_expression: >
    external.put("logins",
      ifelse(external.groups.contains("superusers"),
        external["logins"].add("root"),
        external["logins"]))
```

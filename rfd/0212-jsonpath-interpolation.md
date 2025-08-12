---
authors: Brian Joerger (bjoerger@goteleport.com)
state: implemented
---

# RFD 212 - JSONPath Interpolation

## Required Approvers

* Engineering: @rosstimothy && (@zmb3 || @nklaassen)

## What

Add the ability to handle arbitrary JSON OIDC claims using a new interpolation
function, `jsonpath`. This function will be supported in login rules so that
administrators can map JSON claims to standard user traits for use in
`claims_to_roles` mapping, [role templating](https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/),
etc.

## Why

Currently, Teleport assumes that OIDC claims are either a string or list of
strings. However, technically OIDC claims may be arbitrary JSON objects, and we
have run into some custom OIDC solutions that rely on that capability. This
feature is necessary for Teleport to integrate with these OIDC solutions.

## Details

### JSONPath

[JSONPath](https://support.smartbear.com/alertsite/docs/monitors/api/endpoint/jsonpath.html)
is a query language used to query JSON values in a JSON object, which is
perfect for this use case.

Before continuing to read this RFD, you should familiarize yourself with the
basics of [JSONPath syntax](https://support.smartbear.com/alertsite/docs/monitors/api/endpoint/jsonpath.html).

When going through the JSONPath examples, you may find it useful to run the
queries in a [sandbox](https://serdejsonpath.live/).

#### `jsonpath` Expression Function

The `jsonpath` function will be added as a trait expression function, but will
only be supported for `login_rule` expressions (`traits_map` or `traits_expression`).

It can be used to interpolate a string or strings from arbitrary JSON claims.
For example, see the JSON object and jsonpath examples below:

```json
{
  "a": ["1", "2", "3"],
  "b": {
    "c": "d"
  }
}
```

* `jsonpath("$.a")`   -> `["1", "2", "3"]`
* `jsonpath("$.b.*")` -> `["d"]`
* `jsonpath("$.*.*")` -> `["1", "2", "3", "d"]`

#### JSONPath Libraries

While the JSONPath query language has an [official RFC](https://www.rfc-editor.org/rfc/rfc9535.html)
as of 2024, the inception of the query language was in an [article](https://goessner.net/articles/JsonPath/)
written in 2007. The original article left many questions unanswered, causing
the JSONPath language to morph in several directions as different JSONPath
projects came up with their own answers.

While the RFC has helped to realign these disparate projects, there is much
more work to be done, as can be clearly seen by this [comparison project](https://cburgmer.github.io/json-path-comparison/).

According to the comparison project, https://github.com/ohler55/ojg is
currently the closest to the RFC specification out of the Go projects listed
there.

By using a project close to the specification, we can selectively rely on
official JSONPath documentation, like those linked above, as well as the
[online sandboxes](https://serdejsonpath.live/).

Note: We must be careful when updating the upstream library, as some of the
syntax may be subject to change as it adjusts to the RFC and some of the
unresolved syntax disagreements in the community.

### OIDC Claims

During OIDC login, a user's OIDC claims serve two purposes:

* Setting claims as Teleport user traits, optionally using login rules for custom claims to traits mapping.
* Determining what Teleport roles to give the user, using the OIDC connector's `claims_to_roles` field to map user traits to roles.

The `jsonpath` function will be supported in login rule trait mapping. As a
result, any JSON OIDC claims mapped to traits in the claims to traits mapping
will be available in the traits to roles mapping.

Note: see [this section](#json-traits) for an explanation as to why we decided
not to set arbitrary JSON values as user traits directly, and instead are
requiring the use of login rules.

#### Login Rules - Claims to Traits

You can use a login rule to map JSON claims to users traits.

In the example below, a JSON claim is mapped to individual user traits.

```js
{
  // groups is a JSON object rather than a string array.
  "groups": {
    "roles": ["template"],
    "logins": ["alice"],
    "env": ["staging", "dev"],
  }
}
```

```yaml
kind: login_rule
version: v1
metadata:
  name: my-loginrule
spec:
  priority: 0
  traits_map:
    roles:
      # evaluates to ["template"]
      - jsonpath("$.groups.roles")
    logins:
      # evaluates to ["alice"]
      - jsonpath("$.groups.logins")
    env:
      # evaluates to ["staging", "dev"]
      - jsonpath("$.groups.env")
```

These traits can then be used in claims to roles mappings, role templates,
label expressions, etc.

```yaml
kind: oidc 
version: v2
metadata:
  name: my-idp
spec:
  ...
  claims_to_roles:
    - claim: "roles"
      value: "template"
      roles: ["template"]
```

```yaml
kind: role
version: v7
metadata:
  name: template
spec:
  ...
  allow:
    logins: '{{external.logins}}'
    node_labels_expression: 'contains(external.env, labels["env"])'
```

### UX

#### User stories

The user stories below will explore example custom OIDC solutions with
potential Teleport configurations to consume the custom OIDC claims using the
new `jsonpath` function.

##### Example: IdP with arbitrary JSON claims

Let's say we have a custom IdP which directly supports arbitrary JSON claims to
be set for users. Below is an example claim object for user `alice`.

```json
{
  "groups": {
    "teleport": {
      "roles": ["template"],
      "node": {
        "logins": "alice",
        "labels": {
          "host": "*"
        }
      },
      "app": {
        "labels": {
          "env": "staging"
        }
      }
    }
  }
}
```

We want to map the `groups.teleport.roles` claim to teleport roles, and map the
logins and labels to role conditions using role templating.

First, the we need to create a `login_rule` to map this arbitrary JSON
object into a set of user traits.

```yaml
kind: login_rule
version: v1
metadata:
  name: arbitrary-json-idp
spec:
  priority: 0
  traits_map:
    roles:
      # evaluates to ["template"]
      - jsonpath("$.groups.teleport.roles")
    logins:
      # evaluates to ["alice"]
      - jsonpath("$.groups.teleport.node.logins")
    node_labels_*:
      # evaluates to "*"
      - jsonpath("$.groups.teleport.node.labels['*']")
    node_labels_env:
      # evaluates to []
      - jsonpath("$.groups.teleport.node.labels.env")
    app_labels_*:
      # evaluates to []
      - jsonpath("$.groups.teleport.app.labels['*']")
    app_labels_env:
      # evaluates to "staging"
      - jsonpath("$.groups.teleport.app.labels.env")
```

Note: without [JSONPath-Plus syntax](#jsonpath-plus), it's not possible to grab
the property name value, so we can only map labels that we are aware of. In this
example, we are only looking for the `*` and `env` labels, so if the provider
added a claim like `"team": "devops"`, it would not be mapped without an
additional `traits_map` rule.

The mapped traits can now be referenced in the OIDC connector's `claims_to_roles`
mapping to assign the `template` role to the user.

```yaml
kind: oidc 
version: v2
metadata:
  name: arbitrary-json-idp
spec:
  ...
  claims_to_roles:
    - claim: "roles"
      value: "template"
      roles: ["template"]
```

Lastly, we can create the template role and utilize the mapped traits.

```yaml
kind: role
version: v7
metadata:
  name: template
spec:
  allow:
    logins: '{{external.logins}}'
    node_labels:
      '*': '{{external.node_labels_*}}'
      'env': '{{external.node_labels_env}}'
    app_labels:
      '*': '{{external.app_labels_*}}'
      'env': '{{external.app_labels_env}}'
```

In the end, Alice's effective role will be:

```yaml
kind: role
version: v7
metadata:
  name: template
spec:
  allow:
    logins: ['alice']
    node_labels:
      '*': '*'
    app_labels:
      'env': 'staging'
```

##### Example: Distributed IdP

Imagine a distributed IdP that aggregates claims for a user from multiple
different provider sources, where each provider is associated with a different
set of resources in Teleport.

```json
{
  "aggregated_claims": {
    "okta": {
      "logins": "alice",
      "env": ["staging", "dev"]
    },
    "auth0": {
      "logins": "devops",
      "env": ["prod"]
    },
    "github": {
      // no claims from github for this user.
    }
  }
}
```

Once again, we'll start with a login rule to map the JSON claim to traits.
Rather than mapping them directly to user traits, we map them in a way to
maintain separate labels for each of the aggregated providers. We will also
set a custom `teams` trait to aggregate the root property names of the
aggregated claims (e.g. `okta`).

```yaml
kind: login_rule
version: v1
metadata:
  name: distributed-idp
spec:
  priority: 0
  traits_map:
    okta_logins:
      # evaluates to ["alice"]
      - jsonpath("$.aggregated_claims.okta.logins")
    okta_env:
      # evaluates to ["staging", "dev"]
      - jsonpath("$.aggregated_claims.okta.env")
    auth0_logins:
      # evaluates to ["devops"]
      - jsonpath("$.aggregated_claims.auth0.logins")
    auth0_env:
      # evaluates to ["prod"]
      - jsonpath("$.aggregated_claims.auth0.env")
    github_logins:
      # evaluates to []
      - jsonpath("$.aggregated_claims.github.logins")
    github_env:
      # evaluates to []
      - jsonpath("$.aggregated_claims.github.env")
    teams:
      # evaluates to ["okta", "auth0"]
      - 'ifelse( !isempty( jsonpath("$.aggregated_claims.okta") ), set("okta"), set())'
      - 'ifelse( !isempty( jsonpath("$.aggregated_claims.auth0") ), set("auth0"), set())'
      - 'ifelse( !isempty( jsonpath("$.aggregated_claims.github") ), set("github"), set())'
```

The mapped traits can now be referenced in the OIDC connector's `claims_to_roles`
mapping to assign the roles based on the user's teams.

```yaml
kind: oidc 
version: v2
metadata:
  name: distributed-idp
spec:
  ...
  claims_to_roles:
    - claim: "teams"
      value: "^(okta|auth0|github)$"
      # evaluates to ["okta", "auth0"]
      roles: ["$1"]
```

We can now create the `okta` and `auth0` roles with templating to
reference the relevant claims mapped to user traits.

```yaml
kind: role
version: v7
metadata:
  name: okta
spec:
  allow:
    logins: '{{external.okta_logins}}'
    node_labels:
      'env': '{{external.okta_env}}'
      'team': "okta"
---
kind: role
version: v7
metadata:
  name: auth0
spec:
  allow:
    logins: '{{external.auth0_logins}}'
    node_labels:
      'env': '{{external.auth0_env}}'
      'team': "auth0"
```

### Proto

N/A

### Audit Events

All unaltered OIDC claims are included in the `user.login` audit event,
including claims which are not mapped to traits.

There is currently no audit event for when login rules are applied. The
easiest way to check login rule mapping and claim mapping logic is to use
`tctl sso test`, which can output what login rules successfully applied with
the `--debug` flag.

### Security

This RFD does not raise any security concerns outside of those already covered
in the [label expression RFD](https://github.com/gravitational/teleport/blob/master/rfd/0116-label-expressions.md#security).

### Additional Considerations

#### JSONPath-Plus

As mentioned in the JSONPath library [section](#jsonpath-libraries), there have
been many variations to the JSONPath syntax. One project that goes especially
beyond the JSONPath spec is the [JSONPath-Plus library](https://github.com/JSONPath-Plus/JSONPath).

One useful feature in particular is the ability to grab property names (`~`)
rather than values only. This would be useful for mapping arbitrary traits using
the property name and value in a JSON claim. Using JSONPath-Plus notation,
[the first example](#example-idp-with-arbitrary-json-claims) login rule node label
mapping could be simplified and made generic with an expression.

```json
{
  "groups": {
    "teleport": {
      "node": {
        "labels": {
          "*": "*",
          "env": ["staging", "dev"]
        }
      },
    }
  }
}
```

```yaml
kind: login_rule
version: v1
metadata:
  name: arbitrary-json-idp
spec:
  priority: 0
  # put_many would be a new expression to map an array of keys to an array
  # of values. e.g. ["*", "env"] and ["*", ["staging", "dev"]] would get
  # inserted as {"*": "*", "env": ["staging", "dev"]}
  traits_expression: |
    external.put_many(jsonpath("$.groups.teleport.node.labels~"), jsonpath("$.groups.teleport.node.labels"))
```

If we just need to support property name grabbing, it should be possible to do
so with a new function, `jsonpathprop`. This function would simply grab the
property name at the end of the JSONPath evaluation, so the example above would
be changed to:

```yaml
kind: login_rule
version: v1
metadata:
  name: arbitrary-json-idp
spec:
  priority: 0
  traits_expression: |
    external.put_many(jsonpathprop("$.groups.teleport.node.labels"), jsonpath("$.groups.teleport.node.labels"))
```

#### JSON traits

The [initial design](https://github.com/gravitational/teleport/blob/6e15920b8140ff7834223f3814e1b9de364033a7/rfd/0212-jsonpath-interpolation.md)
would take arbitrary JSON OIDC claims and set them directly as user traits.
For example, the [first user story example](#example-idp-with-arbitrary-json-claims)
would result in this user:

```yaml
kind: user
metadata:
  name: alice
spec:
  ...
  traits:
    "groups": {
      "teleport": {
        "roles": ["template"],
        "node": {
          "logins": "alice",
          "labels": {
            "*": "*"
          }
        },
        "app": {
          "labels": {
            "env": "staging"
          }
        }
      }
    }
```

The `jsonpath` function could then be used to interpolate these JSON values
in role templates, OIDC `claims_to_traits` mappings, and any other trait
mapping logic.

This approach was abandoned due in favor of a login-centric approach (`claims_to_roles` and `login_rule` mapping)
due to the issues below that were identified in the POC phase.

TLDR; login rules provide better administrative UX, avoids the negative side
effects of oversized user traits, and reduces the implementation complexity of
the feature.

##### 1. User traits are represented with a protobuf message that expects string values

```proto
// ### types.proto ###

// UserSpecV2 is a specification for V2 user
message UserSpecV2 {
  ...
  // Traits are key/value pairs received from an identity provider (through
  // OIDC claims or SAML assertions) or from a system administrator for local
  // accounts. Traits are used to populate role variables.
  wrappers.LabelValues Traits = 5 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "traits,omitempty",
    (gogoproto.customtype) = "github.com/gravitational/teleport/api/types/wrappers.Traits"
  ];
}

// ### wrappers.proto ###

// StringValues is a list of strings.
message StringValues {
  repeated string Values = 1;
}

// LabelValues is a list of key value pairs, where key is a string
// and value is a list of string values.
message LabelValues {
  // Values contains key value pairs.
  map<string, StringValues> Values = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "labels"
  ];
}
```

Storing JSON blobs as user trait values would require one of the following,
nontrivial changes:

* Change the protobuf map to one that maps strings to a `oneof` that allows
a string or bytes (JSON). This would likely require the addition of a new
`TraitsV2` field using the new `oneof` map, and all the backwards/forwards
compatibility concerns that comes with adding a new field to represent the same,
albeit altered, information. There is some prior art to this type of migration,
but it is undoubtedly an additional undertaking.
* Just store JSON blobs as strings in the traits values. Then, if `jsonpath` is
used on a trait with a JSON blob string, it can attempt to unmarshal the JSON
and interpolate. In order to make the traits readable with commands like
`tctl get user`, some custom marshalling logic will need to be added. While
simple, this approach is a bit hacky and will lead to tech debt and potential
inefficiencies.

##### 2. JSON blob traits will bloat user traits

Take the [distributed IDP example](#example-distributed-idp), where users could
have a very long list of traits made up from all the different providers.

Let's make the example a bit simpler, with different providers providing access
to the same resources with the shared env and login fields:

```json
{
  "idp_name": "distributed-idp",
  "aggregated_claims": {
    "okta": {
      "groups": ["teleport-access"],
      "logins": "alice",
      "env": ["staging", "dev"]
    },
    "auth0": {
      "groups": ["teleport-devops"],
      "logins": "devops",
      "env": ["prod"]
    },
  }
}
```

In order to minimize and simplify the resulting user traits, it would be much
better to use a login rule like this:

```yaml
kind: login_rule
version: v1
metadata:
  name: distributed-idp
spec:
  priority: 0
  traits_map:
    logins:
      - jsonpath("$.aggregated_claims.*.logins")
    env:
      - jsonpath("$.aggregated_claims.*.env")
```

The resulting traits will be much smaller with no redundancy compared to the
original OIDC claims.

```yaml
kind: user
metadata:
  name: alice
spec:
  ...
  traits:
    logins: ["alice", "devops"]
    env: ["staging", "dev", "prod"]
```

##### 3. JSON traits will be difficult to reason about

As a result, administrators may struggle to create valid `jsonpath`
queries in role templates and elsewhere. If the OIDC claims are ever changed
on the provider side, an admin will need to update every `jsonpath` query rather
than just the OIDC connector and associated login rule.

Therefore, the best UX for administrators is to setup a connector and login
rule to map claims to roles and traits using `jsonpath` once, rather than
worry about `jsonpath` interpolation in role templates and anywhere else.

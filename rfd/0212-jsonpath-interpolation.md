---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 212 - JSONPath Interpolation

## Required Approvers

* Engineering: @zmb3 || @rosstimothy
* Product: @klizhentas

## What

Add the ability to handle arbitrary JSON OIDC claims using a new interpolation
function, `jsonpath`. This function can be used to query JSON values according
to the [JSONPath query language](https://support.smartbear.com/alertsite/docs/monitors/api/endpoint/jsonpath.html).

The `jsonpath` function will be supported in `login_rule` trait maps/expressions,
as well as in `claims_to_roles` mapping in the OIDC connector with a new
`claim_expression` field.

## Why

Currently, Teleport assumes that OIDC claims are either a string or list of
strings. However, technically OIDC claims may be arbitrary JSON objects, and we
have run into some custom OIDC solutions that rely on that capability. This
feature is necessary for Teleport to integrate with these OIDC solutions.

## Details

### JSONPath

Before continuing to read this RFD, you should familiarize yourself with the
basics of [JSONPath syntax](https://support.smartbear.com/alertsite/docs/monitors/api/endpoint/jsonpath.html).

When going through the JSONPath examples, you may find it useful to run the
queries in a [sandbox](https://serdejsonpath.live/).

#### `jsonpath` Expression Function

The `jsonpath` function will be added as another standard trait expression
function. It can be used in `login_rule` trait maps/expressions and the new
sso connector `claim_expression` field to interpolate a string(s) from
arbitrary JSON claims.

Note: currently, login rules are applied after the claims to traits mapping.
Instead, we will apply the login rules to the OIDC claims (`map[string]any`)
before mapping them to traits (`map[string][]string`).

Note: the `jsonpath` function can *technically* be used for any "expression"
in teleport, such as the role field `node_labels_expression`. Since traits
can only be a string(s), and not arbitrary JSON like OIDC claims, `jsonpath`
could only be used for basic string/array expressions. In most of these cases a
predicate function would be more appropriate. For exapmle, to get "2" from the
list ["1","2","3"], you could do either `contains(list, "value1")` or
`jsonpath(list, "$[?(@ == "value1")]")`, but clearly the former is simpler.
Still, it's worth noting as some of the more advanced `jsonpath` features like
dynamic filtering and numerical comparison could be useful to extend the
expression functionality currently available.

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

* Determining what Teleport roles to give the user, using the `claims_to_roles` mapping.
* Setting claims as Teleport user traits.
  * Login rules are applied to these traits before they are set for the user.

Both of these steps will have support for the `jsonpath` function in order to
handle JSON OIDC claims.

#### Claims to Roles - `claim_expression`

To map a JSON claim to a role, admins can set the new `claim_expression`
field in the OIDC `claims_to_roles` mapping, causing the mapping to match
on the evaluated expression value.

In the following example, these OIDC claims would map to the `viewer` role.

```json
{
  "groups": {
    "roles": ["viewer"]
  }
}
```

```yaml
kind: oidc 
...
spec:
  ...
  claims_to_roles:
      # evaluates to ["viewer"]
    - claim_expression: 'jsonpath(external.groups, "$.roles")'
      value: "viewer"
      roles: "viewer"
```

Note: Either `claim_expression` or `claim` can be set, not both.

#### Login Rules - Claims to Traits

You can use a login rule to map JSON claims to users traits.

In the example below, `$.logins` and `$.env` claims are mapped to user traits.

```json
{
  "groups": {
    "roles": ["viewer"],
    "logins": ["alice"],
    "env": ["staging", "dev"]
  }
}
```

```yaml
kind: login_rule
version: v1
metadata:
  name: distributed-idp
spec:
  priority: 0
  traits_map:
    logins:
      # evaluates to ["alice"]
      - jsonpath(external.groups, "$.logins")
    env:
      # evaluates to ["staging", "dev"]
      - jsonpath(external.groups, "$.env")
```

These traits can then be used in role templates, label expressions, etc.

```yaml
kind: role
version: v7
metadata:
  name: template
spec:
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
}
```

The administrator wants to take these claims and map them directly to role
conditions using [role templating](https://goteleport.com/docs/admin-guides/access-controls/guides/role-templates/).

First, the admin needs to create a `login_rule` to map this arbitrary JSON
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
      - jsonpath(external.groups, "$.teleport.roles")
    logins:
      # evaluates to ["alice"]
      - jsonpath(external.groups, "$.teleport.node.logins")
    node_labels_*:
      # evaluates to "*"
      - jsonpath(external.groups, "$.teleport.node.labels[?(@ == '*')]")
    node_labels_env:
      # evaluates to []
      - jsonpath(external.groups, "$.teleport.node.labels.env")
    app_labels_*:
      # evaluates to []
      - jsonpath(external.groups, "$.teleport.app.labels[?(@ == '*')]")
    app_labels_env:
      # evaluates to "staging"
      - jsonpath(external.groups, "$.teleport.app.labels.env")
```

Note: without [JSONPath-Plus syntax](#jsonpath-plus), it's not possible to grab
the property name value, so we can only map labels that we are aware of. In this
example, we are only looking for the `*` and `env` labels, so if the provider
added a claim like `"team": "devops"`, it would not be mapped without an
additional `traits_map` rule.

The mapped traits can now be used as if they are standard OIDC claims in the
OIDC connector's `claims_to_roles` spec to assign the template role to the user.

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

Lastly, the admin can create the template role and utilize the mapped traits.

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
      "groups": ["teleport-access", "github-admin"],
      "logins": "alice",
      "env": ["staging", "dev"]
    },
    "auth0": {
      "groups": "teleport-devops",
      "logins": "devops",
      "env": ["prod"]
    },
    "github": {
      // no claims from github for this user.
    }
  }
}
```

For this example, we will start with the OIDC connector `claims_to_roles` spec,
which also supports the `jsonpath` function.

```yaml
kind: oidc 
version: v2
metadata:
  name: distributed-idp
spec:
  ...
  claims_to_roles:
    - claim_expression: "jsonpath(external.aggregated_claims, "$.okta.groups")"
      value: "teleport-access"
      roles: ["okta-access"]
      # evaluates to "teleport-devops", match.
    - claim_expression: "jsonpath(external.aggregated_claims, "$.auth0.groups")"
      value: "teleport-devops"
      roles: ["auth0-devops"]
      # evaluates to [], no match.
    - claim_expression: "jsonpath(external.aggregated_claims, "$.github.groups")"
      value: "*-admin"
      roles: ["github-admin"]
```

In order to save the other claims as user traits, the admin can create a
`login_rule`.

The admin can also set other custom traits based on the structure of the claims.
For example, a `teams` trait that aggregates the root property names of the
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
      # evaluates to "alice"
      - jsonpath(external.aggregated_claims, "$.okta.logins")
    okta_env:
      # evaluates to ["staging", "dev"]
      - jsonpath(external.aggregated_claims, "$.okta.env")
    auth0_logins:
      # evaluates to "devops"
      - jsonpath(external.aggregated_claims, "$.auth0.logins")
    auth0_env:
      # evaluates to ["prod"]
      - jsonpath(external.aggregated_claims, "$.auth0.env")
    teams:
      # evaluates to ["okta", "auth0"]
      - 'ifelse( !isempty( jsonpath(external.aggregated_claims, "$.okta") ), set("okta"), set())'
      - 'ifelse( !isempty( jsonpath(external.aggregated_claims, "$.auth0") ), set("auth0"), set())'
      - 'ifelse( !isempty( jsonpath(external.aggregated_claims, "$.github") ), set("github"), set())'
```

Note that we exclude `groups` from the `traits_map` as we have already used
the groups to assign the user's roles. Since user traits are included in signed
SSH certificates and JWTs and oversized certs/JWTs can cause issues with some
third-party applications, it can be important to use login rules to trim
unnecessary or redundant traits coming from OIDC claims.

The admin can now create okta and auth0 role templates which rely only on
claims for the corresponding provider:

```yaml
kind: role
version: v7
metadata:
  name: okta-access
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
  name: auth0-devops
spec:
  allow:
    logins: '{{external.auth0_logins}}'
    node_labels:
      'env': '{{external.auth0_env}}'
      'team': "auth0"
```

### Proto

```diff
message ClaimMapping {
  ...
+ // ClaimExpression is an interpolation expression that retrieves a value for claim matching.
+ string ClaimExpression = 4 [(gogoproto.jsontag) = "claim_expression"];
}

message TraitMapping {
  ...
+ // TraitExpression is an interpolation expression that transforms the trait value(s).
+ string TraitExpression = 4 [(gogoproto.jsontag) = "trait_expression"];
}
```

### Security

This RFD should not raise any security concerns outside of those already covered
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
    external.put_many(jsonpath(external.groups, "$.teleport.node.labels~"), jsonpath(external.groups, "$.teleport.node.labels"))
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
    external.put_many(jsonpathprop(external.groups, "$.teleport.node.labels"), jsonpath(external.groups, "$.teleport.node.labels"))
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
      - jsonpath(external.aggregated_claims, "$.*.logins")
    env:
      - jsonpath(external.aggregated_claims, "$.*.env")
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

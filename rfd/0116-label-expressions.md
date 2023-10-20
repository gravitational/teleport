---
authors: Nic Klaassen (nic@goteleport.com)
state: implemented (v13.2.0)
---

# RFD 116 - RBAC Label Expressions

## Required Approvers

* Engineering: @rosstimothy || @codingllama || @fspmarshall
* Security: @reed || @jentfoo
* Product: @xinding33 || @klizhentas

## What

This RFD proposes an addition to the Teleport Role spec to enable granular
matching on resource labels.

Roles currently support the following "label matchers" in the `allow` and `deny`
sections of the spec:

- `node_labels`
- `app_labels`
- `cluster_labels`
- `kubernetes_labels`
- `db_labels`
- `db_service_labels`
- `windows_desktop_labels`

These would be supplemented by new fields supporting predicate expressions that
can match resource labels:

- `node_labels_expression`
- `app_labels_expression`
- `cluster_labels_expression`
- `kubernetes_labels_expression`
- `db_labels_expression`
- `db_service_labels_expression`
- `windows_desktop_labels_expression`

An example role spec allowing access to all nodes *except* those in the
`production` environment would look like:

```yaml
kind: role
version: v6
metadata:
  name: "all_except_prod"
spec:
  allow:
    node_labels_expression: 'labels["env"] != "production"'
    logins: ["root"]
```

## Why

This feature will allow Teleport admins to write fewer roles than what is
necessary today, and unlocks new usecases that are difficult or impossible to
support with the current implementation.

For example, the above `all_except_prod` role is impossible to write with the current role
spec. The closest option would look like:

```yaml
kind: role
version: v6
metadata:
  name: "all_except_prod_legacy"
spec:
  allow:
    node_labels:
      '*': '*'
    logins: ["root"]
  deny:
    node_labels:
      'env': 'production'
```

But this has one major drawback: the `deny` rule prevents all users with this
role from ever accessing `production` nodes via access request or even another
role granting limited access.

Take the following `auditor` role for example:

```yaml
kind: role
version: v6
metadata:
  name: "auditor"
spec:
  allow:
    node_labels:
      '*': '*'
    logins: ["auditor"]
```

If a user `alice` had both roles `[all_except_prod, auditor]` then she would be
able to access `production` nodes with the `auditor` login and all other nodes
with either the `root` or `auditor` login.
But if another user `bob` had both roles `[all_except_prod_legacy, auditor]`
then he would *not* be able to access any `production` nodes due to the matched
`deny` rule.
This is one of the most basic examples, demonstrating the value of a simple
negative match ([feature request](https://github.com/gravitational/teleport/issues/20145)).

Another [popular ask](https://github.com/gravitational/teleport/issues/10204) is
for label matchers to support logical "OR" matches, where a node can match any
one of a set of possibly allowed labels.
Currently this is not possible without creating multiple roles.
Label expressions will be able to support this:

```yaml
kind: role
version: v6
metadata:
  name: example
spec:
  allow:
    logins: ["example"]

    # This label expression would grant access to nodes in one of many allowed
    # environments
    node_labels_expression: |
      labels["env"] == "dev" ||
      labels["env"] == "qa" ||
      labels["env"] == "staging"
```

## Details

### Expression Syntax

Every label expression has access to the "evaluation context" as its input and
must evaluate to a Boolean true/false to indicate a match of the given resource.
Label expressions can appear in `allow` or `deny` role conditions.

#### Evaluation Context

Label expressions will have access to the following context:

| Syntax             | Type                    | Description |
|--------------------|-------------------------|-------------|
| `user.spec.traits` | `map[string][]string`   | `external` or `internal` traits of the user accessing the resource |
| `labels`           | `map[string]string`     | Combined static and dynamic labels of the resource (node, app, db, etc.) being accessed |

#### Helper functions

Supported helper functions are a subset of those already supported in role
templates, `where` expressions, and login rules.

Below, any variable named `list` can contain a list of items (like the list of
values for a specific user trait) or a single value (like the value of a
resource label or a string literal).

| Syntax | Return type | Description | Example |
|--------|-------------|-------------|---------|
| `contains(list, item)` | Boolean | Returns true if `list` contains an exact match for `item` | `contains(user.spec.traits[teams], labels["team"])` |
| `contains_any(list, items)` | Boolean | Returns true if `list` contains an exact match for any element of `items` | `contains_any(user.spec.traits["projects"], labels_matching("project-*"))` |
| `contains_all(list, items)` | Boolean | Returns true if `list` contains an exact match for all elements of `items` | `contains_all(user.spec.traits["projects"], labels_matching("project-*"))` |
| `regexp.match(list, re)` | Boolean | Returns true if `list` contains a match for `re` | `regexp.match(labels["team"], "dev-team-\d+$")` |
| `regexp.replace(list, re, replacement)` | `[]string` | Replaces all matches of `re` with replacement for all items in `list` | `contains(regexp.replace(user.spec.traits["allowed-env"], "^env-(.*)$", "$1"), labels["env"])`
| `email.local(list)` | `[]string` | Returns the local part of each email in `list`, or an error if any email fails to parse | `contains(email.local(user.spec.traits["email"]), labels["owner"])`
| `strings.upper(list)` | `[]string` | Converts all items of the list to uppercase | `contains(strings.upper(user.spec.traits["username"]), labels["owner"])`
| `strings.lower(list)` | `[]string` | Converts all items of the list to lowercase | `contains(strings.lower(user.spec.traits["username"]), labels["owner"])`
| `labels_matching(re)` | `[]string` | Returns the aggregate of all label values with keys matching `re`, which can be a glob or a regular expression. | `contains(labels_matching("^project-(team|label)$"), "skunkworks")`

#### Operators

| Syntax | Description | Example |
|--------|-------------|---------|
| `==` | Equals | `labels["env"] == "staging"` |
| `!=` | Not equals | `labels["env"] != "production"` |
| `\|\|` | Logical "OR" | `labels["env"] == "staging" \|\| labels["env"] == "test"` |
| `&&` | Logical "AND" | `labels["env"] == "staging" && labels["team"] == "dev"` |
| `!` | Not | `!regexp.match(user.spec.traits["teams"], "contractor")` |

### Templates vs Expressions

The existing label matchers support role "templating": user traits are
substituted into the role spec with a special `{{}}` syntax, e.g.:

```yaml
node_labels:
  env: `{{external.access-env}}`
```

These templates are "rendered" for the user with their current traits exactly
once for each RPC.
The rendered template is then held in the request context and can be used for
multiple access checks.
For example, when calling the `ListResources` RPC, all the user's role templates
are rendered with their current traits once, and then used to check access to
each requested resource in the cluster.

The new expression matchers will not support templating.
Instead, user traits can be referenced directly in the expression without any
special template syntax, e.g.:

```yaml
node_labels_expression: 'contains(user.spec.traits["access-env"], labels["env"])'
```

Pros:

- No confusing mix of expression/template syntax.
- Parsed expressions can be cached across all users and roles, keyed by the
  expression string only. If individual user traits were templated into every
  expression, cache size would explode.
- Disallowing templates within expressions prevents any possibility of
  injection attacks.

Cons:

- User traits are not rendered once per RPC, but can be referenced in any
  expression in each access check, which may hurt evaluation performance.

### Performance

Since predicate expressions can require more complex parsing than role
templates, it's reasonable to be concerned that this could negatively impact the
performance of the Teleport cluster.
However, I believe it is feasible for this feature to offer performance no worse
than our existing implementation.

Three reasons to believe that label expressions will not lead to significantly
worse performance:
1. Label expressions make it possible to configure the same RBAC rules with
   fewer roles. This means fewer roles to read, cache, render, and evaluate.
2. Expressions can be parsed once and cached for the lifetime of the instance.
   This is explained in the following subsection on caching.
3. Role templates already use (limited) predicate expressions in label matchers.
   These are not cached, meaning the expression is parsed on each RPC.

Ultimately, performance will be benchmarked for multiple scenarios with the goal
of staying within 10% of the performance of the existing implementation.
Benchmarks will be written comparing similar RBAC constraints written with both
the existing label matchers and the new label expressions.
Benchmarks will run `ListResources` with 50k unique (simulated) nodes and 32
unique roles.

Benchmark results:

```
$ go test ./lib/auth -bench=. -run=^$ -v -benchtime 1x
goos: darwin
goarch: amd64
pkg: github.com/gravitational/teleport/lib/auth
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
BenchmarkListNodes
BenchmarkListNodes/simple_labels
BenchmarkListNodes/simple_labels-16                    1        1079886286 ns/op        525128104 B/op   8831939 allocs/op
BenchmarkListNodes/simple_expression
BenchmarkListNodes/simple_expression-16                1         770118479 ns/op        432667432 B/op   6514790 allocs/op
BenchmarkListNodes/labels
BenchmarkListNodes/labels-16                           1        1931843502 ns/op        741444360 B/op  15159333 allocs/op
BenchmarkListNodes/expression
BenchmarkListNodes/expression-16                       1        1040855282 ns/op        509643128 B/op   8120970 allocs/op
BenchmarkListNodes/complex_labels
BenchmarkListNodes/complex_labels-16                   1        2274376396 ns/op        792948904 B/op  17084107 allocs/op
BenchmarkListNodes/complex_expression
BenchmarkListNodes/complex_expression-16               1        1518800599 ns/op        738532920 B/op  12483748 allocs/op
PASS
ok      github.com/gravitational/teleport/lib/auth      11.679s
```

#### Caching

Predicate expressions are parsed and evaluated in two distinct stages.

The first stage takes the raw string expression as input, parses the syntax, and
returns a closure which is essentially a plain Go function.

The second stage takes closure from the first stage and invokes it with the
evaluation context (resource labels, user traits) as input to produce a result.
For label expressions, the result will be a Boolean true/false to indicate
whether the expression matched the resource.

The first (parse) stage can be easily cached.
It takes as input the literal string expression which is written in the role,
and returns a closure that can be invoked repeatedly for each access check with
the specific inputs for the current user and resource.
The important part is that is always produces the exact same output for each
input, and the total number of inputs is relatively small.

The size of the cache is bounded linearly by the number of unique expressions in
all roles in the cluster.
These expressions will almost always be hand-written, and I estimate there will
usually be tens to (possibly) hundreds of unique expressions in any given
cluster.
In the rare case that someone writes or generates thousands of label expressions
that are all actually used, performance can degrade gracefully by using an LRU
cache.

The cache will be held in-memory of each Teleport process which does access
checks.
It will not be shared across nodes, it should be sufficient for each node to
have to parse each expression once and all future uses can be cached.
Cache entries will be populated on-demand the first time the are required.
`github.com/hashicorp/golang-lru/v2` implements a fixed-size thread safe LRU
cache and is already used for similar purposes (regex caching) within teleport,
so it will also be used here.
The default maximum cache size will be 1000, and this can be overridden by an
environment variable in case it causes problems.

Cache entries never need to be invalidated, updated, or expired, the value for
each input is valid for the entire lifetime of the process.
The only thing that could change the parse output is a new version of Teleport
with a different parsing algorithm, conveniently this will always run in a new
process with a fresh/empty cache.

Note: an App service will never need to parse or evaluate a
`db_labels_expression`, it will only cache `app_labels_expression`s, and
similarly each service will only need to cache the expressions actually relevant
to that service.
The only service that will need to parse/cache all expressions is Auth.

### Security

Label expressions, as part of the Role specification, will be editable by any
users with write/update permissions for Role resources, and readable by any user
with read permissions for Roles.

This design avoids the possibility of any expression injection attacks.
No untrusted input will be parsed as an expression, user traits and resource
labels are only available during evaluation of the already-parsed expression.
Regular expressions can only be built from static strings configured as part of
the expression by the admin, labels and traits will not be compiled into regular
expressions.

It's possible that a diabolical expression could cause terrible enough
performance to threaten a DOS, but expressions can only be written by Teleport
admins who already have permission to edit roles.

### UX

Label expressions are a regular string field within the Teleport role
specification.
Users regularly interact with Teleport Roles via YAML files that can be edited
with `tctl` or within the Web UI.
Roles can also be edited with IaC workflows based on the Teleport Terraform
provider or Kubernetes operator.

Verbose logging will be printed at the `TRACE` level for all access decisions
involving label expressions, to aid users in debugging any issues.

### Proto Specification

The following new fields will be added to the `RoleConditions` proto message:

```
  // NodeLabelsExpression is a predicate expression used to allow/deny access to
  // SSH nodes.
  string node_labels_expression = 27;
  // AppLabelsExpression is a predicate expression used to allow/deny access to
  // Apps.
  string app_labels_expression = 28;
  // ClusterLabelsExpression is a predicate expression used to allow/deny access to
  // remote Teleport clusters.
  string cluster_labels_expression = 29;
  // KubernetesLabelsExpression is a predicate expression used to allow/deny access to
  // kubernetes clusters.
  string kubernetes_labels_expression = 30;
  // DatabaseLabelsExpression is a predicate expression used to allow/deny access to
  // Databases.
  string db_labels_expression = 31;
  // DatabaseServiceLabelsExpression is a predicate expression used to allow/deny access to
  // Database Services.
  string db_service_labels_expression = 32;
  // WindowsDesktopLabelsExpression is a predicate expression used to allow/deny access to
  // Database Services.
  string windows_desktop_labels_expression = 33;
```

### Backward Compatibility

Label expressions will be a set of brand new fields within the Role spec.
They do not replace or supercede the existing label matchers.
A single role may contain both `node_labels` and `node_labels_expression`, both
will be considered independently.

Teleport instances running older versions of Teleport will not "see" or be aware
of any label expressions.
Before introducing label expressions to your cluster, you will be expected to
upgrade relevant teleport instances to a version which supports label expressions, or
else they will not be considered during access decisions.

If a Teleport downgrade is necessary, and no label expressions are currently
used in any roles, there a no consequences.
If label expressions are already being used and Teleport is downgraded to a
version which does not support them, access decisions will not consider the
label expressions.

### Audit Events

No new audit events will be created, nor will any by changed.

### Test Plan

The implementation of this feature will include automated unit and integration
tests, as well as benchmarks.
Running the benchmarks and comparing with past results will be added to the test
plan to make sure performance does not regress too far.

## Extra Examples

Conditional logic with user traits:

```yaml
kind: role
version: v6
metadata:
  name: example
spec:
  allow:
    logins: [example]
    # This label expression would grant access to all non-production nodes
    # owned by one of the user's teams or the qa team.
    node_labels_expression: |
      labels["env"] != "production" &&
        (contains(user.spec.traits["teams"], labels["team"]) || labels["team"] == "qa")
```

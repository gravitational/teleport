---
authors: Hugo Hervieux (hugo.hervieux@goteleport.com)
state: draft
---
# RFD 87 - Access request notification routing

## Required Approvers

* Engineering r0mant && ???
* Security @reedloden
* Product: (@xinding33 || @klizhentas)

## What

Provide a granular way for administrators to configure which access-requests notifications should be delivered to whom
over which medium.

## Why

- Teleport access request routing is not configured the same way depending on the plugin: Pagerduty uses annotations
  while most plugins use a role_to_recipient map in the plugin config. Users leveraging both PagerDuty and Slack plugins 
  have to deal with two configuration languages;
- Current Pagerduty request routing applies the same rule to all requests from the same role, regardless of the
  requested role. All access requests don't have the same severity, users wanting to route access requests differently
  have to create multiple roles. Depending on the company size and structure cardinality can become a problem.
  See https://github.com/gravitational/teleport-plugins/issues/597;
- Access request routing map baked in the plugin deployment:
  - Will become huge for big orgs.
  - Requires redeploying each time a change is done.
  - Does not allow to route requests regarding of the requestor.

## Details

### User story

Bob is a developer. He deploys code using CI/CD pipelines. He can request access to prod in read-only mode and dev in read-write mode for debugging. In case of incident he can request prod read-write access.
Alice is the lead-dev, she grants regular access requests through MsTeams during open hours and can also approve urgent read-write requests.
Alice should not be paged each time Bob needs to debug something during open hours, but in case of incident Bob needs immediate access and Alice should be paged.

In Teleport terms:

- the role `developer` can request roles `dev-rw`, `prod-ro` and `prod-rw`;
- the role `lead-developer` can approve requests;
- only `prod-rw` access requests should trigger a PagerDuty incident;
- `dev-rw` and `prod-ro` access requests should trigger a MsTeams message.

### Terminology

Definitions:
- A target is the pair: (`plugin`, `recipients`);
- `plugin` is a string describing which plugin should pick up this target;
  By default each plugin are listening for their own name (`slack` for the slack plugin,
  `msteams` for the Microsoft Teams one, ...). But the plugin name can be configurable
  in the plugin configuration. This allows support for multiple instances of the same
  plugin (e.g. many orgs have multiple slack workspaces, they can run 1 plugin per workspace);
- `recipients` is a list of strings describing the access request recipients.
  Different kinds of recipients are supported depending on the plugin (user
  name, channel name, service name, user id, channel id, URL, ...).

### New Resource: `access_request_routing_rule`

This proposal introduces a new resource to add routing annotations when an access request is emitted.

The motivation behind adding a new resource, opposed to add new fields in `role` is to
limit the `role` spec inflation and allow more flexibility to create role-agnostic routing rules.

Routing rules are evaluated on access-request creation and will add elements to a new field in
access-requests: `targets`.

An example access_request_routing_rule would look like:

```yaml
kind: access_request_routing_rule
version: v1
metadata:
  name: example
spec:
  targets:
  # simplified syntax
  - condition: 'resource.spec.roles.contains("prod-rw")'
    recipients: ["Teleport Alice"]
    plugin: "pagerduty"
  # raw expression
  - expression: >
      ifelse(
        resource.spec.roles.contains("prod-rw"),
        pair(),
        pair("msteams", set("alice@example.com"))
      )
```

Invoked with the role `"prod-rw"`, this would produce the following `access-request`

```yaml
kind: access_request
spec:
  user: bob
  roles: ["prod-rw"]
  state: 1
  # [some fields were omitted for clarity]
  request_reason: "Prod is down and rollback failed"
  targets:
    - plugin: pagerduty
      recipients: ["Teleport Alice"]
```

From the plugin point of view, the recipients are additive:
- they can come from the `role_to_recipient` map in the plugin configuration;
- they can come from the `targets` on the access-request;
- they can come from the requests additional recipients;
- they can come from annotations (backward compatibility for pagerduty plugin).

### Performance

All `access_request_routing_rules` have to be evaluated against each access request. Even if this only applies
on access request creation, this can be an intensive operation, especially against the database.
The auth server should maintain a cache of all target rules to avoid having to request them for each request.

### Security

In case of conflicts (teamA writes a routing rule matching access-requests from teamB), both targets
will be added to the access request. A misconfigured rule will result in a team being wrongly notified.
The main issue will be spam/noise but this should not lock out the rightful team to get notified by
the access request.

Misconfigured rules should not represent security issues unless the plugin does some kind of automatic approval.
This is the case for the pagerduty plugin which auto-approves if the requestor is on-call for the notified service.
In such situation, `access_request_routing_rules` should be considered as critical as `roles` themselves as they can lead
to automatic access escalation. Such risk is already present in the current implementation.

Such risk can be mitigated by not granting the access-request edition rights to the plugin (at the price of a feature loss).

If the two targets are identical it is the plugin's responsibility to deduplicate before sending notifications
(to avoid sending duplicates and to ensure `PluginData` unity).

### User Experience

#### Complexity/Time to value

In most cases, a simple role_name check is performed and the users wants to set the plugin and the recipients.
This is provided by the simplified syntax:

```yaml
targets:
  - condition: 'resource.spec.roles.contains("prod-rw")'
    recipients: ["Teleport Alice"]
    plugin: "pagerduty"
  - condition: '!resource.spec.roles.contains("prod-rw")'
    recipients: ["alice@example.com"]
    plugin: "msteams"
```

This syntax is equivalent to the pure-predicate one:

```yaml
targets:
  - expression: >
      ifelse(
        resource.spec.roles.contains("prod-rw"),
        pair("pagerduty", set("Teleport Alice")),
        pair()
      )
  - expression: >
      ifelse(
        !resource.spec.roles.contains("prod-rw"),
        pair("msteams", set("alice@example.com")),
        pair()
      )
```

Note: with the pure-predicate syntax could be simplified in this case:

```yaml
targets:
  - expression: >
      ifelse(
        resource.spec.roles.contains("prod-rw"),
        pair("pagerduty", set("Teleport Alice")),
        pair("msteams", set("alice@example.com"))
      )
```

If users want to do complex operations and leverage all the predicate language power they can use the predicate code
in the `expression` field. The predicate evaluation is expected to return a pair containing the plugin name and the recipients.
If one or both fields are empty, the target should not be set.

If `expression` is set, `condition`, `recipients` and `plugin` should not be used. Teleport must validate
this on resource creation and fail immediately instead of silently ignoring fields.

Predicate from `condition` and `expression` is evaluated against the `access_request` resource.

#### Combining with suggested_reviewers

Another UX consideration is how this new feature will mix with the existing `suggested_reviewers`.
The safest way would be to not interact with it and let the plugin deal with it.

Using `suggested_reviewer` implies the user knows which plugin will be used to send the access request.
The string has to make sense and be allowed by the plugin. Today most plugins support email addresses
and reject other recipients, but some are just ignoring the field (pagerduty). We might want to clarify
what can be entered in this field and how different plugins are reacting to it so user has insights on
what to type and what to expect.

Using `suggested_reviewer` with two plugins supporting it (e.g. Slack and MsTeams, or two Slack plugins on different workspaces)
can lead to the suggested reviewer receiving the notification multiple times. One way to mitigate this would be to
offer a plugin configuration flag to ignore suggested reviewers (so for example, suggested reviewers are only
pinged on Slack).

#### JIT resource access request

While this approach should work with both role access request and resource access request, using it with
resource access request can cause some friction for admin writing the rules:

At the time of writing this proposal, only resource IDs are available through access requests: writing a rule to filter
based on requested resources requires knowing resources IDs. Rules with lists of IDs can be hard to read. For resource
access requests routing, admins are likely to want to route the requests based on resources labels/tags.
Roles are also available, but the requested role does not necessarily follow the principle of the least privilege.

When resource labels will be available, users will be able to match access requests routing rules against
the resource labels. The main challenge will be to find a way to express what the users wants in a usable way.
A list of each resource ID and their labels is not usable as-is, the list will need to be flattened in some way to
represent the access-requests labels. Two approaches are available:
- returning the union of all labels: This approach allows users to request several non-related resources and have the
  access-request still reach all relevant teams. However this will cause a risk of privilege escalation through
  automatic approval, like the Pagerduty plugin. If this approach is chosen, we should ensure no automatic approval
  can happen on such requests.
- returning the intersection of all labels: This approach is the sanest from a security point of view but can degrade
  the user experience as any resource with no label will cause the access request to have no match.

To solve this issue, mixed approaches can be used, like exposing both label union and intersection to the users.
For example, two resources with the following labels:
```yaml
resourceID1:
    label1: "value1"
    label2: "value2"
resourceID2:
    label3: "value3"
    label2: "value4"
    label1: "value1"
```
would produce the following `access_request`:
```yaml
spec:
    labelsIntersection:
        label1: "value1"
    labelsUnion:
        label1: ["value1"]
        label2: ["value2", "value4"]
        label3: ["value3"]
```

A user would then be able to route its access requests using the following rule:

```yaml
spec:
  targets:
  # page over teams as soon as one of the resource has label2=value2.
  - condition: 'resource.spec.labelsUnion.label2.contains("value2")'
    recipients: ["Teleport Alice"]
    plugin: "msteams"
  # page over pagerduty only if all resources have label1=value1
  - condition: equals(resource.spec.labelsIntersection.label1 , "value1")
    recipients: ["Teleport Alice"]
    plugin: "pagerduty"
```

#### Pagerduty example scenario

This proposal would allow to solve the [initial feature request]() in multiple
ways.

The naive way with a big `access_request_routing_rule` containing the list of all allowed roles:

```yaml
version: v1
metadata:
  name: notify-allowed-roles-on-pagerduty
targets:
  - expression: >
      ifelse(
        resource.spec.roles.contains("allowedRoleA") or
        resource.spec.roles.contains("allowedRoleB") or
        resource.spec.roles.contains("allowedRoleC") or
        resource.spec.roles.contains("allowedRoleD") or
        resource.spec.roles.contains("allowedRoleE"),
        pair("pagerduty", resource.spec.system_annotations.get("pagerduty_destination")),
        pair()
      )
```

This solution is equivalent to the [PR #596](https://github.com/gravitational/teleport-plugins/pull/596).

Another more versatile solution would be to use a single
`access_request_routing_rule` to implement a simple allowlist logic
based on role request annotations.

```yaml
version: v1
metadata:
  name: pagerduty-notifications
targets:
  - expression: >
      ifelse(
        resource.spec.system_annotations.get("pagerduty_allow_roles").intersects(resource.spec.roles).len() > 0,
        pair("pagerduty", resource.spec.system_annotations.get("pagerduty_destination")),
        pair()
      )
```

With the following role, Bob will trigger a Pagerduty escalation by requesting
the role `"prod-rw"` but not the role `"prod-ro"`.

```
kind: role
metadata:
 name: developer
spec:
 allow:
  request:
   roles: ["prod-ro","prod-rw"]
   annotations:
    pagerduty_destinations: ["Teleport Alice"]
    pagerduty_allow_roles: "prod-rw"
```

Note: relying on the role request annotations allows to keep the number of
`access_request_routing_rules` low, which mitigates possible performance
impacts. We might want to guide users towards model in the documentation.

### Plugin considerations

This change must be backward compatible:
- old plugins should ignore `targets` and get the recipients from the `role_to_recipient` map (or annotation for pagerduty);
- each plugin should expose an `honor_suggested_reviewers` configuration, true by default;
- each plugin should have a default name and listen to it by default in the `targets`, this name should be the same used to store plugin data;
- admins can configure the plugin name through its configuration, names should always be prefixed by the plugin type (e.g. `slack-teamA`,
  `msteams-teamB`) to limit conflicts between two plugins with the same names;
- in INFO mode, each plugin should log if it is ignoring a target (because plugin name does not match for example) to ease troubleshooting;
- each plugin should log its name on startup.

### Predicate Helper Methods

Implementing the following predicate methods would allow users to create more
complex rules like allowlists and blocklists based on role request annotations.

#### `set.intersection(other_set)`

`set.intersection()` takes a set and returns a set containing elements contained in the two sets.
If the intersection is empty, it returns an empty set.
`set("a", "b", "c").intersection(set("a", "c", "d"))` returns the set `("a", "c")`.

#### `set.len()`

`set.len()` returns the number of elements in the set.
`set("a","b","c").len()` returns `3`.

#### dict.get(key)

`dict.get(key)` takes a string key and returns the set stored in `dict[key]`.
If `dict[key]` is empty or it does not exist, it will return an empty set.
```
dict(
  pair("fruits", set("apple", "banana")),
  pair("vegetables", set("asparagus", "broccoli")),
).get("fruits")
```
returns the set `("apple","bananas")`.

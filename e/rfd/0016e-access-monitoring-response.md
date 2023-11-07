---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 0016e - Access Monitoring & Response

## Required approvers

Engineering: @smallinsky
Product: @klizhentas && @xinding33
Security: @reedloden || @jentfoo

## What

Proposes expansion to Access Monitoring feature to include additional security
reports focused on finding unused permissions, and configurable response-type
actions for individual reports.

## Why

Identity Threat Detection & Response (ITDR) is a relatively new term introduced
by Gartner in 2022 to describe collection of tools and best practices to defend
identity systems, in response to the rising threat of identity related attacks
and emergence of solutions targeted at protecting against such attacks and
enabling quick detection and efficient remediation.

This direction is a natural evolution of Teleport Access Monitoring product
which in its current state provides information into access anti-patterns (such
as connections with weak security or as high-privileged accounts) but lacks
insights into other parts of cluster operations that pose potential risks (such
as assigned privileges that aren't being utilized) as well as ability for
administrators to setup automated actions in response to identified risks.

## Overview

Teleport will integrate automated "response" capabilities into various parts of
Access Monitoring (such as security reports) as a set of predefined actions that
can triggered from specific sources based on some criteria.

In addition, this RFD considers Access Request notifications to be a part of the
Access Monitoring response. As such, it supersedes parts of (not-yet-implemented)
[RFD 87: Access Request Notification Routing](https://github.com/gravitational/teleport/blob/master/rfd/0087-access-request-notification-routing.md)
to consolidate how access request notifications are configured via a Teleport
resource.

## Access monitoring rules

Access monitoring rules operate on cluster resources and in effect describe the
invariants which the response system constantly tries to reconcile the system
to. The response engine is periodically evaluating resources and brings them to
the desired state based on some matching conditions.

Access monitoring rules are represented as a new resource of the following
format:

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: unused-roles-are-removed
spec:
  subjects: ['role']
  states: ['removed', 'locked', ...]
  condition: '<predicate-expression>'
  notification:
    # optional notification plugin configurations
```

Each rule has the following main components:

* `subjects`: Subjects the rule operates on, can be a resource kind or a
  particular resource property. Initially the following subjects will be
  supported:
  - `user`
  - `user.spec.roles`
  - `role`
  - `role.spec.allow.logins` (and equivalents like `db_users`)
  - `static_token`
  - `access_request`

* `states`: Desired state which the monitoring rule is attempting to bring the
  subjects matching the condition to. Supported states are:
  - `removed`: The subject is removed from the system.
  - `locked`, `unlocked`: The subject (e.g. user) is locked or unlocked.

* `condition`: Predicate expression that operates on the specified subject
  resources and determines whether the subject will be moved into desired
  state.

* `notification`: Plugin configuration for notifications when the rule is
  triggered.

Supported resources are instrumented with the new `status` field where various
signal sources are recording metadata that is useful for monitoring rules. For
example:

```yaml
kind: role
version: v7
metadata:
  name: example
spec:
  allow:
    logins: [root, ubuntu]
status:
  inactive_since: 20231019T10:45PM
  logins_last_active:
    ubuntu: 20231019T10:45PM
    root: never
```

```yaml
kind: user_state # have to use "user_state" as "user" may expire (e.g. SSO)
version: v3
metadata:
  name: alice
spec:
  ...
status:
  last_active: 20231019T10:45PM
  weak_security: true
  roles_last_active:
    admin: 20231019T10:45PM
  locked_by:
    kind: access_monitoring_rule
    name: insecure-user-is-locked
```

The metadata will be coming from different signal sources: for example, as a
result of a security report or an audit query run, during login hooks or set by
various periodic reconcilers run within the cluster, and so on.

A useful side-effect of recording such metadata is it can be easily integrated
in existing web UI or CLI elements, for example highlighting unused roles in
the roles table.

At a high-level the access monitoring response system can be described as:

|------------------|
| Security reports |------|
|------------------|      |
                          |                                                      |-----------|
|---------------|         |                                                      ▼           |
| Audit queries |---------|                                      |-------------------|   |-------------------|
|---------------|         | updates  |-------------------|  uses | Access monitoring |   | Evaluates         |
                          |--------->| Resources' status |<------| reconciler        |   | access monitoring |
|----------------------|  |          |-------------------|       | loop              |   | rules             |
| Periodic reconcilers |--|                                      |-------------------|   |-------------------|
|----------------------|  |                                                      |           ▲
                          |                                                      |-----------|
|-------------|           |
| Login hooks |-----------|
|-------------|

## "Unused Permissions" report

The new "Unused Permissions" report is one of the signal sources for the
automated response system. It will complement existing High-Privileged Access
report and focus on highlighting permissions that exist in the cluster but
aren't being utilized, which increases the potential attack surface.

The report includes information about:

- Unused roles
- Unused users
- Unused user roles
- Unused bots
- Unused tokens

_Note: This report is available to Teleport Enterprise customers only and_
_requires the Identity Governance feature flag set in their license._

### "Unused Permissions" report query: Unused roles

An unused role is the one that:

- Is not directly assigned to any user, and
- Is not a part of any auth connector role mapping, and
- Has not been requested via an access request, and
- Is not granted by any access list.

The report will update cluster roles that were determined as unused with the
`inactive_since` status field.

### "Unused Permissions" report query: Unused users

An unused user is the one that hasn't logged in within past 4 weeks.

Teleport will track user logins with the `last_active` status field which the
report will query.

### "Unused Permissions" report query: Unused user roles

An unused user role is the one that is assigned to a user (directly, through
connector or access list) and grants access to resources that user hasn't
connected to.

Teleport will track which user roles are being utilized when connecting to
resources with the `roles_last_active` status field on `user_state` object
which the report will query.

### "Unused Permissions" report query: Unused bots

An unused bot is the one whose issued certificates haven't been used to connect
to any resource.

Such bot users and roles will be updated with the `inactive_since` status field
by the security report.

### "Unused Permissions" report query: Unused tokens

An unused token is the one that hasn't been used for joining.

Such tokens will be updated with the `inactive_since` status field by the
security report.

## Examples

### Removing unused roles

This rule makes sure that roles that grant access to production resources and
haven't been used in past 2 weeks do not exist in the system:

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: unused-role-is-removed
spec:
  subjects: ['role']
  states: ['removed']
  condition: contains(role.metadata.labels['env'], 'prod') && date_before(role.status.inactive_since, now()-'4w')
```

### Removing unused role's login

This rule makes sure that production roles don't have unused allowed logins.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: unused-role-login-is-removed
spec:
  subjects: ['role.spec.allow.logins']
  states: ['removed']
  condition: contains(role.metadata.labels['env'], 'prod') && date_before(role.status.logins_last_active[subject], now()-'4w')
```

### Locking insecure users (and unlocking them back)

This rule makes sure that users that were marked as having weak security are
locked in the system.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: insecure-user-is-locked
spec:
  subjects: ['user']
  states: ['locked']
  condition: user.status.weak_security
```

This rule makes sure that users that were previously marked as weak security
(but not anymore) and locked by access monitoring system, are unlocked back.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: secure-user-is-unlocked
spec:
  subjects: ['user']
  states: ['unlocked']
  condition: !user.status.weak_security && user.status.locked_by == 'insecure-user-is-locked'
```

### Removing unused role from user

This rule makes sure that users aren't assigned roles that they don't use to
connect to resources.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: unused-user-role-is-removed
spec:
  subjects: ['user.spec.roles']
  states: ['removed']
  condition: contains(role.metadata.labels['env'], 'prod') && date_before(user.status.roles_last_active[subject], now()-'4w')
```

### Removing old static tokens

This rule makes sure that old static tokens are removed from the system,
regardless of whether they're used or not.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: old-static-tokens-are-removed
spec:
  subjects: ['static_token']
  states: ['removed']
  condition: date_before(resource.metadata.created_at, now()-'4w')
```

### Sending access request notification

This rule makes sure that a PagerDuty plugin is notified on access requests.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: access-request-reviewers-are-notified
spec:
  subjects: ['access_request']
  condition: access_request.spec.roles.contains('prod-rw') && !access_request.status.notified
  notification:
  - name: 'pagerduty'
    recipients: ['alice@acme.com']
```

The access request will be updated with "notified" status field to avoid
resending notifications every time the rule is evaluated.

### Geo and behavior-based locking

The details of behavior and geolocation-based user locking will be discussed
in a separate RFD but assuming there will be a system implemented within
Teleport that will be tracking additional user properties such as login devices,
locations and so on by updating the `status` field proposed in this RFD, it
will be possible to write access monitoring rules based on this information.

For example, to lock users who logged in from a banned country.

```yaml
kind: access_monitoring_rule
version: v1
metadata:
  name: users-from-country-x-are-locked
spec:
  subjects: ['user']
  states: ['locked']
  condition: user.status.last_login_country_code == 'x'
```

---
authors: Russell Jones (rjones@goteleport.com)
state: draft
---

# RFD 0004 - Soft Limits

# Required Approvers

* Engineering @rosstimothy && (@fspmarshall || @espadolini) && (@zmb3 || @r0mant)
* Product: (@xinding33 || @klizhentas )

## What

This RFD defines soft limits for a cluster and updates to the notification
system that will be used to alert an administrator when soft limits are
exceeded.

Soft limits define the limits within which a Teleport cluster has been tested
and known to function. Exceeding soft limits may lead to degraded performance
in the best case and a cluster-wide outage in the worst case.

Soft limits cover items like nodes, roles, leaf clusters, and claims.

## Why

Without a clear definition of soft limits, Teleport cluster administrator may
unknowingly be running a cluster beyond the limits it has been tested for which
may lead to an outage.

Cluster administrators should be informed of the limits and impact of running a
cluster past limit within documentation and the product. Cluster administrators
may choose to disregard soft limit and operate a cluster beyond the defined
limits, but that should be a informed decision.

Soft limits also help in diagnosis and triage of support requests.

## Details

### Soft Limits

| Description       | Limit  |
|-------------------|--------|
| Direct dial nodes | 25,000 |
| Tunnel nodes      | 10,000 |
| Leaf clusters     | 2,500  |
| SSO claims        | 200    |
| Roles             | 100    |

### Visibility

Soft limit alerts will only be shown to users who can take actionable steps to
remediate them. Within a Teleport cluster this correspond to users with the
[cluster `editor`
role](https://github.com/gravitational/teleport/blob/branch/v11/lib/services/presets.go#L30-L83)
or equivalent.

This will be done by using cluster alert labels. Cluster alerts labels with the
prefix `teleport.internal/alert-verb-permit` can define a set of
`<resource>:<verb>` over which a logical OR will be performed to determine if
the alert should be shown to the user.

The label for soft limits will be constructed at process startup by looping
over the allow rules for predefined `editor` and adding all `RW` rules to the
cluster alert label. It will look like the following.

```
teleport.internal/alert-verb-permit=user:create|role:create|...
```

This way any changes to the predefined `editor` role will automatically be
reflected in the soft limit alert label. Runtime removal of the `editor` role
will also not impact soft limit alerts.

Cluster administrators will be able to acknowledge soft limit alerts while they
work to address the issue.

### Implementation Details

A new service will be added to the process supervisor to manage soft alerts on
Auth Service.  This new service will be a loop which ticks every 10 minutes,
acquires a backend lock, then uses the cache to calculate soft limits and raise
and alert if needed.

### UX

Soft limit alerts will be shown in the UI (`tsh`, `tctl`, and Web UI), `tctl
top`, and process logs.

The error message shown to the user will use the following template.

```
Cluster soft limits exceeded.

* Found 4,000 active leaf clusters, soft limit: 2,500
* Found 200 active roles, soft limit: 100

Operating a cluster beyond soft limits may lead to degraded performance in the
best case and a cluster-wide outage in the worst case.

See https://goteleport.com/docs/management/operations/scaling for more
information on soft limits and how to tune your cluster to stay below limits.
```

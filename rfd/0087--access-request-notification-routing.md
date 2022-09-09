---
authors: Hugo Hervieux (hugo.hervieux@goteleport.com)
state: draft
---
# RFD 87 - Access request notification routing

## Required Approvers

* Engineering ???
* Security @reedloden
* Product: (@xinding33 || @klizhentas)

## What

Provide a granular way for administrators to configure which access-requests notifications should be delivered to whom over which medium.

## Why

- Teleport access request routing is not configured the same way depending on the plugin: Pagerduty uses annotations while most plugins use a role_to_recipient map in the plugin config. Users leveraging both PagerDuty and Slack plugins have to deal with two configuration languages.
- Current Pagerduty request routing applies the same rule to all requests from the same role, regardless of the requested role. All access requests don't have the same severity, users wanting to route access requests differently have to create multiple roles. Depending on the company size and structure cardinality can become a problem. See https://github.com/gravitational/teleport-plugins/issues/597
- Access request routing map baked in the plugin deployment:
  - Will become huge for big orgs
  - Requires redeploying each time a change is done
  - Does not allow to route requests regarding of the requestor

## Details

### User story

Bob is a developer. He deploys code using CI/CD pipelines. He can request access to prod in read-only mode and dev in read-write mode for debugging. In case of incident he can request prod read-write access.
Alice is the lead-dev, she grants regular access requests through MsTeams during open hours and can also approve urgent read-write requests.
Alice should not be paged each time Bob needs to debug something during open hours, but in case of incident Bob needs immediate access and Alice should be paged.

In Teleport terms:

- the role `developer` can request roles `dev-rw`, `prod-ro` and `prod-rw`
- the role `lead-developer` can accept requests
- only `prod-rw` access requests should trigger a PagerDuty incident
- `dev-rw` and `prod-ro` access requests should trigger a MsTeams message

### Suggestion 1: With a where clause

```
kind: role
metadata:
  name: developer
spec:
  allow:
    request:
      roles: ["dev-rw", "prod-ro","prod-rw"]
      destinations:
        # Send requests for prod-rw through PagerDuty
      - where: 'equals(resource.spec.requested_role, "prod-rw")'
        plugin: pagerduty
        target: ["Teleport Alice"]
        # Send all other requests through MsTeams
      - where: 'equals(resource.spec.requested_role, "prod-rw")'
        plugin: msteams
        target: ["Alice@example.com"]
```

- `where` is a where clause it can be evaluated server-side or client-side. An empty where evaluates to true.
- `plugin` acts as a label and each plugin instance can filter based on this.

The destinations are additive:
- they can come from the `role_to_recipient map`
- they can come from the `destinations` on the role
- they can be added in the requests additional recpipients
- they can come from annotations (backward compatibility for pagerduty plugin)

### Suggestion 2: With the existing annotation system

```
kind: role
metadata:
 name: developer
spec:
 allow:
  request:
   roles: ["dev-rw","prod-ro","prod-rw"]
   annotations:
    pagerduty_destinations: ["Teleport Alice"]
    pagerduty_send_roles: "prod-rw"
    msteams_services: ["alice@example.com"]
    msteams_ignore_roles: "prod-rw"
```

Each plugin watches its own annotations:

* `_destinations` lists the destinations (keep `_service`in pager duty for backward compatibility)
* `_send_roles`  allowlist
* `_ignore_roles` blocklist

### Comparing the 2 suggestions

Time to feature:
- The where clause implementation requires to extend the role API, it also
  requires evaluating the where clause. If this happens server-side, ths will
  require more logig into the Teleport codebase. If this happens plugin-side this
  will require exporting the existing where clause logic so it can be imported in
  teleport plugins. One can safely estimate the dev time to at least 2/3 weeks.
- The annotation sysytem is a convention, it does not require changing the API
  and can be implemented in less than a week.

Possibilities:
- The where clause is more granular than the annotation system, if needed the
  context passed to the rule evaluation engine can be extended to route access
  requests based on time for example.
- The annotation system only supports allow and blocklist per plugin, more
  logic comparisons cannot be easily added. It also does not easily support
  multiple instances of the same plugin with different destinations

UX considerations:
- The where clause seems less affordant than the annotation, a first time user
  will likely prefer the simpler annotation syntax
- Having a lot (10+) rules can make reading the role itself hard (like
  [managedFields in Kubernetes](https://github.com/kubernetes/kubernetes/issues/90066))

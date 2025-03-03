---
authors: Pawel Kopiczko (pawel.kopiczko@goteleport.com)
state: implemented
---

# RFD 186 - Optionally require reason for Access Request


## Required Approvers

* Engineering: @r0mant && @smallinsky
* Product: @klizhentas && @xinding33


## What

Add the ability to set a policy to make a reason required for an access request
to be submitted
([#20164](https://github.com/gravitational/teleport/issues/20164)).


## Why

Improves user experience for environments with audit/compliance policies that
require a reason. Adding the ability to mark a reason as required would allow
both tsh and the web UI to prompt for a reason without the end user needing to
remember that it's required and as a result reduce the number of invalid access
requests that need to be rejected.


## Details


### UX

Let's assume scenario where there are two roles `kube-access` and `node-access`
user `bob` can request:

```yaml
kind: role
version: v7
metadata:
  name: kube-access
spec:
  allow:
    kubernetes_resources:
      - kind: '*'
        namespace: '*'
        name: '*'
        verbs: ['*']
```

```yaml
kind: role
version: v7
metadata:
  name: node-access
spec:
  allow:
    logins: ['admin']
    node_labels:
      '*': '*'
```

The administrator may want to configure a user `bob` to be able to:

- request `kube-access`, but only if a non-empty reason is provided
- request `node-access` with or without providing a reason

When user `bob` requests access to `kube-access` role without providing a
reason they receive an error:

```
$ tsh request create --roles kube-access
Creating request...
ERROR: request reason must be specified (required by static role configuration)
```

When user `bob` provides a reason the action is successful:

```
$ tsh request create --roles kube-access --reason "Ticket 1234"
Creating request...
Request ID:     0192f6c0-cd34-79bc-9e51-14b983265106
Username:       bob
Roles:          kube-access
Reason:         "Ticket 1234"
Reviewers:      [none] (suggested)
Access Expires: 2024-11-04 22:28:22
Status:         PENDING

hint: use 'tsh login --request-id=<request-id>' to login with an approved request

Waiting for request approval...
```

When user `bob` requests access to `node-access` without providing a reason the
action succeeds:

```
$ e/build/tsh request create --roles node-access
Creating request...
Request ID:     0192f6c7-ba5e-76b6-8af0-372d6a9d2406
Username:       bob
Roles:          node-access
Reason:         [none]
Reviewers:      [none] (suggested)
Access Expires: 2024-11-04 22:28:22
Status:         PENDING

hint: use 'tsh login --request-id=<request-id>' to login with an approved request

Waiting for request approval...
```


### Possible implementations


#### 1. Add a new role.spec.allow.request.reason.mode setting (chosen)

Introduce a new role.spec.allow.reason.mode setting which could be set
to "required" or "optional" (default).

Example:

```yaml
kind: role
version: v7
metadata:
  name: kube-access-requester
spec:
  allow:
    request:
      roles:
        - kube-access
      reason:
        mode: "required"
```

The problems with the approach:

- It isn't clear what should happen when there is a role with
  `options.request_access: "always"` and a role with `allow.request.mode:
  "required"`. Should the reason be requested regardless during login? (yes)
- The setting still spans across roles to some extend. If there is more than
  one role allowing requesting access to the same resource/role but only _some_
  of them require reason, the reason is required.
- Future - as an extension of the problem above: there were ideas of having more
  advanced requirements when it comes to providing a reason, e.g. using regular
  expressions. In case of `options.request_access: reason` - how to satisfy all
  of them?


#### 2. Add a new value to role.options.request_access

Currently a role can have `request_access` option set to one of the following
values: `optional`, `always` and `reason` as described in
https://goteleport.com/docs/admin-guides/access-controls/access-requests/access-request-configuration/#how-clients-request-access.

The idea is to support another value `reason-required`.

> [!CAUTION]
> role.options.request_access is a global configuration, therefore it doesn't
> meet the requirement of requiring reason only for _some_ resources/roles.

It has to be noted that role.options.request_access has a global scope. I.e. if
it's set on any role all access requests to all roles and resources are
affected.

Example:

For two roles `kube-access-requester` and `node-access-requester` as defined
below:

```yaml
kind: role
version: v7
metadata:
  name: kube-access-requester
spec:
  allow:
    request:
      roles:
        - kube-access
  options:
    request_access: reason-required
```

```yaml
kind: role
version: v7
metadata:
  name: node-access-requester
spec:
  allow:
    request:
      roles:
        - node-access
```

A user with both roles `kube-access-requester` and `node-access-requester`
assigned will be affected by request_access set in role `kube-access-requester`
while requesting access to role `node-access` allowed by
`node-access-requester` role.

Because request_access option is global a priority has to be establish when
more than one role has the option specified:

- reason
- always
- reason-required
- optional (default)

Problems:

- It may be confusing/undesired that roles with request_access option set to
  "reason-required" affects other roles.


#### 3. Add a new value to role.options.request_access and change the option to be scoped

> [!NOTE]
> This is a breaking change.

The solution would be like in 1. but with changed semantics of request_access
option. It would become role-scoped instead of global.

Example:

For two roles `kube-access-requester` and `node-access-requester` as defined
below:

```yaml
kind: role
version: v7
metadata:
  name: kube-access-requester
spec:
  allow:
    request:
      roles:
        - kube-access
  options:
    request_access: reason-required
```

```yaml
kind: role
version: v7
metadata:
  name: node-access-requester
spec:
  allow:
    request:
      roles:
        - node-access
  options:
    request_access: optional # default, doesn't need to be set explicitly
```

A user with `kube-access-requester` and `node-access-requester` roles assigned
would have to provide a reason while creating an access request for
`kube-access` role but wouldn't have to provide reason while requesting access
to `node-access` role.

The example above is clear and simple, but there are problems with the
implementation:

- **This is a breaking change.** It may come as a surprise to the existing
  users that only _some_ instead of _all_ roles are automatically requested on
  login when request_access is set to "always" or "reason".
- What happens when there are 2 or more roles allowing requesting the same
  role/resource but they have different request_access set? Should the solution
  fall back to priorities, like in 1.?


#### 4. Add role.spec.allow.request.where expressions

This would be similar to `review_request.deny.where`. See
https://goteleport.com/docs/admin-guides/access-controls/access-requests/access-request-configuration/#allowing-and-denying-reviews-for-specific-roles

Example:

```yaml
kind: role
version: v7
metadata:
  name: request-kube-access-with-reason
spec:
  allow:
    request:
      roles:
        - kube-access
      where: 'request.reason != ""'
```

The problems with the approach:

- It would be difficult for the UI to provide immediate feedback (like
  highlighted reason input) why the request can't be created.


### Limitations

- In the web UI, currently `role.options.request_access` option is evaluated
  only during login, so the user has to re-login to have a correct UX if there
  are changes to their assigned roles.
- Depending on the chosen solution the implementation may be quite involved for
  the value it provides.


### Decision

The chosen implementation is:

```
1. Add a new role.spec.allow.request.reason.mode setting
```

It seems to be least confusing, declarative and straight-forward.

One thing to note is that when a user:

- has any role with `role.spec.options.request_access: always`
- has any role with `role.spec.allow.request.reason.required: true`

It will be effectively equivalent of setting `role.spec.options.request_access:
reason` in any of the roles.


### Test Plan

The IGS section of the test plan needs to be extended with these items:

- [ ] Access Requests
  - [ ] Verify when role.spec.allow.request.reason.mode: "required":
    - [ ] CLI fails to create Access Request displaying a message that reason is required.
    - [ ] Web UI fails to create Access Request displaying a message that reason is required.
    - [ ] Other roles allowing requesting the same resources/roles without reason.mode set or with reason.mode: "optional" don't affect the behaviour.
    - [ ] Non-affected resources/roles don't require reason.
    - [ ] When there is a role with spec.options.request_access: always it effectively becomes role.spec.options.request_access: reason (i.e.) requires reason:
      - [ ] For CLI.
      - [ ] For Web UI.


### References

- Old RFD:
  https://github.com/gravitational/teleport/blob/master/rfd/0003-extended-approval-workflows.md
- request_access documentation:
  https://goteleport.com/docs/admin-guides/access-controls/access-requests/access-request-configuration/#how-clients-request-access
- Private Slack thread:
  https://gravitational.slack.com/archives/C05M9TC5WKD/p1729799236724089

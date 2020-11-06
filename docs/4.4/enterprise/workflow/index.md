---
title: Approval Workflows for SSH and Kubernetes Access
description: How to customize SSH and Kubernetes access using Teleport.
---

# Teleport Approval Workflows

#### Approving Workflow using an External Integration
- [Integrating Teleport with Slack](ssh-approval-slack.md)
- [Integrating Teleport with Mattermost](ssh-approval-mattermost.md)
- [Integrating Teleport with Jira Cloud](ssh-approval-jira-cloud.md)
- [Integrating Teleport with Jira Server](ssh-approval-jira-server.md)
- [Integrating Teleport with PagerDuty](ssh-approval-pagerduty.md)


## Approval Workflows Setup

Teleport 4.2 introduced the ability for users to request additional roles. The
workflow API makes it easy to dynamically approve or deny these requests.

### Setup

**Contractor Role**
This role allows the contractor to request the role DBA.

```yaml
kind: role
metadata:
  name: contractor
spec:
  options:
    # ...
  allow:
    request:
      roles: ['dba']
    # ...
  deny:
    # ...
```

**DBA Role**
This role allows the contractor to request the role DBA.

```yaml
kind: role
metadata:
  name: dba
spec:
  options:
    # ...
    # Only allows the contractor to use this role for 1 hour from time of request.
    max_session_ttl: 1h
  allow:
    # ...
  deny:
    # ...
```

**Admin Role**
This role allows the admin to approve the contractor's request.
```yaml
kind: role
metadata:
  name: admin
spec:
  options:
    # ...
  allow:
    # ...
  deny:
    # ...
# list of allow-rules, see
# https://gravitational.com/teleport/docs/enterprise/ssh-rbac/
rules:
    # Access Request is part of Approval Workflows introduced in 4.2
    # `access_request` should only be given to Teleport Admins.
    - resources: [access_request]
      verbs: [list, read, update, delete]
```


```bash
$ tsh login teleport-cluster --request-roles=dba
Seeking request approval... (id: bc8ca931-fec9-4b15-9a6f-20c13c5641a9)
```

As a Teleport Administrator:


```bash
$ tctl request ls
Token                                Requestor Metadata       Created At (UTC)    Status
------------------------------------ --------- -------------- ------------------- -------
bc8ca931-fec9-4b15-9a6f-20c13c5641a9 alice     roles=dba      07 Nov 19 19:38 UTC PENDING
```

```bash
$ tctl request approve bc8ca931-fec9-4b15-9a6f-20c13c5641a9
```

Assuming approval, `tsh` will automatically manage a certificate re-issued with
the newly requested roles applied. In this case `contractor` will now have have
the permission of the `dba`.

!!! warning

    Granting a role with administrative abilities could allow a user to **permanently**
    upgrade their privileges (e.g. if contractor was granted admin for some reason).
    We recommend only escalating to the next role of least privilege vs jumping directly
    to "Super Admin" role.

    The `deny.request` block can help mitigate the risk of doing this by accident. See
    Example Below.


```yaml
# Example role that explicitly denies a contractor from requesting the admin
# role.
kind: role
metadata:
name: contractor
spec:
options:
    # ...
allow:
    # ...
deny:
    request:
    roles: ['admin']
```

## Adding a reason to approval requests

Teleport 4.4.4 introduced the ability for users to request additional roles. `tctl`
or the Access Workflow API makes it easy to dynamically approve or deny these requests.

In the Supporting users that start in an essentially unprivileged state and must
always go through the dynamic access API in order to gain meaningful privilege.

Leveraging the claims (traits) provided by external identity providers both when
determining which roles a user is allowed to request, and if a specific request
should be approved/denied.

### Example Setup

**Unprivileged User**<br>
In this example we've a employee, who isn't able to access any systems. When they
login they'll always need to provide a reason to access.

```yaml
kind: role
metadata:
  name: employee
spec:
  allow:
    request:
      # the `roles` list can now be a mixture of literals and matchers
      roles: ['common', 'dev-*']
      # the `claims_to_roles` mapping works the same as it does in
      # the oidc connector, with the added benefit that the mapped to roles
      # can also be matchers.  the below mapping says that users with
      # the claims `groups: admins` can request any role in the system.
      claims_to_roles:
        - claim: groups
          value: admins
          roles: ['*']
      # teleport can attach annotations to pending access requests. these
      # annotations may be literals, or be variable interpolation expressions,
      # effectively creating a means for propagating selected claims from an
      # external identity provider to the plugin system.
      annotations:
        foo: ['bar']
        groups: ['{% raw %}{{external.groups}}{% endraw %}']
  options:
    # the `request_access` field can be set to 'always' or 'reason' to tell
    # tsh of the web UI to always create an access request on login.  If it is
    # set to 'reason', the user will be required to indicate *why* they are
    # generating the access request.
    request_access: reason
    # the `request_prompt` field can be used to tell the user what should
    # be supplied in the request reason field.
    request_prompt: Please provide your ticket ID
version: v3
```

**Unprivileged User Login**<br>

```bash
# Login: This will prompt the user to provide a reason in the UI.
tsh login
# Login: The user can provide a reason using tsh.
tsh login --request-reason="..."
```

!!! Note

    Notice that the above role does not specify any logins. If a users's roles specify no logins, teleport will now generate the user's initial SSH certificates with an invalid dummy login of the form `-teleport-nologin-<uuid>` (e.g. `-teleport-nologin-1e02dbfd-8f6e-47a0-a66c-93747b010f88`).

**Admin Flow: Approval/Deny**<br>

A number of new parameters are now available that allow the plugin or administrator to grant greater insight into approvals/denials:

```bash
$ tctl request deny --reason='' --annotations=method=cli,unix-user=${USER} 28a3fb86-0230-439d-ad88-11cfcb213193
```

Because automatically generated requests always include all roles that the user is allowed to request, approvers can now specify a smaller subset of the requested roles that should actually be applied, allowing for sub-selection in cases where full escalation is not a desirable default:

```bash
$ tctl request approve --roles=role-1,role-3 --reason='thats cool, but no role-2 right now' 28a3fb86-0230-439d-ad88-11cfcb213193
```

### Other features of Approval Workflows.

 - Users can request multiple roles at one time. e.g `roles: ['dba','netsec','cluster-x']`
 - Approved requests have no effect on Teleport's behavior outside of allowing additional
   roles on re-issue. This has the nice effect of making requests "compatible" with
   older versions of Teleport, since only the issuing Auth Server needs any particular
   knowledge of the feature.

## Integrating with an External Tool

| Integration | Feature | Type          | Setup Instructions |
|-------------|---------|---------------|--------------------|
| Slack       |         | Chatbot       | [Setup Slack](ssh-approval-slack.md) |
| Mattermost  |         | Chatbot       | [Setup Mattermost](ssh-approval-mattermost.md) |
| Jira Server |         | Project Board | [Setup Jira Server](ssh-approval-jira-server.md) |
| Jira Cloud  |         | Project Board | [Setup Jira Cloud](ssh-approval-jira-cloud.md) |
| PagerDuty   |         | Schedule      | [Setup PagerDuty](ssh-approval-pagerduty.md) |

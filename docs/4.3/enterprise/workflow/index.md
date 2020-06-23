# Teleport Approval Workflows

#### Approving Workflow using an External Integration
- [Integrating Teleport with Slack](ssh_approval_slack.md)
- [Integrating Teleport with Mattermost](ssh_approval_mattermost.md)
- [Integrating Teleport with Jira Cloud](ssh_approval_jira_cloud.md)
- [Integrating Teleport with Jira Server](ssh_approval_jira_server.md)
- [Integrating Teleport with PagerDuty](ssh_approval_pagerduty.md)


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
    # Only allows the contractor to use this role for 1hr from time of request.
    max_session_ttl: 1hr
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
# https://gravitational.com/teleport/docs/enterprise/ssh_rbac/
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

### Other features of Approval Workflows.

 - Users can request multiple roles at one time. e.g `roles: ['dba','netsec','cluster-x']`
 - Approved requests have no effect on Teleport's behavior outside of allowing additional
   roles on re-issue. This has the nice effect of making requests "compatible" with
   older versions of Teleport, since only the issuing Auth Server needs any particular
   knowledge of the feature.

## Integrating with an External Tool

| Integration | Feature | Type          | Setup Instructions |
|-------------|---------|---------------|--------------------|
| Slack       |         | Chatbot       | [Setup Slack](ssh_approval_slack.md) |
| Mattermost  |         | Chatbot       | [Setup Mattermost](ssh_approval_mattermost.md) |
| Jira Server |         | Project Board | [Setup Jira Server](ssh_approval_jira_server.md) |
| Jira Cloud  |         | Project Board | [Setup Jira Cloud](ssh_approval_jira_cloud.md) |
| PagerDuty   |         | Schedule      | [Setup PagerDuty](ssh_approval_pagerduty.md) |

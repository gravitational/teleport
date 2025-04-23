---
authors: Alexander Klizhentas (sasha@goteleport.com)
state: draft
---

# RFD 2 - Open Source Roles

## What

Enable role based access control in open source.

## Why

Provide better user experience for open source users.
Unify user experience, testing and development for open source branches.

## Details

### New Open Source Features

The following features will become available starting 5.1 version.

**Role Based Access Control**

All RBAC features, except FedRamp feature flags.

**Access Workflow Plugins**

Some access workflows plugins will become available in the open source:

* Access Workflows Golang SDK and API
* Webhook
* Slack
* Gitlab
* Mattermost
* JIRA Plugin
* PagerDuty Plugin

### Enterprise Only Features

The following features will remain enterprise only.

**FedRamp**

* Max connections AC control (and all future AC controls):

```yaml
role:
   options:
     max_connections: 3
```

* Teleport fips mode flag

```bash
$ teleport start --fips
```

**Single Sign On**

OIDC and SAML connectors

**Extended access workflows**

New feature flags allowing waiting room and always requesting flow
will be enterprise only.

```yaml
role:
  options:
     # enterprise only values
     request_access: 'note|always'
```

OSS users can request roles using `tsh login --request-roles` on demand.

User interface with approval requests, waiting room remains in enterprise.

### Migration Details

**Open Source Users**

Open source users will be assigned to a new `user` role. This role
is almost backwards compatible with builtin OSS role `admin`,
except it does not allow to modify resources. Otherwise all users
will become admins after migration:

```yaml
role:
   name: user
spec:
  options:
    ssh_port_forwarding:
      remote:
        enabled: true
      local:
        enabled: true
    max_session_ttl: 30h
    forward_agent: true
    enhanced_recording: ['command', 'network']
  allow:
    logins: ['{{internal.logins}}']
    node_labels: '*': '*'
```

Another role, `admin` will be created.

```yaml
kind: role
metadata:
  name: admin
spec:
  allow:
    logins: ['this-login-does-not-exist']
    rules:
    - resources: ['*']
      verbs: ['*']
  deny: {}
```

Migration tutorial will advice to promote designated local user
to admin:

```bash
$ tctl users update alice --set-roles=admin
```

**Github Connector**

Github in open source mode will support both `teams_to_logins`
and `teams_to_roles` modes.

**Adding users in tctl**

Both OSS (legacy) user add will be supported to preserve backwards
compatibility.

```
# Adding a user to Teleport with the principle joe, root & ec2-user
$ tctl users add joe joe,root,e2-user

# Becomes alias of
$ tctl users add joe --traits=internal.logins=joe,root,e2-user --roles=user

# Adding a user to Teleport as role Admin.
$ tctl users add --roles=admin joe
```

**Tsh status**

Tsh status loses `RBAC only` notice:

```
$ tsh status
...
* RBAC is only available in Teleport Enterprise
https://gravitational.com/teleport/docs/enterprise
```

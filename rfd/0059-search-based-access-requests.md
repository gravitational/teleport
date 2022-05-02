---
authors: Alexander Klizhentas (sasha@goteleport.com), Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 59 - Search Based Access Requests

## What

This RFD proposes just-in-time search-based access requests which will enable
users to request access to resources based on a search and/or selection of
individual resources, rather than having to request one or more roles.

Also proposed is a method to automatically request access to an SSH node during
`tsh ssh` when the user does not normally have permission, but is able to
request it.

## Why

Current role-based access requests require users to know in advance which roles
they need or who should grant access. They should not need to know anything
about roles, just the resource(s) which they need access to.

Teleport admins would like to avoid having to create many different custom roles
for different levels of access to different sets of resources.

Requesting access to a "blanket" role violates the principle of least priviledge.
Most of the time incident responders only need access to 1 or 2 nodes,
search-based access requests will allow them to request access to only the nodes
they need.

## Details

Proposed is a new way of requesting access to specific resources rather than
roles. There initially will be 3 ways for a user to request access to a resource:

1. By searching for and explicitly requesting access to a resource from the Web
   UI.

   Users will be able to search for any type of resource using basic or advanced
   search, and request access to a set of resources.

2. By searching for and explicitly requesting access to a resource from the CLI.

   Users will be able to search for any type of resource using `tsh request
   search`.

3. Users will be able to request access to an ssh node automatically when using
   `tsh ssh user@node`.

   If the user does not currently have access to the node but is able to request
   access, they will be prompted to do so. This could be a suggested command or a
   prompt.

Users won’t be asked to think about roles in advance, but will be able to focus
on finding and requesting access to individual resources. Roles will be used
behind the scenes to evaluate which resources a user can search for and request,
and who will be responsible for reviewing and granting those requests.

### Role Spec

To enable this level of matching we propose an extension to the role spec to
determine the final roles that need to be granted.

We will specify two roles that define this flow.

The role `response-team` will allow users with this role to search for nodes as
if they had the role `db-admins` and will allow them to request access to nodes
normally requiring the `db-admins` role.
If the access request is approved, the response team member will assume a
restricted version of the `db-admins` role which only has access to the specific
resources they were approved for.

```yaml
kind: role
metadata:
  name: response-team
spec:
  allow:
    request:
      # search_as_roles allows a member of the response team
      # to search for resources accessible to users with the db-admins role,
      # which they will be allowed to request access to.
      search_as_roles: [db-admins]
```

Users will effectively "assume" the role db-admins when searching for nodes in
the search UI and will also receive the certificate with this role (but
restricted to the specific resources they requested) if access request is
granted.

```yaml
kind: role
metadata:
  name: db-admins
spec:
  allow:
    logins: ["root"]
    # db_labels defines which databases this role will be allowed to search
    # for as a part of a search_as request, and also to evaluate access after
    # the request is approved.
    db_labels:
       owner: db-admins
```

### Query Language and Filter UI

The query language and filter UI is defined by the
[Pagination and Search RFD](./0055-webui-ss-paginate-filter.md).

### Example Flow - Web UI

Alice is a member of the group “splunk”.

She gets an alert that “db-1” is malfunctioning.

Alice needs to locate the "db-1" resource and request access to it.

Alice goes to the “Request access” screen and selects the “select access to resources”
option that is available alongside the “select access to roles” option.

Alice uses standard (fuzzy search) or advanced search (labels) to enter the
resource name.

She finds a single resource (e.g. "db-1") or a group of resources by that name,
selects them, and clicks "request access".

An access request is created. It is sent to the "db-admins" team members via slack,
email or other configured channel based on the labels on the requested resources
(label-based routing will be added to access request plugins).

There are two thresholds set in access requests for this group of resources.
When both Ivan and Mary approve the request, Alice is granted access for one
hour.

Alice can now see the "db-1" server on the "Servers" page and is able to ssh to
it.

### Example Flow - CLI Search

```bash
$ tsh request search --kind db --search db1
Found 2 items:

name kind     id
db-1 database db:388aff7f-459f-4a43-804a-3729854976ab
db-1 node     node:3be2fdad-7c79-4cfa-924e-ec1ea7225320

Create access request by:
> tsh request create --resources "db:388aff7f-459f-4a43-804a-3729854976ab,node:3be2fdad-7c79-4cfa-924e-ec1ea7225320"

$ tsh request create --resources "db:388aff7f-459f-4a43-804a-3729854976ab,node:3be2fdad-7c79-4cfa-924e-ec1ea7225320"
Waiting for request to be approved...
Approved!
$ tsh ssh root@db-1
root@db-1:~$
```

Users can search by kind, labels, and keywords. The `tsh request search` command
will output a `tsh request create` command which can be used to request access.

There will also be a flag `tsh request search --create ...` which will automatically
execute the search and create the access request in a single step.

### Example Flow - On-Demand SSH

Many times users would not want to search and request access in two steps.
`tsh` will have a new flag `--request` that will try accessing a node, and
if access denied it will request access for a node all in one step:

```bash
$ tsh ssh --request root@db-1
You do not have access to the system by default, created access request.

Please wait...

Access request has been approved.
root@db-1:~$
```

Some users would want to have the `--request` flag be the default behaviour.
There are a few options for this:

1. The
   [alias feature](https://github.com/gravitational/teleport/blob/master/rfd/0061-tsh-aliases.md)
   in the tsh profile will allow those users to set `tsh ssh as an alias for `tsh ssh --request`.
2. `tsh` can output a suggested command with the `--request` flag if access is
   denied but could be requested
3. `tsh` could prompt the user to create a request with a reason.

### Which roles will be requested

When creating a search-based access request the underlying roles being requested
will be determined automaticially. For simplicity, all roles which the user has
permission to search as (included in `search_as_roles` on any of the roles the
user has) will be requested. This request will be limited to only the exact
resources found in the search, and (if approved) the user will have access to
all logins granted by those roles.

For "on-demand ssh" (`tsh ssh --request user@node`) we will attempt to find and
request a single role which grants access to the node with the requested login.
If multiple such roles exist, the role with the lowest number of allowed logins
will be requested. In case of a tie, the requested role will be chosen
arbitrarily.

### Certificate issuance and RBAC

Once a user assumes the “db-admins” role the Teleport root cluster issues a cert
including the assumed role and a list of UUIDs of resources granted by the
access request.

This makes sure that the role is scoped to a static list of resources that never
changes for this certificate.

```
Assumed-role: [db-admins]
Resource-UUIDs: [node:uuid-1, node:uuid-2]
```

### Trusted clusters

Leaf clusters will use standard role mapping to validate the cert issued by the
root. If the leaf cluster role maps root cluster’s “db-admins” role to the same
role using cluster mapping:

```yaml
role_map:
   '*': '*'
```

Then leaf cluster behavior will be identical. Leaf cluster may choose to narrow
the scope, for example:

```yaml
role_map:
   'db-admins': 'leaf-db-admins'
```

```yaml
kind: role
metadata:
  name: leaf-db-admins
spec:
  allow:
    logins: ["root"]
    # node_labels defines what nodes this role will be allowed to search
    # as a part of search_as request and in addition to that will be used to
evaluate access
    node_labels:
       owner: db-admin
       class: external-access-allowed
```

In this case, if the cert issued by the root cluster granted access to nodes
with uuid “uuid-1” and “uuid-2”. In case if leaf’s uuid-1 has label “class:
external-access-allowed” and uuid-2 does not. Leaf cluster will reject access to
the node uuid-2 despite the fact that the root cluster “allowed” it.

### Access Requests RBAC Edge-Cases

A user could find and request access to several different types of resources,
some with `label: a` others with `label: b`.
Access to resource `label: a` can be granted by users having role `a`.
Access to resource `label: b` can be granted by users having role: `b`.
In this case both users with role `a` and role `b` have to approve access for
the request to be granted.

### Access Requests RBAC Plugin Notification

Access request slack and mattermost plugin should add routing to channels by
label and resource type.

In addition to the existing `role_to_recipients` configuration
```
[role_to_recipients]
"dev" = "devs-slack-channel" # All requests to 'dev' role will be sent to this
channel
"*" = ["admin@example.com", "admin-slack-channel] # These recipients will receive
review requests not handled by the roles above
```

There will be a new `label_to_recipients` section
```
[label_to_recipients]
"env:prod" = "prod-slack-channel" # All requests for resources labelled "env":
"prod" will go to this slack channel
"env:staging" = "staging-slack-channel" # All requests for resources labelled
"env": "staging" will go to this slack channel
"*" = ["admin@example.com", "admin-slack-channel] # These recipients will receive
review requests not handled by the labels above
```

The labels used to match the recipients will be all labels of all resources
being requested.

Since search-based access requests will also be requesting limited access to
roles (even though the roles will be determined automatically) notifications
will be sent to all matching channels from both sections.

### Audit events

Will create a new audit event `access_request.search` that will include the searches users are
running in the scope of requesting access.

### OpenSSH

OpenSSH clients should work with credentials obtained via search-based access requests, assuming that the proxy and node are both Teleport.
OpenSSH servers won't initially support this flow.

### Search-based vs Role-based access requests

The existing “role-based” access requests and the new “search-based” access
requests proposed here are useful for two different scenarios:

- Sometimes it is helpful to request elevated roles in the system, like
  `db-admin`. For example, Alice may need to run a Ansible playbook on all
  machines in a given class, or perform some system upgrades or troubleshooting.
  In this case, role-based access requests work great.

- In other cases, it’s helpful to request access to a specific subset of
  resources that are necessary. In this case, search.

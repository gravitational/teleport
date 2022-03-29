---
authors: Alexander Klizhentas (sasha@goteleport.com), Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 59 - Search Based Access Requests

## What

This RFD proposes just-in-time search-based access requests which will enable
users to request access to resources based on a search and/or selection of
individual resources, rather than having to request one or more roles.

## Why

Teleport users may not know in advance which roles they need or who should grant
access, just which resource(s) they need access to.

Users would like to avoid having many different custom roles for different
levels of access.

Requesting access to a "blanket" role violates the principle of least priviledge.
Most of the time incident responders only need access to 1 or 2 nodes,
search-based access requests will allow them to request access to only the nodes
they need.


## Details

Users will be able to request access by using search, and roles will be used
behind the scenes to evaluate who will be responsible for reviewing and granting
access requests.

Users won’t be asked to think about roles in advance, but will be able to focus
on finding and requesting access to individual resources.

The existing “role-based” access requests and the new “search-based” access
requests proposed here are useful for two different scenarios:

- Sometimes it is helpful to request elevated roles in the system, like
  `db-admin`. For example, Alice may need to run a Ansible playbook on all
  machines in a given class, or perform some system upgrades or troubleshooting.
  In this case, role-based access requests work great.

- In other cases, it’s helpful to request access to a subset of resources. In
  this case, search.

### Example Flow

Alice is a member of the group “splunk”.

She gets an alert that “db-1” is malfunctioning.

Alice needs to locate the "db-1" resource and request access to it.

Alice goes to the “Request access” screen and selects: “select access to resources”
option that is available alongside the “select access to roles” option.

Alice uses standard (fuzzy search) or advanced search (labels) to enter the
resource name.

She finds a single resource (e.g. "db-1") or a group of resources by that name and clicks
"request access".

An access request is created. It is sent to the "db-admins" team members via slack,
email or other configured channel based on the labels on the requested resources
(we will add label-based routing to access request plugins).

There are two thresholds set in access requests for this group of resources.
When both Ivan and Mary approve the request, Alice is granted access for one
hour.

### Query Language and Filter UI

The query language and filter UI is defined by the
[Pagination and Search RFD](./0055-webui-ss-paginate-filter.md).

### CLI Mock-Up

```bash
$ tsh request search --kind db --labels 'env=prod'
Found 2 items:

name kind     id
db-1 database 388aff7f-459f-4a43-804a-3729854976ab
db-2 database 3be2fdad-7c79-4cfa-924e-ec1ea7225320

Create access request by:
> tsh request create --id 388aff7f-459f-4a43-804a-3729854976ab --id 3be2fdad-7c79-4cfa-924e-ec1ea7225320
```

Users can search by kind, labels, and keywords. The `tsh request search` command
will output a `tsh request create` command which can be used to request access.

The CLI UX may be improved by allowing users to perform multiple searches,
"stage" which resources they want to request access to and store this state
locally, and finally execute the full request without needing each UUID on the
command line. But this may be too complex and confusing. With this MVP version
users can copy-paste UUIDs from multiple searches, the meaning is clear, and it
is fully customizable and scriptable.

### On-Demand SSH

Many times users would not want to search and request access in two steps.
Teleport will have a new alias flag modifier that will try accessing a node, in
case if access denied, it will request access for a node all in one step:

```bash
# -P "Please" activate try and request mode
$ tsh ssh -P root@db-node
# by default, if tsh will succeed to SSH, it will just continue with a
# Session, otherwise it will find a node and create a search based access request
You do not have access to the system by default, created access request.

Please wait...

Access request has been approved. Happy hacking!
$
```

Some customers would want to have `-P` flag as a default. New alias feature in
profile, will allow those users to set `tsh ssh` as an alias for `tsh ssh -P`.

For OpenSSH use-cases, some users would like to load the new certificates in the
agent, `tsh login -A root@db-node` will work like `tsh ssh -P` command except
instead of login, it will load the keys in the agent.

### Role Spec

To enable this level of matching, we propose an extension to the role spec to
determine the final roles that need to be granted.

We will specify two roles that define this flow.

The role `response-team` will allow user with this role to
request access to nodes of teams “dbs” and “splunk”, and members of OIDC/SAML
group “app” developers to request access to their own group.

```yaml
kind: role
metadata:
  name: response-team
spec:
  allow:
    request:
      # search_as_roles allows a member of the response team
      # to search for resources accessible to users with the db-admin-role,
      # which they will be allowed to request access to. If the access request
      # is approved, the response team member will assume a restricted
      # version of the db-admin-role, which only has access to the specific 
      # resources they were approved for.
      search_as_roles: [db-admin-role]
```

Users will assume the role db-admin-role when searching for nodes in the search
UI and will also receive the certificate with this role (but restricted to the specific resources they requested) if access request is
granted.

```yaml
kind: role
metadata:
  name: db-admin-role
spec:
  allow:
    logins: ["root"]
    # db_labels defines which databases this role will be allowed to search
	# for as a part of a search_as request, and also to evaluate access after
    # the request is approved.
    db_labels:
       owner: db-admin
```

### Certificate issuance and RBAC

Once a user assumes “db-admin-role” the Teleport root cluster issues a cert
including the assumed role and a list of UUIDs of resources granted by the
access request.

This makes sure that the role is scoped to a static list of resources that never
changes for this certificate.

```
Assumed-role: [db-admin-role]
Resource-UUIDs: [uuid-1, uuid-2]
```

### Trusted clusters

Leaf clusters will use standard role mapping to validate the cert issued by the
root. If the leaf cluster role maps root cluster’s “db-admin-role” to the same role
using cluster mapping:

```yaml
role_map:
   '*': '*'
```

Then leaf cluster behavior will be identical. Leaf cluster may choose to narrow
the scope, for example:

```yaml
role_map:
   'db-admin-role': 'leaf-db-admin-role'
```

```yaml
kind: role
metadata:
  name: leaf-db-admin-role
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

Will complete this section with an example config, or create another RFD.

### Audit events

Will create a new audit event `access_request.search` that will include the searches users are
running in the scope of requesting access.

### OpenSSH

OpenSSH clients should work with credentials obtained via search-based access requests, assuming that the proxy and node are both Teleport.
OpenSSH servers won't support this flow.

---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 0019e - Okta Access Lists Sync

## Required approvers

Engineering: @smallinsky
Product: @klizhentas && @xinding33
IT: @TravisGary && @wacheung

## What

Describes the way Teleport Okta integration will use Access Lists to provide
users imported from Okta with long-term and short-term access to Okta apps.

## Why

In its current state Okta integration provides users with a lot of flexible
parts and the users are left to assemble the car themselves. This leads to a
number of UX issues.

For example, after setting up Okta integration, Teleport imports all apps and
groups automatically, but cluster administrators still have to configure
appropriate RBAC rules in order for users to be able to see their apps within
Teleport or allow requesting access to them.

The syncing logic proposed here aims to improve the user experience for users
setting up Teleport's Okta integration and give them a reasonable configuration
that's usable out-of-the-box.

## Goal

Okta integration will be updated to facilitate UX improvements in two main
scenarios:

- Long-term access. Users synced from Okta will have their in-Okta access
  reflected in Teleport during initial import, and maintained in-sync thereafter.
  Say, if a user "alice" is assigned to the application "Spacelift" in Okta, she
  will see it in Teleport as well.

- Short-term access. Users synced from Okta will be able to submit access
  requests to applications they're not currently assigned, and the application
  owners will be able to approve or deny such requests.

To enable these workflows, Teleport will be automatically creating configuration
resources like Okta import rules, roles and access lists as part of the Okta
integration service.

## UX scenarios

Before diving into details, let's identify a few usage scenarios that will be
enabled by this automatic Okta configuration.

**Scenario 1.** As a cluster administrator, I want to have all my Okta users
synced to Teleport, make sure they see all apps they're assigned in Okta
and let them request short-term or long-term access to other apps, without
having to write and maintain a lot of Teleport RBAC configuration.

**Scenario 2.** As an app user, I want to be able to see an application I have
access to and connect to it via Teleport web UI.

**Scenario 3.** As an app user, I want to be able to see applications I can
request access to and request access to them via Teleport.

**Scenario 4.** As an app owner, I want to be able to see who's requesting
access to my app, review such requests and be able to decide whether to
approve short-term or long-term access.

## Long-term access

Automatic long-term access configuration aims to achieve two goals: initial
import that makes sure that after configuring Okta connection imported Okta
users have access to the same applications within Teleport as they do within
Okta, and continuous sync that keeps reflecting changes made in Okta to Teleport
RBAC configuration to keep the permissions up-to-date.

Each user group in Okta will be imported as a Teleport access list with the
following properties.

### Access list name and description

Access list name will be set to Okta group ID (since Okta group names are not
unique), and the group name will be pulled into a label so it can be displayed
in the UI.

Access list description will be set to Okta group description.

### Access list members

Okta users assigned to a particular Okta group will be made members of the
access list representing this Okta group in Teleport.

### Access list owners

Unlike members, there is no clear-cut way to automatically determine which
Okta users should be made owners of which synced access lists. Okta does provide
ability to set [group owners](https://help.okta.com/en-us/content/topics/identity-governance/group-owner.htm)
but that appears to be a recent feature that requires a purchase of Okta's own
IGA offering which makes it ineligible for Teleport to rely on since many users
may not or will not have it enabled.

For that reason, Teleport must provide a way to configure this mapping during
the initial import in Teleport Discover flow, allowing users to explicitly
choose access list owners.

Users setting up Okta integration will be asked to:

- (Required) Specify a default criterion by which Teleport will determine
  whether a particular Okta user should be an access list owner. For example,
  an IT team initially can set its members as owners for each list.
- (Optional) Specify an access list owner criterion for each individual access
  list being imported.

The "criterion" here can be either an explicit set of users imported from Okta,
or be determined by Okta group membership (which becomes a Teleport user trait)
e.g. "users from 'IT Admins' group".

The default criterion will be saved as a part of Okta integration configuration
and applied to all access lists where user didn't explicitly specify owners, as
well as all future new access lists being synced from Okta.

After initial import, Teleport cluster administrators will be able to still go
and update owners appropriately using the access lists management UI for each
of the imported access lists.

### Access list roles and traits

Each imported access list will provide its members with access to the same set
of apps that are assigned to the Okta group it corresponds to.

_Note: Initially, Teleport will import **all** Okta groups as access lists._
_In future, we can implement filtering that will be stored as part of Okta_
_integration configuration, for example to allow import of specific groups_
_only using matchers similar to those in Okta import rules._

Teleport will add support for a special kind of access list, with subkind
`okta_group`, that will identify which Okta group it grants access to:

```yaml
kind: access_list
subkind: okta_group
metadata:
  name: <app-Spacelift-group-ID>
  description: Spacelift group description from Okta
  labels:
    teleport.internal/display-name: app-Spacelift
spec:
  okta_group_id: "<app-Spacelift-group-ID>"
```

An access list of this type will not support specifying granted roles and traits,
and instead will configure appropriate Teleport resources automatically to
provide access to the matching group.

To do so, it will label the specified Okta group, and all its assigned apps,
with labels equivalent to the following Okta import rule:

```yaml
kind: okta_import_rule
version: v1
metadata:
  name: import-rule-app-Spacelift
  labels:
    teleport.internal/resource-type: "system" # <-- we already have this label
  owner_references: # <-- similar to k8s to indicate who manages this resource
  - kind: access_list
    id: <id>
spec:
  mappings:
  - match:
    - group_ids: [<app-Spacelift-group-ID>]
    add_labels:
      "teleport.internal/okta-group/<app-Spacelift-group-ID>": "true"
  - match:
    - app_ids: [<Spacelift-app-ID>]
    add_labels:
      "teleport.internal/okta-group/<app-Spacelift-group-ID>": "true"
```

This will ensure that:

a) corresponding `user_group` object in Teleport has the following label
   applied to it: `teleport.internal/okta-group/<group-id>`, and
b) each `app` belonging to this group has the same label applied to it:
  `teleport.internal/okta-group/<group-id>`.

When the app app is added to a group or moves to a different group, the Okta
reconciler service will update labels on imported Teleport resources accordingly.

Next, for each `subkind: okta_group` access list, Teleport will create a system
role that will allow access to Okta groups/apps by these internal labels.

For instance:

```yaml
kind: role
version: v7
metadata:
  name: access-list-group-<app-Spacelift-group-ID>
  labels:
    teleport.internal/resource-type: "system" # <-- we already have this label
  owner_references:
    ...
spec:
  allow:
    group_labels:
      "teleport.internal/okta-group/<app-Spacelift-group-ID>": "true"
    app_labels:
      "teleport.internal/okta-group/<app-Spacelift-group-ID>": "true"
```

This role will ensure that a user with this role will be allowed access to
imported Okta groups/apps due to the Okta import rules we set up above.

System roles (with `teleport.internal/resource-type: system` label) will not
be displayed to users in the web UI or CLI.

_Note: The original design proposal for this used a single `access-list-access`_
_role template requiring user to have an "app-Spacelift" trait, however this_
_templating approach doesn't work well when combined with short-term access_
_setup described below so we're creating a role per imported list instead._

Tying all pieces together, each imported access list will grant its members the
`access-list-<group-Name>` role granting access to its group/apps.

In the Spacelift example above, the `app-Spacelift` access list will grant role
`access-list-app-Spacelift` to its members which will make sure they have access
to Okta group and apps marked with `teleport.internal/okta-group/<app-Spacelift-group-ID>`
labels.

### Other access list properties

We will choose reasonable defaults for other properties. For example, set
review interval and first review deadline to 6 months. Users will be able to
adjust those on a per-list basis after the import if needed.

### Individual assignments

It is common for Okta administrators to assign applications to users directly,
rather than to a group and the "Okta group as access list" sync flow described
above doesn't take those into account.

In order to support such assignments, they will be imported as access lists
that grant access to individual applications, with users directly assigned to
those apps being members of respective access lists.

The implementation will be similar to group import: Teleport will add support
for an access list with subkind `okta_individual_assignment`:

```yaml
kind: access_list
subkind: okta_individual_assignment
metadata:
  name: <Spacelift-app-ID>
  labels:
    teleport.internal/display-name: Spacelift
spec:
  okta_app_id: "<Spacelift-app-ID>"
```

which will create and maintain roles granting access to imported Okta apps
with appropriate `teleport.internal/okta-app/<id>` label set:

```yaml
kind: role
version: v7
metadata:
  name: access-list-app-<app-Spacelift-group-ID>
  labels:
    teleport.internal/resource-type: "system"
spec:
  allow:
    app_labels:
      "teleport.internal/okta-app/<Spacelift-app-ID>": "true"
```

Such access lists (and their corresponding roles and import rules) will only
be created for apps that have assignments. Okta apps that don't have direct
assignments won't be imported as access lists.

Users directly assigned to an application in Okta will be added as members of
this access list. Similar to the group import flow, Teleport Discover flow will
allow users to specify owners of applications which have direct assignments, or
use the default ones.

### Cleanup

If an imported Okta group gets deleted, Teleport will delete the corresponding
access list. When deleting the `subkind: okta_group` access list, all system
objects created for this access list (roles, import rules) will be deleted as
well in a cascade fashion.

Similarly, if an individual assignment gets deleted from Okta, it will be
reflected in Teleport, like removing a member from corresponding list. If
the last direct app assignment is deleted in Okta, corresponding access list
will be removed as well.

## Short-term access

While for long-term access the main goal is to reflect Okta's configuration in
Teleport, for short-term access it's to provide users with ability to request
access to applications they don't have access to via Teleport in a self-serve
manner, and have application owners review such requests.

The short-term configuration will build upon the long-term setup explained
above.

### Requesting access

For the simplicity of the configuration, each user will be able to request
access to any imported Okta application belonging to any Okta group.

To support this, each imported user will be assigned the (already existing)
`requester` preset role. This is already the case with the SSO connector that
Teleport creates automatically when setting up Okta integration.

### Reviewing access

Owners of the imported access lists should be able to review access requests
for apps granted by those access lists.

At the moment, there is no implicit rule which allows access list owners to
review such access requests - owners must still be assigned roles with proper
`review_requests` settings in order to do so.

While introducing such an implicit behavior by default in all circumstances
would likely be undesirable, having to create and maintain an additional set
of roles to enable owners review requests creates more configuration for user
to write and maintain.

As such, in addition to automatically creating a role that allows access to Okta
group apps, each access list with `subkind: okta_group` will also maintain a role
that will allow owners to review requests to this list's resources.

Continuing the Spacelift example:

```yaml
kind: role
version: v7
metadata:
  name: access-list-reviewer-<app-Spacelift-group-ID>
  labels:
    teleport.internal/resource-type: "system"
  owner_references: ...
spec:
  allow:
    review_requests:
      roles:
      - access-list-<app-Spacelift-group-ID>
      preview_as_roles:
      - access-list-<app-Spacelift-group-ID>
```

This role will automatically be assigned to access list owners, giving them
ability to see and review requests for Spacelift application.

### App belonging to multiple groups

If an application belongs to multiple Okta groups, users are currently forced to
select a single group when opening an access request (for example, "Spacelift
reader" or "Spacelift admin").

Both group and app become a part of the access request so only owners of the
appropriate access list corresponding to this group can review and approve this
request.

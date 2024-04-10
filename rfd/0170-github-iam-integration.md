---
title: RFD 0170 - Github IAM integration
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# Required Approvals

- Engineering: @smallinsky
- Product: @klizhentas && @xinding33

# What

This RFD proposes design and implementation of Github integration that will
sync Github repositories, teams and team memberships into Teleport and allow
users to manage access to repositories via access lists and access requests.

# Why

Becoming the cental place for the Identity and Access Management for services
like Github, Okta, AWS and others is a part of the Teleport Identity product's
goals.

We would like to provide users with ability to manage Github memberships and
permissions using Teleport's existing tools such as access requests and access
lists and help them achieve compliance requirements easier by providing an audit
trail for all access changes and ability to perform regular access reviews.

A strong driver for this integration is our internal needs as well. At Teleport
we use Github for all product development and current method of managing access
based on [IaC](https://github.com/gravitational/github-terraform) is tedious,
slow, and non-automatable.

Moving this functionality into Teleport Identity product and removing reliance
on terraform scripts and data files for our Github access control needs is one
of the success criterias for this integration. Specifically, removing csv files
with group memberships like [team-access](https://github.com/gravitational/github-terraform/tree/2faa88613ae9bfef177e356ca22e6e5f6615db80/gravitational/team-access)
and [individual-access](https://github.com/gravitational/github-terraform/tree/2faa88613ae9bfef177e356ca22e6e5f6615db80/gravitational/individual-access)
and managing memberships via Teleport.

# Very high-level overview

Let's first take a birds-eye overview of how the integration will function. The
general idea is similar to [Okta's access lists sync](https://github.com/gravitational/teleport.e/blob/master/rfd/0019e-okta-access-lists-sync.md):

- Administrator sets up Github integration to connect to an organization.
- Administrator configures the integration to sync specific or all Github teams.
- Teleport syncs organization repositories as internal `repo` resources.
- Teleport syncs Github teams as access lists.
- Teleport syncs team members to their respective access lists by matching their
  public email to Teleport username.
- When a user is added to or removed from a Github-synced access list, Teleport
  updates membership appropriately using Github API.

# User scenarios and lifecycle flows

Before diving into implementation details let's review primary use-cases which
this integration is aiming to support (or not support).

We will assume Okta as an identity provider since we're using it internally,
and will use these flows as a primer of how this integration will be deployed
for our own use.

## I want onboarded users to automaticaly get access to appropriate repositories

To simplify onboarding, we want a user to automatically get appropriate access
to Github when they join the team. Assuming Okta access list sync is enabled,
they will get automaticaly added to an Okta-synced access list representing
their team. The Okta-synced list can be made a member of a corresponding
Github-synced list which will grant all its members access to Github.

* core-team-github      // Github-synced access list
  * core-team-okta      // Okta-synced access list
    * alice@example.com // newly onboarded user
    * bob@example.com   // existing user

In future, this can be implemented more elegantly once we re-introduce
[dynamic access lists](https://github.com/gravitational/teleport.e/issues/2196)
which will not require explicitly making an Okta-synced list a member of a
Github-synced list.

## I want offboarded users to automatically lose access to Github

Given similar access lists structure as shown above, during offboarding a user
gets deactivated in Okta, which in turn locks them in Teleport making them a
non-eligible member of the access list and making Teleport reconciler to revoke
their Github assignments.

## I want to manage Github team memberships for my organization using Teleport

With the integration setup, Teleport imports all Github repositories and teams
as access lists and makes appropriate changes to Github organization when users
get moved in and out of respective access lists.

## I want to allow users to request access to repositories in self-serve manner

Teleport end users can follow Teleport's regular access requests flow to get
either temporary access to repositories by getting their request approved, or
long-term access by being added or promoted to an access list. In both cases,
Teleport makes and keeps track of appropriate assignments within Github org.

## I want to give individual users (e.g. external contractors) repository permissions

We want to avoid having to introduce a special-case flow for external users. The
recommendation is to create a Github team for contractors and assign permissions
that way.

## I want to see which users of my organization have which repo permissions

Github integration works with Access Graph that can display Github repositories
and access paths from Teleport users to them based on their assigned roles and
Github-synced access list memberships.

# What's out of scope

Providing Github connectivity/proxying is out of scope of this RFD. There is a
separate effort for implementing a git protocol proxy in Teleport with audit
log support which we'll link here once it's come to fruition.

# Edition and licensing requirements

Github IAM integration will be available only in Teleport Enterprise as both
access requests and access lists are Enterprise specific features.

The integration is a part of Identity product and will require an appropriate
license flag in order to be unlocked.

# Details

## Integration setup and relevant resources

When setting up the integration, administrators can optionally configure the
filter for team names they'd like synced and managed access to via Teleport
access lists:

```yaml
kind: integration
version: v1
metadata:
  name: github
spec:
  type: github
  github:
    organization: gravitational
    default_owners:
    - alice@goteleport.com
    sync:
      teams:
      - * # wildcard to sync all teams
      - employees # sync specific top-level team and all its sub-teams
      - employees/dev-team/core-team # sync specific sub-team and its sub-teams
```

Github teams can be nested so users can define specific sub-teams using `/`
separator as in the example above. The filter defaults to a wildcard.

Once the integration has been setup, Teleport begins sync of all organization's
repositories as `repo` resources. Each imported repository gets automatically
labeled with several default labels such as org and repo name:

```yaml
kind: repo
version: v1
metadata:
  name: teleport
  labels:
    teleport.dev/origin: github
    github/organization: gravitational
    github/repo: teleport
spec:
  type: github
  organization: gravitational
```

In addition to the default labels, users can optionally configure repo import
rules to apply additional labels to specific sets of repos:

```yaml
kind: repo_import_rule
version: v1
metadata:
  name: dev-team-rule
spec:
  priority: 10
  mappings:
  - match:
      source: github
      repo_names: ["teleport", "teleport.e", "cloud"]
      add_labels:
        team: dev
```

The `role` resource is extended to support granting access to repositories and
roles. In Github's model, each team can have different sets of permissions for
different repos so during sync Teleport creates a role policy for each (team, repo)
pair to represent a particular access level to a specific imported repository.
For example, Core team's write access to the "teleport" repo:

```yaml
kind: role
version: v7
metadata:
  name: core-team-teleport-repo-write
spec:
  allow:
    repo_labels:
      "github/organization": "gravitational"
      "github/repo": "teleport"
    repo_roles:
    - push # one of Github roles: pull, triage, push, maintain, admin, or a custom one
```

Roles created and maintained by the integration are considered system roles and
can't be modified or assigned to users directly.

For each imported matching team, Teleport creates a corresponding access list
of `github` type that grants roles that correspond to that team's permissions
within Github. For example, an access list for the Core team:

```yaml
kind: access_list
version: v1
metadata:
  name: core-team
spec:
  type: github # indicates this access list is synced with Github
  parent: dev-team
  grants:
    roles:
    - core-team-teleport-repo-write
    - core-team-teleport-e-repo-write
    - ...
```

Since Github teams can be nested, synced access lists will rely on ability for
access lists to include other [access lists as members](https://github.com/gravitational/teleport/pull/40771).

### Team members vs maintainers

In Github, a user can be either a `member` or a `maintainer` of a team.

Every Github team member is mapped to a Teleport user using Github user's
primary email address and made the member of the corresponding access list.
Only verified emails can be set as primary on Github.

Github team maintaners, in turn, become access list owners. If a team does not
have a maintainer, the integration will use default list of owners from the
integration configuration.

## Sync

As explained above, Teleport syncs all repositories as `repo` resources, teams
and their memberships as access lists, and teams' repo access policies as roles.

### Github->Teleport membership sync

Changes made to teams within Github are reflected in Teleport access lists
during periodic reconciliation. Github does not implement SCIM client so Teleport
uses periodic polling and API to reconcile access lists with Github teams.

### Teleport->Github membership sync

Changes made to Teleport's Github-synced access lists are propagated back to
Github. As such, if a member of a Github-synced access lists is removed from the
list, they're removed from the corresponding Github team. When a user is added
to such a list, they're added to the corresponding Github team. To do that,
Teleport fetches the organization's user list and find the proper Github user
by Teleport's user email.

Similarly, whenever a Github-synced access list owner list is updated, a
corresponding user either gets or loses their team `maintainer` status.

### Github->Teleport permission sync

Changes in teams' access on Github side are reflected in Teleport access lists
and roles as a part of the same regular reconciliation that manages membership.
In an event a particular team's permission for a repository changes, say, from
"read" to "write", Teleport updates the roles granted by that team's access
list accordingly (create a new role and remove the old one).

### Teleport->Github permission sync

Within Teleport, access to Github repositories can be granted in two ways.

#### Role assignments

By assigning user a role with appropriate `repo_labels` and `repo_roles`,
Teleport matches the role set against its synced Github repos and performs
individual collaborator assignments using Github API. Similar to Okta integration,
this happens as a part of the login hook during user login and in the reconciler
that's watching for user changes.

The same behavior occurs when a user submits an access request and gets approved
for a role that includes access to Github repos. Again, similar to Okta, Teleport
keeps track of Github repo assignments and cleans those up once access expires
leading to the removal of collaborator's permissions from Github.

#### Access list grants and memberships

Adding a user to a Github-synced access list makes the reconciler add the
corresponding Github user to the appropriate Github team. In effect, this is
the same as Teleport->Github membership sync explained above - a new member
gets their permissions through a team membership rather than invididual
collaborator assignment.

Owners of Github-synced lists become Github team maintainers.

Access lists synced from Github do not permit changing their role grants or their
generated roles. As such, for Github-imported lists, Github remains the source
of truth for permissions.

*Future work:* As an extension of this feature, we can provide a way for admins
to create Github-synced access lists within Teleport, which Teleport assumes
full ownership of - such lists will result in a creation of a team in Github
with the permissions determined by the lists' role grants (which will be
maintained up-to-date and updated in Github accordingly if those grants change
in Teleport).

## CLI

Both `tsh` and `tctl` will support listing Github repositories like any other
Teleport resources:

```bash
$ tsh repo ls
$ tctl get repo
```

In addition to listing, `tctl` will support removing `repo` resources which may
come in handy during troubleshooting and cleanup via `tctl rm repo/<name>`.

It will not support creating `repo` resources.

## Web UI

In addition to the Github integration setup flow itself, which will be a part
of the Access Management UI like other similar flows, the feature will be
integrated into other parts of the web UI as well.

One is, the unified resource view will be updated to support displaying Github
repositories. For each repository, Teleport will display a Github role a user
has in that repository (e.g. pull, push, maintain, etc).

The access requests flow will be updated to support displaying of Github
repositories as requestable resources. For each repository, Teleport will
calculate what Github roles can be requested based on user's requestable
Teleport roles and access lists present in the cluster.

Finally, Github-synced access lists will be visually distinguishable from the
lists synced from Okta or created directly in Teleport by having an appropriate
icon on them.

## Interaction with Access Graph

Github integration with Access Graph will function similarly to the existing
Gitlab integration, with the difference that Github repositories will actually
be synced as native Teleport resources as described above before being pushed
to the Graph API.

The Access Graph UI will support displaying imported Github repositories and
their access paths. Since Github teams are represented in Teleport as access
lists, the graph will support them out of the box.

## Interaction with scoped RBAC

[RFD 164](https://github.com/gravitational/teleport/pull/38078) introduced the
scoped RBAC.

When Github integration is enabled, Teleport automatically creates a top-level
parent `github` resource group and child resource group for each corresponding
Github organization that all synced repositories are placed into by default:

```yaml
kind: resource_group
metadata:
  name: github
---
kind: resource_group
metadata:
  name: gravitational
spec:
  parent: github
  match_labels:
    github/organization: gravitational
```

For access lists and roles synced *from Github*, the scope is technically
redundant as they would limit access to specific repositories by labels but
it is still set for consistency (and isn't user-modifyable):

```yaml
kind: access_list
version: v1
metadata:
  name: core-team
spec:
  type: github
  scopes: ["/github/gravitational"]
  grants:
    ...
```

For Github-synced lists originating *from Teleport*, users can define scopes
to limit a subset of repositories a particular access list grants access to.

For example, imagine a user has a role that grants push access to all repos of
an organization:

```yaml
kind: role
version: v7
metadata:
  name: github-access
spec:
  allow:
    repo_labels:
      github/organization: gravitational
    repo_roles:
    - push
```

They could use scopes to create access lists for 2 teams to grant them access
to different sets of repositories belonging to different resource groups using
the same role grant:

```yaml
kind: access_list
version: v1
metadata:
  name: core-team
spec:
  type: github
  scopes: ["/github/gravitational/core-team"]
  grants:
    roles:
    - github-access
---
kind: access_list
version: v1
metadata:
  name: cloud-team
spec:
  type: github
  scopes: ["/github/gravitational/cloud-team"]
  grants:
    roles:
    - github-access
```

This work is out of the initial scope and will follow up the implementation of
the main scoped RBAC RFD.

# Audit events

The integration will emit the following audit events:

- When integration itself is created, modified or deleted.
- When integration moves user added to an access list into a Github team.
- When integration moves user removed from an access list out of a Github team.
- When integration creates individual assignment based on approved access request.
- When individual assignment is removed based on expired access request.

Specific event codes and names will be determined during the implementation.

# Product metrics

To help us track the integration usage we will implement the following product
metrics:

- The integration setup funnel showing how many users started enrolling the
  integration and succesfully completed or fell off at a certain stage, similar
  to existing Discover flows.
- Periodic reporting of the number of Github-synced access lists per tenante
  and the number of members in them to be able to see how the overall usage of
  the feature grows over time.
- Actions taken by the integration (adding/removing users from teams, creating/
  removing assignments) to be able to see usage patterns.
- Cohort retention table based on the synced lists.

# IaC

IaC users should be able to fully configure the integration without having to
use the web UI. To enable this, Terraform provider and Kubernetes operator will
receive support for:

- Github integration resource.
- `repo_import_rule` resource.
- New fields in existing resources like roles and access lists.

The `repo` resource also introduced in this RFD is not meant to be created by
users and as such will not be implemented in IaC.

# Implementation

## Github API vs SCIM

Github Enterprise implements SCIM server in beta: https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-saml-single-sign-on-for-your-organization/about-scim-for-organizations#supported-identity-providers.

However, Teleport may not necessarily be an IdP for Github (and we don't want
to force it) and SCIM is a fairly limited protocol so this Github integration
will use regular Github REST API.

## Authentication

The integration will use Github App and basic authentication flow. The token
will be kept similar to other hosted plugins and integrations like Okta.

https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api?apiVersion=2022-11-28#authenticating-with-a-token-generated-by-an-app

## Rate limits

Github API rate limits should be sufficient for the integration's normal
operation.

> You can use a personal access token to make API requests. Additionally, you
> can authorize a GitHub App or OAuth app, which can then make API requests on
> your behalf.

> All of these requests count towards your personal rate limit of 5,000 requests
> per hour. Requests made on your behalf by a GitHub App that is owned by a
> GitHub Enterprise Cloud organization have a higher rate limit of 15,000
> requests per hour.

https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api?apiVersion=2022-11-28#primary-rate-limit-for-authenticated-users

## API reference

To implement the reconciler functionality, the integration will use the following
Github APIs.

### `List organization repositories`

https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#list-organization-repositories

The reconciler calls this periodically to fetch the list of repos for the
integration's configured organization and reconcile them against the cluster's
`repo` resources.

### `List teams`

https://docs.github.com/en/rest/teams/teams?apiVersion=2022-11-28#list-teams

The reconciler calls this periodically to fetch the list of teams for the
integration's configured organization and reconcile them against the cluster's
Github-synced access lists.

Parent teams become parent access lists in Teleport.

### `List team members`

https://docs.github.com/en/rest/teams/members?apiVersion=2022-11-28#list-team-members

The integration uses this to fetch members of a particular team and make them
members of the corresponding Teleport access list.

Members of child teams are returned by this API as well so the reconciler needs
to start with the "innermost" team and work its way to the top parent to see
how to distribute members across access lists.

### `Add or update team membership for a user` and `Remove team membership for a user`

https://docs.github.com/en/rest/teams/members?apiVersion=2022-11-28#add-or-update-team-membership-for-a-user
https://docs.github.com/en/rest/teams/members?apiVersion=2022-11-28#remove-team-membership-for-a-user

The integration uses this to add a member to or remove them from a team when
they get added to or removed from an access list.

### `Get a user`

https://docs.github.com/en/rest/users/users?apiVersion=2022-11-28#get-a-user

The integration uses this to determine Github user's primary email to be able
to map Github user by their username to Teleport user by email.

### `List team repositories`

https://docs.github.com/en/rest/teams/teams?apiVersion=2022-11-28#list-team-repositories

The integration uses this to see which repositories and permissions a particular
team has access to and generate appropriate role definitions that will be granted
to members of the corresponding access list in Teleport.

### `Add a repository collaborator` and `Remove a repository collaborator`

https://docs.github.com/en/rest/collaborators/collaborators?apiVersion=2022-11-28#add-a-repository-collaborator
https://docs.github.com/en/rest/collaborators/collaborators?apiVersion=2022-11-28#remove-a-repository-collaborator

The integration uses this to assign members to individual repositories when
they submit access requests.

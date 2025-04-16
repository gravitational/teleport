---
authors: Guillaume Charmes (guillaume.charmes@goteleport.com)
state: draft
---

# RFD 0194 - Kubernetes RBAC Simplification

## Required Approvers

- Engineering: @rosstimothy && @hugoShaka && @tigrato

## What

The purpose of this RFD is to propose a simplification of the Kubernetes
RBAC (Role-Based Access Control) within the Teleport project.

The goal is to streamline the user experience and reduce the complexity of
getting up and running on day 1 and managing permissions later on.

## Why

Today, the Kubernetes RBAC in Teleport is complex and can be difficult for
users to manage.
A proper setup requires some external Kubernetes config and matching role
setup.
This complexity leads to misconfigurations and security issues.

Simplifying the RBAC model will help improve usability and security.

### References

A common issue currently is the un-intuitive result of complex rule sets and
more critically, difficult to get setup on day 1.

- @klizhentas struggling to setup a cluster following the `self-hosted` flow:
  (internal) <https://gravitational.slack.com/archives/C03SULRAAG3/p1715394587181739>
- Customer confusion on role resolution:
  - <https://github.com/gravitational/teleport/issues/45475>
  - (internal) <https://goteleport.slack.com/archives/C0582MBSMHN/p1738187497666819>
- Unexpected `exec` grant on read-only user:
  (internal) <https://gravitational.slack.com/archives/C32M8FP1V/p1739462454321459?thread_ts=1739462384.714419&cid=C32M8FP1V>

## Goals

- Make the Teleport agent authoritative
  - Handle permissions for the users instead of relying on impersonating a
    different user/group
- Discourage (or even deprecate?) the use of the `kubernetes_groups` and
  `kubernetes_users` in favor of `kubernetes_resources`
- Clarify / Improve documentation for the various ways to setup Kubernetes in
  Teleport.

## Proposal

### Background

Currently, when enrolling a Kubernetes cluster and setting it up to use the
group `system:masters`, everything works as expected (except on GKE Autopilot
where this group is forbidden).
Teleport is super admin and yields reduced permissions to the user based on
the `kubernetes_resources` and `kubernetes_label` fields from the Teleport
role.

### Impersonation

To limit the number of code changes, the proposal is to default the
impersonation to use the Teleport service account.

We will need to make sure the service account has `cluster-admin` permissions.

To avoid unexpected access being granted, the `kubernetes_resources` field
should default to `null` as well instead of the current wildcard.

The `kubernetes_user` should default to the Teleport user auth email.

### UX

#### Deprecate `namespace`

We currently have the `namespace` kind mean "everything within a namespace",
which has caused confusion with customers and resulting in unexpected access
granted.

This should be deprecated and removed. It would require a bump on the role
model to v8 to avoid changing behavior for current customers.

Upgrading to v8 would map the deprecated `namespace` field to `*` while
swapping `name`/`namespace` as these are equivalent within a namespace:

```yaml
kind: namespace
name: foo
```

and:

```yaml
kind: '*'
namespace: foo
```

Note that the later also grants access to cluster-wide resources outside a
namespace.

To avoid confusion, this behavior would change and when a `namespace` value is
set (unless it is `*`), only namespaced resources would be matched.
To match cluster-wide resource, the namespace field should be omitted or set to
`*`.

#### CRD / Subresources support

Currently, we only support a hard-coded list of pre-set resources. To enable
CRD support, i.e. arbitrary kind, a new field `api` will be introduced.
When not set, the `kind` field would still be restricted to the existing
hard-coded list, but once set, it can be an arbitrary string.

Similarly, to avoid confusion around sub-resources and the common mistake
of granting `exec` permission with the `get` verb, a new field `sub_resources`
will be introduce. If not set, then only the main resource would match, which
removes the unexpected side-effect for `exec`.
To avoid confusion, when `sub_resources` are set, the `verbs` field will be
ignored.

The role would look like this:

```yaml
kubernetes_resources:
  - kind: '*'
    api: 'API'
    name: '*'
    namespace: '*'
    verbs: ['*']
    sub_resources: ['*']
```

NOTE: The API field is granular at the group level, encompassing all versions.

#### Web Role Editor

To avoid confusion, in the web role editor, we would remove the ability to set
and view `kubernetes_group` and `kubernetes_users`.

NOTE: The existing preset role `require-trusted-device` that contains
`kubernetes_group` will be removed, i.e. new install from Teleport v18 will not
create that role. Existing installs will not be impacted.

The `kubernetes_resources` section currently "hidden" behind a button should be
open by default.

The UI will hide the fields when the role is selected to >= 8 but will remain
as it is today to keep support for v7 roles.

#### Direct enrollment

When enrolling a cluster "directly" (i.e. by adding the `kubernetes_service`
section in the `teleport.yaml` config), the initial setup of roles/rolebindings
still needs to be performed calling the following script:
[get-kubeconfig.sh](../examples/k8s-auth/get-kubeconfig.sh).

This script will be updated to ensure the group for the Teleport SA exists and
has the `cluster-admin` permissions.

#### Helm enrollment

When using the Helm chart, nothing is expected to change, the chart sets up the
service account for the agent, as well as the role/binding for impersonation.

As _Helm_ is the favored way to get up and running, we can easily update the
service account to have the `cluster-admin` permissions.

#### Resource enrollment UI

On the Web UI, the initial page generates a _Helm_ commandline.

After enrollment, a test page is shown, prompting for a `kubernetes_group`
value, which defaults to the user's trait.
With the proposed changes, that page would be skipped, using the Teleport SA
instead.

#### Cluster discovery

Similar to the _Helm_ enrollment, the discovery service sets up the
role/rolebindings for impersonation. It will be responsible to make
sure the Teleport SA has the proper permissions.

### Backward/Forward compatibility

The idea is to keep the impersonation and remove the ability to set the
`kubernetes_group` and `kubernetes_users` field in favor of preset values
(Teleport agent SA and user email).
To do so, a new role version, `v8` will be introduced, which will indicate
the web ui not to show those fields.
When a `v8` role is found, if either the `kubernetes_group` or
`kubernetes_users` fields are set when a `v8` role is created, updated, or
upserted the request will be rejected.
This will allow existing setups will keep working, as `v7` behavior will not
change.

#### Upgrade UX

For existing customers upgrading to the new version, no change is expected in
behavior for existing roles. Creating/editing a role with `v8` will behave
differently than they are used to in `v7` though so displaying a warning for a
couple of major version with a link to the docs may be warranted to avoid
confusion.

### Documentation

We should move all mentions of `kubernetes_group` and `kubernetes_users` under
a dedicated "advanced" documentation page.
We may consider deprecating the fields and thus remove any links to that page.

As it wouldn't be relevant anymore, we should also remove all mention of
Kubernetes roles/rolebindings/clusterroles/clusterrolebindings from the main
documentation.

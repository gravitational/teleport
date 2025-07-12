---
authors: Guillaume Charmes (guillaume.charmes@goteleport.com)
state: draft
---

# RFD 0219 - Kubernetes RBAC Simplification

## Required Approvers

- Engineering: @rosstimothy && @hugoShaka && @tigrato
Product: klizhentas

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

- Discourage the use of `kubernetes_groups` and `kubernetes_users` fields from
role with custom values in favor of a preset cluster-admin one.
- Clarify / Improve documentation for the various ways to setup Kubernetes in
  Teleport.

## Proposal

### Background

Currently, when enrolling a Kubernetes cluster and setting it up to use the
group `system:masters`, everything works as expected (except on GKE Autopilot
where this group is forbidden).
Teleport is setup to impersonate to yield reduced permissions to the user
based on the `kubernetes_resources` and `kubernetes_label` fields from the
Teleport role.

### Prior work

While initially planned to be included in this RFD, due to customer requests,
the following has already been implemented and shipped:

- Support arbitrary resources, including CRDs #54426
- Simplify Namespace/Kind behavior #54938
- Handle all known resources on the Teleport side #55099

Given this, as RBAC has already been simplified quite a lot, a scoped down
version of the proposal would be to focus on new installs without impacting
existing customers.

### Authoritative RBAC Documentation

To simplify initial setup and long-term management, we'll update the
documentation to encourage users to use a cluster-wide admin group that will be
created by provisioning scripts or auto-discovery.
This will allow users to avoid the complexity of managing RBAC in multiple
places, avoid confusion around the `exec` subresource.
While we won't change or remove the `kubernetes_groups` and `kubernetes_users`
fields, we will discourage their use, allowing existing customers as well as
advanced users to continue leveraging the underlying Kubernetes RBAC for more
special use cases.

### Provisioning

To enable easy setup, provisioning methods will be updated to create an
cluster admin ClusterRole as well as a ClusterRoleBinding with a known group
name.

#### Helm Chart

The Helm Chart will provide the ability to specify custom names for all
resources, (clusterrole, clusterrolebinding) and for the Group.

Example:

```yaml
roles: kube,app,discovery
authToken: foo
proxyAddr: example.devteleport.com:443
kubeClusterName: myCluster
rbac:
  adminClusterRoleName: teleport-agent-cluster-admin
  adminClusterRoleBindingName: teleport-agent-cluster-admin
  adminGroupName: teleport-agent-cluster-admin
```

#### Provision Script

To adjust the values when provisioning with the provided script, environment
variables will be used to follow the existing pattern.

Example:

```bash
TELEPORT_KUBE_ADMIN_GROUP=teleport-cluster-admin ./get-kubeconfig.sh
```

#### Auto Discovery

##### EKS / AKS

For both EKS and AKS, auto-discovery will be updated to create the proper admin
roles/bindings.

##### GKE

For GKE, we will need to change the documented GCP IAM role to enable admin
privileges.

#### Auto-update

Auto-update will not attempt to apply the new model, with the assumption that
existing setups are already working.

### UX

#### Exec confusion

As the underlying Kubernetes RBAC gets discouraged, it will help with the
confusion around _exec_ being allowed with a `get` verb is gone.
Teleport uses a dedicated verb to control access to `exec`.

#### Resource enrollment UI

On the Web UI, the initial page generates a _Helm_ command line.

After enrollment, a test page is shown, prompting for a `kubernetes_groups`
value, which defaults to the user's trait.
This step will be updated to verify the underlying Kubernetes RBAC permissions
and notify the user if the impersonation doesn't have sufficient
permissions to be authoritative.

#### Role Editor

The Web UI Role Editor will move the `kubernetes_groups` and `kubernetes_users`
based on the role version dropdown to be less prominent (currently they are
the first thing shown). They will still be available under a "summary" toggle,
folded by default, to update as needed for advanced use cases and existing
customers.

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/web/packages/teleport/src/Roles/RoleEditor/StandardEditor/Resources.tsx#L291-L321

#### Error management

The error messages when using `kubectl` will be improved to include a link to
the documentation and more details on what is expected. This will help with
initial custom setups that skipped the provided provisioning scripts.

#### New preset role

To help onboarding, as currently no preset role grants Kubernetes access,
a new preset role will be added, `kube-editor`, which will have wildcard access
to Kubernetes resources as well as the predefined `kubernetes_groups` mapping
to the cluster admin group.

```yaml
kind: role
version: v8
metadata:
  name: kube-editor
spec:
  allow:
    kubernetes_groups:
      - teleport-cluster-admin
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
      - api_group: '*'
        kind: '*'
        namespace: '*'
        name: '*'
        verbs:
          - '*'
```

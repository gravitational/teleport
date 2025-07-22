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

## Goal

Provide preset roles that can be used to get started quickly.

## Proposal

### Prior work

While initially planned to be included in this RFD, due to customer requests,
the following has already been implemented and shipped:

- Support arbitrary resources, including CRDs #54426
- Simplify Namespace/Kind behavior #54938
- Handle all known resources on the Teleport side #55099

Given this, as RBAC has already been simplified quite a lot, a scoped down
version of the proposal would be to focus on new installs without impacting
existing customers.

### Preset roles

We will introduce the following new preset roles:

- `kube-access`: Maps to the Kubernetes preset `edit`. Can CRUD most basic
  builtin resources and read some limited administrative resources.
- `kube-editor`: Maps to the Kubernetes preset `cluster-admin`. Provides full
  access to all Kubernetes resources, including CRDs.
- `kube-auditor`: Maps the the Kubernetes preset `view`. Provides read-only
  access to some Kubernetes resources.

The Teleport roles will look like this:

```yaml
---
kind: role
version: v8
metadata:
  name: kube-access
spec:
  allow:
    kubernetes_groups:
      - 'teleport:preset:access'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
      - api_group: '*'
        kind: '*'
        namespace: '*'
        name: '*'
        verbs:
          - '*'
---
kind: role
version: v8
metadata:
  name: kube-editor
spec:
  allow:
    kubernetes_groups:
      - 'teleport:preset:editor'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
      - api_group: '*'
        kind: '*'
        namespace: '*'
        name: '*'
        verbs:
          - '*'
---
kind: role
version: v8
metadata:
  name: kube-auditor
spec:
  allow:
    kubernetes_groups:
      - 'teleport:preset:auditor'
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

### Provisioning

To enable easy setup, provisioning methods will be updated to create the
required new ClusterRoles and ClusterRoleBindings.

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
  accessClusterRoleBindingName: teleport:preset:access
  accessGroupName: teleport:preset:access
  editorClusterRoleBindingName: teleport:preset:editor
  editorGroupName: teleport:preset:editor
  auditorClusterRoleBindingName: teleport:preset:auditor
  auditorGroupName: teleport:preset:auditor
```

Note that if a user decides to change the preset names, they will not be able
to use the preset Teleport roles out of the box, as they will need to update
them to match the new names.

#### Provision Script

To keep it simple and to follow the current pattern, the provision script
will not allow custom names. It will create all the required resources with
names matching the Teleprot preset roles.

#### Auto Discovery

##### EKS / AKS

For both EKS and AKS, auto-discovery will be updated to create the expected
RBAC resources.

##### GKE

For GKE, we will need to change the documented GCP IAM role to enable admin
privileges.
Teleport will attempt to impersonate a privileged group, but there is a risk
GKE users will not be able to use the new preset roles out of the box.

A special attention to error management will be required to help users
understand the issue and how to resolve it, which can be easily done by
running the provision script for example.

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
value, which defaults to the user's trait and updates it if changed in this
page.

The following page is about testing the connection, and also have a form to
populate a subset of the users/groups.

The initial idea was to allow the user to update it's traits on the fly to not
have to logout/log back in, however, as there is no mention of traits, and
because the form is present on multiple pages with different behavior, and
because it requires the user to initially already have a role assigned that
contains the trait interpolation, we will remove both the 'Setup access' page
and the 'users/groups' form from the 'Test connection' page.

This will result in using the user's auth to test the connection instead of
requiring the user to enter a subset or all of it's groups/users, knowing
that the user cannot make add nor change any.

#### Role Editor

The Web UI Role Editor will move change the default `kubernetes_groups`
field to be pre-populated with the `teleport:preset:access` value instead of
`{{internal.kubernetes_groups}}`, which is the current default.

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/web/packages/teleport/src/Roles/RoleEditor/StandardEditor/Resources.tsx#L291-L321

#### Error management

The error messages when using `kubectl` will be improved to include a link to
the documentation and more details on what is expected. This will help with
initial custom setups that skipped the provided provisioning scripts.

### User flow

#### Current flow

- Day 0 - Initial setup:
  - User installs Teleport and sets up a Kubernetes cluster.
  - User provisions the Kubernetes cluster using a script, discovery, or Helm
    chart to create the impersonation ClusterRole/ClusterRoleBinding.
  - User configures Kubernetes RBAC with a custom ClusterRole with some
    permissions
  - User configures Kubernetes RBAC with a custom ClusterRoleBinding, which
    needs to be understood matches with the `kubernetes_groups` field in the
    Teleport role.
  - User configures Teleport role with `kubernetes_groups`, (unclear what
    `kubernetes_users` does from the docs) fields to match the custom
    ClusterRoleBinding.
  - User configures Teleport role with `kubernetes_resources` and
    `kubernetes_labels` to match to match or reduce the permissions granted by
    the custom ClusterRole.
- Day 1 - Ongoing management:
  - User needs to understand the interaction between the Teleport role and the
    underlying Kubernetes RBAC.
  - User may need to update the Teleport role if the underlying Kubernetes
    RBAC changes.
  - No clear guidance on how to which permission should be set where (Teleport
    or Kubernetes).
  - User may need to troubleshoot unexpected permissions or access issues.

#### Proposed flow

- Day 0 - Initial setup:
  - User installs Teleport and sets up a Kubernetes cluster.
  - User provisions the Kubernetes cluster using a script, discovery, or Helm
    chart to create all required RBAC resources.
  - User configures the Teleport role with `kubernetes_resources` and
    `kubernetes_labels` to match or reduce the permissions granted by the
    ClusterRole. The `kubernetes_groups` field is pre-populated to
    `teleport:preset:access` in the Web UI and well documented in the Teleport
    documentation for the YAML version.
- Day 1 - Ongoing management:
  - User can reduce the scope of either Kubernetes or Teleport's roles by
    changing the counterpart
  - User has the option to use the `teleport:preset:editor` role to avoid the
    complexity of managing the underlying Kubernetes RBAC, only managing
    resources/labels on the Teleport side instead.

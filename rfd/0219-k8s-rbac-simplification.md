---
authors: Guillaume Charmes (guillaume.charmes@goteleport.com)
state: draft
---

# RFD 0219 - Kubernetes RBAC Simplification

## Required Approvers

- Engineering: (@zmb3 || @rjones) && (@hugoShaka || @tigrato)
Product: @klizhentas

## What

The purpose of this RFD is to propose a simplification of the Kubernetes
RBAC (Role-Based Access Control) within the Teleport project.

The goal is to streamline the user experience and reduce the complexity of
getting up and running on day 0 and managing permissions later on.

## Why

Today, the Kubernetes RBAC in Teleport is complex and can be difficult for
users to manage.
A proper setup requires some external Kubernetes config and matching role
setup.
This complexity leads to misconfigurations and security issues.

Simplifying the RBAC model will help improve usability and security.

### References

A common issue currently is the un-intuitive result of complex rule sets and
more critically, difficult to get setup on day 0.

- @klizhentas struggling to setup a cluster following the `self-hosted` flow:
  (internal) <https://gravitational.slack.com/archives/C03SULRAAG3/p1715394587181739>
- Customer confusion on role resolution:
  - <https://github.com/gravitational/teleport/issues/45475>
  - (internal) <https://goteleport.slack.com/archives/C0582MBSMHN/p1738187497666819>
- Unexpected `exec` grant on read-only user:
  (internal) <https://gravitational.slack.com/archives/C32M8FP1V/p1739462454321459?thread_ts=1739462384.714419&cid=C32M8FP1V>

## Goal

Allow preset roles to be used to get started quickly.

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

#### Background

Originally, Teleport preset roles were inspired by Kubernetes roles,
specifically, `edit`, `cluster-admin` and `view`.

- `cluster-admin` allows everything everywhere.
- `edit` allows CRUD on most builtin resources, but not administrative
  resources.
  - get/list/watch pods (including attach, exec, portforward and proxy),
    secrets and services/proxy
  - impersonate service accounts
  - create/delete/deletecollection/patch/update pods (including exec/attach/
    portforward/proxy)
  - create pods/evictions
  - create/delete/deletecollection/patch/update configmaps, events,
    persistentvolumeclaims, replicationcontrollers (including scale), secrets,
    serviceaccounts, services (including proxy)
  - create serviceaccounts/token
  - create/delete/deletecollection/patch/update deployments (including
    rollback, scale), replicasets (including scale), statefulsets (including
    scale), ingresses, networkpolicies
  - create/delete/deletecollection/patch/update horizontalpodautoscalers, cronjobs,
    jobs, poddisruptionbudgets, leases
  - get/list/watch controllerrevisions, daemonsets, deployments, replicasets,
    statefulsets, horizontalpodautoscalers, cronjobs, jobs, poddisruptionbudgets,
    ingresses, networkpolicies, leases
- `view` allows read-only access to most builtin resources.
  - get/list/watch pods, configmaps, endpoints, persistentvolumeclaims,
    replicationcontrollers (including scale), serviceaccounts, services,
    bindings, events, limitranges, namespaces, resourcequotas, endpointslices,
    controllerrevisions, daemonset, deployments, replicasets, statefulsets,
    horizontalpodautoscalers, cronjobs, jobs, poddisruptionbudgets, ingresses,
    networkpolicies, leases

This resulted in the following Teleport roles:

- `editor`, which maps to `cluster-admin` and can do everything.
- `access`, which maps to `edit` and can do most CRUD operations.
- `auditor`, which maps to `view` and can do read-only operations.

#### New Kubernetes Bindings

In order to be consistent and allow for seamless use of Kubernetes within
Teleport, we will introduce the following new preset bindings:

- `kubernetes_groups` -> `ClusterRole`
- `teleport:preset:edit` -> `edit`
  Can CRUD most basic builtin resources and read some limited administrative resources.
- `teleport:preset:cluster-admin` -> `cluster-admin`
  Provides full access to all Kubernetes resources, including CRDs.
- `teleport:preset:view` -> `view`
  Provides read-only access to some Kubernetes resources.

On the Kubernetes side, we will create the corresponding `ClusterRoleBindings` based on `kubernetes_groups`.

While there is an existing binding for `cluster-admin` via `system:masters`, it
is not available everywhere and there are no binding for `edit` nor `view`.
For consistency and reliability, we will create one binding for each preset
role.

Those bindings will be available to Teleport users via the user trait feature.

Example:

```bash
tctl users add hugo --logins root --kubernetes-group teleport:preset:edit --roles=access
```

### Provisioning

To enable easy setup, provisioning methods will be updated to create the
required new ClusterRoleBindings.

#### Helm Chart

The Helm Chart will by default provision the new Kubernetes resources starting
in v19.0.0. An option will be added to the chart to allow opting out of creating
the default ClusterRoleBindings. All existing release branches will have this
flag set to disable the new functionality to minimize breaking changes.


Example:

```yaml
roles: kube,app,discovery
authToken: foo
proxyAddr: example.devteleport.com:443
kubeClusterName: myCluster
rbac:
  defaultClusterRoleBindings: true
```

The ClusterRoleBindings created by the chart will be prefixed with the
chart release and the namespace to allow multiple deployments in the
same Kubernetes cluster not to conflict since ClusterRoleBinding resources
are global and not namespaced.

```yaml

{{- if .Values.rbac.defaultClusterRoleBindings -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:metadata:
  name: {{ .Release.Name }}-{{ .Release.Namespace }}-preset-edit
```


#### Provision Script

For simplicity, the provision script will be updated to always create the
Kubernetes resources, and will not provide a means for users to opt out of
the new behavior.

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
    ClusterRole.
  - User applies the desired `kubernetes_groups` trait to the Teleport user.
- Day 1 - Ongoing management:
  - User can reduce the scope of either Kubernetes or Teleport's roles by
    changing the counterpart
  - User has the option to use the `teleport:preset:editor` role to avoid the
    complexity of managing the underlying Kubernetes RBAC, only managing
    resources/labels on the Teleport side instead.

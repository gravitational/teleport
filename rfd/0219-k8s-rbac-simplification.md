---
authors: Guillaume Charmes (guillaume.charmes@goteleport.com)
state: draft
---

# RFD 0219 - Kubernetes RBAC Simplification

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
- Remove the `kubernetes_groups` and `kubernetes_users` fields from role
  - This will require a role model version bump to v9
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

### Being authoritative

#### New Kubernetes Cluster Role / Binding

To make Teleport authoritative, we need a dedicated Cluster Role / Binding with
admin privileges.

We'll need to update the Helm chart, the `get-kubeconfig.sh` provisioning
script as well as the discovery service for ASK, EKS, and GCP.

The new Cluster Role will be named `teleport-cluster-admin` and will have the
same definition as `cluster-admin`. We don't rely on `cluster-admin` being
present as the user may have renamed/removed it.

We then will create a Cluster Role Binding to a group also named
`teleport-cluster-admin`, which will allow us to achieve the same behavior as
if we were impersonating `system:masters`.

Here is how it would be handled within the `get-kubeconfig.sh` script:

```diff
diff --git a/examples/k8s-auth/get-kubeconfig.sh b/examples/k8s-auth/get-kubeconfig.sh
index 77adaae2ea..66b7135194 100755
--- a/examples/k8s-auth/get-kubeconfig.sh
+++ b/examples/k8s-auth/get-kubeconfig.sh
@@ -100,6 +100,35 @@ subjects:
 - kind: ServiceAccount
   name: ${TELEPORT_SA}
   namespace: ${NAMESPACE}
+---
+apiVersion: rbac.authorization.k8s.io/v1
+kind: ClusterRole
+metadata:
+  name: teleport-cluster-admin
+rules:
+- apiGroups:
+  - '*'
+  resources:
+  - '*'
+  verbs:
+  - '*'
+- nonResourceURLs:
+  - '*'
+  verbs:
+  - '*'
+---
+apiVersion: rbac.authorization.k8s.io/v1
+kind: ClusterRoleBinding
+metadata:
+  name: teleport-cluster-admin
+subjects:
+- kind: Group
+  name: teleport-cluster-admin
+  apiGroup: rbac.authorization.k8s.io
+roleRef:
+  kind: ClusterRole
+  name: teleport-cluster-admin
+  apiGroup: rbac.authorization.k8s.io
 EOF

 # Checks if secret entry was defined for Service account. If defined it means that Kubernetes server has a
```

The Helm chart will be updated to use the new role name (not keeping the
existing one as it doesn't mention 'teleport').

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/examples/chart/teleport-kube-agent/templates/admin_clusterrolebinding.yaml

#### Unknown resources

To be fully authoritative, we'll need to update the code to deny access to
unknown resources instead of falling back to the underlying Kubernetes cluster.

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/lib/kube/proxy/self_subject_reviews.go#L125-L130
- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/lib/kube/proxy/url.go#L234-L239

To maintain a good user experience, we'll need to implement watchers to monitor
CRD activity and register any new ones 'live' (as opposed to currently waiting
5 minutes).
This should result in new resources being registered under 500ms, which would
make it transparent to regular users, while automation tools like Terraform
will automatically retry the failed request and result in the second attempt to
be successful (or not, based, based on the role).

### Impersonation

The existing impersonation logic will be leveraged. Older role versions will
still impersonate based on the role-defined user/group, but the new role
version will use the predefined `teleport-cluster-admin` value.

The main logic change would look something like

```diff
diff --git a/lib/services/role.go b/lib/services/role.go
index b2aa02fcb4..db4b2050b7 100644
--- a/lib/services/role.go
+++ b/lib/services/role.go
@@ -1453,11 +1453,17 @@ func (set RoleSet) CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool,
                maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
                if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
                        matchedTTL = true
-                       for _, group := range role.GetKubeGroups(types.Allow) {
-                               groups[group] = struct{}{}
-                       }
-                       for _, user := range role.GetKubeUsers(types.Allow) {
-                               users[user] = struct{}{}
+
+                       switch role.GetVersion() {
+                       case types.V3, types.V4, types.V5, types.V6, types.V7:
+                               for _, group := range role.GetKubeGroups(types.Allow) {
+                                       groups[group] = struct{}{}
+                               }
+                               for _, user := range role.GetKubeUsers(types.Allow) {
+                                       users[user] = struct{}{}
+                               }
+                       default:
+                               groups["teleport-cluster-admin"] = struct{}{}
                        }
                }
        }
```

### New Teleport Role Model

The new role model version will be the same as V8 with `kubernetes_groups` and
`kubernetes_users` fields deprecated. While we can't remove them from the model,
if they are set, the validation will reject the role.
If a role with version 9 attempts to set the `kubernetes_users` or
`kubernetes_groups` fields, or if an older role with those values set attempt
to update the version to 9, it will be rejected.

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/api/types/role.go#L1979

#### Downgrade

When working with older agents not supporting the role version, we can't
downgrade the role, so we'll inject a wildcard deny to make sure there is no
unexpected access being granted.

This is handled as part of the grpcserver auth:

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/lib/auth/grpcserver.go#L1950

### Configuration

To accommodate a scenario where a customer already has a Group with the preset
name, we need to add a new field in the kube_service configuration file to
define which admin group will be impersonated.

This can also be used for customer upgrading from older versions that don't
want to provision a new clusterrole/clusterrolebinding. The config value could
be set to `cluster-admin` or `system:masters` to remove the need to provision
new resources.

#### Helm

The Helm Chart will provide the ability to specify custom names for all
resources, (clusterrole, clusterrolebinding) and for the Group.

### UX

#### Exec confusion

As the Kubernetes RBAC doesn't get used anymore, the confusion around _exec_
being allowed with a `get` verb is gone. Teleport uses a dedicated verb to
control access to `_exec`.

#### Resource enrollment UI

On the Web UI, the initial page generates a _Helm_ command line.

After enrollment, a test page is shown, prompting for a `kubernetes_groups`
value, which defaults to the user's trait.
With the proposed changes, that page would be skipped, using Teleport Cluster
Admin instead.

#### Role Editor

The Web UI Role Editor will hide the `kubernetes_groups` and `kubernetes_users`
based on the role version dropdown.

- https://github.com/gravitational/teleport/blob/22eb8c6645909a26d1493d01d291e222a87b35e6/web/packages/teleport/src/Roles/RoleEditor/StandardEditor/Resources.tsx#L291-L321

### Documentation

As we still support older role versions, the documentation should specify the
role version and the impersonation behavior each time it gets mentioned.

The install/setup instructions will be updated with only the latest role
behavior. Older ways to setup will remain thanks to our versioned
documentation.

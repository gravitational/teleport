---
authors: Anton Miniailo (anton@goteleport.com)
state: draft
---

# RFD 174 - Kubernetes RBAC Bootstrap

## Required Approvers

- Engineering: `@rosstimothy` && (`@tigrato` || `@hugoShaka`)
- Product: `@klizhentas` || `@xinding33`

## What

Improve RBAC setup capabilities for Kubernetes and make it easier to use with Teleport by
adding the ability to define Kubernetes RBAC completely in Teleport and adding the ability to define RBAC setup to be
automatically provisioned to Kubernetes clusters.

## Why

Currently, users have to rely on third-party tools or manual actions
when making sure their Kubernetes clusters have the correct roles/role bindings and that those are in sync with the RBAC setup defined in Teleport.
The changes we propose will make it easier for clients to start using Teleport for accessing Kubernetes clusters.

## Scope

This RFD is focused only on Kubernetes resources related to RBAC functionality, other types of resources are out of scope
and we will not allow them to be provisioned. Although other resources might be added in the future.

## Details

### Improving Day 1 experience - Adding capability to manage Kubernetes RBAC from Teleport. 

Currently users rely on `kubernetes_groups` and/or `kubernetes_users` fields of the role definition when setting up
access for Kubernetes. This requires roles/bindings to be created by the client in Kubernetes cluster independently from the 
RBAC setup in the Teleport. These roles/bindings also should be kept in sync with the Teleport RBAC side,
which creates additional hurdles for clients, especially the new ones, when using Kubernetes access. 
To improve this situation we will introduce auto-provisioning of the Kubernetes RBAC based on 
the Teleport roles, which means users will be able to define required permissions completely in the Teleport, and under the hood
we will keep them in sync transparently for the users.

We will add a new field to the role definition - `kubernetes_permissions`. That field will contain desired Kubernetes RBAC permissions
for that role and will be similar in structure to the native Kubernetes RBAC:

```yaml
kind: role
version: v7
metadata:
  name: kube-role
spec:
  allow:
    kubernetes_labels:
      "*": "*"
    kubernetes_groups: ["kube_group"]
    kubernetes_permissions:
      namespaces: ["namespace1", "namespace2"]
      rules:
      - resources: ["pod", "pod/exec"]
        verbs: ["get", "list", "create"]
      - resources: ["deployments"]
        apiGroups: ["app"]
        verbs: ["*"]
      - resources: ["secrets"]
        apiGroups: [""]
        resourceNames: ["secret1", "secret2"]
        verbs: ["get", "list""]
```

Presence of this field will signal that user wants Teleport to auto-provision these permissions into the Kubernetes clusters.
Teleport will translate permissions defined in that field into Kubernetes RBAC definitions and will automatically create roles and bindings
on the target Kubernetes clusters (as specified by the labels). If `namespaces` field includes '*' Teleport will create ClusterRole/ClusterRoleBinding
pair on the Kubernetes cluster, otherwise Role/Binding pair will be created in the each listed namespace. 

Subject for the bindings will be a group named `teleport:*RoleName*`, e.g. for a Teleport role `kube-role` the resulting 
group name will be `teleport:kube-role`. This will allow Kube service to setup correct impersonation group headers based on the roles available to the user.

If `kubernetes_permissions` field is set in the `allow` part of the Teleport role, fields `kubernetes_resources`, `kubernetes_users` and `kubernetes_groups` 
should not be set, we will return an error if user will try to create a role mixing those fields. In the future we might relax this rule.

The field `kubernetes_permissions` will be forbidden from appearing in the `deny` part of the role definition since we are trying to keep its meaning close to
native Kubernetes RBAC, which can only allow access. Although the field `kubernetes_resource` could appear in the `deny` part of the role, 
even if `kubernetes_permissions` is set in the `allow` part, giving user more flexibility using Teleport-level permissions check.

Pattern matching for resource names that is supported by the `kubernetes_resources` field will not be supported (same as in native Kubernetes RBAC)
by the `kubernetes_permissions` in the
first implementation, but we will be able to add it later. Not allowing pattern matching, though limiting functionality compared to the `kubernetes_resources` field,
will allow us to translate permissions defined in Teleport 1-to-1 into Kubernetes RBAC definitions. Since this feature is targeted primarily for the new users of 
Teleport and/or users with less complicated RBAC setups, this should not be a critical issue. If we add pattern matching later, it will need to rely on 
additional checks at the Teleport level - the way it currently works for the `kubernetes_resources`.

Kubernetes RBAC will be auto-provisioned using the KubeProvisions functionality described in the next section of this RFD. For the best UX any change to
a role that contains `kubernetes_permissions` will trigger auto-provisioning immediately to reduce latency between the moments when role was created 
or changed and when clients can use Kubernetes cluster with the updated RBAC.

Introducing `kubernetes_permissions` field will make enrolling into Teleport and starting using Kubernetes clusters
a much simpler process, since user will be able to setup RBAC completely in Teleport if they want. We will also be able
to amend our documentation to make our guides simpler, which will also help users to start with Teleport more easily. 

#### UX

Let's look at an example user story. Alice is a system administrator and she explores Teleport capabilities.
She doesn't know much about Teleport yet, except for basic foundation (agents, roles). She just enrolled her first Kubernetes cluster into Teleport.
This cluster contains the staging environment and she wants to give read-only pod access to the developers in an app namespace.
She goes to the UI and creates a new role, putting this content into it:

```yaml
kind: role
metadata:
  name: staging-kube-access
version: v7
spec:
  allow:
    kubernetes_labels:
      'env': 'staging'
    kubernetes_permissions:
      - namespaces: ["main-company-app"]
      - resources: ["pod"]
        apiGroups: [""]
        verbs: ["get", "list", "watch"]
  deny: {}
```

After that Alice assigns this role to the developers she wants to try out and at this point she is done. Alice will not need to create a 
separate role binding in Kubernetes for it to work - once the role is created Teleport will automatically create required Role/Binding in the
staging Kubernetes cluster.

#### Upgrade path

Existing users will not see any changes and don't need to migrate anything. To use the new feature clients will need to explicitly 
set the new `kubernetes_permissions` field in the role definition.

### Improving Day 2 experience - Automatically provisioning RBAC resources to Kubernetes clusters.

We will add a new type of resources, called "KubeProvision", where user will be able to specify Kubernetes RBAC resources they want to
be provisioned onto their Kubernetes clusters. This can be helpful for users with more complicated RBAC setup and who want to manage
Teleport roles linking to Kubernetes RBAC more easily, without relying on the third-party tools.

#### UX

Bob is an administrator of Teleport resources and access. He works in a large company that has hundreds of engineers/support staff etc and also
has multiple Kubernetes clusters which those users need to access. The Kubernetes clusters might be dynamically created or destroyed, RBAC access patterns
can change etc, so Bob wants all the help to make it easier to manage. If some access patterns change and Bob amends Kubernetes groups used in the
Teleport roles he needs to make sure Kubernetes clusters RBAC is amended correctly as well. Right now Kubernetes clusters themselves are managed by different people, so
syncing RBAC resources state often takes a long time and increases the possibility of security incidents. With the new KubeProvision resource Bob can centralize control
of the access in Teleport, without depending on another team. Bob can export RBAC resources to the new KubeProvision resources, set labels accordingly, so
Kubernetes clusters in every type of environment have desired set of RBAC resources set up. Bow will login to Teleport on tsh, then will use tctl to create 
a new resource:

```bash
$ cat kube_provision_staging.yaml

kind: kube_provision
version: v1
metadata:
  name: staging-rbac
  labels:
    env: staging
spec:
  clusterRoles:
  - metadata:
      name: tech-support
    rules:
    - apiGroups: [""]
      resources: ["pods", "pods/exec", "secrets"]
      verbs: ["create", "get", "update","watch", "list"]
  - metadata
    name: developers
    ...
  clusterRoleBindings:
  ...
  
$ tctl create -f kube_provision_staging.yaml
kube provision "staging-rabc" had been created.
```

To edit provisioned resource later Bob can use `tctl edit kube_provisions/staging-rbac`.

A single Teleport resource of that type can define multiple Kubernetes RBAC resources. There might be limit on size of a resource Teleport
can save on the backend (DynamoDB has a limit of 400kb), therefore, if some users would have an unusually large amount of Kubernetes RBAC resources they 
want to provision, they would need to split it into two KubeProvision resources.

For the ease of use we will provide `--import` flag for the tctl when creating KubeProvisions, which can be used to convert raw Kubernetes RBAC source 
into Teleport KubeProvisions definition. It will be a client-side helper, that could be used like this:


```bash
$ cat kube_provision_import.yaml

kind: kube_provision
version: v1
metadata:
  name: staging-rbac
  labels:
    env: staging
# Spec is missing

$ kubectl get clusterRoles -o yaml > raw_cluster_roles.yaml
$ tctl create -f kube_provision_staging.yaml --import raw_cluster_roles.yaml
kube provision "staging-rbac" had been created.
```

#### Technical details

```protobuf
import "teleport/header/v1/metadata.proto";

// KubeProvision represents a Kubernetes resources that can be provisioned on the Kubernetes clusters.
// This includes roles/role bindings and cluster roles/cluster role bindings.
// For rationale behind this type, see the RFD 174.
message KubeProvision {
  // The kind of resource represented.
  string kind = 1;
  // Not populated for this resource type.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources share.
  teleport.header.v1.Metadata metadata = 4;
  // The specific properties of kube provision.
  KubeProvisionSpec spec = 5;
}

// KubeProvisionSpec is the spec for the kube provision message.
message KubeProvisionSpec {
  // cluster_roles contains definitions for ClusterRole Kubernetes resources to provision.
  repeated ClusterRole cluster_roles = 1;

  // cluster_role_bindings contains definitions for ClusterRoleBinding Kubernetes resources to provision.
  repeated ClusterRoleBinding cluster_role_bindings = 2;

  // roles contains definitions for Role Kubernetes resources to provision.
  repeated Role roles = 3;

  // role_bindings contains definitions for RoleBinding Kubernetes resources to provision.
  repeated RoleBinding role_bindings = 4;
}

// KubeProvisionService is a service that provides methods to manage KubeProvisions.
service KubeProvisionService {
  // CreateKubeProvision creates a new KubeProvision.
  rpc CreateKubeProvision(CreateKubeProvisionRequest) returns (KubeProvision);
  
  // GetKubeProvision gets a KubeProvision by name.
  rpc GetKubeProvision(GetKubeProvisionRequest) returns (KubeProvision);
  
  // ListKubeProvisions returns a list of KubeProvisions. It supports pagination.
  rpc ListKubeProvisions(ListKubeProvisionsRequest) returns (ListKubeProvisionsResponse);
  
  // UpdateKubeProvision updates an existing KubeProvision.
  rpc UpdateKubeProvision(UpdateKubeProvisionRequest) returns (KubeProvision);
  
  // UpsertKubeProvision upserts a KubeProvision.
  rpc UpsertKubeProvision(UpsertKubeProvisionRequest) returns (KubeProvision);
  
  // DeleteKubeProvision deletes a KubeProvision.
  rpc DeleteKubeProvision(DeleteKubeProvisionRequest) returns (google.protobuf.Empty);
}
```

Every five minutes we will run a reconciliation loop that will compare the current state on Kubernetes clusters under
Teleport management with the desired state and update the cluster's RBAC if there's a difference. KubeProvision resources
will be added to the KubeService cache, so we won't need to request potentially large but rarely changed resources from the 
Auth server every reconciliation iteration. 

As described in the earlier in the RFD, Teleport roles with `kubernetes_permissions` field set will also trigger the reconciliation.
Relevant Teleport roles will be treated as a "ephemeral KubeProvision" - meaning that we will not actually create KubeProvision for them
on the backend, but they will reuse same code paths that serves provisioning - Teleport roles will be just another source for the 
reconciliation process.

Teleport will mark RBAC resources under its control with the "app.kubernetes.io/managed-by: Teleport" label. That way we will
separate resources managed by Teleport and those managed by the user manually.

Resource labels will be taken into account when doing the reconciliation - users will be able to match different
Kubernetes clusters for different KubeProvision resources. If a KubeProvision resource doesn't have any labels defined it
will not match any clusters, effectively being disabled. If a Kubernetes cluster has label `teleport.dev/kube-provision-disabled`, 
we will exclude this cluster from the provisioning resources, even if some resources will match it.

Existing clusters will not be affected by this feature by default, since there will be not KubeProvision resources to process. User 
will need to make decision and create KubeProvisions resources to actively start using this feature.

In order to be able to reconcile the state, Teleport will need to perform CRUD operations on Kubernetes RBAC resources.
When performing CRUD operations, Teleport will impersonate the "system:masters" group for most cases, but on GKE `cluster-admin` 
group will be used instead - it is a group we create on GKE cluster enrollment, since GKE doesn't allow impersonation of `system:masters`.

## Security

The introduction of this functionality does not directly impact the security of Teleport
itself, however it introduces a new vector of attack for a malicious actor.
Teleport user with sufficient permissions to create/edit KubeProvision resources
will be able to amend RBAC setup on all Kubernetes clusters enrolled in Teleport.
We will emphasize in the user documentation the need to be vigilant when giving out
the necessary permissions. We will also add new special label `teleport.dev/kube-provision-disabled`, that can
exclude a Kubernetes cluster from the provisioning, selectively turning off this feature for the cluster.

Even though we will always run the reconciliation loop, by default it will be a no-op, since three will
be no resources to provision, so users need to explicitly create KubeProvision resources to start actively using this new feature.
We also will explicitly require labels to be present for the resource to be provisioned to the Kubernetes cluster, making it 
more difficult to accidentally misuse the feature.

## Alternatives

Alternative approach for improving Day 1 experience that was explored was always exposing the default Kubernetes role - `view` and `edit`.
We would create bindings for these roles on Cluster enrollment and allow using them through groups `default-view`/`default-edit`. This
would allow users to always have an underlying role they could use when defining Teleport roles for Kubernetes access. This is a less complicated
but also less powerful solution.

We could also alternatively reverse targeting labels direction for Kube Provisioning. Instead of KubeProvision resource's labels
selecting Kubernetes clusters for provisioning we could add a new field to the Kube service config, `kubeProvisionLabels`, 
and these labels would select which resources should be provisioned to this cluster. E.g. if you don't have any `kubeProvisionLabels`
setup on the Kube service, it won't "pull" any resources, which might be a bit more secure approach. But this also introduced more complexity
in management, since the user can't easily change targeting just by creating/editing KubeProvision resource, they need to change config on every
Kube service. This also looses granularity of control when doing discovery and proxying dynamic clusters, since all cluster will have same
`kubeProvisionLabels` setup and user might need to run multiple Kube service to get the granularity back.

## Future work

As mentioned above adding pattern matching to the `kubernetes_permissions` field is probably something we will need to add in the future, improving
fine-grained control of RBAC users can setup.

Described scheme of auto-provisioning requires Kube service to have access to all Teleport roles, but in the future we plan migration to a more
centralised approach to authorization, where edge services will not see the roles. When this happens we could replace the "ephemeral KubeProvision"
approach with a real KubeProvision approach - for example some watcher on the Auth service could monitor roles and create/update actual KubeProvisions
on the backend, designated with a special kind. And then Kube service will just use regular KubeProvision reconciliation to distribute desired RBAC
to the target clusters.

KubeProvision capabilities might be expanded to include any type of resource in the future, effectively acting something like a 
Terraform for Kubernetes, but defined completely in Teleport. We should gather feedback after releasing initial KubeProvision
feature to understand if there's demand for such extension of capabilities.

Also in the future we can add a dashboard showing current state of KubeProvision reconciliation. We could add a `status` field to
the KubeProvision resource definition, where we could track any errors that happened when KubeService tried to provision the resource and then 
propagate that information to the UI.

## Audit

We will track changes to the KubeProvision resources in the Audit log. There will be three new events:

* `kube.provision.create`
* `kube.provision.update`
* `kube.provision.delete`

Issued when a KubeProvision resource is created/updated/deleted accordingly.

## Test plan

We will use integration tests to verify the reconciliation functionality.
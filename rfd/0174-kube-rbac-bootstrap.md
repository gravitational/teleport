---
authors: Anton Miniailo (anton@goteleport.com)
state: draft
---

# RFD 174 - Kubernetes RBAC Bootstrap

## Required Approvers

- Engineering: `@rosstimothy` && (`@tigrato` || `@hugoShaka`)
- Product: `@klizhentas` || `@xinding33`

## What

Improve RBAC setup capabilities for Kubernetes and make it easier to use with Teleport by exposing
the default Kubernetes user-facing roles and adding the ability to define RBAC setup to be
automatically provisioned to Kubernetes clusters.

## Why

Currently, users have to rely on third-party tools or manual actions
when making sure their Kubernetes clusters have the correct roles/role bindings and that those are in sync with the RBAC setup defined in Teleport.
The changes we propose will make it easier for clients to start using Teleport for accessing Kubernetes clusters.

## Scope

This RFD is focused only on Kubernetes resources related to RBAC functionality, other types of resources are out of scope
and we will not allow them to be provisioned. Although other resources might be added in the future.

## Details

### Improving Day 1 experience - Exposing the default Kubernetes user facing cluster roles (cluster-admin, view, edit)

Kubernetes clusters have default user-facing cluster roles: cluster-admin, view, edit. By default they are not usable, because
they are not exposed through role bindings to any subjects (the cluster-admin role has internal Kubernetes bindings to "system:masters" group). 
Currently, we only expose the cluster-admin role on GKE
installations, since system:masters group is not available for impersonation there. We will expand on this and when
discovering a cluster/installing kube-agent/creating a service account, we will
create cluster role bindings for those roles, accordingly linking them to the groups "default-cluster-admin", "default-view" and
"default-edit". It will give users an opportunity to always have standard set of Kubernetes permissions which they can use together
with Teleport's fine-grained RBAC definitions for Kubernetes. This will make enrolling and starting using Kubernetes clusters
into Teleport a much simpler process, since user will be able to setup RBAC completely in Teleport if they want. We will also be able
to amend our documentation to make our guides simpler, which will also help users to start with Teleport more easily. 

#### UX

Let's look at an example user story. Alice is a system administrator and she explores Teleport capabilities.
She doesn't know much about Teleport yet, except for basic foundation (agents, roles). She just enrolled her first Kubernetes cluster into Teleport.
This cluster contains the staging environment and she wants to give read-only pod access to the developers in an app namespace.
When she was enrolling Kubernetes cluster, she saw that she can rely on `default-view` group for giving a readonly access, so she goes to the UI 
and creates a new role, putting this content into it:

```yaml
kind: role
metadata:
  name: staging-kube-access
version: v7
spec:
  allow:
    kubernetes_labels:
      'env': 'staging'
    kubernetes_resources:
      - kind: pod
        namespace: "main-company-app"
        name: "*"
    kubernetes_groups:
    - default-view
    kubernetes_users:
    - developer
  deny: {}
```

After that Alice assigns this role to the developers she wants to try out and at this point she is done. Alice will not need to create a 
separate role binding in Kubernetes for it to work - once the cluster is enrolled in Teleport, the default-view group binding is already there.

#### Upgrade path

Default roles will be exposed on the new Kubernetes clusters enrollments. To expose those roles in an already enrolled Kubernetes cluster users will 
need to perform a manual helm chart upgrade by `helm upgrade ...` command (or manually create bindings).

For users who want to avoid this default behaviour, we will add an option to the teleport-kube-agent chart and to our
kubeconfig generation script to not create those cluster role bindings, but it will be enabled by default. 
In the teleport-kube-agent Helm chart we will add another field to the `rbac` section, `bindDefaultRoles`, that would control whether we expose the default
roles through cluster bindings or not.

Example content of a `cluster-values.yaml` file that could be used when installing the Helm chart:
```yaml
roles: kube,app,discovery
authToken: test-auth-token
proxyAddr: tele.local:3080
kubeClusterName: test
rbac:
  create: true           <-- existing setting, defaults to true
  bindDefaultRoles: true <-- new setting, also defaults to true
labels:
  teleport.internal/resource-id: b34f651c-32de-45f6-86dc-ab5173f716c9
enterprise: true
```

If either `rbac.create` or `rbac.bindDefaultRoles` is false, we won't expose the default roles.

### Improving Day 2 experience - Automatically provisioning RBAC resources to Kubernetes clusters.

We will add a new type of resources, called "KubeProvision", where user will be able to specify Kubernetes RBAC resources they want to
be provisioned onto their Kubernetes clusters. This can be helpful for users with more complicated RBAC setup and who want to manage
Teleport roles linking to Kubernetes RBAC more easily, without relying on the third-party tools.

#### UX

Bob is an administrator of Teleport resources and access. He works in a large company that has hundreds of engineers/support staff etc and also
has multiple Kubernetes clusters which those users need to access. The Kubernetes clusters might be dynamically created or destroyed, RBAC access patters
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

To edit provisioned resource later Bob can use `tctl edit kube_provisions/staging-brac`.

A single Teleport resource of that type can define multiple Kubernetes RBAC resources. There might be limit on size of a resource Teleport
can save on the backend (DynamoDB has a limit of 400kb), therefore, if some users would have an unusually large amount of Kubernetes RBAC resources they 
want to provision, they would need to split it into two KubeProvision resources.


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
  
  // DeleteAllKubeProvisions removes all KubeProvisions.
  rpc DeleteAllKubeProvisions(DeleteAllKubeProvisionsRequest) returns (google.protobuf.Empty);
}
```

Every five minutes we will run a reconciliation loop that will compare the current state on Kubernetes clusters under
Teleport management with the desired state and update the cluster's RBAC if there's a difference. KubeProvision resources
will be added to the KubeService cache, so we won't need to request potentially large but rarely changed resources from the 
Auth server every reconciliation iteration.

Teleport will mark RBAC resources under its control with the "app.kubernetes.io/managed-by: Teleport" label. That way we will
separate resources managed by Teleport and those managed by the user manually.

Resource labels will be taken into account when doing the reconciliation - users will be able to match different
Kubernetes clusters for different KubeProvision resources. If a KubeProvision resource doesn't have any labels defined it
will not match any clusters, effectively being disabled. If a Kubernetes cluster has label `teleport.dev/kube-provision-disabled`, 
we will exclude this cluster from the provisioning resources, even if some resources will match it.

Existing clusters will not be affected by this feature by default, since there will be not KubeProvision resources to process. User 
will need to make decision and create KubeProvisions resources to actively start using this feature.

In order to be able to reconcile the state, Teleport will need to perform CRUD operations on Kubernetes RBAC resources.
When performing CRUD operations, Teleport will impersonate the "default-cluster-admin" group, that was described in the previous
section. This means that only newly enrolled
Kubernetes clusters, or cluster where the user performed a manual upgrade of permissions will be in scope of provisioning the resources,
since otherwise the "default-cluster-admin" role won't be available.

## Security

The introduction of this functionality does not directly impact the security of Teleport
itself, however it introduces a new vector of attack for a malicious actor.
Teleport user with sufficient permissions to create/edit KubeProvision resources
will be able to amend RBAC setup on all Kubernetes clusters enrolled in Teleport.
We will emphasize in the user documentation the need to be vigilant when giving out
the necessary permissions. We will also add new special label `teleport.dev/kube-provision-dsiabled`, that can
exclude a Kubernetes cluster from the provisioning, selectively turning off this feature for the cluster.

Even though we will always run the reconciliation loop, by default it will be a no-op, since three will
be no resources to provision, so users need to explicitly create KubeProvision resources to start actively using this new feature.
We also will explicitly require labels to be present for the resource to be provisioned to the Kubernetes cluster, making it 
more difficult to accidentally misuse the feature.

## Alternatives

Alternatively, we could take a bit of a different approach regarding permissions and default roles exposure.
We could directly add permissions required to perform CRUD operations on Kubernetes RBAC resources to 
Teleport kube agent/service account credentials on enrollment. We would also ship Teleport with a default KubeProvision 
resource that defines role binding for the default user facing roles for edit and view. This default resource will not have any
labels defined, so it would not be provisioned anywhere by default. If users wanted to enable the exposure of default view/edit roles,
they would need to set labels on that resource. They can then use these roles with the fine-grained RBAC definitions for Kubernetes
in Teleport roles. This is a more conservative scenario that will require more explicit decision-making from the user. 
To expose the default Kubernetes user-facing roles, users would need to add labels to the default resource we provide,
and KubeProvisioning will only be active for clusters where Teleport has the required permissions added to its credentials.

We could also alternatively reverse targeting labels direction for Kube Provisioning. Instead of KubeProvision resource's labels
selecting Kubernetes clusters for provisioning we could add a new field to the Kube service config, `kubeProvisionLabels`, 
and these labels would select which resources should be provisioned to this cluster. E.g. if you don't have any `kubeProvisionLabels`
setup on the Kube service, it won't "pull" any resources, which might be a bit more secure approach. But this also introduced more complexity
in management, since the user can't easily change targeting just by creating/editing KubeProvision resource, they need to change config on every
Kube service. This also looses granularity of control when doing discovery and proxying dynamic clusters, since all cluster will have same
`kubeProvisionLabels` setup and user might need to run multiple Kube service to get the granularity back.

## Future work

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
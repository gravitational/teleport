---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---

# RFD 101 - Kubernetes Per-pod RBAC

## Required Approvers

- Engineering: `@r0mant`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden`

## What

This RFD proposes a just-in-time access request to Kubernetes Pods that will
allow users to request access to specific Pods and a static process for Teleport
administrators to limit the access that a particular Role has to certain Pods.

### Related issues

- [#19573](https://github.com/gravitational/teleport/issues/19573)

## Why

Teleport does not manage individual Kubernetes RBAC rules assigned to users or roles.
Instead, Teleport allows users to impersonate a defined set of Kubernetes RBAC
principals - groups or users - existing in the target cluster. Each user request
will include the associated principals as impersonation headers, and Kubernetes
RBAC will use them to control whether the request can be allowed.

To change the user's permissions, you must create or edit a Kubernetes RBAC role
in the Kubernetes cluster and associate it with an RBAC principal. Afterward, you
append it into the Teleport user properties and the user can impersonate it.
Currently, Teleport appends the RBAC principals to Teleport Roles or to the user
itself. It means that if a user wants temporary access to a single Kubernetes POD,
the cluster administrator has to create a Kubernetes RBAC Role, assign it to a
Kubernetes RBAC principal and create a role with the principal. After that, the user
has to request access to the temporary role. Once someone approves the role access
request, the user can impersonate it.

In addition to the complex process, limiting access to pods using Kubernetes RBAC
has restrictions because Kubernetes RBAC does not support controlling access
resources using pattern matching.
The RBAC Role must include the resource name to grant access, but it requires
knowing the Pod name in advance. In many cases, it is impossible to know them
because Pods generated from Kubernetes objects like Deployments include random
strings in their names and change each time the Pod is deleted or relocated from
one node to the other.

This RFD proposes an extension to Teleport RBAC where it will support
managing access to Pods - including Pods names following a pattern - without
relying on requesting access to Teleport Roles.

## Role definition

This RFD proposes an alternative to limit or extend user access to certain pods
in a Kubernetes cluster from a Teleport Role.

To achieve this, it proposes changes to Teleport's Roles to introduce a new
Role version - `v6` - that extends the `v5` to include a field `kubernetes_resources`
where it's possible to specify the Kubernetes Resources to allow access.

```yaml
kind: role
version: v6
metadata:
  name: my-kube-role
spec:
  allow:
    kubernetes_labels:
      "*": "*"
    kubernetes_groups: ["kube_group"]
    kubernetes_resources:
    - kind: pod
      name: A
      namespace: namespace
    - kind: pod
      name: B
      namespace: namespace
    - kind: pod
      name: C
      namespace: other-namespace
... other v5 fields
```

For roles with `version: v6`, if `kubernetes_resources` is empty, it will reject
any access to Pods within the allowed clusters. To allow access to all Pods in
every namespace, the Role must include

```yaml
kubernetes_resources:
- kind: pod
  name: *
  namespace: *
```

Restricting access to Pods will only apply to Kubernetes clusters that satisfy
the `kubernetes_labels` values. This means that the following role:

```yaml
kind: role
version: v6
metadata:
  name: restrict-prod-access-role
spec:
  allow:
    kubernetes_labels:
      "env": "prod"
    kubernetes_groups: ["kube_group"]
    kubernetes_resources:
    - kind: pod
      name: deploy-*
      namespace: *
... other v5 fields
```

It will only restrict the Pod access to clusters with `env=prod`. If a user has another
Role that grants him access to every Pod in any cluster - `kubernetes_labels: "*":"*"`
and `kubernetes_resources: [kind: pod, name:diffPod*, namespace:*]`, he would
still be able to access resources in other clusters started by `diffPod-*`,
but he only has access to `deploy*` pods for the prod clusters. Check
[Single Role](#single-role) and [Multi Roles](#multi-roles) sections for more
detailed examples.

## Details

We investigated two different approaches to extend the Teleport's RBAC to control
access to specific Pods. The ultimate goal of both approaches is to allow users
to temporarily request access to certain Pods and for administrators to limit
the scope of each Teleport Role without creating multiple Kubernetes RBAC Groups
per rule.

The two approaches are considerably different, and we will detail each one to
briefly describe the implementation details and their limitations.

1. Teleport RBAC controls access to each Pod.

When a user has at least one role that has a non-empty list of `kubernetes_resources`,
Teleport will only allow access to the Pods included in the list. For cases
where multiple roles match the same Pod, Teleport will forward all Kubernetes
RBAC principals defined for roles that fullfill the cluster's labels and pod names.
`kubernetes_resources` is a subset of the super-set allowed by the user's
Kubernetes RBAC principals. It means that Teleport won't be able to include
pods allowed by Teleport `kubernetes_resources` but whose access is denied by
Kubernetes RBAC principals.

To do this, Teleport will intercept requests destined for Pod endpoints - Appendix A -
to either authorize requests that are destined for specific Pods or to
filter the response coming from Kubernetes API to include only the list of
authorized Pods.

Once Teleport intercepts a request destined for a specific Pod, it validates if
the allowed list includes the Pod name and namespace. If the Pod is present, Teleport will
forward the request to the Kubernetes API server. In cases where the Pod name is
available but is not included in the allowed list, Teleport will reject the
request and will not forward it to the Kubernetes API.

If the request is destined to a Pod endpoint that does not have Pod names associated -
`kubectl get pods`, `kubectl get pods --all-namespaces`,
Teleport will forward the request to the cluster and will modify the received
response. Once it receives a response from the Kubernetes API server, Teleport
will filter out any pod the user should not have access.

2. Teleport will delegate access control to Kubernetes RBAC.

A second possible approach is, instead of intercepting/rewriting API calls, for Teleport
to manage Kubernetes RBAC policies directly using Kubernetes API and keep relying
on Kubernetes access controls. This allows us to support access requests but won't
allow configuring role policies in Teleport allowing/denying access to specific pods.

When a user requests access to a specific Pod that currently he has no access to,
Teleport will modify his Kubernetes RBAC principals to include a new group that grants
him access to the Pod. This indeed will extend his permissions when interacting
with the Kubernetes Cluster instead of restricting them as in approach 1.

To do this, Teleport will create temporary `Roles` in the Kubernetes cluster and
will associate them with Kubernetes groups that the user can impersonate.

Once Teleport receives an access request to a Pod, it will create the Kubernetes
RBAC `Role` and will create `RoleBinding` to assign it to an RBAC group. Afterward,
Teleport will include the newly created group in the user's certificate.

Once the user has the certificate with the new group embedded, Teleport will
forward the certificate details into the Kubernetes cluster as Impersonation Headers.

When the access request expires, Teleport will clean the temporary Kubernetes
RBAC `Role` and `RoleBinding` objects.

The final decision was to implement the first option, however, the second option
details are kept in this RFD.

Pod RBAC only rules only apply to Pod endpoints. Any other Kubernetes Resource
access must be controlled via Kubernetes RBAC policies.

## UX

### Limit access to Pods

The introduction of `kubernetes_resources` field in the Teleport Role Spec allows an
administrator to limit what each team/user can see and access when reaching
a Kubernetes Cluster.

```yaml
kind: role
version: v6
metadata:
  name: kube-role
spec:
  allow:
    kubernetes_labels:
      "*": "*"
    kubernetes_groups: ["kube_group"]
    kubernetes_resources:
    - kind: pod
      name: A
      namespace: namespace
    - kind: pod
      name: B
      namespace: namespace
    - kind: pod
      name: C
      namespace: other-namespace
... other v5 fields
```

The Role spec above allows any user with the attached role to access Pod `A` and
`B` from the `namespace` and Pod `C` from the `other-namespace`.

`kubernetes_resources[:].name` and `kubernetes_resources[:].namespace` support Regex pattern
matching but also support wildcards - `*` - for simplifying their usage instead
of requiring full regex expressions.

For Role versions `<v6`, Teleport will automatically include the following spec
to keep compatibility with previous behavior.

```yaml
    kubernetes_resources:
    - kind: pod
      name: *
      namespace: *
```

This means that older versions will allow unrestricted access to every pod and
action that the user's principals grant access to.

### Request Access Flow

Extending [RFD 59](https://github.com/gravitational/teleport/blob/master/rfd/0059-search-based-access-requests.md),
Teleport will consider Kubernetes Pods as native resources.
This allows users to search Pods that normally they do not
have access to and request access to them. Besides requesting access to
specific Pods - using the Pod full name, Teleport allows users to request
access to Pods that follow a pattern like Pods automatically created from a
Deployment - `deploymentname-*-*`.

It is not mandatory for the user to previously have access to the target
Kubernetes cluster. Once the Pod access request is approved, Teleport will allow the
user to connect to the target cluster without requiring an extra access request
to the cluster.

```bash
$ tsh request search --kind pod --kube-cluster=<kube-cluster>

Found 3 items:
name    namespace     id
nginx-1 default       /<teleport-cluster>/pod/<kube-cluster>/default/nginx-1
nginx-1 dev           /<teleport-cluster>/pod/<kube-cluster>/dev/nginx-1
nginx-2 dev           /<teleport-cluster>/pod/<kube-cluster>/dev/nginx-2

$ tsh request create --resources "..."
Waiting for request to be approved...
Approved!
$ kubectl get pods 
```

An alternative process is available by using the `tsh kubectl` wrapper. This
functionality will create the access requests, wait for their approval, and
retry the command afterward.

```bash
$ tsh kubectl exec/logs/edit -it <podName> -n <namespace>
Access to pod <namespace>/<podName> denied.
Submiting an access request.
Waiting for request to be approved...
Approved!
# <podName>
```

Similarly to Resource Based Access Requests, Pod Access requests will leverage
`search_as_roles` to control which Pods the user can view while searching.

```yaml
kind: role
metadata:
  name: response-team
spec:
  allow:
    request:
      # search_as_roles allows a member of the response team
      # to search for resources accessible to users with the kube-admins role,
      # which they will be allowed to request access to. This includes Pods in 
      # the target Kubernetes Cluster.
      search_as_roles: [kube-admin]
```

```yaml
kind: role
version: v6
metadata:
  name: kube-admin
spec:
  allow:
    # kubernetes_groups will be used by Teleport to fetch the Pods available in 
    # the target cluster. The kubernetes_groups field is used by Kubernetes RBAC 
    # to limit the Pods the user can request access to.
    kubernetes_groups: ["system:masters"]
    # kubernetes_labels defines which Kubernetes clusters the user is able to 
    # request Pod access into.
    kubernetes_labels:
       owner: prod_team
    kubernetes_resources:
    - kind: pod
      name: nginx* # allow access to pods prefixed with "nginx".
      namespace: * # match pod name in every namespace.
```

While requesting access to a Pod or searching for Pods, the user must identify
which cluster he wants to request access to. If the user does not provide any
cluster, `tsh` will fill it with the default Teleport context from `kubeconfig`.

When a access request is of Type `pod`, Teleport allows the user to access other
Kubernetes resources such as `secrets`, `configmaps`... In order to control the
scope, the `search_as_role`'s Kubernetes principals must allow access to
`pods` and forbid every other resource.

## Restrict Pod access (**selected**)

Proposed is a new way to restrict access to specific Pods on a Kubernetes cluster.
Given a set of roles with Kubernetes RBAC principals - groups or users,
that normally grant access to a larger set of Pods, once `kubernetes_resources`
is specified in at least one of the roles, Teleport will restrict the user's
access to the list of pods specified under `kubernetes_resources`.
`kubernetes_resources` must define a subset of the Pods allowed by the user's Kubernetes
RBAC principals otherwise Teleport can let the request through and Kubernetes RBAC
will deny it. The user's permissions are the intersection of the
permissions allowed by Kubernetes RBAC and the list of Pods defined in `kubernetes_resources`.

When accessing a pod in a Kube cluster, Teleport evaluates if any Role matches
this Kube cluster labels and if it allows the access to the Pod - pod's name and
namespace. If everything matches, Teleport collects the
Kubernetes RBAC principals associated with those roles and forwards the request to
the Kubernetes Cluster with the collected principals. If no match is found, Teleport
rejects the request with an access denied message.
Given a user with the following roles:

| Name | `kubernetes_labels` | `kubernetes_groups` | `kubernetes_resources` |
|---|---| --- |  --- |
| `role1` | `env`:`prod` | `kube_group1` | name: `*`, namespace: `*`, kind: `pod` |
| `role2` | `env`:`dev` | `kube_group2`  | name: `*`, namespace: `*`, kind: `pod` |
| `role3` | `env`:`prod` | `kube_group3` | name: `special_pod`, namespace: `default`, kind: `pod` |

If the user executes `kubectl logs pod_name_1 -n default` targeted to a cluster with labels
`env`:`prod`, Teleport matches the cluster's labels against the role's `kubernetes_labels`.
Only `role1` and `role3` match - `role2` misses because the cluster has `env`:`prod`.
At the same time, Teleport matches the pod name
and namespace {`name`:`pod_name_1`, `namespace`:`default`} against `kubernetes_resources`
for the roles that also match the cluster labels. For this request,
`role3` does not match the pod's name and Teleport collects the
`kubernetes_groups`: [ `kube_group1` ]. If the user tries to execute
`kubectl logs special_pod -n default` into the same cluster, Teleport collects
`kubernetes_groups`: [`kube_group1`, `kube_group3`] because the `role3` now matches the pod
name. When forwarding the request to the Kubernertes Cluster, the user will
impersonate `kube_group1` and `kube_group2` instead of `kube_group1` for the previous
request. 

When listing pods - `kubect get pods [--all-namespaces,-n=<namespace>]`, Teleport
will collect all `kubernetes_groups` that are apllicable to the cluster based on
its labels - pod name is not available - and forward them to the cluster.
Once it receives the response, it
will filter out the pods the user doesn't have access to by checking if no role denies
the pod and if one of the roles allow access to the Pod. Since Kubernetes does not
identify which Kubernetes Principal granted access to a resource, Teleport does
not know if the rule is correctly applied. It means that if one Role allows access to
`kind: pod, name: *, namespace:*` with a Kubernetes RBAC group that only allows access
to pods in the `default` namespace, and a role that grants `system:masters` access
to `kind: pod, name: some_pod, namespace:default`, Teleport will
leak every Pod in the cluster because the `*/*` rule is always true against
any pod. In order to avoid this issues, if your Kubernetes RBAC principal is
namespaced, lock the `kubernetes_resources` to that namespace with `<namespace>/*`.

Below we introduce a more indepth examples and combinations.

### Single Role

```yaml
kind: role
version: v6
metadata:
  name: my-kube-role
spec:
  allow:
    kubernetes_labels:
      "*": "*"
    kubernetes_groups: ["kube_group"]
    kubernetes_resources:
    - kind: pod
      name: B 
      namespace: default
    - kind: pod
      name: C
      namespace: default
    - kind: pod
      name: podname-*-*
      namespace: default
```

Having a Kubernetes RBAC group `kube_group` allowing access to a set of Pods
{A,B,C,D, podname-1-1} and a Teleport role with `kubernetes_resources` set to
{B,C,podname-\*-\*}, a request would have the following behavior:

- `kubectl get pods`: returns pods {B,C,podname-1-1}.
- `kubectl edit pod/B`: request allowed.
- `kubectl edit pod/A`: request denied by Teleport.
- `kubectl delete pod/B`: request allowed.
- `kubectl logs pod/B`: request allowed.
- `kubectl logs pod/A`: request denied by Teleport.
- `kubectl logs pod/podname-1-1`: request allowed.

Using this method, Teleport allows you to restrict the pods that a particular
user has access to. However, if there is at least one role that specifies a deny
rule to a pod, Teleport will restrict access to the pod specified regardless
of what the other roles allow.

### Multi Roles

This section describes examples when a user has multiple Teleport Roles assigned.

#### Kubernetes Clusters

| Name | Labels | Roles |
|---|---| --- |
| `cluster1` | `env`:`dev` | `dev-admin` (allows anything) |
| `cluster2` | `env`:`prod` | `system:masters`, `viewer` (only lists pods in the `default` namespace)  |

#### Roles

| Name | kubernetes_labels | kubernetes_groups | kubernetes_resources{kind=pod} |
|---|---|---|---|
| `role1` | `env`:`prod` | `viewer` | `name`:`*`, `namespace`:`*` |
| `role2` | `env`:`prod` | `viewer` | `name`:`*`, `namespace`:`default` |
| `role3` | `env`:`prod` | `system:masters` | `name`:`owned_pod`, `namespace`:`default` |
| `role4` | `env`:`dev` | `dev-admin` | `name`:`*`, `namespace`:`*` |

#### Examples

Given the clusters and the Roles represented above, the following table shows
describes execution flows.

| User Name | Cluster |  Roles | `kubectl get pods --all-namespaces` | `kubectl exec owned_pod` | `kubectl exec other_pod` |
|---|---|---|---|---|---|
| user1 |  `cluster1` | `role4`, `role1` |  pods in every namespace (sends `dev-admin`) | allowed by Teleport, allowed by `dev-admin` | allowed by Teleport, allowed by `dev-admin` |
| user2 |  `cluster2` | `role1` |  pods from `default` namespace (sends `viewer`) | allowed by Teleport, denied by `viewer` | allowed by Teleport, denied by `viewer` |
| user2 |  `cluster2` | `role2` |  pods from `default` (sends `viewer`) | allowed by Teleport, denied by `viewer` | allowed by Teleport, denied by `viewer` |
| user3 |  `cluster2` | `role3` |  `owned_pod` from `default` (sends `system:masters`) | allowed by Teleport, alowed by `system:masters` (sends: `system:masters`) | denied by Teleport |
| user4 |  `cluster2` | `role1`, `role3` |  leaks every pod in every namespace (sends `viewer`,`system:masters`) | allowed by Teleport, alowed by `system:masters` (sends: `system:masters`,`viewer`) | allowed by Teleport, denied by `viewer` (sends:  `viewer`) |
| user5 |  `cluster2` | `role2`, `role3` | returns every pod in `default` namespace (sends: `viewer`,`system:masters`) | allowed by Teleport, alowed by `system:masters` (sends: `system:masters`,`viewer`) | allowed by Teleport, denied by `viewer` (sends:  `viewer`) |

### Implementation details

Given a set of roles that include at least one role that includes one or more Pod
names to limit access to, Teleport will restrict the Pods the user can list and
to the intersection of the set of Pods granted by the Kubernetes RBAC principals and
the list of Pod names in the role - ${Pods\ from\ RBAC}\ \cap \ {kubernetes\\_ resources\\{kind=pod\\}}$.

To achieve it without relying on Kubernetes RBAC, Teleport needs to intercept
user requests and manipulate them. Depending on the endpoint accessed, Teleport
may only need to validate that the target Pod's name is included in the allowed list.
In other endpoints where the target is not a single Pod, but a group of Pods like when
listing pods, Teleport requires hijacking the Kubernetes response to filter out
unwanted Pods that Kubernetes RBAC allowed.

This RFD separates the approaches described above into two different subsections
to extend their requirements and pitfalls.

#### Pod Request Access

When a user dynamically requests access to a pod, it follows the same procedure
used for normal resource access requests.

```bash
$ tsh request create \
  --resource /<teleport_cluster>/pod/<kube_cluster>/<pod_namespace>/<pod_name> \
  --reason <reason>
```

This request is represented as `ResourceId` in the following way:

```go
types.ResourceId{
   Cluster: "<teleport_cluster>",
   Kind: "pod",
   Name: "<kube_cluster>", //does not support wildcards 
   SubResource: "<pod_namespace>/<pod_name>" // both support wildcards matching
}
```

Once the approver aproves the request, the `ResourceId` is encoded into the user's
certificate and *every* role available in `search_as_role` that matches the target
cluster and the pod name is included in the certificate's roles.

When a `ResourceId.Kind` is `pod`, Teleport allows the user to access the Kubernetes
cluster - even if only the `search_as_role` has RBAC rules to
access it - and validates if the Pod name is allowed.

#### Endpoints with POD's name

For Endpoints whose path starts with `/api/v1/namespaces/{namespace}/pods/{name}`,
the Teleport RBAC layer can extract the `{name}` from the request
URI and validate if the pod name is in the user's allowed list. Teleport also computes
at runtime the Kubernetes principals defined in the roles that match the cluster labels
and pod name/namespace.

Each time a user requests an endpoint prefixed with `/api/v1/namespaces/{namespace}/pods/{name}`,
Teleport Kubernetes Service intercepts the request and extracts the `{name}`.
After, it will validate if the `{name}` is included in the `kubernetes_resources`
array or if it satisfies any wildcard pattern defined on it. If allowed, Teleport
will forward the request to Kubernetes with the RBAC principals as Impersonation
headers. Kubernetes will apply a second authorization layer using Kubernetes RBAC
to validate the request. Finally, Teleport would forward the Kubernetes API
response directly to the user without any modification.

#### Special handlers

When the Pod name is not accessible at request time, Teleport will have
different behavior. Depending on the action the user is doing, Teleport will
handle them separately.

##### Listing Pods

When listing Pods in the cluster or at a certain namespace, Teleport will receive
the list of Pods from Kubernetes itself. Since Teleport depends on what Kubernetes
RBAC allows users to access, Teleport would need to hijack the Kubernetes API
response to exclude any Pod that is not included in the allowed list.

To do that, Teleport will automatically forward the request to Kubernetes
API for the following two endpoints:

- `/api/v1/pods`
  - `GET`: list or watch objects of kind Pods in every namespace.

- `/api/v1/namespaces/{namespace}/pods`
  - `GET`: list or watch objects of kind Pods in namespace `{namespace}`.

Once Teleport receives the response from Kubernetes API, it will unmarshal the
response and exclude any Pod that is not in the allowed list. After excluding them,
Teleport will return the filtered response to the user.

##### Deleting Pods

Kubernetes allows a user to delete every Pod in a certain namespace using a single
call. The call does not include the name of the Pods to delete, so Teleport won't
forward the request to Kubernetes. Instead, Teleport will list the user's Pods,
exclude the ones that he shouldn't have access to, and issue a delete request for
each one of the resulting Pod names.

- `/api/v1/namespaces/{namespace}/pods`
  - `DELETE`: delete a collection of Pods in namespace `{namespace}`.

Another option is to prevent this type of request in Teleport. This means that
if you try to delete every Pod in the namespace while your role limits the Pods
you have access to, Teleport will reject the request and inform the user that
he is not allowed to do it. This option will restrict one of the `kubectl`
functionalities.

##### Creating Pods

The endpoint for creating pods won't be restricted, and if restrictions exist
it's the Kubernetes RBAC layer that applies them.

- `/api/v1/namespaces/{namespace}/pods`
  - `POST`: create a POD in namespace `{namespace}`.

### Security

When a user has multiple roles assigned and his access is expected to be restricted
to a subset of the pods available, it's a requirement that the union of every
`kubernetes_resources` defined does not give broader permissions as exemplified by `user4` in
[Restricted Pod Access](#restrict-pod-access-selected) examples. Otherwise,
Teleport will leak the Pods when the user lists them.

When a user lists pods - `kubectl get pods`, using the Kubernetes RBAC given by
`search_as_roles` or by a Kubernetes RBAC principal that allows listing pods
before requesting access to a certain Pod, the user has access
to other details besides the Pod name. It happens because `kubectl get pods -o json|yaml`
API response returns Pod description that includes env variables, secrets used,
and other details. This might be a security concern because Pods YAML might contain
secrets that can be leaked even before creating an access request.

When a user has the ability to `exec` into a Pod, it's also a security concern
because at that time Teleport access control can be bypassed and the user is able
to send requests directly to the Kubernetes API endpoint using the Pod's credentials.
If the target Pod has RBAC permissions atached such as `Impersonation`, delete pods, logs...
the user bypasses Teleport and is able to execute the commands. Take into consideration
each RBAC policy attached to each pod.

### Limitations

1. Difficult to extend to other resources such as namespaces, deployments... It's 
difficult to extend them because it would require special handlers for each
resource type (unmarshal logic + filtering).

2. Teleport must hijack every request that is targeted to endpoints without Pod names.
Methods to list Pods or delete Pods from a certain namespace must be tampered 
with to filter out Pods that the user should not access.

3. Might cause incompatibilities concerns while dealing with different Pod objects.

4. If in the future Pod endpoints are moved to `v2`, Teleport needs to upgrade them accordingly

5. Difficult to support `kubectl delete pods --namespace {namespace}` requests.

## Teleport will delegate access control to Kubernetes RBAC (**alternative option**)

Once Teleport receives an access request to Pods, Teleport will create a new
Kubernetes RBAC  `Role` and `RoleBinding` - can be extended to `ClusterRole` as
well - and will, temporarily, inject the Kubernetes Group created by the
`RoleBinding` into the user's certificate.

It allows Teleport to forward requests into Kubernetes without any special handler
for Pod endpoints neither requires Teleport to manipulate Kubernetes API responses.

Since Teleport will leverage Kubernetes RBAC Roles to grant access to other Pods
while supporting wildcards, Teleport must circumvent the Kubernetes RBAC limitation
that forbids specifying resources with wildcards. To circumvent it, Teleport must
include in the Kubernetes `Role` definition every Pod name that matches a certain
pattern and update the Kubernetes `Role` each time a new Pod is created or deleted.
This means that Teleport must introduce a watcher mechanism that is responsible for
constantly updating Kubernetes `Roles` created from Pod requests that include
wildcards.

### Kubernetes RBAC Permissions

To be able to create `Roles` and `RoleBindings`, Teleport must be allowed - via
Kubernetes RBAC - to create them. To be able to list Pods, Teleport requires the
`list` verb in the Pods section.

The changes to Teleport `Role` are defined below.

```diff
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  metadata:
    name: teleport-role
  rules:
  - apiGroups:
    - ""
    resources:
    - users
    - groups
    - serviceaccounts
    verbs:
    - impersonate
  - apiGroups:
    - ""
    resources:
    - pods
    verbs:
    - get
+   - list
  - apiGroups:
    - "authorization.k8s.io"
    resources:
    - selfsubjectaccessreviews
    - selfsubjectrulesreviews
    verbs:
    - create
+ - apiGroups: 
+   - rbac.authorization.k8s.io
+   resources:
+   - rolebindings
+   - role
+   verbs:
+   - create
+   - update
+   - delete
+   - list
```

### Implementation Details

Once an admin approves a user access request, Teleport will create a new temporary
Kubernetes RBAC `Role` and `RoleBinding` with a random name: `random_name`.
At `RoleBinding`, Teleport will define an RBAC group with the same name. The
user's certificate `kubernetes_groups` field will then include the temporary
Kubernetes group created before.

The `Role` includes the Pods mentioned in the access request as part of the field
`{pod_names}`. If the access request includes pods with any pattern, Teleport
will create a watcher responsible for monitoring the cluster's pods and updating
the `Role` with the Pods that match the pattern.

An example of a `Role` and `RoleBinding` can be found below.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: {target_namespace}
  name: {random_name}
rules:
  - apiGroups: ["*"]
    resources:
    - pods
    - pods/log
    - pods/exec
    resourceNames:
    - {pod_names}
    verbs:
    - get
    - list
    - watch
    - create
    - update
    - delete
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: {target_namespace}
  name: {random_name}
subjects:
- kind: Group
  name: {random_name}
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: {random_name}
  apiGroup: rbac.authorization.k8s.io
```

Once the access request expires, Teleport will clean every `Role` and `RoleBinding`
created in the target cluster during the process.

Creating, updating, and deleting `Roles` and `RoleBindings` is the responsibility
of Teleport Kubernetes Service. 

### Limitations

1. Teleport Kubernetes Service - agentless or not - must have permission to create
`Role` and `RoleBindings` on the cluster. This might become a security concern.

2. Teleport must have watchers to constantly monitor the cluster's Pods and update 
the `Role` object with every pod that matches the pattern.

## Appendix

### Appendix A: Pod API endpoints in Kubernetes API

In this appendix, you can find the list of endpoints and their verbs supported
by the Kubernetes API Server to list, edit or interact with Pods.

#### A1: Endpoints without Pod's name

- `/api/v1/pods`
  - `GET`: list or watch objects of kind Pod in every namespace.

- `/api/v1/namespaces/{namespace}/pods`
  - `DELETE`: delete a collection of Pods in namespace `{namespace}`.
  - `GET`: list or watch objects of kind Pods in namespace `{namespace}`.
  - `POST`: create a Pod in namespace `{namespace}`.

#### A2: Endpoints with Pod's name

- `/api/v1/namespaces/{namespace}/pods/{name}`
  - `DELETE`: delete a Pod in namespace `{namespace}`.
  - `GET`: reads the specified Pod in namespace `{namespace}`.
  - `PATCH`: partially updates the specified Pod in namespace `{namespace}`.
  - `PUT`: replace the specified Pod in namespace `{namespace}`.

- `/api/v1/namespaces/{namespace}/pods/{name}/log`
  - `GET`: read the log of the specified Pod in namespace `{namespace}`.

- `/api/v1/namespaces/{namespace}/pods/{name}/exec`
  - `GET`: connect GET requests to exec of Pod.
  - `POST`: connect POST requests to exec of Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/proxy`
  - `DELETE`: connect DELETE requests to proxy of Pod.
  - `GET`: connect GET requests to proxy of Pod.
  - `HEAD`: connect HEAD requests to proxy of Pod.
  - `OPTIONS`: connect OPTIONS requests to proxy of Pod.
  - `PATCH`: connect PATCH requests to proxy of Pod.
  - `POST`: connect POST requests to proxy of Pod.
  - `PUT`: connect PUT requests to proxy of Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/attach`
  - `GET`: connect GET requests to attach of Pod.
  - `POST`: connect POST requests to attach of Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/status`
  - `GET`: read the status of the specified Pod.
  - `PATCH`: partially updates the status of the specified Pod.
  - `PUT`: replace the status of the specified Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/binding`
  - `POST`: create binding of a Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/eviction`
  - `POST`: create eviction of a Pod.

- `/api/v1/namespaces/{namespace}/pods/{name}/portforward`
  - `GET`: connect GET requests to port forward of Pod
  - `POST`: connect POST requests to port forward of Pod

- `/api/v1/namespaces/{namespace}/pods/{name}/proxy/{path}`
  - `DELETE`: connect DELETE requests to proxy of Pod.
  - `GET`: connect GET requests to proxy of Pod.
  - `HEAD`: connect HEAD requests to proxy of Pod.
  - `OPTIONS`: connect OPTIONS requests to proxy of Pod.
  - `PATCH`: connect PATCH requests to proxy of Pod.
  - `POST`: connect POST requests to proxy of Pod.
  - `PUT`: connect PUT requests to proxy of Pod.

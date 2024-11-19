---
author: Lisa Kim (lisa@goteleport.com)
state: implemented
---

# RFD 183 - Access Request Kubernetes Resource Allow List

# Required Approvers

- Engineering: @r0mant && @tigrato
- Product: @klizhentas || @xinding33

# What

Allow admins to specify what kind of Kubernetes resources a user can request.

# Why

Currently there are no access request settings that allows admins to enforce a certain Kubernetes resource request. Current settings allow users to request to a `kube_cluster` and to [any of its sub resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201).

The most permissive request is if a user requested for a resource kind `kube_cluster`. This gives users access to all (limited by whatever role user assumes) the kube subresources inside it eg: `namespaces, pods, etc`.

Selecting subresources however (eg: kind `namespace`) scopes down what user has access to. For example, if the user requested for select few namespaces for a specific `kube_cluster`, once approved and assumed, the user is only able to access those approved namespaces. Versus, if the user were to get approved for a `kube_cluster` instead, the user has access to all namespaces.

The admin could limit what user has access to a `kube_cluster` by defining limits with the [kubernetes_resources](https://goteleport.com/docs/enroll-resources/kubernetes-access/controls/#kubernetes_resources) role field, but it hides visibility for the reviewer.

For example, if a request came in for a `kube_cluster`, the reviewer sees this:

| Cluster Name     | Requested Resource Kind | Requested Resource Name |
| ---------------- | :---------------------: | ----------------------: |
| teleport-cluster |      kube_cluster       |    pumpkin-kube-cluster |

The reviewer may not remember what this `kube_cluster` has access to and will have to look up the requested role to double check access which can be annoying.

If a request came in for a subresource `namespace` for a `kube_cluster` instead, the reviewer sees this:

| Cluster Name     | Requested Resource Kind |      Requested Resource Name |
| ---------------- | :---------------------: | ---------------------------: |
| teleport-cluster |        namespace        |     pumpkin-kube-cluster/dev |
| teleport-cluster |        namespace        | pumpkin-kube-cluster/staging |

The reviewer is more likely to understand what access is being granted.

This RFD proposes a new role request field that will enforce users to request for certain Kubernetes subresources instead.

# User Scenarios

Listing use cases from most permissive to most restrictive:

## As an admin, I don't care what kind of Kubernetes resources a user requests

As a requester, this means I can request to Kubernetes clusters and its resources (how it behaves now).

This will be the default behavior if no configurations are specified. This will also be the default behavior of existing roles unless modified.

In case of a `search_as_role` role being defined in multiple roles assigned to a user, the role without a `request.kubernetes_resources` configured will take precedence. For example, if a user is assigned the following roles:

```yaml
kind: role
metadata:
  name: requester-role-1
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
      kubernetes_resource:
      - kind: "namespace"

----

kind: role
metadata:
  name: requester-role-2
spec:
  allow:
    # no kubernetes_resource specified
    request:
      search_as_roles:
      - kube-access
```

Even though a `request.kubernetes_resources`is defined in one role, since the other role doesn't define a `request.kubernetes_resources`, it results in no restrictions (allow user to request to Kubernetes cluster and its resources)

## As an admin, if a user requests for namespaces, I want to limit what namespaces a user can request

This is already supported. The admin can use `allow.kubernetes_resources` field to restrict what list of namespaces the requester can list and request for:

```yaml
kind: role
metadata:
  name: kube-access
spec:
  allow:
    # Only lists namespaces starting with "pumpkin-" prefixes
    kubernetes_resources:
      - kind: namespace
        name: pumpkin-*
        verbs:
          - list
    kubernetes_groups:
      - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_users:
      - '{{internal.kubernetes_users}}'

----

kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
```

Requesting for namespaces not matching rule will be denied.

## As an admin, if a user requests for namespaces, I want to limit namespaces using role templates and traits

Trait interpolation will be supported in [v17](https://github.com/gravitational/teleport/pull/45277) for `allow.kubernetes_resources` field:

```yaml
kind: user
metadata:
  name: lisa@goteleport.com
spec:
  created_by:
    connector:
      id: okta-integration
  roles:
  - requester
  # Attributes from Okta
  traits:
    searchable_kube_namespaces:
    - pumpkin-*

----

kind: role
metadata:
  name: kube-access
spec:
  allow:
    kubernetes_resources:
      # Only lists namespaces starting with "pumpkin-" prefixes
      - kind: namespace
        # Inserts user's trait value
        name: '{{external.searchable_kube_namespaces}}'
        verbs:
          - list
    kubernetes_groups:
      - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_users:
      - '{{internal.kubernetes_users}}

----

kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
```

## As an admin, I want to require users to request for Kubernetes subresources instead of the whole Kubernetes cluster

Admins can specify in a role, a `allow.request.kubernetes_resources` option that requires users to select [Kubernetes subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

```yaml
kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      # Disables requesting whole Kubernetes cluster, but can request any of the kube subresources
      kubernetes_resources:
      - kind: *
```

User requesting kube_cluster will be denied.

## As an admin, I want to limit Kubernetes resource requesting to pods and namespaces only

Admins can specify in a role, a `allow.request.kubernetes_resources` that requires users to select either pods or namespaces.

```yaml
kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      # Can request only pods or namespaces
      kubernetes_resources:
        - kind: namespace
        - kind: pod
```

User requesting any other Kubernetes kind will get denied.

# Details

## New request field for role spec

We will introduce a new RoleCondition field under the `request` section named `kubernetes_resources` and copy the existing data structure used for [KubernetesResource](https://github.com/gravitational/teleport/blob/c49eb984648b506f3223804784a54ddec8d4d2d3/api/proto/teleport/legacy/types/types.proto#L3293). Not all fields in KubernetesResource object will initially be supported. The first iteration will only support the [`kind` field](https://github.com/gravitational/teleport/blob/c49eb984648b506f3223804784a54ddec8d4d2d3/api/proto/teleport/legacy/types/types.proto#L3297).

The `kind` field will support `asterisks` and the names of all [Kubernetes subresource kinds](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

```yaml
kind: role
metadata:
  name: role-name
spec:
  allow:
    request:
      # new field
      kubernetes_resources: [<list of KubernetesResource objects>]
```

| kubernetes_resources list example                |                           Explanation                            |
| ------------------------------------------------ | :--------------------------------------------------------------: |
| [] or `request.kubernetes_resources` not defined |                         no restrictions                          |
| [{ kind: '*' }]                                  | requires users to request for any of the Kubernetes subresources |
| [{ kind: 'namespace' }]                          |          requires users to request for only namespaces           |
| [{ kind: 'namespace'}, { kind: 'pod' }]          |    requires users to request for only namespaces and OR pods     |

The `kind` values will be validated for expected value upon role create or update.

## Merging between multiple roles

If a requester is assigned multiple roles with differing `request.kubernetes_resources`, then it will be merged into a single list.

```yaml
kind: role
metadata:
  name: requester-role-1
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
      - some-other-kube-access
      kubernetes_resources:
      - kind: "namespace"

----

kind: role
metadata:
  name: requester-role-2
spec:
  allow:
    request:
      search_as_roles:
      - kube-access  # same role assigned in requester-role-1, so "kind" will get merged to ["namespace", "secret"]
      kubernetes_resources:
      - kind: "secret"
```

The above roles will enforce, that when requesting search_as_role `kube-access`, the backend will only allow Kubernetes resource requests for `namespaces` and or `secrets`. If requesting search_as_role `some-other-kube-access`, the backend will only allow `namespace`.

In case of a `wildcard` present in the `request.kubernetes_resources`, `wildcard` will take precedence over other configured values, and will allow requesting to any Kubernetes subresources (but not allow requesting to a Kubernetes cluster).

In case of multiple roles assigned where one role has NO `request.kubernetes_resources` configured (or is empty list) and the other role configures this field, then this will be translated as `allow any request` since no configuration takes precedence over configured field.

## Deny Rules

Deny can be possible and will be respected. It works similarly as allow.

One difference is, given these two roles assigned to a user:

```yaml
kind: role
metadata:
  name: requester-role-1
spec:
  allow:
    request:
      search_as_roles:
      - some-other-kube-access
      kubernetes_resources:
      - kind: "namespace"

----

kind: role
metadata:
  name: requester-role-2
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
  deny:
    request:
      kubernetes_resources:
      - kind: "namespace"
```

The `deny` rule will reject ANY `namespace` request regardless of which search_as_roles is requested because deny rules are globally matched.

## Request Validation

Validations for allow/deny `request.kubernetes_resources` fields will be placed:

1. When a resource Access Request is being validated
1. When populating the request roles automatically for a new resource access request
    - Roles with no matching allow/deny fields will be pruned

## New proto fields

Data structure that will hold a list of Kubernetes "kinds":

```proto
message RequestKubernetesResource {
  string Kind = 1 [(gogoproto.jsontag) = "kind,omitempty"];
}
```

New field for RoleOptions:

```proto
message AccessRequestConditions {
  ...other existing fields

  // kubernetes_resources optionally allows admins to enforce a requester to request only certain resources.
  // Eg: Users can make request to either a resource kind "kube_cluster" or any of its
  // subresources like "namespaces". The mode can be defined such that it prevents a user
  // from requesting "kube_cluster" and enforces requesting any of its subresources.
  repeated RequestKubernetesResource kubernetes_resources = 8 [(gogoproto.jsontag) = "kubernetes_resources,omitempty"];
}
```

## Web UI (and Teleterm)

Our UI's currently only supports requesting for kind `kube_cluster`.

With this RFD implementation, we will add support for reading/selecting only `namespaces` initially. See [figma design](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity?node-id=2220-50559&node-type=frame&t=sPqQkrRd0mRRXlzO-0) for namespace selecting.

These are the current states the UI's can take regarding Kubernetes resource requesting depending on errors received when performing a initial `dry run` of creating an access request:

- if no errors returned then both `kube_cluster` and/or `namespace` is requestable
- else, if error is returned, and it mentions Kubernetes kinds supported by the UI's (namespace), the error is ignored (implies that namespaces is required)
- else, if error does NOT mention Kubernetes kinds supported by web UI (namespace), then an error will render with a hint

This is an example of an error returned related to `kubernetes_resources`, where the configuration is not supported by the UI, thus we direct the user to use the `tsh` tool instead:

```
your Teleport role's "request.kubernetes_resources" field did not allow requesting to some or all of the requested Kubernetes resources. allowed kinds for each requestable roles: access-kube-pumpkin: [pod], access: [pod]

The listed allowed kinds are currently only supported through the `tsh CLI tool`. Use the `tsh request search` command that will help you construct the request.
```

## CLI

tsh already has support for requesting [all these resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201).

There is nothing to add to the CLI, we will just let it error out upon request creation validation.

## Querying for Kubernetes Resources

Requesters can query for resources that they can request to, for `tsh` this is the [tsh request search](https://goteleport.com/docs/admin-guides/access-controls/access-requests/resource-requests/#search-for-kubernetes-resources) command. The `request.kubernetes_resources` will be applied when querying for Kubernetes resources.

For example, if the user has this role assigned:

```yaml
kind: role
metadata:
  name: requester-role-1
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
  deny:
    request:
      kubernetes_resources:
      - kind: "pod"
```

And the user tries to query for requestable pods:

```bash
$ tsh request search --kind=pod --kube-cluster=some-cluster-name
```

The query request will be rejected with `access denied`, since users will be denied requesting to pods.

## Future Field Support

In future iterations we can add support for more fields in the [KubernetesResource](https://github.com/gravitational/teleport/blob/c49eb984648b506f3223804784a54ddec8d4d2d3/api/proto/teleport/legacy/types/types.proto#L3293) structure to provide admins more ways to restrict a request.

For example given these roles:

```yaml
kind: role
metadata:
  name: kube-access
spec:
  allow:
    kubernetes_resources:
      # Only allow lists namespaces starting with "coffee-" prefixes
      - kind: namespace
        name: 'coffee-*'
        verbs:
          - list

-----

kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles:
      - kube-access
      kubernetes_resources:
      - kind: 'namespace'
        # Adding support for this field
        name: 'coffee-latte
```

The `kube-access` role will allow user to list and select namespaces starting with `coffee-*` and the `request.kubernetes_resources` will further restrict this to only allow requesting for namespace `coffee-latte`.

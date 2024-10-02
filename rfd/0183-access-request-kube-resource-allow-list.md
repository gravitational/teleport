---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 183 - Access Request Kubernetes Resource Allow List

# Required Approvers

- Engineering: @r0mant && @tigrato
- Product: @klizhentas || @xinding33

# What

Allow admins to specify what kind of Kubernetes resources a user can request.

# Why

Currently there are no access request settings that allows admins to enforce a certain Kubernetes resource request. Current settings allow users to request [all kinds of resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201). For kind `kube_cluster`, users can request to any of these [subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

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

This RFD proposes a new role request option that will enforce users to request for Kubernetes subresources instead.

# User Scenarios

Listing use cases from most permissive to most restrictive:

## As an admin, I don't care what kind of Kubernetes resources a user requests

As a requester, this means I can request for any supported Kubernetes resources (`kube_cluster` and its subresources).

This will be the default behavior if no request options are specified. This will also be the default behavior of existing roles unless modified.

## As an admin, if a user requests for namespaces, I want to limit what namespaces a user can request

This is already supported. The admin can use `kubernetes_resources` field to restrict what list of namespaces the requester can list and request for:

```yaml
kind: role
metadata:
  name: kube-access-request
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
```

Requesting for namespace not in this list is denied.

## As an admin, if a user requests for namespaces, I want to limit namespaces using role templates and traits

Trait interpolation will be supported in [v17](https://github.com/gravitational/teleport/pull/45277) for `kubernetes_resources` field:

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
  name: kube-access-request
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
      - '{{internal.kubernetes_users}}'
```

## As an admin, I want to require users to request for Kubernetes subresources instead of the whole Kubernetes cluster

Admins can specify in a role, a Kubernetes request mode option that requires users to select [Kubernetes subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

```yaml
kind: role
metadata:
  name: requester
spec:
  options:
    request_mode:
      # Disables requesting whole Kubernetes cluster, but can request any of the kube subresources
      kubernetes: ['resource']
```

User requesting kube_cluster will be denied.

## As an admin, I want to limit Kubernetes resource requesting to pods and namespaces only

Admins can specify in a role, a Kubernetes request mode option that requires users to select either pods or namespaces.

```yaml
kind: role
metadata:
  name: requester
spec:
  options:
    request_mode:
      # Can request only pods or namespaces
      kubernetes: ['pod', 'namespace']
```

User requesting any other Kubernetes kind will get denied.

# Details

## New request field for role spec

We will introduce a new role option under the `options` section named `request_mode`:

```yaml
kind: role
metadata:
  name: role-name
spec:
  options:
    # new field
    request_mode:
      kubernetes: [<list of kinds>]
```

The `options.request_mode.kubernetes` list will accept the same const we use in our backend [kube_cluster](https://github.com/gravitational/teleport/blob/5924c97d7fc831aeed20990e4c72fe6a0bc95fba/api/types/constants.go#L203) and any of its [Kubernetes subresource kinds](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

An admin may not care what kind of subresources a user requests, as long as the user is not requesting the whole Kubernetes cluster. For this case we will introduce a separate const `resource` that means a user can request to any Kubernetes subresources.

| Request Mode Values                           |                           Explanation                            |
| --------------------------------------------- | :--------------------------------------------------------------: |
| []                                            |                         no restrictions                          |
| `options.request_mode.kubernetes` not defined |                         no restrictions                          |
| [kube_cluster, resource]                      |                         no restrictions                          |
| [resource]                                    | requires users to request for any of the Kubernetes subresources |
| [kube_cluster]                                |    requires users to request for the whole Kubernetes cluster    |
| [namespace]                                   |          requires users to request for only namespaces           |
| [namespace, pod]                              |      requires users to request for only namespaces OR pods       |

The `options.request_mode.kubernetes` values will be validated for expected value upon role create or update.

## Request Validation

During the request creation flow is where we will validate/enforce the `options.request_mode.kubernetes`.

1. Determine if requested `kind` is in the `options.request_mode.kubernetes` list
1. Allow creation if request kind is in allow list, otherwise reject creation with error message: `Not allowed to request Kubernetes resource kind "<KIND>". Allowed kinds: "<options.request_mode.kubernetes>"`

## New proto fields

Data structure that will hold different modes:

```proto
message AccessRequestMode {
  repeated string Kubernetes = 1 [(gogoproto.jsontag) = "kubernetes,omitempty"];
}
```

New field for RoleOptions:

```proto
message RoleOptions {
  ...other existing fields

  // RequestMode optionally allows admins to define a create request mode for applicable resources.
  // It can enforce a requester to request only certain resources.
  // Eg: Users can make request to either a resource kind "kube_cluster" or any of its
  // subresources like "namespaces". The mode can be defined such that it prevents a user
  // from requesting "kube_cluster" and enforces requesting any of its subresources.
  AccessRequestMode RequestMode = 32 [(gogoproto.jsontag) = "request_mode,omitempty"];
}
```

New field for AccessCapabilities. AccessCapabilities is how the web UI and teleterm will determine to conditionally render support for selecting subresources (currently only namespaces).

```proto
message AccessCapabilities {
  ...other existing fields

  AccessRequestMode RequestMode = 7 [(gogoproto.jsontag) = "request_mode,omitempty"];
}
```

## Web UI (and Teleterm)

The web UI currently only supports requesting for kind `kube_cluster`. With this RFD implementation, we will add support for selecting `namespaces` as well. See [figma design](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity?node-id=2220-50559&node-type=frame&t=sPqQkrRd0mRRXlzO-0) for namespace selecting.

These are the current states the web UI can take regarding Kubernetes resources depending on `options.request_mode.kubernetes` list:

- if list is empty, the UI will render whatever UI options we support, which is currently just allowing user to either select `kube_cluster` or `namespace`
- if list contains only `kube_cluster`, the UI will not render options for selecting namespaces
- if list contains only `namespace`, the UI will prevent user from clicking `submit` button until user has selected at least one namespace for the selected `kube_cluster`
- if list contains kinds not supported by the web UI, the UI will prevent user from clicking `submit` button, with a disabled message `Your role requires requesting Kubernetes resource <kind>'s, but the web UI does not support this request yet. Use "tsh request create --resource /<teleport-cluster-name>/<subresource kind>/<kube-cluster-name>/<subresource-name>" instead`

## CLI

tsh already has support for requesting [all these resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201).

There is nothing to add to the CLI, we will just let it error out upon request creation validation.

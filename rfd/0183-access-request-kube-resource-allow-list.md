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

## As an admin, I want to limit what namespaces a user can request

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

## As an admin, I want to limit namespaces using role templates and traits

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
  allow:
    request:
      # Disables requesting whole Kubernetes cluster
      kubernetes:
        request_mode: 'resource'
```

# Details

## New request field for role spec

We will introduce a new role option under the `request` field `kubernetes.request_mode`:

```yaml
kind: role
metadata:
  name: role-name
spec:
  allow:
    request:
      kubernetes:
        request_mode: cluster | resource | both
```

| Request Modes |                             Explanation                             |
| ------------- | :-----------------------------------------------------------------: |
| cluster       |     requires users to request for the whole Kubernetes cluster      |
| resource      |        requires users to request for Kubernetes subresources        |
| both          | users can request for either Kubernetes cluster or its subresources |

The request_mode will be validated for expected value upon role create or update.

If `request_mode` field is not defined, that means any Kubernetes resource can be requested.

## Validation

During the request creation flow is where we will validate/enforce the `kubernetes.request_mode`.

1. We will determine if requested `kind` is allowed:
   - request mode `cluster`: will only accept request kind `kube_cluster`
   - request mode `resource`: will only accept supported Kubernetes resource kinds, check similarly done [here](https://github.com/gravitational/teleport/blob/3310dfcba358e761c8be42fcb62b4ed27e43737d/api/types/resource_ids.go#L28)
1. Allow creation if request meets request specs, otherwise reject creation with error message: `Your role requires you to request for Kubernetes <cluster | subresources (eg: namespace, pods)>`

## New proto field in AccessRequestSpecV3

The web UI currently does `dry runs` of access request to get info like `max duration`, `suggested reviewers`, `requestable roles`, etc, that determines what to render for the user. The dry run response will also need to include a field to hold `kubernetes.request.mode` to determine what Kubernetes options to render for the user:

In the [AccessRequestSpecV3](https://github.com/gravitational/teleport/blob/fd6cc06ea7e766c5d0ab7d113a5ecd04114aca34/api/proto/teleport/legacy/types/types.proto#L2462) proto type, we will add another field:

```proto
message AccessRequestSpecV3 {
    ...other existing fields
    string kubernetes_request_mode = 22;
}
```

This field will only be set during dry runs.

## Web UI

The web UI currently only supports requesting for kind `kube_cluster`. With this RFD implementation, we will add support for selecting `namespaces` as well. See [figma design](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity?node-id=2220-50559&node-type=frame&t=sPqQkrRd0mRRXlzO-0) for namespace selecting.

These are the current states the web UI can take regarding Kubernetes resources depending on `kubernetes.request_mode` list:

- if request mode is `both` or empty, the UI will render whatever UI options we support, which is currently just allowing user to either select `kube_cluster` or `namespace`
- if request mode is `cluster`, the UI will not render options for selecting subresources (namespace)
- if request mode is `resource`, the UI will prevent user from clicking `submit` button until user has selected at least one subresource (namespace) for the selected `kube_cluster`

## CLI

tsh already has support for requesting [all these resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201).

There is nothing to add to the CLI, we will just let it error out upon request creation validation.

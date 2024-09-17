---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 183 - Access Request Kubernetes Resource Allow List

# Required Approvers

- Engineering: @r0mant (roman) && @tigrato (tiago)

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

This RFD proposes a new request option that will enforce users to request for Kubernetes subresources instead.

# User Scenarios

Listing use cases from most permissive to most restrictive:

## As an admin, I don't care what kind of kubernetes resources a user requests

As a requester, this means I can request for any supported kubernetes resources (`kube_cluster` and its subresources).

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

## As an admin, I want to limit Kubernetes resource requesting to pods and namespaces only

Currently, users can request any of [these supported Kubernetes subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

Admins can specify in a role, a request Kubernetes allow list that limits request to just `pod` and `namespace` kind:

```yaml
kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      # Only allows requesting for kube_cluster's pod and namespace resource
      kubernetes:
        allow: ['pod', 'namespace']
```

User requesting any other Kubernetes kind will get denied.

## As an admin, I want to require users to only request namespaces

Admins can specify in a role, a request Kubernetes allow list that only defines `namespace`.

```yaml
kind: role
metadata:
  name: requester
spec:
  allow:
    request:
      # Only allows requesting for kube_cluster's namespace resources
      kubernetes:
        allow: ['namespace']
```

## As an admin, I want to require users to specify a namespace from select few namespaces

Admin's can combine the use of request kuberenetes allow list and `kubernetes_resources` field to customize what users can request to:

```yaml
kind: role
metadata:
  name: kube-access-request
spec:
  allow:
    request:
      # Only allows requesting for kube_cluster's namespace resources
      kubernetes:
        allow: ['namespace']
    # Only lists namespaces starting with "pumpkin-" prefixes
    kubernetes_resources:
      - kind: namespace
        name: pumpkin-*
        verbs:
          - list
```

# Details

## New request field for role spec

We will introduce a new role option under the `request` field `kuberenetes.allow` that holds a list of supported [Kubernetes resource kinds](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

```yaml
kind: role
metadata:
  name: role-name
spec:
  allow:
    request:
      kubernetes:
        allow: <list of supported Kubernetes resource strings>
```

This allow list will be validated upon role create or update, to only allow `kube_cluster` or [its subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233).

If this field is not defined, that means any Kubernetes resource can be requested (which is how it works today).

If defined, it limits users to what kind of Kubernetes resources they can request.

This is particularly useful for the web UI. We can pre-fetch the request options (as a [dry run](https://github.com/gravitational/teleport/blob/fd6cc06ea7e766c5d0ab7d113a5ecd04114aca34/api/types/access_request.go#L129)), and render only the options that is defined in the allow list.

## Validation

During the request creation flow is where we will validate/enforce the requestable kind.

1. We will determine if requested `kind` is among the list of supported Kubernetes resources, similarly done [here](https://github.com/gravitational/teleport/blob/3310dfcba358e761c8be42fcb62b4ed27e43737d/api/types/resource_ids.go#L28)
1. We will determine if the requested kind is in the `request.kubernetes.allow` list (gathered from all the static roles assigned to user, similar to how we get a list of [suggested reviewers](https://github.com/gravitational/teleport/blob/3310dfcba358e761c8be42fcb62b4ed27e43737d/lib/services/access_request.go#L1191)).
1. Allow creation if request meets request specs, otherwise reject creation with error message: `Your role does not allow requesting for Kubernetes resource "<kind>". Allowed Kubernetes resources: [list of kinds].`

## New proto field in AccessRequestSpecV3

The web UI currently does `dry runs` of access request to get info like `max duration`, `suggested reviewers`, `requestable roles`, etc, that determines what to render for the user. The dry run response will also need to include a field to hold `allowed Kubernetes kind` to determine what Kubernetes options to render for the user:

In the [AccessRequestSpecV3](https://github.com/gravitational/teleport/blob/fd6cc06ea7e766c5d0ab7d113a5ecd04114aca34/api/proto/teleport/legacy/types/types.proto#L2462) proto type, we will add another field:

```proto
message AccessRequestRequestableKinds {
    string kubernetes = 1;
}

message AccessRequestSpecV3 {
    ...other existing fields
    AccessRequestRequestableKinds requestable_kinds = 22;
}
```

This field will only be set during dry runs.

## Web UI

The web UI currently only supports requesting for kind `kube_cluster`. With this RFD implementation, we will add support for selecting `namespaces` as well. See [figma design](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity?node-id=2220-50559&node-type=frame&t=sPqQkrRd0mRRXlzO-0) for namespace selecting.

These are the current states the web UI can take regarding Kubernetes resources depending on `request.kubernetes.allow` list:

- if list is empty, the UI will render whatever UI options we support, which is currently just allowing user to either select `kube_cluster` or `namespace`
- if list contains only `kube_cluster`, the UI will not render options for selecting namespaces
- if list contains only `namespace`, the UI will prevent user from clicking `submit` button until user has selected at least one namespace for the selected `kube_cluster`
- if list contains kinds not supported by the web UI, the UI will prevent user from clicking `submit` button, with a disabled message `Your role requires requesting Kubernetes resource <kind>'s, but the web UI does not support this request yet. Use "tsh request create --resource /<teleport-cluster-name>/<subresource kind>/<kube-cluster-name>/<subresource-name>" instead`

## CLI

tsh already has support for requesting [all these resources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1201).

There is nothing to add to the CLI, we will just let it error out upon request creation validation.

# Alternative

We could make everything simpler, if instead of an `allow list`, we use a `boolean` instead, so something like:

```yaml
kind: role
metadata:
  name: kube-access-request
spec:
  allow:
    request:
      kubernetes:
        force_subresource_request: true
```

This means, when true, users are not allowed to just request for a `kube_cluster`, but choose which [subresources](https://github.com/gravitational/teleport/blob/110b23aefb3c4b91c9d8cca594041b93f0e078dd/api/types/constants.go#L1233) to request.

This also simplifies the web UI where we just render all the subresource options the web UI supports at the moment.

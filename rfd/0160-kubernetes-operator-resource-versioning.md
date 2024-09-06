---
authors: @hugoShaka (hugo.hervieux@goteleport.com)
state: draft
---

# RFD 0160 - Kubernetes Operator Resource Versioning

## Required Approvers

* Engineering @r0mant && (@tigrato || @marcoandredinis)
* Security: (@reedloden || @jentfoo)
* Product: (@xinding33 || @klizhentas )

## What

This RFD discusses how we can implement multiple version support for the same
resource in the Teleport Kubernetes Operator. For example, the operator currently
only supports `RoleV5` while we have released `RoleV6` and `RoleV7` resources
with new capabilities.

## Why

Users want to manage their Teleport resources via the Teleport Kubernetes Operator.
However, in its current state, the operator does not support when we introduce a
newer version of a resource. This blocks users from leveraging new Teleport
features such as granular Kubernetes Access Control with RoleV7.

## Details

### Current state

We currently version the Teleport CRs like the Teleport resources. While this
makes sense from a user point of view, this is not compatible with the way
Teleport manages versions. Teleport does not support exposing resources through
separate per-version APIs. The version conversion happens on the Teleport side,
when establishing new defaults.

This means we end up with two sources of truth:
- the resource stored in Kubernetes.
  The Kubernetes storage is version-aware but does not convert between versions
- the resource stored in Teleport.
  The Teleport storage is not entirely version-aware as it converts all resources
  to the latest version on startup and during `CheckAndSetDefaults()`.

Both storages are treating versions differently and don't agree as to how to
represent a resource in a given version.

### Suggested approach

Put the Teleport resource version in the Kubernetes Kind and treat different
versions as different resources.

This approach completely avoids any conversion problem by not doing conversions.
This way, we don't have to deal with how Kubernetes does API versioning and the
fact it is not compatible with how Teleport manages versions.

For example, to support roles v6 and v7 we would introduce:

```yaml
kind: TeleportRoleV6
apiVersion: v1
---
kind: TeleportRoleV7
apiVersion: v1
```

All CRs could be managed by the same controller using the unstructured client,
or multiple controllers if we need it.

### Pros

- This approach allows us to continue building CRDs and relying on Kube's
  apiserver for doing schema validation and rejecting wrong CRs early in the
  process.
- This approach is quite Kubernetes friendly as Kubernetes tooling can rely on
  well-defined CRDs.
- This approach implementation cost is low and the additional complexity is
  minimal as we don't add new components and reuse our existing CRD tooling and
  generic controllers.
- This approach is backward compatible.

### Cons

- Migrating a role from v5 to v6 will take an extra step (disable reconciliation
  of v5 + remove finalizers, then create a v6 role and delete the v5?).
- `TeleportRole` vs `TeleportRoleV7` can be a bit confusing, especially when using kubectl.
  We can do a breaking change to edit the short names and make the CLI
  experience more consistent if needed. For example:

  ```code
  # get roles v5
  kubectl get teleportrolev5
  
  # get roles v6
  kubectl get teleportrolev6
  
  # get roles v7
  kubeclt get teleportrolev7
  
  # get roles v5, we could remove it but it would break
  kubectl get teleportrole
  ```
- Users can create multiple resources with the same name udner different
  versions (e.g., two CRs `TeleportRole` and `TeleportRoleV7` with the same
  name.) This would cause a non-deterministic behaviour. We can mitigate this
  risk by labeling the resource with the CR kind/version. This risk already
  exists if you run two operators against the same cluster.

### Security

This design adds little changes to the current Teleport Kubernetes Operator
security model. The only risks are:

- worse visibility as you need to `kubectl get` all versions of a resource to
  get a full view.
- potential conflicts if the same resource is defined in multiple versions with
  different specs.

### UX

- This change is backward compatible and offers a straightforward migration
  path when we introduce a new resource version.
- The atypical versioning might be confusing for users as it doesn't follow
  usual Kubernetes patterns.

### API evolution

This design is future-proof as it will accommodate any new Teleport resource or
version.

### Alternatives

When writing this RFD, two alternatives were considered:

#### Conversion hooks

The Kubernetes-friendly approach would be to make the operator aware of how the
resource is stored in Kubernetes, and do the conversions for every Kubernetes CR
API call via webhooks.

Handling resource conversion at the operator level requires the operator to
validate the resource, set its defaults, and convert between versions. This
causes several problems:
- For every other client, this is Teleport's responsibility to run
  `CheckAndSetDefaults` and to handle conversion. This makes the operator a
  client behaving differently, and blurs the responsibility between Teleport and 
  its clients.
- the operator can disagree with Teleport about the rules to validate a resource
  or set its defaults. e.g. If the operator runs a different version than
  Teleport, creating a resource via the operator and via `tctl` will result in
  different behaviors.
- the operator might not have all the information to sanely set the defaults for
  a resource, validate it, and convert it. The resource manifests are not purely
  descriptive and owned by the user, many of the resources have spec fields set
  server-side, sometimes depending on external factors such as other resources
  or third party services. This is not a good design, and we're converging to a
  formal `Spec`/`Status` separation in RFD 0153, but the existing resources are
  flawed.
- We need to handle both upward conversion and backward conversion as per
  Kubernetes versioning design. Resources created through newer APIs might not
  be representable in the old versions, and we would need to write all the
  downgrade logic as we don't have it today, potentially introducing new bugs.

This approach can also cause additional friction:
- We currently handle resource conversion only from the old version to the newer
  versions, this would require implementing bidirectional conversion between
  each resource version (or use a Hub & Spoke approach by converting every 
  version to a common version). This would increase the cost of adding new
  resource versions and increase the chances of adding bugs in the conversions
  due to the size of conversion matrix.
- We are currently working on hiding `CheckAndSetDefaults()` from the client and consolidating 
  defaults injection and resource conversion server-side. This approach is not compatible
  with the conversion hooks pattern as we'd
  need to run `CheckAndSetDefaults` in the operator and duplicate the logic.
- This requires relying on conversion webhooks, overall making the operator
  setup more difficult and harming both user experience and availability:
  - Hooks add new failure modes: the operator is not healthy, kubernetes cannot
    talk to the operator.
  - Not all Kubernetes distributions support kube control plane to workload
    communication by default. Examples can be found [in cert-manager's
    documentation](https://cert-manager.io/docs/concepts/webhook/#known-problems-and-solutions).
  - Kubernetes must trust the webhook server. This implies creating a CA,
    issuing certs, and inserting x509 material in the CRD resource.
- The cost of implementing this solution is way higher than the other alternatives.

#### Putting the version in the CR

We can break the relation between the CRD version and the Teleport resource, and
specify the version in the CR spec. This means users would use a single version
of `resources.teleport.dev` for all their resources.

Before, an admin would create a RoleV5 by creating a `TeleportRole` through via the API
`resources.teleport.dev/v5` and a UserV2 through the api `resources.teleport.dev/v2`.

```yaml
apiVersion: resources.teleport.dev/v5
kind: TeleportRole
metadata:
  name: myrole
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','create','read','update','delete']
---
apiVersion: resources.teleport.dev/v2
kind: TeleportUser
metadata:
  name: myuser
spec:
  roles: ['myrole']
```

With this approach, both `TeleportRole` and `TeleportUser` resources would be created
through the `resources.teleport.dev/vX` API. The Teleport resource version would
be specified in a separate field: `teleportResourceVersion`.

```yaml
apiVersion: resources.teleport.dev/vX
teleportResourceVersion: "v5"
kind: TeleportRole
metadata:
  name: myrole
spec:
  allow:
    rules:
      - resources: ['user', 'role']
        verbs: ['list','create','read','update','delete']
---
apiVersion: resources.teleport.dev/vX
teleportResourceVersion: "v2"
kind: TeleportUser
metadata:
  name: myuser
spec:
  roles: ['myrole']
```

The version `vX` in `resources.teleport.dev/vX` needs to be higher than the highest
current version (TeleportRole is served under `v6`). We can set it to the current
Teleport version (`v15` or `v16` depending on the timing).

This approach has the following limitations:
- This design doesn't follow usual Kubernetes resource versioning.
- Settling on `vX` can be confusing. See [the API evolution section](#api-evolution).
- This design relies on the fact we don't radically change the resource
  structure. See [the API evolution section](#api-evolution). If this were to
  happen, we would need to relax the CRD and stop validating. Users would be able
  to create invalid CRs and not receive instant feedback.

#### Introducing a new API

This is a variant of
the ["putting version in the CR"](#putting-the-version-in-the-cr) approach, but
instead of using `resources.teleport.dev/vX` with `vX` being the Teleport
version when this was implemented, we introduce a new `v1` API.

For example: `operator.teleport.dev/v1`.

This is cleaner and semantically easier to understand than using `vX`. However, this does
not provide easy upgrade paths from existing resources under `resources.teleport.dev` to
the new API.

One workaround would be to write lightweight controllers reconciling resources
`resources.teleport.dev` with `operator.teleport.dev` but this would add complexity
that might not be compensated by the benefits which are mostly cosmetic.

### Test Plan

The test plan is the following:
- validate that existing CRs are reconciled the same way by the operator
- validate that users can create `TeleportRoleV6` and `TeleportRoleV7` once this is implemented

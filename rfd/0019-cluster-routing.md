---
authors: Russell Jones (rjones@goteleport.com)
state: draft
---

# RFD 19 - Cluster Routing

## What

This RFD proposes changes to how Teleport issues certificates and routes requests within clusters. These changes would make using certificate issues by a root cluster directly on leaf clusters substantially easier.

## Why

At the moment connections to a leaf clusters typically go through a root cluster. If the latency to the root cluster is high, accessing servers within a leaf cluster leads to a frustrating user experience. Allowing direct use of certificates issues by a root cluster on a leaf cluster mitigates this problem.

Teleport supports this use case.

Users can run `tsh login clusterName` to obtain a certificate with `RouteToCluster` set to a leaf cluster then use `tsh ssh -J clusterName severName` to bypass the root cluster. However, switching between multiple leaf clusters is cumbersome as `tsh login clusterName` has to be issued each time the user wishes to switch between clusters and it also makes working on multiple clusters concurrently almost impossible.

## Details

To support this use case, this RFD proposes `tsh login` behavior be changed to issue certificates with an empty `RouteToCluster` field. An empty `RouteToCluster` field on a certificate would indicate to a cluster that the request should be routed locally baring any other routing directives.
 
This would allow users to use `tsh ssh -J clusterName serverName` and work on multiple leaf clusters concurrently without having to obtain a new certificate. When these requests arrive on the leaf cluster, the leaf cluster would route them locally instead of attempting (and failing) to route them to the root cluster.

No changes would be made to role mapping or to the behavior when the `RouteToCluster` field is set.

### Considerations

#### Wildcard

An empty field was chosen over the wildcard symbol `*` as a user could potentially have a cluster named `*`.

#### Other Protocols

Similarly to `tsh ssh -J`, Kubernetes Access users would use `kubectl --server=clusterName` and Database Access Users would use something like `psql "host=clusterName"`.

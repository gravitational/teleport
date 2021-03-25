---
authors: Russell Jones (rjones@goteleport.com), Andrej Tokarcik (andrej@goteleport.com)
state: draft
---

# RFD 21 - Cluster Routing

## What

This RFD proposes changes to how Teleport issues certificates and routes SSH requests within clusters. These changes would make using certificate issued by a root cluster directly on leaf clusters substantially easier.

## Why

At the moment connections to a leaf cluster typically go through a root cluster. If the latency to the root cluster is high, accessing servers within a leaf cluster leads to a frustrating user experience. Allowing direct use of certificates issued by a root cluster on a leaf cluster mitigates this problem.

Teleport supports this use case.

Users can run `tsh login clusterName` to obtain a certificate with `RouteToCluster` set to a leaf cluster then use `tsh ssh -J clusterName serverName` to bypass the root cluster. However, switching between multiple leaf clusters is cumbersome as `tsh login clusterName` has to be issued each time the user wishes to switch between clusters. It also makes working on multiple clusters concurrently almost impossible.

## Original Solution: jumphost address to cluster name

The behavior of `tsh ssh -J` would be changed to request a certificate re-issue for the target cluster if the proxy address does not match the root proxy address (saved in the profile).

In that situation the first step would be to go to the root proxy, loop over all remote clusters and check if there is a match between the given jumphost address and one of the leaf proxies and if so re-issue the certificate with a `RouteToCluster` pointing to the leaf cluster. To prevent re-issue performance hit we should cache certificates at the client as in [PR #5938](https://github.com/gravitational/teleport/pull/5938/).

This would allow users to use `tsh ssh -J clusterName serverName` to work on multiple leaf clusters concurrently.

### Problems

1. It is not immediately clear what data the jumphost address should be matched against. Clusters, and especially trusted clusters, need not be configured with options like `ssh_public_addr` or `public_addr`. Even if these options were defined there is no guarantee to their accuracy. There might be other, equally functional public addresses that can be entered by some users. It seems possible to even have multiple trusted clusters that are configured with identical `*public_addr` values (whether by accident or as part of some unconventional setup).

2. The re-issue request itself is impacted by the `-J` override since `-J` sets the proxy address not only for SSH node connections but also for more general Auth API requests. Hence, if the usual [`TeleportClient.ReissueUserCerts`](https://github.com/gravitational/teleport/blob/026d3419c2454163678de9b43d5c69b81702fb7f/lib/client/api.go#L1092) were called, it would send this [request to the jumphost](https://github.com/gravitational/teleport/blob/026d3419c2454163678de9b43d5c69b81702fb7f/lib/client/api.go#L1910-L1921) (i.e., leaf proxy) instead of to the root proxy. (Even though apparently deliberate, this behavior may be confusing. One would expect only `--proxy` to be applicable to such requests, not `-J`.)

## Improvement: cluster name inferred from jumphost cert

To address Problem 1, we should not use the jumphost address as such to establish the cluster name. Instead, the host certificate presented while connceting to the jumphost should be analyzed and if it indeed belongs to a Teleport proxy, it should come with an [extension containing its cluster name](https://github.com/gravitational/teleport/blob/026d3419c2454163678de9b43d5c69b81702fb7f/lib/auth/native/native.go#L225). From that it should be reliably known which `RouteToCluster` value is expected of the user certificate, and request its re-issue if it is not available.

However, Problem 2 still remains. It seems unavoidable to simply ignore the `-J` override for the needed cert re-issual request. Ignoring `-J` would presumably make the client fall back to the root proxy capable of issuing an SSH certificate pointing to the leaf cluster.

## Future Work

It is not necessary to force users to pass leaf proxy addresses using `-J`. When establishing a trust relationship, the trusted cluster could in its `trusted_cluster` resource indicate that it should be directly available to the clients of the root proxy, together with an appropriate jumphost/public address. (Note: This is not always the case. The trusted cluster relationship may be used not only to establish trust but also to expose network connectivity to otherwise unavailable leaf clusters.)

These indications of being "direct-jumphost-friendly" would be received by the root cluster and recorded in its own corresponding `remote_cluster` resource. Consequently, whenever the root auth should issue a SSH certificate with `RouteToCluster` pointing to such a leaf cluster, it would also set a "prefer jumphost" extension in the new certificate including the jumphost address advertised by the leaf. Clients would be advised to automatically use the jumphost when connecting to the leaf cluster, bypassing the root proxy, without any explicit action to be taken by the user.

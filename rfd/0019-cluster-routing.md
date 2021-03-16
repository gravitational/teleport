---
authors: Russell Jones (rjones@goteleport.com)
state: draft
---

# RFD 19 - Cluster Routing

## What

This RFD proposes changes to how Teleport issues certificates and routes SSH requests within clusters. These changes would make using certificate issued by a root cluster directly on leaf clusters substantially easier.

## Why

At the moment connections to a leaf cluster typically go through a root cluster. If the latency to the root cluster is high, accessing servers within a leaf cluster leads to a frustrating user experience. Allowing direct use of certificates issued by a root cluster on a leaf cluster mitigates this problem.

Teleport supports this use case.

Users can run `tsh login clusterName` to obtain a certificate with `RouteToCluster` set to a leaf cluster then use `tsh ssh -J clusterName serverName` to bypass the root cluster. However, switching between multiple leaf clusters is cumbersome as `tsh login clusterName` has to be issued each time the user wishes to switch between clusters. It also makes working on multiple clusters concurrently almost impossible.

## Original Solution: `-J` only, proxy to cluster mapping

The behavior of `tsh ssh -J` would be changed to request a certificate re-issue for the target cluster if the proxy address does not match the root proxy address (saved in the profile).

In that situation the first step would be to go to the root proxy, loop over all remote clusters and check if there is a match between the given jumphost address and one of the leaf proxies and if so re-issue the certificate with a `RouteToCluster` pointing to the leaf cluster. To prevent re-issue performance hit we could cache certificates either at the proxy (similar to what we do on the recording proxy) or at the client.

This would allow users to use `tsh ssh -J clusterName serverName` to work on multiple leaf clusters concurrently.

### Problems

It is not immediately clear what data the jumphost address should be matched against. Clusters, and especially trusted clusters, need not be configured with options like `ssh_public_addr` or `public_addr`. Even if these options were defined there is no guarantee to their accuracy. There might be other, equally functional public addresses that can be entered by some users. It seems possible to even have multiple trusted clusters that are configured with identical `*public_addr` values (whether by accident or as part of some unconventional setup).

If a jumphost input resolution fails, the client will fall back to routing through the root cluster, in spite of the user's expectation to the contrary. This may cause poor user experience as there is no easy way for an ordinary user to find out the exact leaf proxy address in the form known to the root cluster.

## Alternative: `-J` with `--cluster`

Another approach is based on the idea to split the problem described in the RFD into two parts. The goals are:

1. to bypass the root proxy when connecting to leaf clusters; and
2. to conveniently (yet reliably) fetch the appropriate cert for the targeted leaf cluster, without requesting repeated reissues from the root auth.

Ad 1: The `-J` flag covers this aspect. Note that the jumphost is not required to be a Teleport proxy. In general it could be any other intermediary able to provide the expected routing functionality.

Ad 2: See [PR #5938](https://github.com/gravitational/teleport/pull/5938/) (*Cache per-cluster SSH certificates under ~/.tsh*). SSH certs issued for the user are to be stored under `~/.tsh` and reused on an as-needed basis. The `--cluster` flag is also made to trigger SSH cert reissue when a new cluster is selected -- the cert is then loaded just for the scope of the command, i.e. without changing the cluster selected in the profile.

### Problems

Unfortunately, the combination of `-J` and `--cluster` does not allow simple one-liners like
```bash
tsh ssh --cluster=$leafCluster -J $leafProxy ...
```
in full generality, i.e. for an arbitrary leaf cluster right away. The problem is that the implicit cert reissue requests sent by `--cluster` are also impacted by the `-J` override. The requests are thus sent to the leaf proxy that is unable to handle such requests properly.

Wouldn't it be possible to ignore `-J` for the reissue requests? Unfortunately, [some Teleport code](https://github.com/gravitational/teleport/blob/cd399e704c45f1ff8dfec6cb93262597de7ac3ba/lib/client/api.go#L1756-L1759) indicates the use of `-J` to direct `tsh` to the root proxy is indeed a deliberate feature. Can we confirm there would be a risk of breaking backward compatibility involved if `-J` were ignored? Shouldn't `--proxy` and `--user` be preferred for this purpose anyway?

### Functional Workflow

It is possible to work around this obstacle by manually pre-fetching all the needed SSH certificates. The following `bash` workflow allowing concurrent access to multiple clusters is already fully supported, modulo #5938:

```bash
# create a map of cluster -> proxy
declare -A clusters
clusters=([leafA]=proxy.example.com:5003 [leafB]=gravitational.io:3023)

# iterate over the clusters
for cluster in ${!clusters[@]} ; do
  # fetch SSH cert for the cluster
  tsh login $cluster
  # define a convenient alias for connecting to the cluster
  alias ssh-$cluster="tsh ssh --cluster=$cluster -J ${clusters[$cluster]}"
done
```

After a similar preparation, it becomes possible to switch between the listed clusters absolutely freely, by using `-J` in conjunction with `--cluster`.

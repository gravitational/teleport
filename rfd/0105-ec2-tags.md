---
authors: Roman Tkachenko (roman@goteleport.com), Scale Team, Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 105 - Importing EC2 tags via API

## Required approvers

Engineering: @fspmarshall, @espadolini, @rosstimothy

## What

Proposes a way to import instance tags using AWS EC2 API instead of metadata
endpoint during EC2 auto-discovery.

## Why

Teleport nodes running on AWS currently automatically import EC2 instance tags
as server labels using instance metadata endpoint, as was implemented by the
[RFD 72](https://github.com/gravitational/teleport/blob/master/rfd/0072-ec2-tags.md).

Some users disable metadata endpoint due to security concerns but still would
like to have Teleport import server labels from AWS.

This RFD describes a mechanism by which instance labels will instead be fetched
by the discovery service and reconciled with the node's own labels.

## Scope

This feature will only apply to nodes enrolled in the cluster using auto-discovery
mechanism. Other nodes, even if they're running on EC2, will keep functioning
as-is (at least for now), i.e. importing tags from EC2 metadata endpoint if it's
available.

## How

We will discuss design and implementation of 3 components of the solution:

- Fetching EC2 instance tags.
- Reconciling tags during heartbeat.
- Propagating tags to the node.

### Fetching tags

Teleport discovery service is currently responsible for scanning user's AWS
account on a regular cadence and enrolling matching instances into the cluster
([RFD 57](https://github.com/gravitational/teleport/blob/master/rfd/0057-automatic-aws-server-discovery.md)).
It is a service that users run on their own infrastructure.

Discovery service uses `DescribeInstances` API. Right now the returned instance
list is only used for cluster enrollment (i.e. running appropriate commands on
them through SSM). With the changes proposed in this RFD, the discovery service
will convert each EC2 `Instance` into a new helper Teleport resource that we'll
call `ServerInfo`. `ServerInfo` will include the instance's tags as
labels, as well as other supporting information like instance ID, account ID
and region. Each label key will be prefixed with `aws/`.

After each discovery loop cycle, the discovery service will save all new and
updated `ServerInfo` resources in the cluster backend. Discovery service
maintains an in-memory cache of all discovered EC2 instances and will only
upsert resources that are new or had their labels changed.

To reduce load on auth and backend, discovery service will send them in batches
of minimum size of 5 using `UpsertServerInfos()` API and will send 1
batch per second at a maximum. The loop will target propagation of all labels
within a max interval of 15 minutes which means for very large clusters some
batch sizes will need to be increased. The algorithm will be:

- Get all EC2 instances. Let's say, we have N of them.
- Determine batch size. We divide N by our minimum batch size, say 5.
  - If the result is <= 900, we can send them all in minimum batches over 15
    minutes. We send them 5 per second. 100 node cluster will have labels
    propagated within 20 seconds, and we'll support up to 4.5K clusters with
    minimum batch size.
  - If the result is > 900, we need to increase the batch size so we can send
    them all within 15 minutes.

Rough pseudocode illustrating this algorithm:

```go
instances := getInstances()
batchSize := defaults.MinBatchSize
if dynamicBatchSize := (len(instances) / 900) + 1; dynamicBatchSize > batchSize {
    batchSize = dynamicBatchSize
}
for len(instances) > 0 {
    send(instances[:batchSize])
    instances = instances[batchSize:]
    <-interval.Next()
}
```

`ServerInfo` resources will have a 90 minute TTL (with some jitter) and
the discovery service will re-upsert the resource 30 minutes prior to its
expiration, in addition to when the resource changes in the local cache.

### Reconciling tags

`ServerInfo` resources will not be cached on the auth server because in
the worst case scenario it will double the number of resources the cache has to
load during initialization leading to extra backend strain.

Instead, each auth server will maintain a single "reconciler" goroutine that
will periodically iterate over all discovered servers (so there's no performance
hit in clusters not using auto-discovery) and update labels for each connected
instance matching the discovered server. Node heartbeats will be updated to
include AWS metadata information to be able to match the node to a discovered
server.

The reconciler will be rate-limited and load `ServerInfo` resources in
batches of 100 (using `GetRange()`) at the rate of one batch per second.

When applying tags from discovered servers all `aws/` labels heartbeat by the
node will be cleared and replaced with `aws/` labels from the discovered server.

### Propagating tags

Nodes need to be aware of their labels to be able to make RBAC decisions. This
means that discovered labels need to be propagated to nodes. The reconciler will
use the existing inventory control stream mechanism to do this:

https://github.com/gravitational/teleport/blob/v12.0.0/lib/inventory/inventory.go

A new control stream message will be added to the list of existing messages,
`UpdateLabels`:

https://github.com/gravitational/teleport/blob/v12.0.0/api/proto/teleport/legacy/client/proto/authservice.proto#L1913-L1922

On the auth server side a corresponding method will be added to the `UpstreamHandle`:

https://github.com/gravitational/teleport/blob/v12.0.0/lib/inventory/inventory.go#L270

The reconciler will execute `auth.Server.inventory.GetControlStream(serverID)`
to get the handle for the connected node and call `UpdateLabels` which will
both send the downstream message and cache the labels for the node's future
heartbeats.

On the agent side, the `DownstreamHandle` will be updated to receive and process
the discovered labels message:

https://github.com/gravitational/teleport/blob/v12.0.0/lib/inventory/inventory.go#L201-L204

The new discovered labels control stream message implementation will closely
follow the flow of the existing Ping message as an example.

## Future work

In addition to discovered nodes, `ServerInfo` resources could be generalized to
act as a label authority for any nodes. `ServerInfo`s would be editable via
`tctl` and the Teleport API, allowing users to bulk set labels for groups of
nodes.

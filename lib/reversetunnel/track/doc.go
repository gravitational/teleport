// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package track provides a simple interface to keep track of proxies as described
via "gossip" messages shared by other proxies as part of the reverse tunnel
protocol, and to decide if and when it's appropriate to attempt a new connection
to a proxy load balancer at any given moment.

The [Tracker] object gives out [Lease] objects through its TryAcquire() function
whenever it's appropriate to attempt a new connection - the Lease object
represents the permission to open a new connection and to try and Claim
exclusivity over the remote proxy that was reached.

A Lease starts "unclaimed", and can be switched into a "claimed" state given a
proxy name; if no other Lease has claimed the same proxy, the claim is
successful; otherwise, the Lease should be released with Release() so that a new
connection can be spawned in its stead. The Lease should also be released if the
connection fails, or after it's closed.

The Tracker keeps track of how many Leases are in unclaimed state, and of which
proxies have been claimed. In addition, it receives gossip messages (usually
from the reverse tunnel clients themselves, through the reverse tunnel "gossip"
protocol, see `handleDiscovery` in lib/reversetunnel/agent.go) containing
information about proxies in the cluster - namely, which proxies exist, and
which group ID and generation, if any, they belong to.

A proxy can report a group ID and a group generation (it's valid for a proxy to
belong to the "" group or the zero generation); the group ID represents
independent deployments of proxies, the group generation is a monotonically
increasing counter scoped to each group.

The Tracker will grant Leases (to attempt to connect to new proxies) based on
which proxies have been claimed, how many Leases have been granted but have yet
to claim a proxy (they're still "inflight") and which proxies are known and
which group ID and generation they have reported:

  - the set of desired proxies is defined to be the union of all proxies in the
    biggest known generation of each proxy group; if proxies A, B and C belong
    to the same group but proxy A is in generation 2 while B and C are in
    generation 1, only proxy A is in the desired set
  - if in proxy peering mode, the count of desired proxies is limited by the
    connection count, such that if the connection count is 40 but only 3 proxies
    are in the desired set, the effective count is going to be 3 - however, if
    the connection count is 3 and there's 40 proxies in the desired set, the
    effective count is also going to be 3
  - the desired proxy count is compared with the sum of the count of currently
    claimed proxies in the desired set and the current amount of inflight
    connections (Lease objects that haven't been released and haven't claimed a
    proxy); new connections can be spawned if the desired proxy count is
    strictly greater.

Proxies are removed from the tracked set after a TTL (defaulting to 3 minutes),
which is refreshed whenever they're mentioned in a gossip message.
*/
package track

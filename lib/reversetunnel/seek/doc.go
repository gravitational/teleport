/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Proxy-Seeking alogirthm
//
// Premise: An unknown number of proxies exist behind a "fair"
// load-balancer. Proxies will share their peerset via gossip, but
// this gossip is asynchronous and may suffer from ordering/timing
// issues. Furthermore, rotations may cause a complete and permanent
// loss of contact with all known proxies.
//
// Goals: Ensure that we have one agent managing a connection to each
// available proxy.  Minimize unnecessary discovery attempts. Recover
// from bad state (e.g. due to full rotations) in a timely manner.
// Mitigate resource drain due to failing or unreachable proxies.
//
//
// Each known proxy has an associated entry which stores
// its seek state (seeking | claimed | backoff).
//
// When an agent discovers (connects to) a proxy, it attempts to
// acquire an exclusive claim to that proxy.  If sucessful, the agent
// takes responsibility for the proxy, releasing its claim when the
// connection terminates (regardless of reason).  If another agent
// has already claimed the proxy, the connection is dropped.
//
// Unclaimed entries are subject to expiry.  Expiration timers are
// refreshed by gossip messages.
//
// If a claim is released within a very short interval after being
// acquired, termination is said to be premature.  Premature
// termination triggers a backoff phase which pauses discovery
// attempts for the proxy.  The length of the backoff phase is
// determined by an incrementing multiplier.  If backoff is entered
// too often to allow the counter to reset, the backoff phase will
// grow beyond the expiry limit and the associated entry will be
// removed.
//
//  +---------+
//  |         |                  acquire
//  |  START  +------------------------------------------------+
//  |         |                                                |
//  +----+----+                                                v
//       |                                               +-----+-----+
//       |      refresh               release (ok)       |           |
//       +-----+--------+   +----------------------------+  Claimed  |
//             ^        |   |                            |           |
//             |        v   v                            +--+-----+--+
//             |    +---+---+---+                           ^     |
//             |    |           |          acquire          |     |
//             +----+  Seeking  +---------------------------+     |
//                  |           |                                 |
//  +--------+      +---+---+---+                                 |
//  |        |          |   ^        +-----------+                |
//  |  STOP  |          |   |  done  |           |  release (err) |
//  |        |          |   +--------+  Backoff  +<---------------+
//  +---+----+          |            |           |
//      ^               |            +-----+-----+
//      |               v   expire         |
//      +---------------+------------------+
//
package seek

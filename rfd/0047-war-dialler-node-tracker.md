---
authors: Naji Obeid (naji@goteleport.com), David Boslee (david@goteleport.com)
state: draft
---

# RFD 47 - Node Tracker Service

## What

This is part one of a two part RFD that aims to improve node-to-proxy connectivity by allowing node agents to connect to a single proxy and be reachable through any other proxy in the same cluster.

This RFD proposes a new service that tracks node-to-proxy relationships, in other terms which nodes are connected to which proxies.
The second RFD proposes a new proxy-to-proxy tunneling system, and can be tracked [here](insert rfd link here)

## Why

The current behavior of node agents dialing over a reverse tunnel are required to connect to every proxy instance in the cluster. This allows a client to connect to a node through any proxy but it also causes other problems when running proxies behind a load balancer like in our cloud environment. These problems include:

- Ephemeral port exhaustion between a NAT gateway and load balancer. This limits the number of nodes that can be connected behind a single NAT gateway. As more proxies are added fewer nodes are able to connect.
- Thundering herd when adding, removing, or restarting a proxy. Node agents retry connecting until they randomly get balanced to the desired proxy. The more node agents connected the worse this problem becomes.

Both these issues are mitigated by changing the node agent behavior to connect to a single proxy. Ephemeral port exhaustion is no longer tied to the number of proxies and node agents no longer need to retry until they connect to a specific proxy.

## Details

Here's a small example comparing how proxies would interact with the node tracker:

```
	Original behaviour:
                       ┌──────┐
   ┌───connection─────>│proxy1│<──reverse-tunnel──┐
   │                   └──────┘                   │
┌──┴───┐                                        ┌─┴──┐
│client│                                        │node│
└──┬───┘                                        └─┬──┘
   │                   ┌──────┐                   │
   └───connection─────>│proxy2│<──reverse-tunnel──┘
                       └──────┘
	A client can reach the requested node through any proxy because the node has a reverse tunnel open to all proxies in a cluster.



    New behaviour:
                         ┌──────┐         ┌────────────┐
   ┌─(1)─connection-1───>│proxy1├──(2)───>│node tracker│
   │                     └──╥───┘         └────────────┘
┌──┴───┐                    ║
│client│               (3) tunnel
└──┬───┘                    ║
   │                     ┌──╨───┐                    ┌────┐
   └────connection-2────>│proxy2│<───reverse-tunnel──┤node│
                         └──────┘                    └────┘

	Case 1 - connection 1:
	1- A client connects to proxy-1 that can't reach the requested node.
	2- Proxy-1 sends a lookup request to the node tracker asking which proxy is connected to the requested node. The node tracker replies with the address of proxy-2.
	3- Proxy-1 opens a tunnel (RFD 48?) to proxy-2 and asks proxy-2 to dial the requested node.

	Case 2 - connection 2:
	nothing changes when a client connects to a proxy that has got a direct connection to the requested node.
```

### Scope

The first iteration of the node tracker service will be a barebones implementation of an in memory cache represented by a simple golang map.

### How it works

The node tracker service runs a grpc server and exposes three endpoints to teleport proxies:
- AddNode: which takes a `node ID`, `proxy ID` and `proxy address` as parameters among others. A Proxy will call this endpoint when it receives a heartbeat request from a node agent connected via a reverse tunnel, signaling to the node tracker that the node can be reached through this proxy, thus creating a node-to-proxy relationship.
- RemoveNode: which takes a `node ID` as a parameter. A proxy will call this endpoint when it fails to receive heartbeat requests from a node agent after a certain threshold.
- GetProxies: which takes `node ID` as a parameter. A proxy will call this endpoint when looking up which proxy is connected to a node it doesn't have a direct connection to. The node tracker will reply with the addresses of proxies that can reach the node.


The service also runs a background job responsible for cleaning stale relationships that haven't been updated before a certain threshold.

### Limits

Currently we only support running one instance of a node tracker and because of that we're missing key features usually found in distributed systems, such as:
- Scalability
- High availabitly
- Consistency
- Redundancy
- Consensus
- Sharding

Persistence is also an issue due to the nature of our in memory design. A reboot or a failure will render the majority of the nodes in the cluster unreachable while the node tracker rebuilds its database.

These will hopefully be implemented in future iterations to this project as we iron out the issues and get a better sense of the requirements.

There are some low hanging fruits that can improve the performance and reliability of the current design:
- Proxy triggered updates: Proxies can ping their connected nodes in order to trigger an update to the node tracker instead of waiting for heartbeats. This can improve the speed of rebuilding the node tracker database after a reboot.
- Add a cleanup endpoint to the node tracker to remove all node-to-proxy relationships from the node tracker for cases when a proxy is recycled or becomes unavailable.
- Caching recent lookups at the proxy level. (Note to David/Kevin: this should be part of the initial implementation)

## Code

### Config

A typical config for the node tracker will look like
```
node_tracker_service:
  enabled: yes
  listen_addr: ...

  proxy_keep_alive_interval: ... // Ex: 5m. Represents the cleaning threshold for stale node-to-proxy relationships.

  https_keypairs:
   - key_file: ...
     cert_file: ...
```

Proxies will be able to connect to a node tracker by adding this line to their config:
```
proxy_service:
  ...

  node_tracker_addr: ...
```

### Modulability

The node tracker library is designed to be modular sitting behind a common api, that way different implementations of the node tracker can be implemented for different customer needs or even hidden behind teleport enterprise without affecting the normal usage of teleport.

## Performance

The current implementation of the node tracker has been tested for memory usage and the grpc server has been put under load tests in a way to meet or exceed the requirement of maintaining good service for 100K connected nodes.

### Memory

Here's the memory profile of the node tracker after adding 1M relationships
```
File: implementation.test
Type: alloc_space
Time: Nov 23, 2021 at 1:29pm (EST)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) top10
Showing nodes accounting for 1087.28MB, 99.62% of 1091.39MB total
Dropped 18 nodes (cum <= 5.46MB)
      flat  flat%   sum%        cum   cum%
  855.52MB 78.39% 78.39%   855.52MB 78.39%  github.com/gravitational/teleport/lib/nodetracker/implementation.(*routeDetails).Add
      92MB  8.43% 86.82%       92MB  8.43%  github.com/pborman/uuid.UUID.String (inline)
   72.25MB  6.62% 93.44%  1087.28MB 99.62%  github.com/gravitational/teleport/lib/nodetracker/implementation.BenchmarkTracker
   35.50MB  3.25% 96.69%    35.50MB  3.25%  github.com/google/uuid.NewRandomFromReader
      32MB  2.93% 99.62%    67.50MB  6.18%  github.com/pborman/uuid.NewRandom
         0     0% 99.62%    35.50MB  3.25%  github.com/google/uuid.NewRandom (inline)
         0     0% 99.62%   855.52MB 78.39%  github.com/gravitational/teleport/lib/nodetracker/implementation.(*tracker).AddNode
         0     0% 99.62%   159.51MB 14.61%  github.com/pborman/uuid.New
         0     0% 99.62%  1087.28MB 99.62%  testing.(*B).run1.func1
         0     0% 99.62%  1087.28MB 99.62%  testing.(*B).runN

```

### Load testing

Note these results will probably change with:
- adding mtls to the grpc server
- testing on machines with different hardware configurations that best match what we're looking for.

We used [ghz.sh](https://ghz.sh/) to load test the node tracker's grpc server on a:
MacBook Pro (16-inch, 2019)
2.6 GHz 6-Core Intel Core i7
16 GB 2667 MHz DDR4

Here's the result after making 1M `AddNode` requests over 10 connections.
```
Summary:
  Count:        1000000
  Total:        109.66 s
  Slowest:      273.69 ms
  Fastest:      0.21 ms
  Average:      1.48 ms
  Requests/sec: 9119.44

Response time histogram:
  0.210   [1]      |
  27.557  [999268] |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
  54.905  [488]    |
  82.253  [90]     |
  109.600 [66]     |
  136.948 [22]     |
  164.296 [9]      |
  191.643 [6]      |
  218.991 [0]      |
  246.339 [0]      |
  273.686 [50]     |

Latency distribution:
  10 % in 0.42 ms
  25 % in 0.56 ms
  50 % in 0.90 ms
  75 % in 1.68 ms
  90 % in 2.90 ms
  95 % in 4.11 ms
  99 % in 8.69 ms

Status code distribution:
  [OK]   1000000 responses
```

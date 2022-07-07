---
authors: Edoardo Spadolini (edoardo.spadolini@gmail.com)
state: draft
---

# RFD 0076 - Downtime minimization

## Required Approvers
* Engineering: TBD
* Security: TBD
* Product: TBD

## What

This RFD describes the causes of downtime that can occur during the course of normal cluster operation, regular maintenance (upgrades of cluster components - mainly auth nodes and proxy nodes, scheduled CA rotations with long grace period for all CAs) and emergency maintenance (typically user CA rotation with little to no grace period), and discusses best practices and recommendations for Teleport operators, changes that can be made to Teleport itself to improve the availability of services within the constraints of the current design, and recommends potential future development steps to further improve things by completely sidestepping some problems in certain situations.

## Why

As Teleport becomes a fundamental part of how infrastructure and computing resources are accessed throughout an organization, it's important to know when and why certain parts of the cluster can be unavailable and for how long, and if it's possible to reduce or ideally completely negate this downtime by spending more or less effort and resources.

## Details

### Auth connectivity and upgrades

For various reasons, all Teleport components need a valid connection to an auth server to start up and work correctly: service startup in Teleport will wait until a connection to Auth is established, the SSH service needs to know if it's connecting to auth through a proxy to know if it's running in direct connection mode or reverse tunnel mode, `tsh` opens an auth client to know (among other things) if the cluster is running in FIPS mode, new sessions (of various kinds) can't be established because we need to create a session object in the backend, push audit log events and check for locks. If the connection to the auth drops, connections will be unaffected if connectivity is restored within a grace time (at least 5 minutes for locks, depending on the `locking_mode` config option; any session by a user with a nonzero `max_connections` has 2 minutes before the loss of the associated semaphore lease causes the session to be terminated) but other functionality might be affected - for instance, if a session resource expires before connectivity is restored, the session can't be joined by other parties anymore (the new `SessionTracker` mechanism creates resources with a one hour expiration time which should be plenty, but the old-style `session.Session` only has a 30 second TTL which is often not enough for the node to reconnect).

We're currently requiring that upgrades to the Auth components happen while a single instance of the Auth is running: in other words, even in a HA configuration of Teleport, we first have to shut down all Auth servers from the old version, then spin up a single instance of the new version, then (once it signals readiness, presumably) the other replicas can be started. The reason for this is that we reserve the right to run once-off data migrations or format changes in the backend when Auth starts, and that could make any currently running Auth very very confused. This process doesn't usually take very long when running containerized Auth replicas in Kubernetes or Docker, but doing the same with VMs can take quite a bit longer, and cluster functionality will still be impacted while proxies and agents reconnect; a rolling rollout is often times easier to configure and execute, and only affects a portion of the cluster at a time.

The Auth service doesn't support graceful shutdowns; every client must be capable of reconnecting and retrying. Because of this and the current restrictions on different Auth versions running at the same time, in-place upgrades (with SIGHUP) are already unsupported. In-process restarts as a result of CA rotations will be discussed later.

### Proxy connectivity and upgrades

Most Teleport agents (all but SSH and Desktop Access in direct connection mode) and all Teleport clients connect through a Proxy to provide or use services: the Proxy acts as a proxy (sic!) for all services, sometimes exclusively, and it includes web clients for SSH, App Access and Desktop Access. It can be sidestepped in rare cases, like when connecting to a leaf cluster's proxy (or agents) directly, or when using `ssh` from OpenSSH to connect to a Teleport SSH server in direct connection mode, but normally all data will flow through a Proxy. The Proxy allows for graceful shutdowns and graceful restarts: upon receiving a QUIT (or a HUP, which will also trigger an in-place upgrade) the listeners will close, reverse tunnel clients will get notified of the shutdown (more on this later), and then the Proxy will wait until the last user connection terminates - only when that happens, the Proxy will actually shut down.

For direct connection agents (and for in-process App Access agents, technically, but only in setups with a single Proxy) full functionality is (re)gained as soon as the Proxy becomes available (again); connections through a Proxy that's shutting down will continue working (including connections to an Auth) until the connection ends (because of a session close) or the Proxy is forcibly terminated after some external timeout (the typical situation with managed containers or VMs). There is currently no notification sent to the user of a service about the Proxy that's currently serving the connection shutting down, nor does it seem like there's a reasonable way of doing that with the current design.

In-process restarts as a result of CA rotations will be discussed later.

### Reverse tunnels in mesh mode

Agents in reverse tunnel mode are only accessible through their reverse tunnel connection through one of the Proxies. In agent mesh mode, a connection to an agent through a proxy is only possible if the agent has a reverse tunnel connection to that specific proxy. Agents identify the Proxies that they're connected to using UUIDs as a key, receiving informations about the existence of other proxies via the same reverse tunnel connection using an ad-hoc "discovery" protocol in the same connection; the discovery protocol also uses the UUIDs of the Proxies as identifiers. During the restart of an agent, the new instance is going to be unavailable to each Proxy until it has connected to its reverse tunnel service, but the old instance will still maintain connections as long as at least one user connection is open - this could be extended to have a grace period in which the old instance will keep running, giving time to the new instance to connect. Multiple instances with the same UUID connected to the same reverse tunnel server can coexist: connections are routed in order from most recent to least recent, but even in case of a reconnection, functionality shouldn't be impacted (as both instances should behave in the same way).

When a Proxy itself restarts, the new instance begins with zero reverse tunnels and will thus error out on any client connection attempt; the old one will send a "please reconnect" message (new in v10, still needs to be backported) to all the reverse tunnel agents, which will begin reconnecting while still keeping the connection alive for existing connections (and for connections that were in-flight at the moment that the restart began); this will eventually restore connectivity, but some potential downtime is almost inevitable in this case. The same concerns apply when spinning up a new proxy, which doesn't have the downside of replacing an existing working proxy, but shares the issue of lacking reverse tunnel availability as soon as it starts up. It's possible to sidestep this problem by configuring the proxy load balancer to only route user connections to the well-established proxies, while letting reverse tunnel connections through to all the proxies; this, combined with an artificial delay before shutdown, would allow a zero-downtime upgrade of Proxies. How to actually do this split-world load balancing is left as an exercise for the reader (in TLS routing mode, reverse tunnel connections include `teleport-reversetunnel` in the ALPN protos in their ClientHello, which allows them to be distinguished from user connections).

### Reverse tunnels in proxy peering mode

In proxy peering mode, reverse tunnel agents become fully available as soon as they are connected to at least one proxy that's reachable by other proxies (and they heartbeat as such). It should be easy to maintain availability across a restart of an agent (as long as we add some grace period during the shutdown of the previous instance). Restarting a proxy in-place has the same problems that in-place restarting causes when in mesh mode, and should be avoided in this case as well. Spinning up a new proxy in proxy peering mode incurs no downtime, as the newly added proxy will be able to serve user connections immediately, by connecting to the other proxies - shutting down a proxy ungracefully will cause a loss of connectivity for all agents that are connected to just that proxy, however.

### CA rotations

The current implementation of CA rotations involves restarting every component in the cluster after each phase (other than `init`); as such, connectivity loss for reverse tunnel agents is inevitable (requiring potentially more effort than a full shutdown and startup, as newly restarted agents might initially connect to proxies that haven't restarted yet), and the fact that all Auths are restarted means that at least some disruption is to be expected, even for agents in direct connection mode. As most of the best practices around maintaining high availability and low downtime involve rolling upgrades and not restarting Auth and Proxy in place, it's clear that the current implementation of CA rotations is problematic. It should be possible to swap out credentials without a full restart on a rotation, and that would make most of the availability concerns around CA rotations disappear.

## Potential future steps

* Remove the old-style `session.Session`: already underway, will be completed in v11.
* Allow running different Auth builds from the same major version at the same time (potentially restricted to only two versions, upgrading from old to new): potentially good value, requires engineering care and prevents us from running migrations outside of major version upgrades.
* Properly deprecate in-place upgrades when `auth_service` is enabled and `auth_servers` is pointing to just localhost (auth and other services in the same process).
* Add a grace period on shutdown of reverse tunnel agents and servers: will delay shutdowns - potentially for nothing, if the shutdown is meant to be a shutdown rather than a restart and we're not in proxy peering mode, but we could extend the internal "shutdown" protocol to also carry this information, as we always know if we have spawned a new Teleport ourselves or not.
* Backport the reconnection advisory mechanism for reverse tunnels.
* Don't close the proxy peering listeners when shutting down rather than restarting.
* Deprecate in-place restarts for Proxies in proxy peering mode.
* Restartless CA rotations: requires engineering effort, but would simplify things around the initialization code of `TeleportProcess` quite a bit. Would require some input from security, to decide what to do about preexisting connections after old certs become untrusted.

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

## Future changes

* Remove the old-style `session.Session`: already underway, will be completed in v11.
* Allow running different Auth builds from the same major version at the same time (potentially restricted to only two versions, upgrading from old to new): potentially good value, requires engineering care and prevents us from running migrations outside of major version upgrades.
* Properly deprecate in-place upgrades when `auth_service` is enabled.

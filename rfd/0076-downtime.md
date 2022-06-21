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

For various reasons, all Teleport components need a valid connection to an auth server to start up and work correctly: service startup in Teleport will wait until a connection to Auth is established, the SSH service needs to know if it's connecting to auth through a proxy to know if it's running in direct connection mode or reverse tunnel mode, `tsh` opens an auth client to know (among other things) if the cluster is running in FIPS mode, new sessions (of various kinds) can't be established because we need to create a session object in the backend, push audit log events and check for locks. If the connection to the auth drops, connections will be unaffected if connectivity is restored within a grace time (at least 5 minutes for locks, infinite if the `locking_mode` config option is set to `best_effort`) but other functionality might be affected - for instance, if a session resource expires before connectivity is restored, the session can't be joined by other parties anymore (the new `SessionTracker` mechanism creates resources with a one hour expiration time which should be plenty, but the old-style `session.Session` only has a 30 second TTL).

We're currently requiring that upgrades to the Auth components happen while a single instance of the Auth is running: in other words, even in a HA configuration of Teleport, we first have to shut down all Auth servers from the old version, then spin up a single instance of the new version, then (once it signals readiness, presumably) the other replicas can be started. The reason for this is that we reserve the right to run once-off data migrations or format changes in the backend when Auth starts, and that could make any currently running Auth very very confused. This process doesn't usually take very long when running containerized Auth replicas in Kubernetes or Docker, but doing the same with VMs can take quite a bit longer, and cluster functionality will still be impacted while proxies and agents reconnect; a rolling rollout is often times easier to configure and execute, and only affects a portion of the cluster at a time.

## Future changes

* Remove the old-style `session.Session`: already underway, will be completed in v11.
* Allow running different Auth builds from the same major version at the same time (potentially restricted to only two versions, upgrading from old to new): potentially good value, requires engineering care and prevents us from running migrations outside of major version upgrades.

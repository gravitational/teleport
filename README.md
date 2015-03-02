# Medusa

Proxying SSH server with dynamic backend and ACL support

## Rationale

The major pain points for any operations is the absense of clear and unified access to the server.

The question operations have is:

* How and where do I log in and where's the password?

The question users allowing access have:

* How do I grant/revoke access to anyone?
* How do I grant temporary access?
* How do I track the actions on the servers?
* What is my entrypoint?

## Implementation

The goal of this project is to define a simple SSH infrastrucure that satisfies the following requirements:

* It is ridiculously easy to setup. Ideally it should be zero-configuration for users.
* Embraces the notion of "bastion", where the entry point server grants access to the cluster.
* Provides workflow for granting/revoking access to users and groups.
* Provides infrastructure for logging SSH activity in the cluster in structured format
* Supports dynamic configuration backend.

### Implementation thoughts and open questions:

* Choice of a dynamic backend (Etcd?)
* Choice of auth (PKI or OpenSSH CA?)
* SSH server implementation (OpenSSH, go ssh?)


## Proposed design

![medusa overview](/doc/img/MedusaOverview.png?raw=true "Medusa Overview")

* Use Etcd for configuration, discovery, presence and key store
* Each server has ssh server heartbeating into the cluster
* Use OpenSSH key infractrucure for authorizing/rejecting access
* Use Go SSH server to multiplex connections, talk to Etcd for checking access by public keys
* Implement a service and a CLI for managing/revoking and signing certificates
* Use structured logging in syslog and json format and plug it into ES cluster













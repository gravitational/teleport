---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 00143 - External Kubernetes joining

## Required approvers

* Engineering: @zmb3 && (@hugoShaka || @tigrato)
* Product: @klizhentas || @xinding33
* Security: @reedloden

## What

This RFD proposes a method for allowing entities (such as a Machine ID instance
or an Agent) to join Teleport Clusters via federation between the Kubernetes 
Cluster they reside in and a Teleport Cluster, where that Teleport Cluster does 
not reside in the same Kubernetes Cluster.

This is distinct from 
[RFD 0094 - Kubernetes node joining](./0094-kubernetes-node-joining.md) which
only supports the joining entity existing within the same Kubernetes Cluster
as the Teleport Cluster.

## Why

It is not unusual for users to wish to deploy Machine ID bots and Teleport
Agents in Kubernetes Clusters which is not the same cluster in which their
Auth Service resides. For example, they may operate many Kubernetes Clusters or
they may use Teleport Cloud (in which we host their Teleport Cluster).

Currently, these users must use a different join method, and, for users
deploying into environments where there is no platform-specific delegated
join method, they must fallback to using the `token` join method which is a
sensitive, long-lived, secret. This also requires some element of state, and
is inconvenient in more ephemeral use-cases.

To provide a more concrete use-case, many users currently deploy Teleport
Plugins (such as the Slack Access Request plugin). Adding support for External
Kubernetes Joining will provide a golden pathway for this deployment using
Machine ID that avoids long-lived shared secrets and provides a seamless 
experience.

## Details

## Security Considerations

## Alternatives

### Extending the existing Kubernetes join method
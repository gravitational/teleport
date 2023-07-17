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

Extending the existing Kubernetes join method to support external joining
has two key issues.

The first issue is connectivity. The existing Kubernetes join method relies on
the Auth Service's ability to call the TokenReview endpoint exposed by the
Kubernetes API server. This means there must be some sort of network
connectivity and the appropriate firewall rules to allow the request to pass.

The second issue is authentication. Calling the TokenReview RPC requires some 
form of authentication. At the moment, the Auth Service relies on a Kubernetes
service account available to it by virtue of running in the cluster. 

The issue with authentication could be solved by allowing an operator to
configure credentials for the Kubernetes cluster as part of the join token
specification. This, however, does not solve the issue of connectivity.

One solution that would solve the connectivity and authentication problems
would be to use an existing Teleport Kubernetes Agent deployed into a
Kubernetes Cluster as a "stepping stone". This Agent would have a service
account with a role that granted it the ability to call TokenReview, hence
solving the authentication problem, and the Auth Service would be able to
communicate with the Kubernetes Agent over the Proxy reverse tunnel.

This solution has two key problems:

- It presents a "chicken and egg" problem. The initial Kubernetes Agent
  deployed to the cluster would not be able to use the Kubernetes Join method.
- It grants a potentially dangerous amount of access to the Kubernetes Agent.
  As it would complete the TokenReview, a hijacked Kubernetes Agent could be
  used to trick the Auth Service into accepting any token as legitimate for
  that cluster. This presents a pathway for privilege escalation.
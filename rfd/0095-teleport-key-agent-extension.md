---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 95 - Teleport Key Agent Extension

## Required approvers

* Engineering: @jakule && @r0mant
* Product: @klizhentas
* Security: @reedloden

## What

Extend Teleport Key Agent to allow users to make `tsh` requests from within a remote session started with `tsh ssh -A`.

## Why

It's already possible to `ssh` from a remote session in this way, but `tsh` requires more than just an SSH certificate and private key to start a session. At the minimum, `tsh` also needs a TLS certificate and proxy address to form connections to a Teleport proxy. To avoid using `tsh --insecure`, the remote session will also need access to a list of SSH and TLS CA certificates.

Thus, we need a secure protocol for sharing certificates and other relevant data on the local host with the remote host. We can lean on the [extensible](https://datatracker.ietf.org/doc/html/rfc4251#section-4.2) SSH Agent protocol for this, by implementing custom extensions for retrieving TLS certificates, CA certificates, and even perform per-session MFA signatures with the Teleport Key Agent.

## Details

### Teleport Key Agent Changes

We will add new [SSH Agent extensions](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.7) to forward additional `tsh` keystore items and functions, namely TLS certificates, CA certificates, known hosts, and per-session MFA reissue. Basic SSH Agent functionality will not be impacted by adding these extensions, so existing integrations will remain unchanged.

To support these extensions, `tsh` will need to be updated to extend the current Teleport Key Agent and add a new Teleport Key Agent client. The Teleport Key Agent client can be used by `tsh` calls on a remote host to send SSH Agent requests to the Teleport Key Agent running on the local host.

#### SSH Agent Extensions

**Extension Structure:**

Extension requests consist of:

        byte            SSH_AGENTC_EXTENSION
        string          extension type (type@domain)
        byte[]          extension contents

An Extension response can be any custom message, but failures should result in `SSH_AGENT_EXTENSION_FAILURE`. Unsupported extensions should result in an `SSH_AGENT_FAILURE` response to differentiate from actual extension failures.

**Teleport Identity Extension:**

This extension requests a partial Teleport Identity from the agent matching the provided public key and Teleport identifiers. Clients should provide the public key part of a Teleport agent key, since a `tsh` profile should be available in the local file key store. The Teleport cluster, proxy, and user can also be found in the Teleport agent key comment.

        byte            SSH_AGENTC_EXTENSION
        string          identity@goteleport.com
        string          public key SHA
        string          proxy
        string          cluster
        string          user

The returned identity data can be used in place of the standard filesystem identity (~/.tsh) for `tsh` operations, including connecting to a Teleport Proxy or connecting to a Teleport Node.

        byte            SSH_AGENT_SUCCESS
        string          TLS certificate
        byte[]          TLS CA certs
        byte[]          SSH CA certs

Combined with the information stored in the user's SSH agent (Crypto signer, SSH certificate, Teleport Proxy:ClusterName:UserName), `tsh` will have enough information to carry out standard operations.

**Add MFA Key Extension:**

This extension can be used to issue an MFA verified agent key to connect to a specific Teleport Node. The local agent will prompt the user for MFA touch to reissue MFA certificates, and then it will add the certificate as an agent key to the local agent with the [key lifetime constraint](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.1) so that it will automatically expire from the agent after 1 minute.

        byte            SSH_AGENTC_EXTENSION
        string          add-mfa-key@goteleport.com
        string          Teleport Node name

If the extension returns `SSH_AGENT_SUCCESS`, then the MFA agent key was successfully added to the local agent and can be accessed through the forwarded agent.

        byte            SSH_AGENT_SUCCESS

Note: this extension can also be used to extend Per-session MFA support to some OpenSSH integrations, including [OpenSSH Proxy Jump](https://github.com/gravitational/teleport/issues/17190). In this use case, `tsh proxy ssh` would call the extension to prompt the user for MFA tap and add the agent key to the user's system agent. The parent `ssh` call which calls this ProxyCommand would then have access to this key when forming a proxied ssh connection.

### Security

#### SSH Agent Forwarding Risks

SSH Agent forwarding introduces an inherent risk. When a user does `ssh -A` or `tsh ssh -A`, their forwarded keys could be used by a user on the remote machine with root permissions over the remote user. It should not be possible to export any sensitive data from a user's forwarded Key Agent. This criteria constrains some potential options, such as providing the user's raw private key via the `identity@goteleport.com` extension.

However, even with these new extensions contrained in this way, a user can abuse the forwarded agent to reissue certificates with new private keys held on the remote host. This raises a major security concern that is not present in standard ssh agent forwarding, since the user's login session can essentially be exported to the remote host via a reissue command, rather than being contingent on the forwarding agent session providing access to a private key on the local host.

For this reason, we may want to consider limiting certificate reissue commands to using the same public key as the active identity, or at least limit the TTL of non-matching certificates to 1 minute. Alternatively, we can just convey this risk by adding a new option like `ForwardAgent local-insecure` to enable this feature.

### UX

Users will need to forward the Teleport Key Agent, rather than the System Key Agent forwarded by `tsh ssh -A`. As described in [RFD 22](https://github.com/gravitational/teleport/blob/master/rfd/0022-ssh-agent-forwarding.md), this can be done with `tsh ssh -o "ForwardAgent local"`. Due to the security concerns described above, we can instead use the new option `local-insecure`.

To improve UX, we can add an option via an env variable - `TELEPORT_FORWARD_AGENT`, and a [tsh config file](https://goteleport.com/docs/reference/cli/#tsh-configuration-files) value - `AgentForward`. This way, `tsh ssh -A` could check these values before defaulting to the System Key Agent.

### Additional Considerations

#### Kubernetes

Kubernetes access requires the ability to load a TLS certificate and raw private key pair provided to a kubeconfig file through the `tsh kube credentials` exec plugin. Since we do not want to provide the user's raw private key through the forwarded agent, `tsh kube credentials` will need a different way to aquire a valid TLS certificate and private key.

As explained in the security section above, it is currently possible to reissue certificates over the forwarded agent with new private keys. By utilizing this feature, we can enable kuberenetes access on remote hosts without introducing any new systems. We will just need to update `tsh kube credentials` to make a reissue request with a new raw private key if the available private key is not exportable.

If we decide to disable reissue requests with non-matching public keys, then we will need to get a bit more creative. One option would be to create a generalized forward proxy ssh channel that could be used by the remote host to form connections to Teleport services by proxying through the local host. This approach would warrant a separate RFD.

#### Per-session MFA support for non SSH services

We can add a new `reissue-mfa-cert` command to issue MFA verified TLS certificates usable for Kubernetes, DB, App, and Desktop access. I'm leaving this out of the RFD for brevity and since it isn't currently planned, but it would be similar to the extensions laid out above.

#### Key Constraint Extensions

The ssh agent protocol also provides the ability to create custom [key constraint extension](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.3), which seemed like a promising option for adding Per-session MFA keys into the user's SSH agent. The idea would be that when the key is used for an ssh connection, it would prompt the user for MFA tap on demand.

Unfortunately, in my testing this ended up not working as expected. The SSH agent will always be called first through it's `List` command, which looks at every key's public key together. Each key is then check for public key authentication, and passing keys will continue on to an SSH handshake. Since Per-session MFA is currently enforced by public key (SSH certificate) rather than key signature, the initial public key from the `List` command would need to have an MFA verified certificate already. This means that *every* ssh connection would prompt for MFA, including non-Teleport connections.

The utilization of key constraint extensions might still be promising in the future. For example, SSH's own fido keys use a similar key constraint extension approach. However, with our current Per-session MFA system, this approach and all of the workaround necessary to make it work would do more harm than good.
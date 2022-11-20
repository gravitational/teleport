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

### UX

#### `ForwardAgent local`

In order to use the Teleport SSH Agent extensions described below, user's will need to forward the Teleport Key Agent (`/tmp/teleport-xxx/teleport-xxx.socket`) rather than the System Key Agent (`$SSH_AUTH_SOCK`) usually forwarded by `tsh ssh -A`. As described in [RFD 22](https://github.com/gravitational/teleport/blob/master/rfd/0022-ssh-agent-forwarding.md), this can be done with `tsh ssh -o "ForwardAgent local"`.

To improve UX, we can add an option via an env variable - `TELEPORT_FORWARD_AGENT`, and a [tsh config file](https://goteleport.com/docs/reference/cli/#tsh-configuration-files) value - `AgentForward`. This way, `tsh ssh -A` could check these values before defaulting to the System Key Agent.

#### `tsh` Profile

When a user does `tsh ssh -o "ForwardAgent local"`, their local key store will be forwarded to the remote host. Since the remote session doesn't have any `tsh` profile metadata (`~/.tsh/current-profile`, `~/.tsh/proxyhost.yaml`), `tsh` will need a new way to discern the remote session's current profile and profile information.

##### Option 1 - use current connection to determine profile

Within a remote session, in the absence of a `~/.tsh` profile, `tsh` can check the env variables `SSH_SESSION_WEBPROXY_ADDR`, `SSH_TELEPORT_CLUSTER_NAME`, and `SSH_TELEPORT_USER` to determine the current profile. `tsh` calls will ping the proxy on each request to get retrieve other important profile information, such as tls routing mode.

Pros:

* avoids leaving files in the remote host's `~/.tsh` directory

Cons:

* introduces additional proxy round trips to retrieve profile information on each call to `tsh`
* introduces a separate `tsh` profile system to maintain, which may not support all `tsh` features evenly
* prevents users from swapping profiles without re-logging in

##### Option 2 - copy local profile

Within a remote session, in the absence of a `~/.tsh` profile, `tsh` can check the forwarded Teleport SSH Agent for profile information. This will include the current profile and the `yaml` files for each available profile. Only profiles with a corresponding forwarded ssh agent key will be returned. These files will be copied into the remote session's `~/.tsh` directory so that `tsh` can function as usual with the keys available.

Pros:

* only one additional ssh agent roundtrip for the first call to `tsh`
* extends remote `tsh` support evenly across different features

Cons:

* leaves files in the remote hosts's `~/.tsh` directory
* potentially unintuitive UX, as the `tsh` profiles appears to be forwarded, but is not actually synchronized with the local host `tsh` profiles

##### Option 3 - forward local profile

Within a remote session, in the absence of a `~/.tsh` profile, `tsh` can check the forwarded Teleport SSH Agent for profile information. This will include the current profile and the `yaml` files for each available profile on the local host. Only profiles with a corresponding forwarded ssh agent key will be returned by the forwarded agent.

Pros:

* seamless UX - users can switch between nodes and sessions while maintaining the same `tsh` profile across all sessions
* avoids leaving files in the remote host's `~/.tsh` directory
* extends remote `tsh` support evenly across different features

Cons:

* more difficult to implement
* introduces additional ssh agent round trips to retrieve profile information on each `tsh` call

##### Option Choice: 2

In my opinion, option 3 provides the best UX and will fit best into the current profile system, but option 2 provides us with a simpler initial implementation that can be extended to option 3 in the future.

Both options would use the same ssh agent profile extension, but option 2 can simply query the profiles once before normal `tsh` operations and fill out the remote session's `~/.tsh` directory with current profile information. From an implementation perspective, this allows us to avoid a lot of additional changes necessary to interface with the ssh agent profile directly, as opposed to the `~/.tsh` profile.

If in the future, users request the seamless UX of option 3, or request that `~/.tsh` files are not left after the remote session, we can implement option 3 without wasting any of the work done for option 2.

### Teleport Key Agent Changes

We will add new [SSH Agent extensions](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.7) to forward `~/.tsh` profile information, certificates, and additional functions (per-session MFA). Basic SSH Agent functionality will not be impacted by adding these extensions, so existing integrations will remain unchanged.

To support these extensions, `tsh` will need to be updated to extend the current Teleport Key Agent and add a new Teleport Key Agent client. The Teleport Key Agent client can be used by `tsh` calls on a remote host to send SSH Agent requests to the Teleport Key Agent running on the local host.

#### SSH Agent Extensions

**Extension Structure:**

Extension requests consist of:

        byte            SSH_AGENTC_EXTENSION
        string          extension type (type@domain)
        byte[]          extension contents

An Extension response can be any custom message, but failures should result in `SSH_AGENT_EXTENSION_FAILURE`. Unsupported extensions should result in an `SSH_AGENT_FAILURE` response to differentiate from actual extension failures.

**Teleport Profile Extension:**

This extension requests `tsh` profile information, including the current profile and profile yaml files.

        byte            SSH_AGENTC_EXTENSION
        string          profiles@goteleport.com  

The returned profile information can be used by `tsh` to initiate new Teleport clients.

        byte            SSH_AGENT_SUCCESS
        string          current profile
        byte[][]        profile yaml files

**Teleport Keys Extension:**

This extension requests a list of keys matching the given key index. Partial key indexes can be provided.

        byte            SSH_AGENTC_EXTENSION
        string          keys@goteleport.com
        keyindex[]      key index

Where a keyindex consists of:

        string          proxy host       
        string          cluster name
        string          username

The returned keys can be used in concert with the signers available through the SSH agent to perform TLS and SSH handshakes.

        byte            SSH_AGENT_SUCCESS
        key[]           keys
        byte[]          known hosts

Where a key consists of:

        keyindex        key index
        byte[]          SSH certificate
        byte[]          TLS certificate
        byte[][]        Trusted CA TLS certificates

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

SSH Agent forwarding introduces an inherent risk. When a user does `ssh -A` or `tsh ssh -A`, their forwarded keys could be used by a user on the remote machine with OS permissions equal to or above the remote user. Still, these forwarded keys can only be used as long as the user maintains the agent forwarding sessions, since agent forwarding does not allow keys to be exported (only certificates). This security principle constrains some potential options, such as providing the user's raw private key via the `identity@goteleport.com` extension.

However, even with the new extensions constrained in this way, a user can abuse the forwarded agent to reissue certificates with new private keys held on the remote host. This raises a security concern that is not present in standard ssh agent forwarding, since the user's login session can essentially be exported to the remote host via a reissue command, rather than being contingent on the forwarding agent session providing access to a private key on the local host.

For this reason, we may want to consider limiting certificate reissue commands to using the same public key as the active identity, or at least limit the TTL of non-matching certificates to just 1 minute. These restrictions may impact other features, including remote kubernetes support, so it is not currently planned.

### Additional Considerations

#### Kubernetes

Kubernetes access requires the ability to load a TLS certificate and raw private key pair provided to a kubeconfig file through the `tsh kube credentials` exec plugin. Since we do not want to provide the user's raw private key through the forwarded agent, `tsh kube credentials` will need a different way to acquire a valid TLS certificate and private key.

As explained in the security section above, it is currently possible to reissue certificates over the forwarded agent with new private keys. By utilizing this feature, we can enable kubernetes access on remote hosts without introducing any new systems. We will just need to update `tsh kube credentials` to make a reissue request with a new raw private key if the available private key is a forwarded agent key.

Note: If we decide to disable reissue requests with non-matching public keys, then we will need to get a bit more creative. One option would be to create a generalized forward proxy ssh channel that could be used by the remote host to form connections to Teleport services by proxying through the local host. This approach would warrant a separate RFD.

#### Per-session MFA support for non SSH services

We can add a new `reissue-mfa-cert` command to issue MFA verified TLS certificates usable for Kubernetes, DB, App, and Desktop access. I'm leaving this out of the RFD for brevity and since it isn't currently planned, but it would be similar to the extensions laid out above.

#### Key Constraint Extensions

The ssh agent protocol also provides the ability to create custom [key constraint extension](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.3), which seemed like a promising option for adding Per-session MFA keys into the user's SSH agent. The idea would be that when the key is used for an ssh connection, it would prompt the user for MFA tap on demand.

Unfortunately, in my testing this ended up not working as expected. The SSH agent will always be called first through it's `List` command, which looks at every key's public key together. Each key is then check for public key authentication, and passing keys will continue on to an SSH handshake. Since Per-session MFA is currently enforced by public key (SSH certificate) rather than key signature, the initial public key from the `List` command would need to have an MFA verified certificate already. This means that *every* ssh connection would prompt for MFA, including non-Teleport connections.

The utilization of key constraint extensions might still be promising in the future. For example, SSH's own fido keys use a similar key constraint extension approach. However, with our current Per-session MFA system, this approach and all of the workaround necessary to make it work would do more harm than good.

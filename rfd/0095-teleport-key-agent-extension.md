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

In order to use the Teleport SSH Agent extensions described below, users will need to forward the Teleport Key Agent (`/tmp/teleport-xxx/teleport-xxx.socket`) rather than the System Key Agent (`$SSH_AUTH_SOCK`) usually forwarded by `tsh ssh -A`. As described in [RFD 22](https://github.com/gravitational/teleport/blob/master/rfd/0022-ssh-agent-forwarding.md), this can be done with `tsh ssh -o "ForwardAgent local"`.

To improve UX, we can add an option via an env variable - `TELEPORT_FORWARD_AGENT`, and a [tsh config file](https://goteleport.com/docs/reference/cli/#tsh-configuration-files) value - `AgentForward`. This way, `tsh ssh -A` could check these values before defaulting to the System Key Agent.

#### `tsh` Profile

When using `tsh ssh -o "ForwardAgent local"`, the user's local `~/.tsh` profile will be forwarded through a new SSH Agent extension. This will include the current profile and the `yaml` files for each available profile on the local host. Only profiles with a corresponding forwarded ssh agent key will be returned by the forwarded agent. This will lead to seamless UX between local and remote sessions.

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

**List Profiles Extension:**

This extension requests `tsh` profile information, including the current profile and profile yaml files.

        byte            SSH_AGENTC_EXTENSION
        string          list-profiles@goteleport.com  

The returned profile information can be used by `tsh` to initiate new Teleport clients.

        byte            SSH_AGENT_SUCCESS
        string          current profile name
        byte[]          profiles blob (json)

Where profiles blob is a json encoded `[]api/profile.Profile`.

Note: The underlying `api/profile.Profile` struct is subject to change, so there is a possibility of backwards compatibility concerns. However, this backwards compatibility should be handled already since the same issue exists for `~/.tsh` profile yaml files.

**List Keys Extension:**

This extension requests a list of keys matching the given key index. Partial key indexes can be provided to return partial matches.

        byte            SSH_AGENTC_EXTENSION
        string          list-keys@goteleport.com
        keyIndex[]      key index

Where a keyIndex consists of:

        string          proxy host       
        string          cluster name
        string          username

The returned keys can be used in concert with the signers available through the SSH agent to perform TLS and SSH handshakes.

        byte            SSH_AGENT_SUCCESS
        key[]           keys
        byte[]          known hosts

Where a key consists of:

        keyIndex        key index
        byte[]          SSH certificate
        byte[]          TLS certificate
        byte[][]        Trusted CA TLS certificates

**Sign Extension:**

This extension requests a signature using the provided key.

        byte            SSH_AGENTC_EXTENSION
        string          sign@goteleport.com
        byte[]          key blob (ssh public key/certificate)
        byte[]          digest
        string          hash name
        string          salt length

The resulting signature will be returned alongside the signing algorithm used.

        byte            SSH_AGENT_SIGN_RESPONSE
        string          signature format
        byte[]          signature blob

Unlike the standard `sign@openssh.com` request, `sign@goteleport.com` expects to receive digested data (pre-hashed) rather than the raw message to digest on the agent side. This makes it possible to perform signatures with algorithms other than the ssh signing algorithms (e.g. `ssh-rsa`, `ssh-rsa-cert-v01@openssh.com`). For example, this enables the use of `RSASSA-PSS` algorithms used in TLS 1.3 handshakes.

Hash name should be the stringified representation of a golang `crypto.Hash` value. For example, `SHA-1`, `SHA-256`, or `SHA-512`.

Salt length can be empty for algorithms that don't use a salt, or a positive integer for those that do (such as `RSASSA-PSS`). Salt length can also be set to `auto` to automatically use the largest salt length possible during signing, which can be auto-detected during verification.

**Prompt MFA Challenge Extension:**

This extension can be used to issue an MFA challenge prompt to the user's local machine, which enables support for MFA functionality including `tsh` MFA login and per-session MFA verification.

        byte            SSH_AGENTC_EXTENSION
        string          add-mfa-key@goteleport.com
        string          proxy address
        []byte          challenge blob (json)

Where challenge blob is a json encoded `api/client/proto.MFAAuthenticateChallenge`.

The resulting challenge response can then be used for MFA verification.

        byte            SSH_AGENT_SUCCESS
        []byte          challenge response blob (json)

Where challenge response blob is a json encoded `api/client/proto.MFAAuthenticateResponse`.

Note: The protobuf structs used above are subject to change, which may lead to backwards compatibility concerns. In this case, this extension should be taken into consideration before making changes to these structs.

### Security

#### SSH Agent Forwarding Risks

SSH Agent forwarding introduces an inherent risk. When a user does `ssh -A` or `tsh ssh -A`, their forwarded keys could be used by a user on the remote machine with OS permissions equal to or above the remote user. Still, these forwarded keys can only be used as long as the user maintains the agent forwarding sessions, since agent forwarding does not allow keys to be exported (only certificates). This security principle constrains some potential options, such as providing the user's raw private key via the `list-keys@goteleport.com` extension.

However, even with the new extensions constrained in this way, a user can abuse the forwarded agent to reissue certificates with new private keys held on the remote host. This raises a security concern that is not present in standard ssh agent forwarding, since the user's login session can essentially be exported to the remote host via a reissue command, rather than being contingent on the forwarding agent session providing access to a private key on the local host.

For this reason, we may want to consider limiting certificate reissue commands to using the same public key as the active identity, or at least limit the TTL of non-matching certificates to just 1 minute. These restrictions may impact other features, including remote kubernetes support, so it is not currently planned.

### Additional Considerations

#### Kubernetes

Kubernetes access requires the ability to load a TLS certificate and raw private key pair provided to a kubeconfig file through the `tsh kube credentials` exec plugin. Since we do not want to provide the user's raw private key through the forwarded agent, `tsh kube credentials` will need a different way to acquire a valid TLS certificate and private key.

As explained in the security section above, it is currently possible to reissue certificates over the forwarded agent with new private keys. By utilizing this feature, we can enable kubernetes access on remote hosts without introducing any new systems. We will just need to update `tsh kube credentials` to make a reissue request with a new raw private key if the available private key is a forwarded agent key.

Note: If we decide to disable reissue requests with non-matching public keys, then we will need to get a bit more creative. One option would be to create a generalized forward proxy ssh channel that could be used by the remote host to form connections to Teleport services by proxying through the local host. This approach would warrant a separate RFD.

#### OpenSSH Per-session MFA Support

Currently, using OpenSSH client to connect to a Teleport Node with Per-session MFA required is not possible. This limitation is a result of `tsh` doing all of the heavy lifting to check for MFA requirements and issue MFA challenges. However, it may be possible to utilize the user's SSH agent to overcome this issue.

For example, it is possible for `tsh proxy ssh` to handle the MFA verification before the `ssh` connection connects to the proxy. `tsh proxy ssh` can then reissue MFA certificates for the connection and add the certificate and key as an agent key to the user's SSH agent with a [key lifetime constraint](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.1) of 1 minute (in addition to the certificate's 1 minute TTL). Additionally, this agent key can be added to the `tsh proxy ssh` call's local key agent, rather than the system agent, to prevent this agent key from escaping local memory. This flow would preserve the current security principles of the Per-session MFA feature.

#### Key Constraint Extensions

The ssh agent protocol also provides the ability to create custom [key constraint extension](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.3), which seemed like a promising option for adding Per-session MFA keys into the user's SSH agent. The idea would be that when the key is used for an ssh connection, it would prompt the user for MFA tap on demand.

Unfortunately, in my testing this ended up not working as expected. The SSH agent will always be called first through it's `List` command, which looks at every key's public key together. Each key is then check for public key authentication, and passing keys will continue on to an SSH handshake. Since Per-session MFA is currently enforced by public key (SSH certificate) rather than key signature, the initial public key from the `List` command would need to have an MFA verified certificate already. This means that *every* ssh connection would prompt for MFA, including non-Teleport connections.

The utilization of key constraint extensions might still be promising in the future. For example, SSH's own fido keys use a similar key constraint extension approach. However, with our current Per-session MFA system, this approach and all of the workaround necessary to make it work would do more harm than good.

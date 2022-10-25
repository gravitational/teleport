---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 95 - Teleport Key Agent Extension

## Required Approvers (TBD)

## What

Extend Teleport Key Agent to allow users make `tsh` requests from within a remote session started with `tsh ssh -A`.

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

**TLS Certificate Extension:**

This extension requests a TLS certificate from the agent matching the provided public key. Clients should provide the public key part of a Teleport certificate, since a matching TLS certificate should be available in the local agent's key store.

For session requests that require per-session MFA, clients can provide the `require_mfa` flag to issue a single time use mfa-verified certificate.

        byte            SSH_AGENTC_EXTENSION
        string          tls-cert@goteleport.com
        string          public key SHA
        bool            require_mfa

The returned x509 certificate can be used to perform a TLS handshake.

        byte            SSH_AGENT_SUCCESS
        string          certificate contents

**Trusted CAs Extension:**

This extension requests a list of trusted CA certificates.

        byte            SSH_AGENTC_EXTENSION
        string          ca-certs@goteleport.com

The retured x509 certificates can be used to form a TLS CA pool.

        byte            SSH_AGENT_SUCCESS
        string[]        certificate contents list

**Known Hosts Extension:**

This extension requests a list of known hosts.

        byte            SSH_AGENTC_EXTENSION
        string          known-hosts@goteleport.com

The retured known hosts can be used to perform an SSH host key callback.

        byte            SSH_AGENT_SUCCESS
        string[]        known hosts contents

#### SSH Agent Key Constraint Extensions

For Per session MFA, we *could* add an `ssh-cert` extension with the same structure as the `tls-cert` extension above, but we'd be missing a great opportunity. Instead, we can add a new [key constraint extension](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.3) to add per-session MFA functionality into the Teleport Key Agent in a way that would work with `ssh` as well. This would extend per-session MFA functionality to OpenSSH integrations, including [OpenSSH Proxy Jump](https://github.com/gravitational/teleport/issues/17190).

**Extension Structure:**

Key constraints can be used to limit when and how a key can be used. They can be included when adding any of the existing key types.

Key constraint extensions consist of:

        byte            SSH_AGENT_CONSTRAIN_EXTENSION
        string          extension name (name@domain)
        byte[]          extension-specific details

If an unsupported constraint extension is provided, the add key request will return an `SSH_AGENT_FAILURE`.

**Per-session MFA Key Constraint Extension:**

This key constraint extension can be used to add a per-session MFA key.

        byte            SSH_AGENT_CONSTRAIN_EXTENSION
        string          per-session-mfa@goteleport.com

The private key part of this key will be used for signing operations, but the certificate part, if any, will be ignored. Instead, the Teleport Key Agent will issue a new `IssueUserCertsWithMFA` request and use the returned certificates directly. The `tsh` process running the Teleport Key Agent will thus prompt the user for touch during the reissue request.

To avoid using this key when a non per-session MFA key is suitable, these keys will always be returned last by the [list keys operation](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.4).

### UX

Users will need to forward the Teleport Key Agent, rather than the System Key Agent forwarded by `tsh ssh -A`. As described in [RFD 22](https://github.com/gravitational/teleport/blob/master/rfd/0022-ssh-agent-forwarding.md), this can be done with `tsh ssh -o "ForwardAgent local"`.

To improve UX, we can add an option via an env variable - `TELEPORT_FORWARD_AGENT`, and a [tsh config file](https://goteleport.com/docs/reference/cli/#tsh-configuration-files) value - `AgentForward`. This way, `tsh ssh -A` could check these values before defaulting to the System Key Agent.

### Security

#### SSH Agent Forwarding Risks and Constraints

SSH Agent forwarding introduces an inherent risk. When a user does `ssh -A` or `tsh ssh -A`, their forwarded keys could be used by a user on the remote machine with root permissions over the remote user. However, it should not be possible to export any sensitive data from a user's forwarded Key Agent. This criteria prohibits some potential options, such as providing an SSH Agent extension to retrieve a Teleport identity file for the user, or returning a raw private key in the `tls-cert@goteleport.com` extension.

#### Per-session MFA

Additionally, the `tls-cert@goteleport.com` introduces a slight vulnerability in per-session MFA. Currently, per-session MFA certificates are only held in memory and are never exportable. The new extension makes it possible for a user to run the extension and export the certificate. This certificate would still be limited to a 1 minute TTL, but could in theory be used for unintended purposes outside of ["establishing a single session"](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md#constraints). This factor also contributed to the choice to use a key constraint extension for the mfa-verified SSH certificate instead.

### Additional Considerations

#### Incompatibilities

The majority of `tsh` calls will work, but features which require a raw private key will not.

TODO: investigate which features won't work, experiment with getting Kubernetes Access to work.
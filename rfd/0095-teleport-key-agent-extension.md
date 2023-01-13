---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 95 - Teleport Key Agent Extension

## Required approvers

* Engineering: @jakule && @r0mant
* Product: @klizhentas
* Security: @reedloden @jentfoo

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

When using `tsh ssh -o "ForwardAgent local"`, the user's active `tsh` profile will be forwarded through a new SSH agent extension. This will include the current profile name, profile `yaml` file, client certificates, and trusted CA certificates. In the remote session, `tsh` will see an empty `~/.tsh` directory and instead check for the forwarded ssh agent for profile and certificate data. Each call to `tsh` will use the forwarded profile and certificates to carry out requests.

Within a remote agent forwarding session, users can continue to forward their `tsh` profile by using `tsh ssh -A`. We use normal agent forwarding instead of "local" agent forwarding because on the remote host we want to forward the existing `$SSH_AUTH_SOCK`, which is connected to the user's local agent and has access to their local certificates.

```bash
$ tsh login --user=dev --proxy=proxy.example.com:3080
Enter password for Teleport user dev:
> Profile URL:        https://proxy.example.com:3080
  Logged in as:       dev
  Cluster:            root-cluster
  Roles:              dev
  Logins:             dev, -teleport-internal-join
  ...
  
$ tsh ssh -o "ForwardAgent local" server01
### successfully connect

<server01> $ ls ~/.tsh
# empty

<server01> $ tsh status
> Profile URL:        https://proxy.example.com:3080
  Logged in as:       dev
  Cluster:            root-cluster
  Roles:              dev
  Logins:             dev, -teleport-internal-join
  ...

$ tsh ls
Node Name Address        Labels                                                                             
--------- -------------- --------------------------------
server01  127.0.0.1:3022 arch=x86_64,cluster=root-cluster
server02  127.0.0.1:3022 arch=x86_64,cluster=root-cluster

tsh ssh -o "ForwardAgent local" server02
### successfully connect
```

The remote session will only have access to the client certificates used for the current session, rather than the entirety of `~/tsh`. Additionally, these certificates will only be returned by the forwarded agent if the corresponding agent keys are also still available. This means that if the local uses `tsh logout` or changes profiles (`tsh login other-cluster`), the remote session will no longer have access to the `tsh` profile.

```bash
$ tsh ssh -o "ForwardAgent local" server01
### successfully connect

### switch tabs and `tsh logout` locally

<server01> $ tsh status
ERROR: Not logged in.
```

### Security

#### SSH Agent Forwarding Risks

SSH Agent forwarding comes with an inherent security risk. When a user does `ssh -A` or `tsh ssh -A`, their forwarded keys could be used by a user on the remote machine with OS permissions equal to or above the remote user. This is possible due to the way SSH channels are forwarded within a remote session, and this problem is shared by other protocols like X11 forwarding.

The primary redeeming security principle for forwarded SSH channels is that forwarded keys cannot outlive the original session. This means that once the user who called `ssh -A` exits out of their session, no agent keys remain usable on the remote machine, and no sensitive data could have been exfiltrated to outlive the session. We'll call this security principle "session contingency" in this RFD.

We can follow this same framework to forward a user's certificates and agent key without exposing their actual private key, only an interface to send the forwarded agent cryptographic challenges. This would allow users to forward their certificates and agent keys relatively safely to be used in a remote session and guarantee that malicious users cannot exfiltrate their `~/.tsh` and impersonate them. These forwarded keys could be used for any `tsh` and `tctl` request, or more generally, any Teleport API request.

#### Other concerns

Please take a look at [this issue](https://github.com/gravitational/teleport-private/issues/299) for more details on another security concern related to these changes.

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

**Key Extension:**

This extension requests a forwarded key, holding the `tsh` profile and certificates for the forwarded key.

        byte            SSH_AGENTC_EXTENSION
        string          key@goteleport.com

The returned profile and certificates can be used in concert with the `sign@goteleport.com` extension to perform standard cryptographic operations, such as TLS handshakes.

        byte            SSH_AGENT_SUCCESS
        byte[]          profile blob (json)
        byte[]          certificates blob (json)

Where profile blob is a json encoded `Profile` and certificates blob is a json encoded `ClientCertificates`:

```go
type Profile struct {
        WebProxyAddr          string `json:"web_proxy_addr,omitempty"`
        SSHProxyAddr          string `json:"ssh_proxy_addr,omitempty"`
        KubeProxyAddr         string `json:"kube_proxy_addr,omitempty"`
        PostgresProxyAddr     string `json:"postgres_proxy_addr,omitempty"`
        MySQLProxyAddr        string `json:"mysql_proxy_addr,omitempty"`
        MongoProxyAddr        string `json:"mongo_proxy_addr,omitempty"`
        Username              string `json:"user,omitempty"`
        SiteName              string `json:"cluster,omitempty"`
        ForwardedPorts        []string `json:"forward_ports,omitempty"`
        DynamicForwardedPorts []string `json:"dynamic_forward_ports,omitempty"`
        Dir                   string `json:"dir,omitempty"`
        TLSRoutingEnabled     bool `json:"tls_routing_enabled,omitempty"`
        AuthConnector         string `json:"auth_connector,omitempty"`
        LoadAllCAs            bool `json:"load_all_cas,omitempty"`
        MFAMode               string `json:"mfa_mode,omitempty"`
}

type ClientCertificates struct {
        KeyIndex `json:"key_index"`
        SSHCert      []byte `json:"ssh_cert"`
        TLSCert      []byte `json:"tls_cert"`
        TrustedCerts []TrustedCerts `json:"trusted_certs"`
}

type KeyIndex struct {
        ProxyHost   string `json:"proxy_host"`
        Username    string `json:"username"`
        ClusterName string `json:"cluster_name"`
}

type TrustedCerts struct {
        ClusterName      string `json:"domain_name"`
        HostCertificates [][]byte `json:"checking_keys"`
        TLSCertificates  [][]byte `json:"tls_certs"`
}
```

Note: The underlying `teleport/api/profile.Profile` struct is subject to change, so there is a possibility of backwards compatibility concerns. However, this backwards compatibility should be handled already since the same concern is already applied for `~/.tsh` profile yaml files.

**Sign Extension:**

The standard `SSH_AGENTC_SIGN_REQUEST` expects to receive an un-hashed cryptograpic message challenge, and performs the hash and signature together on the ssh-agent server side. In `x/crypto`, this is sufficient for the `ssh.Signer` interface, but most uses of the `crypto.Signer` interface send hashed digests. This means that the `SSH_AGENTC_SIGN_REQUEST` can not be wrapped into a `crypto.Signer` without making modifications to the common `x/crypto` libraries. Additionally, some signature schemes are not supported by the `x/crypto` implementation of ssh-agent, including the `RSAPSS` signature schemes used in TLS 1.3.

Instead, we will introduce a new sign extension that is more interoperable with the `crypto.Signer` interface.

This extension requests a signature for the provided certs.

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

Hash name should be the stringified representation of a golang `crypto.Hash` value. For example, `SHA-1`, `SHA-256`, or `SHA-512`.

Salt length can be empty for algorithms that don't use a salt, or a positive integer for those that do (such as `RSAPSS`). Salt length can also be set to `auto` to automatically use the largest salt length possible during signing, which can be auto-detected during verification.

**Prompt MFA Challenge Extension:**

This extension can be used to issue an MFA challenge prompt to the user's local machine, which enables support for MFA functionality including `tsh` MFA login and per-session MFA verification.

        byte            SSH_AGENTC_EXTENSION
        string          prompt-mfa-challenge@goteleport.com
        string          proxy address
        []byte          challenge blob (json)

Where challenge blob is a json encoded `api/client/proto.MFAAuthenticateChallenge`.

The resulting challenge response can then be used for MFA verification.

        byte            SSH_AGENT_SUCCESS
        []byte          challenge response blob (json)

Where challenge response blob is a json encoded `api/client/proto.MFAAuthenticateResponse`.

Note: The protobuf structs used above are subject to change, which may lead to backwards compatibility concerns. In this case, this extension should be taken into consideration before making changes to these structs.

### Additional Considerations

#### Syncing local/remote `tsh` profiles

Instead of only forwarding a single `tsh` profile in a remote forwarding session, we could forward a user's entire `~/.tsh` and keep their local and remote `tsh` profile synced. This means that the remote session would follow the local `tsh` profile through profile switches, log outs, new log ins, etc. It could even be possible to perform these actions from the remote session. This experience would also be more in line with OpenSSH agent forwarding, which forwards all keys in the user's `~/.ssh` directory.

While this would provide the most seamless UX experience between local and remote sessions, this approach would increase the security risk of local agent forwarding. For example, if a user two active `tsh` profiles, one for administration and one for basic operations, the user could `tsh ssh -o "ForwardAgent local"` into a widely accessible server. The user's administrative `tsh` profile could potentially be used through their forward agent connection, due to the inherent risks of ssh agent forwarding.

If we were to enable this functionality, we would need to do one or both of the following:

1) Make this feature opt-in, with a new forwarding option. e.g. `ForwardAgent local-all`

2) Implement agent restrictions similar to [OpenSSH's agent restrictions](https://www.openssh.com/agent-restrict.html#:~:text=Agent%20restriction%20in%20OpenSSH). This would be a more sophisticated approach that would provide a way to limit how forwarded keys can be used. This option would warrant a separate RFD.

#### Kubernetes

Kubernetes access requires the ability to load a TLS certificate and raw private key pair provided to a kubeconfig file through the `tsh kube credentials` exec plugin. Since we do not want to provide the user's raw private key through the forwarded agent, `tsh kube credentials` will need a different way to acquire a valid TLS certificate and private key.

One option would be to create a generalized forward proxy ssh channel that could be used by the remote host to form connections to Teleport services by proxying through the local host. This approach would warrant a separate RFD.

#### OpenSSH Per-session MFA Support

Currently, using OpenSSH client to connect to a Teleport Node with Per-session MFA required is not possible. This limitation is a result of `tsh` doing all of the heavy lifting to check for MFA requirements and issue MFA challenges. However, it may be possible to utilize the user's SSH agent to overcome this issue.

For example, it is possible for `tsh proxy ssh` to handle the MFA verification before the `ssh` connection connects to the proxy. `tsh proxy ssh` can then reissue MFA certificates for the connection and add the certificate and key as an agent key to the user's SSH agent with a [key lifetime constraint](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.1) of 1 minute (in addition to the certificate's 1 minute TTL). Additionally, this agent key can be added to the `tsh proxy ssh` call's local key agent, rather than the system agent, to prevent this agent key from escaping local memory. This flow would preserve the current security principles of the Per-session MFA feature.

#### Key Constraint Extensions

The ssh agent protocol also provides the ability to create custom [key constraint extension](https://datatracker.ietf.org/doc/html/draft-miller-ssh-agent#section-4.2.6.3), which seemed like a promising option for adding Per-session MFA keys into the user's SSH agent. The idea would be that when the key is used for an ssh connection, it would prompt the user for MFA tap on demand.

Unfortunately, in my testing this ended up not working as expected. The SSH agent will always be called first through its `List` command, which looks at every key's public key together. Each key is then check for public key authentication, and passing keys will continue on to an SSH handshake. Since Per-session MFA is currently enforced by public key (SSH certificate) rather than key signature, the initial public key from the `List` command would need to have an MFA verified certificate already. This means that *every* ssh connection would prompt for MFA, including non-Teleport connections.

The utilization of key constraint extensions might still be promising in the future. For example, SSH's own fido keys use a similar key constraint extension approach. However, with our current Per-session MFA system, this approach and all of the workaround necessary to make it work would do more harm than good.

---
authors: Andrew LeFevre (andrew.lefevre@goteleport.com)
state: implemented
---

# RFD 98 - Registered OpenSSH Nodes

## Required approvers

* Engineering: @jakule && @r0mant
* Product: @klizhentas
* Security: @reedloden || @jentfoo

## What

Allow OpenSSH nodes to be registered in a Teleport cluster.

## Why

[Agentless EC2 discovery mode](https://github.com/gravitational/teleport/issues/17865) will discover and configure OpenSSH nodes so they can authenticate with a cluster. But those OpenSSH nodes aren't registered as `node` resources in the backend. We need a way to register agentless OpenSSH nodes as `node` resources so they can be viewed and managed by users. RBAC and session recording should function correctly with registered OpenSSH nodes as well.

## Details

### Registering nodes

A new sub-kind to the `node` resource will be added for registered OpenSSH nodes: `openssh`. The absence of a `node` resource sub-kind (or presence of a `teleport` sub-kind) will imply that a node is a Teleport agent node, making this change backwards compatible.

Kubernetes operators and Terraform providers will be updated to support registering and managing agentless nodes. Teleport Discover and Cloud will switch to provisioning agentless nodes by default, but on-prem users will be required to use `tctl` to register OpenSSH nodes, as described below.

#### Registering with `tctl`

`tctl` should not require as many fields to be set when creating nodes. This is an example `node` resource that will work with `tctl create --force` today:

```yaml
kind: node
metadata:
  name: 5da56852-2adb-4540-a37c-80790203f6a9
spec:
  addr: 1.2.3.4:22
  hostname: agentless-node
version: v2
```

`tctl create` will not require `--force` when creating `node` resources. `tctl` will auto-generate `metadata.name` if it is not already set so users don't have to generate GUIDs themselves if `sub_kind` is `openssh`. Also, if `sub_kind` is set to `openssh`, `spec.public_addr` will not be allowed for registered OpenSSH nodes as it is not needed. An example of a registered OpenSSH node resource:

```yaml
kind: node
sub_kind: openssh
metadata:
  name: 5da56852-2adb-4540-a37c-80790203f6a9
spec:
  addr: 1.2.3.4:22
  hostname: agentless-node
version: v2
```

When manually registering an OpenSSH node, create a registered OpenSSH node resource file and create the resource in the cluster with `tctl create <node.yml>`. Then follow the steps outlined below in `Manual certificate rotation` to configure the registered OpenSSH node.

#### API

No changes to the API client will be needed in order to support registered SSH nodes. Adding, deleting, listing, connecting etc. will work with existing API methods.

### OpenSSH CA

For security related reasons that will be discussed below, a new CA will be added called OpenSSH CA. The OpenSSH CA will be responsible for generating and signing user certificates that will be used to authenticate with registered OpenSSH nodes. The public key of the OpenSSH CA will be copied to all registered OpenSSH nodes and configured as `TrustedUserCAKeys` in `sshd_config`. The Host CA will continue to be used to sign SSH host certificates.

#### Manual certificate rotation

Users not using EC2 discovery mode and hosting registered OpenSSH nodes themselves will have to rotate the OpenSSH and Host CAs manually and generate new SSH host certificates. They can do so by:

1. Exporting the OpenSSH CA: `tctl auth export --type=openssh | sed "s/cert-authority\ //"`
2. Write the exported CA to the configuration folder of the node's SSH daemon, ex. `/etc/ssh/` for OpenSSH on Linux
3. Replace the `TrustedUserCAKeys` setting in `sshd_config` to point to the newly created CA
4. Generate a new host certificate: `tctl auth sign --host=<addr,hostname> --format=openssh --out=host-cert`
5. Replace the existing host certificate with the new one on the node and set `HostKey` and `HostCertificate` in `sshd_config`

### RBAC

When a user sends a request to a Proxy to connect to a node, the Proxy will attempt to find a node resource by either its hostname or IP, whichever the user specified. If the resource exists and has the `openssh` `sub_kind`, an RBAC check will be performed. If the resource does not exist or isn't an `openssh` node, the connection flow will continue as normal.

### Security

Both RBAC checks and session recording for registered OpenSSH nodes require that users connect through a Proxy. If users are able to connect to registered OpenSSH nodes directly, they can bypass both features. Currently the User CA public key is copied to OpenSSH nodes and configured to be trusted by `sshd`. This means OpenSSH nodes will accept certificates that are signed by the User CA. The problem with that is when Teleport users authenticate with a cluster, the Auth server replies with a certificate that is signed by the User CA. Users could potentially use this certificate to directly connect to registered OpenSSH nodes.

Using the new OpenSSH CA to sign certificates used to authenticate with registered OpenSSH nodes solves this problem, as Teleport users do not have access to any certificates signed by the OpenSSH CA.

#### Forwarding SSH Agents

If a certificate signed by the OpenSSH CA is added to a forwarded SSH agent, then anyone with access to the forwarded SSH agent socket will have access to any other registered OpenSSH node that trusts OpenSSH CA. To prevent this, certificates signed by the OpenSSH CA will never be added to SSH agents. Furthermore, forwarding SSH agents from users' machines will not be required to access registered OpenSSH nodes and will only happen when a user explicitly requests it (ie `tsh ssh -A`) and it is allowed. The `forward_agent` role option will continue to be respected, allowing administrators to control if Teleport users can forward SSH agents.

### UX

The following Teleport node features won't work with registered OpenSSH nodes:

- Enhanced session recording and restricted networking
- Host user provisioning (without using PAM)
- Session recording without SSH session termination
- Dynamic labels
- Outbound persistent tunnels to Proxies

Due to this and other potential future differences, `tsh ls` and the node listing on the web UI should be updated to display if nodes are registered OpenSSH nodes or Teleport agent nodes.

### Session recording

`proxy` session recording mode will continue to be an option even after support for registered SSH nodes is implemented. However, it will not be needed if a cluster consists exclusively of Teleport and registered OpenSSH nodes. When connecting to a registered OpenSSH node, the Proxy will create a forwarding SSH server, terminate and record the SSH session. This is exactly how `proxy` session recording mode works now, except it will be done on-demand when connecting to registered OpenSSH nodes. In other words:

If session recording is in `node` or `node-sync` mode:

- If the node is a Teleport Agent node, the node would record the session and upload it as normal.
- If the node is a registered OpenSSH node, the Proxy would terminate and record the SSH session and upload it.

If session recording is in `proxy` or `proxy-sync` mode:

- Behavior would be unaffected. The Proxy would terminate, record and upload all sessions. This mode will still be required if users of a cluster wish to connect to unregistered OpenSSH nodes.

Unregistered OpenSSH nodes using `proxy` session recording mode can migrate to become registered OpenSSH nodes, either automatically if they are on AWS or via `tctl` as described above. The `proxy` session recording mode will be deprecated in the future and replaced by registered OpenSSH nodes, and the ability to directly dial to arbitrary IPs will be removed from both `tsh` and the Web UI as well.

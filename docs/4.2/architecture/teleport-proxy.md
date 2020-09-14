## The Proxy Service

**Table of Contents**

[TOC]

The proxy is a stateless service which performs three main functions in a
Teleport cluster:

1. It serves as an authentication gateway. It asks for credentials from
   connecting clients and forwards them to the Auth server via [Auth
   API](teleport_auth.md#auth-api).

2. It looks up the IP address for a requested Node and then proxies a connection
   from client to Node.

3. It serves a Web UI which is used by cluster users to sign up and configure
   their accounts, explore nodes in a cluster, log into remote nodes, join
   existing SSH sessions or replay recorded sessions.

## Connecting to a Node

### Web to SSH Proxy

In this mode, Teleport Proxy implements WSS - secure web sockets - to proxy a
client SSH connection:

![Teleport Proxy Web](../img/proxy-web.svg)

1. User logs in to Web UI using username and password, and 2nd factor token if
   configured (2FA Tokens are not used with SSO providers).
2. Proxy passes credentials to the Auth Server's API
3. If Auth Server accepts credentials, it generates a new web session and
   generates a special ssh keypair associated with this web session. Auth server
   starts serving [OpenSSH ssh-agent
   protocol](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.agent)
   to the proxy.
4. The User obtains an SSH session in the Web UI and can interact with the node
   on a web-based terminal. From the Node's perspective, it's a regular SSH
   client connection that is authenticated using an OpenSSH certificate, so no
   special logic is needed.

!!! note "SSL Encryption"

    When using the web UI, the Teleport Proxy terminates SSL traffic and re-encodes data for the SSH client connection.

### CLI to SSH Proxy

**Getting Client Certificates**

Teleport Proxy implements a special method to let clients get short-lived
authentication certificates signed by the [Auth Service User Certificate
Authority (CA)](./auth/#authentication-in-teleport).:

![Teleport Proxy SSH](../img/proxy-ssh-1.svg)

1. A [`tsh` client](../cli-docs.md#tsh) generates an OpenSSH keypair. It forwards
   the generated public key, username, password and second factor token to the
   proxy.
2. The Proxy Server forwards request to the Auth Server.
3. If Auth Server accepts credentials, it generates a new certificate signed by
   its user CA and sends it back to the Proxy Server. The certificate has a TTL
   which defaults to 12 hours, but can be configured in
   [`tctl`](../cli-docs.md#tctl).
4. The Proxy Server returns the user certificate to the client and client stores
   it in `~/.tsh/keys`. The certificate is also added to the local SSH agent if
   one is running.

**Using Client Certificates**

Once the client has obtained a certificate, it can use it to authenticate with
any Node in the cluster. Users can use the certificate using a standard OpenSSH
client `ssh` or using `tsh`:

![Teleport Proxy Web](../img/proxy-ssh-2.svg)

1. A client connects to the Proxy Server and provides target node's host and
   port location. There are three lookup mechanisms a proxy uses to find the
   node's IP address:

    * Use DNS to resolve the name requested by the client.
    * Asks the Auth Server if there is a Node registered with this `nodename`.
    * Asks the Auth Server to find a node (or nodes) with a label that matches
      the requested name.

2. If the node is located, the Proxy establishes an SSH connection to the
   requested node and starts forwarding traffic from Node to client.
3. The client uses the established SSH tunnel from Proxy to Node to open a new
   SSH connection. The client authenticates with the target Node using its
   client certificate.

!!! tip "NOTE"

    Teleport's proxy command makes it compatible with [SSH jump hosts](https://wiki.gentoo.org/wiki/SSH_jump_host) implemented using OpenSSH's `ProxyCommand`. It also supports OpenSSH's ProxyJump/ssh -J implementation as of Teleport 4.1. See [User Manual](../user-manual.md#ssh-proxy-configuration)

## Recording Proxy Mode

In this mode, the proxy terminates (decrypts) the SSH connection using the
certificate supplied by the client via SSH agent forwarding and then establishes
its own SSH connection to the final destination server, effectively becoming an
authorized "man in the middle". This allows the proxy server to forward SSH
session data to the auth server to be recorded, as shown below:

![recording-proxy](../img/recording-proxy.svg)

The recording proxy mode, although _less secure_, was added to allow Teleport
users to enable session recording for OpenSSH's servers running `sshd`, which is
helpful when gradually transitioning large server fleets to Teleport.

We consider the "recording proxy mode" to be less secure for two reasons:

1. It grants additional privileges to the Teleport proxy. In the default mode,
   the proxy stores no secrets and cannot "see" the decrypted data. This makes a
   proxy less critical to the security of the overall cluster. But if an
   attacker gains physical access to a proxy node running in the "recording"
   mode, they will be able to see the decrypted traffic and client keys stored
   in proxy's process memory.
2. Recording proxy mode requires the SSH agent forwarding. Agent forwarding is
   required because without it, a proxy will not be able to establish the 2nd
   connection to the destination node.

However, there are advantages of proxy-based session recording too. When
sessions are recorded at the nodes, a root user can add iptables rules to
prevent sessions logs from reaching the Auth Server. With sessions recorded at
the proxy, users with root privileges on nodes have no way of disabling the
audit.

See the [admin guide](../admin-guide.md#recorded-sessions) to learn how to turn
on the recording proxy mode. Note that the recording mode is configured on the
Auth Server.

## More Concepts

* [Architecture Overview](teleport_architecture_overview.md)
* [Teleport Users](teleport_users.md)
* [Teleport Auth](teleport_auth.md)
* [Teleport Proxy](teleport_proxy.md)

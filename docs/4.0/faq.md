
# FAQ

### Can I use Teleport in production today?

Teleport has been deployed on server clusters with thousands of nodes at
Fortune 500 companies. It has been through several security audits from
nationally recognized technology security companies, so we are comfortable with
the stability of Teleport from a security perspective.

### Can Teleport be deployed in agentless mode?

Yes. Teleport can be deployed with a tiny footprint as an authentication
gateway/proxy and you can keep your existing SSH servers on the nodes. But some
innovating Teleport features, such as cluster introspection, will not be
available unless the Teleport SSH daemon is present on all cluster nodes.

### Can I use OpenSSH with a Teleport cluster?

Yes, this question comes up often and is related to the previous one. Take a
look at [Using OpenSSH client](user-manual.md##using-teleport-with-openssh)
section in the User Manual and [Using OpenSSH servers](admin-guide.md) in the
Admin Manual.

### Can I connect to nodes behind a firewall?

Yes, Teleport supports reverse SSH tunnels out of the box. To configure
behind-firewall clusters refer to [Trusted Clusters](admin-guide.md#trusted-clusters)
section of the Admin Manual.

### Can individual nodes create reverse tunnels to a proxy server without creating a new cluster?

This has been a long standing [request](https://github.com/gravitational/teleport/issues/803) of Teleport and
it has been fixed with Teleport 4.0.   Once you've upgraded your Teleport Cluster, change the node config
option `--auth-server` to point to web proxy address (this would be `public_addr` and `web_listen_addr`
in file configuration). As defined in [Adding a node located behind NAT - Teleport Node Tunneling](quickstart/#adding-a-node-located-behind-nat-teleport-node-tunneling)

### What's Teleport scalability and hardware recommendations?

We recommend setting up Teleport with a [High Availability configuration](admin-guide.md#high-availability). Below is our
recommended hardware for the Proxy and Auth server. If you plan to connect more than 10,000 nodes, please
[get in touch](mailto:info@gravitational.com) and we can help architect the best solution for you.

Scenario | Max Recommended Count | Proxy | Auth server
------------ | -------------|---------|-------
Teleport nodes connected to auth server | 10,000 |2x  2-4 vCPUs/8GB RAM | 2x 4-8 vCPUs/16GB RAM
Teleport nodes connected to proxy server (IoT) | 2,000* | 2x 2-4 vCPUs/8GB RAM |2x 4-8 vCPUs/16+GB RAM


* Teleport 4.1 release will focus on increasing Teleport IoT supported count to 10,000



### Does Web UI support copy and paste?

Yes. You can copy&paste using the mouse. For working with a keyboard, Teleport employs
`tmux`-like "prefix" mode. To enter prefix mode, press `Ctrl+A`.

While in prefix mode, you can press `Ctrl+V` to paste, or enter text selection
mode by pressing `[`. When in text selection mode, move around using `hjkl`, select
text by toggling `space` and copy it via `Ctrl+C`.

### What TCP ports does Teleport use?

Please refer to the [Ports](admin-guide.md#ports) section of the Admin Manual.

### Does Teleport support authentication via OAuth, SAML or Active Directory?

Gravitational offers this feature for the [commercial versions of Teleport](enterprise.md#rbac).

## Commercial Teleport Editions

### What is included in the commercial version, Teleport Enterprise?

The Teleport Enterprise offering gives users the following additional features:

* Role-based access control, also known as [RBAC](enterprise#rbac).
* Authentication via SAML and OpenID with providers like Okta, Active
  Directory, Auth0, etc. (aka, [SSO](http://localhost:6600/ssh_sso/)).
* Premium support.

We also offer implementation services, to help you integrate
Teleport with your existing systems and processes.

You can read more in the [Teleport Enterprise section of the docs](enterprise.md)

### Does Teleport send any data to Gravitational?

The open source edition of Teleport does not send any information to
Gravitational and can be used on servers without internet access. The
commercial versions of Teleport may or may not be configured to send anonymized
information to Gravitational, depending on the license purchased. This
information contains the following:

* Anonymized user ID: SHA256 hash of a username with a randomly generated prefix.
* Anonymized server ID: SHA256 hash of a server IP with a randomly generated prefix.

This allows Teleport Pro to print a warning if users are exceeding the usage limits
of their license. The reporting library code is [on Github](https://github.com/gravitational/reporting).

Reach out to `sales@gravitational.com` if you have questions about commercial
edition of Teleport.

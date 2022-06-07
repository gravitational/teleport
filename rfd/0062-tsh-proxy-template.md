---
authors: Roman Tkachenko (roman@goteleport.com)
state: implemented
---

# RFD 62 - Proxy template support for tsh proxy

## What

Proposes the UX for better `tsh proxy` command interoperability with `ssh`
client and connecting to leaf cluster proxies directly.

## Why

Proposed features are most useful for users who have many leaf clusters and
prefer to use plain `ssh` client to connect to nodes using jump hosts i.e.
connecting directly via leaf cluster's proxy rather than going through the
root cluster's proxy.

Consider the following scenario:

- User has multiple leaf clusters in different regions, for example
  `leaf1.us.acme.com`, `leaf2.eu.acme.com`, etc.
- Each leaf cluster has nodes like `node-1`, `node-2`, etc.
- User wants to log into a root cluster once to see the whole node inventory
  across all trusted clusters, but connect to the nodes within a particular
  leaf cluster through that cluster's proxy for better latency.

In order for the user to be able to, say, `ssh root@node-1.leaf1.us.acme.com`
currently they can create an SSH config similar to this:

```
Host *.leaf1.us.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@leaf1.us.acme.com -s proxy:$(echo %h | cut -d '.' -f1):%p@leaf1

Host *.leaf2.eu.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@leaf2.eu.acme.com -s proxy:$(echo %h | cut -d '.' -f1):%p@leaf2
```

This is not ideal because users need to maintain complex SSH config, update it
every time a new leaf cluster is added, and use non-trivial bash logic in the
proxy command to correctly separate node name from the proxy address.

An ideal state would be where user's SSH config is as simple as:

```
Host *.acme.com
    HostName %h
    Port 3022
    ProxyCommand <some proxy command>
```

Where `<some proxy command>` is smart enough (and user-controllable) to
determine which host/proxy/cluster user is connecting to.

## UX

The proposal is to extend the existing `tsh proxy ssh` command with support for
parsing out the node name and proxy address from the full hostname token `%h` in
SSH config.

Specifically, the syntax for the `<some proxy command>` would look like:

```
Host *.acme.com
    HostName %h
    Port 3022
    ProxyCommand tsh proxy ssh -J {{proxy}} %r@%h:%p
```

With the `-J` flag set, the command connects directly to the specified proxy
instead of the default behavior of connecting to the proxy of the current
client profile. This usage of the `-J` flag is consistent with the existing
proxy jump functionality (`tsh ssh -J`) and [Cluster Routing](https://github.com/gravitational/teleport/blob/master/rfd/0021-cluster-routing.md).

When a template variable `{{proxy}}` is used, the host name and proxy address
are extracted from the full hostname in the `%r@%h:%p` spec. Users define the
rules of how to parse node/proxy from the full hostname in the tsh config file
`$TELEPORT_HOME/config/config.yaml` (or global `/etc/tsh.yaml`). Group captures
are supported:

```yaml
proxy_templates:
# Example template where nodes have short names like node-1, node-2, etc.
- template: '^(\w+)\.(leaf1.us.acme.com)$'
  host: "$1" # host is optional and will default to the full %h if not specified
  proxy: "$2:3080"
# Example template where nodes have FQDN names like node-1.leaf2.eu.acme.com.
- template: '^(\w+)\.(leaf2.eu.acme.com)$'
  proxy: "$2:443"
```

Templates are evaluated in order and the first one matching will take effect.

Note that the proxy address must point to the web proxy address (not SSH proxy):
`tsh proxy ssh` will issue a ping request to the proxy to retrieve additional
information about the cluster, including the SSH proxy endpoint.

In the example described above, where the user has nodes `node-1`, `node-2` in
multiple leaf clusters, their template configuration can look like:

```yaml
proxy_templates:
- template: '^([^\.]+)\.(.+)$'
  host: "$1"
  proxy: "$2:3080"
```

In the node spec `%r@%h:%p` the host name `%h` will be replaced by the host from
the template specification and will default to full `%h` if it's not present in
the template.

So given the above proxy template configuration, the following proxy command:

```bash
tsh proxy ssh -J {{proxy}} %r@%h:%p
```

is equivalent to the following when connecting to `node-1.leaf1.us.acme.com`:

```bash
tsh proxy ssh -J leaf1.us.acme.com:3080 %r@node-1:3022
```

### Auto-login

To further improve the UX, we should make sure that `tsh proxy ssh` command
does not require users to log into each individual leaf cluster beforehand.

Users should be able to login once to their root cluster and use that
certificate to set up proxy with the extracted leaf's proxy (similar to proxy
jump scenario).

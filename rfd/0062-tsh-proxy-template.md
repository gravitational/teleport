---
authors: Roman Tkachenko (roman@goteleport.com), Brian Joerger (bjoerger@goteleport.com)
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

## Details

### Proxy switching

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

### Cluster switching

Alternatively, consider the following scenario:

- User has multiple leaf clusters, for example `leaf1.us.acme.com`, `leaf2.eu.acme.com`, etc.
- The leaf cluster do not have public proxies, so all requests must pass through
  the root cluster proxy.
- Each leaf cluster has nodes like `node-1`, `node-2`, etc.
- User wants to log into a root cluster to connect to any node in the trusted cluster.

In order to `ssh root@node01.leaf1.us.acme.com` you can create an SSH config similar to this:

```
Host *.leaf1.us.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@root.us.acme.com -s proxy:%h:%p@$(echo %h | cut -d '.' -f2)

Host *.leaf2.eu.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@root.us.acme.com -s proxy:%h:%p@$(echo %h | cut -d '.' -f2)
```

This strategy suffers from the same challenges as proxy switching noted above.

### Solution

An ideal state would be where user's SSH config is as simple as:

```
Host *.acme.com
    HostName %h
    Port 3022
    ProxyCommand <some proxy command>
```

Where `<some proxy command>` is smart enough (and user-controllable) to
determine which host/proxy/cluster user is connecting to.

### UX

The proposal is to extend the existing `tsh proxy ssh` command with support for
parsing out the node name, proxy address, and cluster from the full hostname `%h:%p`
in SSH config.

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

When a template variable `{{proxy}}` is used, the desired hostname, proxy address,
and cluster are extracted from the full hostname in the `%r@%h:%p` spec. Users define
the rules of how to parse node/proxy/cluster from the full hostname in the tsh config
file `$TELEPORT_HOME/config/config.yaml` (or global `/etc/tsh.yaml`). Group captures
are supported:

```yaml
proxy_templates:
# Example template where nodes have short names like node-1, node-2, etc.
- template: '^(\w+)\.(leaf1.us.acme.com):(.*)$'
  proxy: "$2:3080"
  host: "$1:$3" # host is optional and will default to the full %h:%p if not specified
# Example template where nodes have FQDN names like node-1.leaf2.eu.acme.com.
- template: '^(\w+)\.(leaf2.eu.acme.com):(.*)$'
  proxy: "$2:443"
# Example template where we want to connect through the root proxy.
- template: '^(\w+)\.(leaf3).us.acme.com:(.*)$'
  cluster: "$2"
```


Templates are evaluated in order and the first one matching will take effect. For each
replace rule set (`cluster`, `proxy`, and `host`), the corresponding cli value will be
overridden (`--cluster`, `-J`, and `%h:%p`). If `template` and all replace rules are empty,
the template is invalid.

Note that the proxy address must point to the web proxy address (not SSH proxy):
`tsh proxy ssh` will issue a ping request to the proxy to retrieve additional
information about the cluster, including the SSH proxy endpoint.

In the example described above, where the user has nodes `node-1`, `node-2` in
multiple leaf clusters, their template configuration can look like:

```yaml
proxy_templates:
- template: '^([^\.]+)\.(.+):(.*)$'
  proxy: "$2:3080"
  host: "$1:$3"
```

In the node spec `%r@%h:%p` the hostname `%h:%p` will be replaced by the host from
the template specification and will default to full `%h:%p` if it's not present in
the template.

So given the above proxy template configuration, the following proxy command:

```bash
tsh proxy ssh -J {{proxy}} %r@%h:%p
```

is equivalent to the following when connecting to `node-1.leaf1.us.acme.com`:

```bash
tsh proxy ssh -J leaf1.us.acme.com:3080 %r@node-1:3022
```

#### Auto-login

To further improve the UX, we should make sure that `tsh proxy ssh` command
does not require users to log into each individual leaf cluster beforehand.

Users should be able to login once to their root cluster and use that
certificate to set up proxy with the extracted leaf's proxy (similar to proxy
jump scenario).

### Proxy Templates for `tsh ssh`

Proxy Templates can also be used with `tsh ssh` and have feature parity with `ssh`.
This means that users can go from using a proxy template to `ssh node-1.leaf1.us.acme.com`
to `tsh ssh node-1.leaf1.us.acme.com` without making any additional change to the template.
`tsh ssh` will support the same features as `tsh proxy ssh`, including [auto login](#auto-login).

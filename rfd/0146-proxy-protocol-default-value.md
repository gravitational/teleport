---
authors: Tiago Silva (tiago.silva@goteleport.com) && Anton Miniailo (anton@goteleport.com)
state: Implemented in (v14.0)
---

# RFD 0146 - PROXY Protocol Defaults

## Required Approvals

* Engineering: @r0mant
* Product: @xinding33 || @klizhentas
* Security: @reedloden || @jentfoo

## Related issues

- [teleport-private#907](https://github.com/gravitational/teleport-private/issues/907)

## What

This RFD proposes to change the default behavior of the Teleport Proxy and Teleport
Auth servers configuration to have more secure approach to PROXY Protocol by changing default value for the `proxy_protocol` setting from `on` to the new default-only mode `unspecified`.
It also proposes changing the behavior of the `on` mode to mean PROXY header to be required, instead of current meaning of "allowed, but not required".

## Scope

This RFD only applies to unsigned PROXYv1 and PROXYv2 headers, sent by external to Teleport systems, such as load balancers.
It doesn't affect signed PROXYv2 headers used internally by Teleport to propagate client IP address between different parts of the system.

## Why

`proxy_protocol` setting controls behavior related to unsigned PROXY Protocol headers.
Currently, it defaults to `on` mode, in which Teleport accepts, but does not
require PROXY headers. This creates a security issue, since it's very easy for
the user to end up with unsafe setup because of misconfiguration.

The PROXY Protocol is a prefix to the TCP stream that contains information about
the client connection. It is used by the Teleport Proxy/Auth to determine the
client's IP address and port when the service is behind a L4 load balancer.
L4 load balancers don't have the ability to preserve the client's IP address
and port when forwarding the connection to the service. The PROXY Protocol
allows them to do that by prepending the connection with the client's IP address
and port.

Configuring the L4 load balancer to use the PROXY Protocol is a manual process
that requires the user to enable the load balancer to use the PROXY Protocol.
As a result, the user needs to configure the load balancer to use the PROXY
Protocol and configure the Teleport Proxy/Auth to use the PROXY Protocol - enabled
by default.

An example of the PROXYv1 Protocol header:

```text
PROXY TCP4 127.0.0.1 127.0.0.2 12345 42\r\n
```

After the PROXY line, the TCP stream continues as usual. It's the target responsibility
to parse the header and extract the client's IP address and port and pass the remaining
stream to the target service.

The PROXY Protocol is a simple protocol that is easy to parse and implement. It
is also a well-known protocol that is supported by many L4 load balancers and
services. However, natively it lacks any authentication and can be easily spoofed by
an attacker. The attacker can send a fake PROXY Protocol header to the Teleport
Proxy/Auth and spoof the client's IP address and port if the Teleport isn't behind
a well-configured L4 load balancer.

The problem is that the PROXY Protocol is enabled by default in the Teleport config
and the user needs to explicitly disable it in the config when the Teleport isn't
behind a L4 load balancer with PROXY Protocol enabled.

Given the following configuration:

1. The Teleport Proxy/Auth is behind a L4 load balancer with the PROXY Protocol
   enabled.

```mermaid
flowchart LR
    A[User] -->|STREAM| B(L4 LB)
    B -->|PROXY LINE\n STREAM| C(TELEPORT PROXY)
```

2. The Teleport Proxy/Auth is behind a L4 load balancer with the PROXY Protocol
   disabled.

```mermaid
flowchart LR
    A[User] -->|STREAM| B(L4 LB)
    B -->|STREAM| C(TELEPORT PROXY)
```

3. The Teleport Proxy/Auth is directly exposed to the Internet.

```mermaid
flowchart LR
    A[User] -->|STREAM| C(TELEPORT PROXY)
```

In the case (1), the PROXY Protocol is enabled in the Teleport Proxy/Auth and
the L4 load balancer. The L4 load balancer prepends the connection with the Proxy
Protocol header and the Teleport Proxy/Auth parses the header and extracts the
client's IP address and port. This is the expected behavior and if the attacker
sends a fake PROXY Protocol header, the Teleport Proxy/Auth will reject the connection because
it only allows a single unsigned PROXY Protocol header. Since the L4 load balancer is configured
to use the PROXY Protocol, it will prepend the connection with the correct PROXY
Protocol header and the Teleport Proxy/Auth will extract the correct client's but
will reject the connection if a second PROXY Protocol header is sent.

In the case (2), the PROXY Protocol is disabled in the L4 load balancer but if
not explicitly disabled in the Teleport Proxy/Auth, the Teleport Proxy/Auth will
accept requests prefixed with the PROXY Protocol header and requests without the
PROXY Protocol header. Since the L4 load balancer doesn't prepend the connection
with the PROXY Protocol header, any attacker can send a fake PROXY Protocol header
and spoof the client's IP address and port. In this case, the user MUST explicitly disable
the PROXY Protocol in the Teleport Proxy/Auth config.

In the case (3), the Teleport Proxy/Auth is directly exposed to the Internet
and if the PROXY Protocol isn't explicitly disabled in the Teleport Proxy/Auth
config, any attacker can send a fake PROXY Protocol header and spoof the client's
IP address and port. In this case, the user MUST explicitly disable the PROXY Protocol.

The implication of this is that the attacker can spoof the client's IP address and
generate audit logs with the spoofed IP address. This can also be used to bypass
IP Pinning protection if the attacker steals a valid certificate and uses it to
connect to the Teleport Proxy/Auth. The certificate contains the client's IP address
so it's trivial to generate a fake PROXY Protocol header with the client's IP address
and port and bypass the IP Pinning protection.

Given the above, Teleport should disable the PROXY Protocol by default and so the
user doesn't need to explicitly disable it in the config. This gives the user better
security by default and the user can explicitly enable the PROXY Protocol if the
Teleport Proxy/Auth is behind a L4 load balancer with the PROXY Protocol enabled.

The `proxy_protocol` setting in Teleport currently supports the following values:

- `off` - PROXY Protocol is disabled. If a PROXY Protocol header is received, it
  will reject the connection with an error.
- `on` - PROXY Protocol is enabled. If a PROXY Protocol header is received, it
  will parse the header and extract the client's IP address and port. If the header
  isn't present, it will assume the IP address and port are the same as the connection
  source IP address and port. **This is the default value.**

As a result of the current default value and behavior of the `proxy_protocol: on`
setting, it's possible to spoof IP address in the audit and bypass IP Pinning protection
if the Teleport Proxy/Auth is directly exposed to the Internet or behind a L4 LB with
the PROXY Protocol disabled without the user realizing it.

Given the above, we propose to change the default value of the `proxy_protocol`
setting.

## Proposal

The proposal is to change the default value of the `proxy_protocol` setting to
have a new mode `unspecified` if it was not set. The `unspecified` mode will process the PROXY Protocol
header if present, and will allow connection to continue, but it will also
mark such connections - we will set source address' port to be `0`, which denotes
that this is not real client IP.
IP Pinning protection will reject the connection with these `sourcePort = 0` connections
because we know we can't trust these IPs. This will prevent possibility
of IP pinning avoidance.
For users that don't use IP Pinning protection, the connection will be
accepted.

This will be a breaking change for users that didn't explicitly enable
the PROXY Protocol in the Teleport Proxy/Auth config but are behind a L4 load
balancer with the PROXY Protocol enabled. For users that have the IP Pinning protection
enabled, this change will prevent them from accessing the Teleport Proxy/Auth
because the Teleport Proxy will replace the client's port with
a dummy value and the IP Pinning protection will reject the connection.
For users that don't have the IP Pinning protection enabled, the only change will
be that logs and audit events will contain `sourcePort = 0`, but the PROXY specified IP address
will remained correct.
To mitigate this, every time Teleport receives unsigned PROXY header from a load
balancer in the `unspecified` mode it will log an error and issue an audit event. Inside
those errors and events we will clearly inform customers about the situation, that
they are probably have wrong configuration, and will tell them how to fix the misconfiguration.
In that error, we will also log full source IP address (including port) of the connection before setting port to be 0.
For users that use IP Pinning protection, they will also receive an error when
trying to connect to the Teleport Proxy/Auth.

This will give the user better security by default and the user can explicitly
enable the PROXY Protocol if the Teleport Proxy/Auth is behind a L4 load balancer
with the PROXY Protocol enabled. This will also prevent users from accidentally
exposing the Teleport Proxy/Auth to the Internet without the PROXY Protocol enabled
at the L4 load balancer level.

New `unspecified` mode is supposed to be safer default value, but we should still encourage
users to explicitly set their `proxy_protocol` setting to `on` or `off` mode
depending on their setup. Extensive logging and auditing of the situations when
there's probable misconfiguration is intended to push users to explicitly
choosing the mode. We also should notify existing users about the change and
suggest them to pay attention to this setting.

There are also changes to the behavior of the `proxy_protocol: on` setting. The
`proxy_protocol: on` setting will parse the PROXY Protocol header and extract
the client's IP address and port. If the header isn't present, it will reject
the connection with an error. This is a breaking change because the Teleport Proxy
will reject connections that don't have the PROXY Protocol header. In order to
be a breaking change, the user had to explicitly enable the PROXY Protocol in
the Teleport Proxy/Auth config which means the user is aware of the PROXY Protocol
and it's expected to have the PROXY Protocol header in the connection.

Teleport `proxy_protocol` will have the following modes:

- `off` - PROXY Protocol is disabled. If a PROXY Protocol header is received, it
  will reject the connection with an error.
- `unspecified` - PROXY Protocol is allowed, but not required. If a PROXY Protocol header is received,
  it will replace the client's IP address source port with `0`.
  If the header isn't present, it will assume the IP address and port are
  the same as the connection source IP address and port. **This is the default value.**
- `on` - PROXY Protocol is enabled. If a PROXY Protocol header is received, it
  will parse the header and extract the client's IP address and port. If the header
  isn't present, it will reject the connection with an error.

We also need to update the Teleport documentation to reflect the recommended
settings, the security implications of the PROXY Protocol and how to configure
it in different L4 load balancers.

## Alternative
As a straightforward alternative we could change default value to be `off` but that
can lead to a lot of sudden problems for the users who didn't change their default
setting of the `proxy_protocol` and use PROXY-enabled load balancers in their setup.
In this case connection from load balancer will be rejected when they will try to
send PROXY header with real client IP. And since we have a reason to believe that
big portion of our users will have this situation it's not a desired scenario.

## Security

The PROXY Protocol is a simple protocol that lacks native authentication and can be
easily spoofed by an attacker.
The attacker can send a fake PROXY Protocol header to the Teleport
Proxy/Auth and spoof the client's IP address and port if the Teleport isn't behind
a well-configured L4 load balancer. The attacker can use this to bypass IP Pinning
protection if the attacker steals a valid certificate and uses it to connect to
the Teleport Proxy/Auth. The certificate contains the client's IP address so it's
trivial to generate a fake PROXY Protocol header with the client's IP address
and port and bypass the IP Pinning protection.

The problem is that the PROXY Protocol is enabled by default in the Teleport config
and the user needs to explicitly disable it when the Teleport isn't
behind a L4 load balancer with PROXY Protocol enabled.

Given the above, Teleport should disable the PROXY Protocol by default so the
user doesn't need to explicitly disable it in the config. Ideally, Teleport should
set the `proxy_protocol` setting to `off` by default but this is a breaking change
for almost all users that have the PROXY Protocol enabled in their L4 load balancer.
As a result, we propose a compromise between breaking the majority of users
access to their Teleport clusters and breaking a subset of them that run with IP
Pinning enabled. The change of the default value of the `proxy_protocol` setting
to a new mode called `unspecified` will reduce the impact of this change and
guarantee that the Teleport Proxy/Auth will reject connections that try to spoof
the client's IP address and port to bypass IP Pinning protection.
---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 34 - Desktop Access - Windows

## What

RFD 33 defines the high-level goals and architecture for Teleport Desktop
Access.

This RFD specifies how Teleport Desktop Access integrates with Windows hosts,
including Microsoft Active Directory domains.

## Details

### Architecture

Windows Desktop Access is implemented using `windows_desktop_service` that
translates the Teleport desktop protocol into RDP:

```
+--------+
| web UI |
+--------+
    ^
    | desktop protocol over websocket
    v
+--------+
| proxy  |
+--------+
    ^
    | desktop protocol over mTLS
    v
+-------------------------+
| windows_desktop_service |--------------\
+-------------------------+-\            |
   ^                        |            |
   | RDP                    | RDP        | LDAP
   v                        v            |
+---------------------+  +-----------+  +-------------------+
| windows server 2012 |  | windows 7 |  | domain controller |
+---------------------+  +-----------+  +-------------------+
```

`windows_desktop_service` can talk to any number of remote Windows RDP hosts.
It can also talk to `localhost` RDP service, if installed on a Windows machine
in agent mode (described below).

`windows_desktop_service` has the ability to automatically discover available
Windows hosts from Active Directory by performing an LDAP search. In addition,
`windows_desktop_service` can use a static list of Windows hosts provided in
`teleport.yaml`.

### Supported versions

Windows Desktop Access supports Windows Server 2012 R2, Windows 7 and newer
versions. However, it should also work with any server that implements RDP and
supports Smart Card authentication.

### Protocol

Windows Desktop Access uses Remote Desktop Protocol (RDP). RDP service is built
into Windows and requires no additional software to be installed on the hosts.

RDP is a very complex protocol, with cruft accumulated over its long history
and frequent vulnerability discoveries (see [Security
Concerns](#security-concerns) below). It was not my first choice for this
implementation. However, it is the only option for supporting concurrent
sessions and allows easier customer rollout without per-host agents.

#### RDP client

`windows_desktop_service` implements an RDP client. There are no existing Go
libraries implementing enough of the RDP spec for a basic desktop session.

There are a few options, in order of preference:
- use [rdp-rs Rust library](https://crates.io/crates/rdp-rs) via FFI from Go,
  assuming it supports smart cards
- use [libfreerdp](https://www.freerdp.com/) via CGO, assuming it supports
  smart cards and concurrent outbound connections
- implement RDP protocol in Go from scratch; last resort due to the amount of
  work needed

### Modes

`windows_desktop_service` can run in two modes: gateway and agent.

#### Gateway mode

Gateway mode is for connecting to multiple remote Windows hosts, similar to
Kubernetes Access. This mode is shown in the [Architecture
diagram](#architecture) above.

This mode is easy for admins to set up and has minimal resource overhead.
However, this requires the Windows hosts to expose RDP ports to the
`windows_desktop_service`, which is more risky (see [Security
Concerns](#security-concerns) below).

#### Agent mode

Agent mode is for running a `windows_desktop_service` instance on the Windows
host as a system service. In this mode, connection is made over `localhost` and
the RDP port does not need to be exposed on the network.
`windows_desktop_service` can reverse-tunnel to the Teleport proxy, allowing
the Windows host to completely block all inbound connections. In the future,
`windows_desktop_service` could also collect more session information (like
eBPF on Linux) and enforce extra restrictions.

```
+--------+
| web UI |
+--------+
    ^
    | desktop protocol over websocket
    v
+--------+
| proxy  |
+--------+
    ^
    | desktop protocol over mTLS over reverse tunnel
    |
+---|------------------------------+
|   v                              |
| +-------------------------+      |
| | windows_desktop_service |      |
| +-------------------------+      |
|   ^                              |
|   | RDP over localhost           |
|   v                              |
| +-------------+                  |
| | RDP service |                  |
| +-------------+     Windows host |
+----------------------------------+
```

### Authentication

In Windows, authentication is handled differently for standalone and
Active Directory-enrolled machines. Standalone machines validate credentials
using the local user database (SAM). Active Directory-enrolled machines send
credentials to the domain controller for validation. Teleport Desktop Access
will support both modes.

Windows generally exposes 3 authentication mechanisms:
- smart cards
- username/password
- kerberos tickets

#### Smart cards

Smart cards implement asymmetric cert-based authn, using PKI configured on the
server/AD side. Since this is the only known way for us to support cert-based
authn, Teleport will implement a smart card emulator to authenticate over RDP.
This will be the primary authentication method.

Smart card authentication and emulator are described in more detail in RFD 35.

#### Username/password

Although username and password are the most universal, a major part of
Teleport's value is to provide strong authentication using short-lived
certificates. Supporting username and password authentication would weaken the
overall security of the system and as such will not be implemented.

#### Kerberos tickets

When a user is already authenticated against Kerberos (using one of the above
methods), they get a Ticket-granting ticket (TGT) which can be used to get
other tickets for specific services, such as an RDP host. Since Teleport has no
exposure to Kerberos tickets (even when using AAD as SSO provider) and we can't
assume that all clients use Windows machines, Teleport will not use TGTs for
RDP authentication.

### Host discovery

When a user starts a new desktop session, they must specify the Windows host to
connect to. Internally, Teleport tracks known Windows hosts using
`WindowsDesktop` objects.

There are 3 ways that `windows_desktop_service` discovers Windows hosts to
register:
- hardcoded list of hosts provided in the config file (see
  [configuration](#configuration))
- list of Active Directory-enrolled hosts obtained from AD via LDAPS (LDAP over
  SSL)
  - LDAP library: https://pkg.go.dev/github.com/go-ldap/ldap/v3
- local host, when running on a Windows machine in agent mode

By default, Teleport will only register hosts that are provided in the
configuration file. To enable host discovery over LDAP, additional configuration
is necessary (see [configuration](#configuration)).

#### Automatic Host Labels

Teleport will automatically apply the following host labels to hosts which are
discovered from Active Directory.

| Label | LDAP Attribute | Example |
| ----- | -------------- | ------- |
| `teleport.dev/computer_name` | `name` | `WIN-I5G06B8RT33`
| `teleport.dev/dns_host_name` | [`dNSHostName`](https://docs.microsoft.com/en-us/windows/win32/adschema/a-dnshostname) | `WIN-I5G06B8RT33.example.com`
| `teleport.dev/os` | [`operatingSystem`](https://docs.microsoft.com/en-us/windows/win32/adschema/a-operatingsystem) | `Windows Server 2012`
| `teleport.dev/os_version`| [`osVersion`](https://docs.microsoft.com/en-us/windows/win32/adschema/a-operatingsystemversion) | `4.0`
| `teleport.dev/windows_domain`| Sourced from config | `example.com`

### Concurrent sessions

RDP supports concurrent user sessions on the same host. It allocates a virtual
desktop for each user to isolate their activities. However, it only allows a
single session per user per host. If a user starts a new session on the host
that they are already logged into, they will log out the other session. This is
RDP behavior we cannot change.

All the usual controls on Teleport sessions apply to Desktop Access, like user
locking, concurrent session limits and idle timeouts.

### Configuration

New `teleport.yaml` section for `windows_desktop_service`:

```yaml
windows_desktop_service:
  enabled: yes # default false
  # listen_addr can share the port with the proxy web port using SNI.
  listen_addr: 0.0.0.0:3080
  public_addr: [rdp.example.com:3080]
  mode: "gateway" # or "agent"
  # (optional) ldap contains hostname and credentials for the LDAP server on
  # the Active Directory domain controller.
  # If specified, windows hosts will be automatically discovered by Teleport.
  #
  # Note: Teleport will only connect to LDAP over TLS and will always
  # validate the server certificate.
  ldap:
    host: ldap.example.com
    username: "ldap_user"
    password: "ldap_pass"
    # optional CA cert to use for LDAP server certificate validation.
    ca_cert: "/var/lib/teleport/ldap_ca.pem"
  # (optional) hosts is a list of hostnames to register as WindowsDesktop
  # objects in Teleport.
  # These are usually non-AD hosts.
  hosts:
  - win1.example.com
  - win2.example.com
  - ...
  # (optional) settings for enabling automatic desktop discovery via LDAP
  discovery:
    base_dn: '*' # wildcard searches from the root, leave empty to disable discovery
    filters:  # additional LDAP filters: https://ldap.com/ldap-filters/
    - filter1 # note: multiple filters are combined into an AND filter
    - filter2
  # (optional) host_labels applies labels to windows hosts for RBAC.
  # Each entry maps to a subset of hosts by regexp and applies a group of labels.
  # A host can match multiple regexps and will get a union of all the labels.
  # The regexp is matched against the host's DNS name, for example:
  # WIN-I5G06B8RT33.example.com (where example.com is the domain name)
  host_labels:
  - match: ".*"
    labels:
      env: prod
  - match: "^db.*"
    labels:
      type: database
      kind: postgres
  - match: "^dc\.example\.com$"
    labels:
      type: domain_controller
```

### Security concerns

RDP is a very complex protocol with a history of vulnerabilities. Using a
memory-safe client that we fully control and not exposing raw RDP publicly is
essential. Running Teleport in agent mode is and extra hardening step to avoid
exposing RDP even on private networks.

Generally, admins are still responsible for patching their Windows hosts to
pick up any RDP server fixes that come out.

The RDP client implementation in Teleport should be reasonably paranoid, verify
the identity of the target RDP host and treat it as potentially malicious.

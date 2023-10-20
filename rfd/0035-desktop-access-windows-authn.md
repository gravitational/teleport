---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 35 - Desktop Access - Windows Certificate Authentication

## What

RFD 33 defines the high-level goals and architecture for Teleport Desktop
Access. RFD 34 defines the integration details for Windows hosts.

Desktop Access for Windows supports:
- certificate-based authentication, by emulating smart cards
- username/password authentication, as a fallback when certificates fail

This RFD describes the details of the former, certificate-based approach.

## Details

### Background

While Windows does not support generic mTLS or similar PKI-based
authentication, it does support smart cards. Smart cards are similar to HSMs or
U2F/WebAuthn tokens - they store private keys in and expose certificates and
basic asymmetric crypto operations.

The server side (Windows host or domain controller) validate the credentials
against a trusted Certificate Authority. More information at
https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities

Smart cards are typically hardware devices, but a driver can be
implemented to emulate them in software
([example](http://frankmorgner.github.io/vsmartcard/virtualsmartcard/README.html)).
Since this is the only known way for us to support cert-based authn, Teleport
will implement a smart card emulator to authenticate over RDP. This emulator
will use Teleport TLS certs and expose the User TLS CA for import into the
Windows trust store.

Note that RDP implements smart card authentication by proxying low-level driver
commands to the client (`windows_desktop_service` in our case) over the RDP
connection:

```
+---------+                           +---------+                              +---------+
| driver  | (installed in client)     | client  |                              | server  |
+---------+                           +---------+                              +---------+
     |                                     |                                        |
     |                                     | init RDP connection                    |
     |                                     |--------------------------------------->|
     |                                     |                                        |
     |                                     |                        authn challenge |
     |                                     |<---------------------------------------|
     |                                     |                                        |
     |                                     | smart card info (name, slot, PIN)      |
     |                                     |--------------------------------------->|
     |                                     |                                        |
     |                                     |         smart card RDP virtual channel |
     |                                     |<---------------------------------------|
     |                                     |                                        |
     |      smart card RDP virtual channel |                                        |
     |<------------------------------------|                                        |
     |                                     |                                        |
     |                                     |      smart card info (name, slot, PIN) |
     |<-----------------------------------------------------------------------------|
     |                                     |                                        |
     |                                     |            low-level hardware messages |
     |<-----------------------------------------------------------------------------|
     |                                     |                                        |
     | low-level hardware messages         |                                        |
     |----------------------------------------------------------------------------->|
     |                                     |                                        |
     |                                     |                               authn OK |
     |                                     |<---------------------------------------|
```

More information:
- [CredSSP](https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-CSSP/%5bMS-CSSP%5d.pdf) - authentication part of RDP
- [Smart Card virtual channel](https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPESC/%5bMS-RDPESC%5d.pdf)

Yubico implements the same authentication flows, using an actual hardware
device. https://www.yubico.com/products/computer-login-tools/ and linked docs
can be used for inspiration.

### Limitations

Smart cards are only supported for logins into domain accounts - only in Active
Directory environments. There is no builtin support for standalone Windows
hosts, however it's possible to implement similar to [Yubico Login for
Windows](https://support.yubico.com/hc/en-us/articles/360013708460-Yubico-Login-for-Windows-Configuration-Guide)
in the future.

### Client certificates

When `windows_desktop_service` receives a client mTLS connection, it needs to
issue an ephemeral smart card certificate to present to RDP server. This
certificate will be largely based on the received client certificate, with [some
tweaks](https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration#client-certificate-requirements-and-mappings).

> Subject Alternative Name = Other Name: Principal Name= (UPN). For example:
> UPN = user1@name.com
> The UPN OtherName OID is: "1.3.6.1.4.1.311.20.2.3"
> The UPN OtherName value: Must be ASN1-encoded UTF8 string

[UPN](https://docs.microsoft.com/en-us/windows/win32/adschema/a-userprincipalname)
looks like an email address. But note that the user part is the target Windows
user (not the Teleport username) and the domain part is the name of AD domain
that we're connecting to (not the email domain in Teleport SSO login).

### Forwarding credentials

In theory, RDP server should keep forwarding any smart card hardware commands
to the client (`windows_desktop_service` in this case). If a user opens another
RDP client within their remote desktop, they should just be able to log into
any host in the same domain without a password.

This is a nice-to-have and not a strict requirement. If it does not work right
away, we can try emulating a local smart card using an agent on the remote
host.

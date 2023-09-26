# RDP client library

This library consists of 2 parts: the actual Rust library and the Go wrapper
around it. Go wrappers calls into Rust code using CGO. Rust library is compiled
as a static C library.

## High-level

You should read the RFDs
[34](https://github.com/gravitational/teleport/blob/master/rfd/0034-desktop-access-windows.md)
and
[35](https://github.com/gravitational/teleport/blob/master/rfd/0035-desktop-access-windows-authn.md)
first.

The gist of it is: between Teleport and the Windows host we use classic RDP. In
order to authenticate with X.509 certificates instead of passwords, we emulate
a smartcard device over the RDP connection. This emulated smartcard uses
Teleport-issued X.509 certificates for Windows to verify. The Active Directory
Windows environment should be configured with Teleport CA in its trust store.

## Go

All the wrapper code is in `client.go`. This wrapper calls into Rust to
establish an RDP connection and provides it with credentials (key/cert). The
wrapper then proxies input/output data, translating between Teleport Desktop
Protocol (TDP) and RDP.

## Rust

Rust code is under `src`. It uses several libraries (see `Cargo.toml`), but the
main one is `rdp-rs`. `rdp-rs` is a pure Rust (partial) implementation of the
RDP protocol. We forked that library at
https://github.com/gravitational/rdp-rs/. Our fork implements several changes
necessary to support smartcard emulation.

Notes on specific Rust modules:

### lib.rs

Entrypoint of the library. This implements the CGO API surface and basic RDP
connection establishment. During RDP connection, it negotiates the "rdpdr"
static virtual channel, which is used for smartcard device "redirection" (read
more below).

Most of the protocol is implemented in the `rdp-rs` crate and the spec is
https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPBCGR/%5bMS-RDPBCGR%5d.pdf

### rdpdr.rs

The device redirection layer. This lives inside of the RDP connection, under
the MCS layer. Device redirection can mirror any disk, serial, smartcard or
printer from the RDP client to the server. In our case, we redirect a hardcoded
fake smartcard.

The spec is at:
https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPEFS/%5bMS-RDPEFS%5d.pdf

### scard.rs

The smartcard reader emulation layer. This is redirected over RDPDR. Smartcards
typically connect to a computer via a reader, e.g. a USB card reader. The code
in `scard.rs` emulates a single hardcoded reader called "Teleport" with a
single smartcard inserted.

The spec is at:
https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPESC/%5bMS-RDPESC%5d.pdf

### piv.rs

The smartcard emulation and authn layer. This is the fake smartcard "inserted"
into the "Teleport" reader above. This smartcard has a PIV applet "installed"
and implements the protocol at:
https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf

It basically has file storage for some user-identifying info and an X.509
certificate. It also has an RSA private key that it does challenge/response
authentication with, to prove the ownership of that X.509 certificate.

## Memory management

Note that all memory allocated on the Go side must be freed on the Go side.
Same goes for Rust. These languages have different memory management systems
and can't free each others' memory. This is why you'll see the weird `free_*`
hops from one side to the other.

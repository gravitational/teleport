# RDP client

This package consists of 2 parts: the Rust RDP client and the Go
wrapper. The Rust RDP client runs as a standalone process and Go
wrappers communicate with it over a local gRPC connection.

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

All the wrapper code is in `client.go`. This wrapper communicates with the Rust client
to establish an RDP connection and provides it with credentials (key/cert). The
wrapper then proxies input/output data between the browser and the Rust process.

## Rust

Rust code is under `src`. It uses several libraries (see `Cargo.toml`), but the
main one is [`IronRDP`](https://github.com/Devolutions/IronRDP/). `IronRDP` is a
pure Rust implementation of the RDP protocol.

Notes on specific Rust modules:

### main.rs

Entry point of the binary. This parses the configuration from CLI arguments,
connects to the gRPC server, and starts the RDP session.

### client.rs

Core RDP session logic. This establishes the RDP connection and drives the RDP session.
During RDP connection, it negotiates the "rdpdr" static virtual channel,
which is used for smartcard device "redirection" (read more below).

### rdpdr.rs

The device redirection layer. This lives inside the RDP connection, under
the MCS layer. Device redirection can mirror any disk, serial, smartcard or
printer from the RDP client to the server. In our case, we redirect a hardcoded
fake smartcard.

The spec is at:
https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPEFS/%5bMS-RDPEFS%5d.pdf

### rdpdr/scard.rs

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

## Go/Rust Interface

Each Go `rdpclient.Client` (`client.go`) has a corresponding Rust `client::Client` (`client.rs`).
When a desktop session starts, the Go client creates and starts its corresponding Rust client process.
The Rust process receives the gRPC server address through the CLI argument and connects to it.
All communication between the Go and Rust components is then performed over gRPC.

## Logging

The log level is configured via the `RUST_LOG` environment variable. The Rust client writes logs to `stdout`.

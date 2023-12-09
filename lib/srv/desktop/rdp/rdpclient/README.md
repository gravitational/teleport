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

### Go/Rust Interface

Each Go `rdpclient.Client` has a corresponding Rust `client::Client` (`client.rs`). The Go
`rdpclient.Client` calls `client_run`, which calls `client::Client::run`, which creates the
Rust `client::Client` in (Rust) memory where it fields incoming messages from the RDP server
to convert to TDP and pass back into the Go `rdpclient.Client`, and waits for messages coming
from Go (typically user input from the browser) which it sends on to the RDP server.

When the Rust [`client::Client` is created](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client.rs#L92),
it is passed a `CgoHandle`, which is the pointer to it's corresponding Go `rdpclient.Client`.
It also calls [`ClientHandle::new`](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client.rs#L112),
which creates a `ClientHandle` (proxy to the `Sender` half of a channel) and `FunctionReceiver`
(proxy to the `Receiver` half of a channel). It places this `ClientHandle` in a
[global, static map](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client/global.rs#L43-L47) of `ClientHandle`s indexed by `CgoHandle`, and spawns
[a loop that waits to receive messages on the `FunctionReceiver`](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client.rs#L300-L308).

Now, when the Go `rdpclient.Client` needs to call a function (such as communicate a mouse-click event) on it's corresponding Rust `client::Client`, it passes in it's own `CgoHandle`, which is used to find it's corresponding `ClientHandle` in the global
cache, which is used to send the corresponding message (remember, `CgoHandle` is a proxy to a channel's `Sender` half) to the
waiting `FunctionReceiver` (the channel's `Receiver` half), which then handles it accordingly. See [`client_write_rdp_pointer`](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/lib.rs#L377-L391)
for an example.

While this may seem at first glance like a very indirect way of communicating between the Go and Rust halfs of the `Client`,
it has the virtue of saving us a lot of Rust concurrency enforcement headaches as compared to passing a Rust pointer to the
`client::Client` to and from Go. In this system, we only need a small piece of the global cache of channels to remain `Send + Sync` (see [the module level documentation for `global.rs`](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client/global.rs#L15-L32)),
and can use the "automatic" synchronization inherent to message passing over channels to take care of many downstream
concurrency concerns. In a previous iteration of the system where the Rust `client::Client` was reconstructed from a pointer
shared between Go and Rust, we needed the entire `Client` to be `Send + Sync`, and therefore to concern ourselves with the
manual use of various `Mutex`es and other more error prone concurrency primitives.

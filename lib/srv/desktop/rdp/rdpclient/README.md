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
main one is [`IronRDP`](https://github.com/Devolutions/IronRDP/). `IronRDP` is a
pure Rust implementation of the RDP protocol.

Notes on specific Rust modules:

### lib.rs

Entrypoint of the library. This implements the CGO API surface and basic RDP
connection establishment. During RDP connection, it negotiates the "rdpdr"
static virtual channel, which is used for smartcard device "redirection" (read
more below).

### rdpdr.rs

The device redirection layer. This lives inside of the RDP connection, under
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

## Memory management

Note that all memory allocated on the Go side must be freed on the Go side.
Same goes for Rust. These languages have different memory management systems
and can't free each others' memory. This is why you'll see the weird `free_*`
hops from one side to the other.

### Go/Rust Interface

Each Go `rdpclient.Client` (`client.go`) has a corresponding Rust `client::Client` (`client.rs`).
When a desktop session is started, the Go client is created, and in turn creates and
starts its corresponding Rust client.

When the Rust client is created, it is passed a [`cgo.Handle`](https://pkg.go.dev/runtime/cgo#Handle)
(`CgoHandle` in the Rust codebase) that points to the Go client that created it. A custom Rust type
`ClientHandle`, which functions as a handle to the Rust `client::Client`, is then added to a global
map indexed by `CgoHandle` (`global.rs`). In this way we maintain a mapping between corresponding objects
in Rust and Go memory.

From that point on, whenever the Go client needs to call a function that the Rust client implements,
it passes in it's own `cgo.Handle` (look for `pub unsafe extern "C" fn` in `lib.rs`), which tells Rust
where to find the correct `ClientHandle`, and whenever the Rust client needs to call a function the Go
client implements, it passes in the Go client's `CgoHandle` (look for functions with `//export funcname`
comments), which Go uses to re-construct the `rdpclient.Client`.

##### A note on "why not reconstruct the Rust client from a pointer as well?"

While this may seem at first glance like a very indirect way of communicating between the Go and Rust halfs of the `Client`,
it has the virtue of saving us a lot of Rust concurrency enforcement headaches as compared to passing a Rust pointer from Go.

See a [previous iteration of this document](https://github.com/gravitational/teleport/pull/26874/commits/c2edddfcd84a41d4a5554c52fd0688e235128d7c)
for a deeper exploration of all of the memory safety footguns we were dealing with in such a system.

In this current system we have far less `unsafe` code, and we only need a small piece of the global cache of channels to remain
`Send + Sync` (see [the module level documentation for `global.rs`](https://github.com/gravitational/teleport/blob/acb22584f5423f7b184cb1a8e30e2ada62bafb16/lib/srv/desktop/rdp/rdpclient/src/client/global.rs#L15-L32)).
In this system we can use the "automatic" synchronization inherent to message passing over channels to take care of many downstream
concurrency concerns.

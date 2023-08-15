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

The `Client` struct in Rust is the object we pass back and forth between Rust and Go,
so that Go can decide when to call its methods. We do this by allocating it on the heap
in Rust and then leaking the memory into a raw pointer which is passed back to Go:

```rust
// Create the client
let client = Client::new()
// Move it to the heap
let client = Box::new(client)
// Convert it to a raw pointer, pass this back to Go.
// Go is now responsible for the proper usage and
// freeing of this memory.
let client_ptr = Box::into_raw(client)
```

From that point on, the Client should be considered to be "owned" by Go (in Rust parlance),
meaning that it's the Go side's responsibility to manage its memory. The two key elements
of that responsibility are:

1. Freeing the memory (once and only once) after it's done using it.
2. Ensuring that the object isn't used after it's been freed.

#### Enforcing proper memory semantics in Rust

We call back into Rust from Go using functions like

```rust
#[no_mangle]
pub unsafe extern "C" fn call_into_rust_from_go(client_ptr: *const Client) -> CGOErrCode {
  // Get immutable reference of Client
  let client: &Client = client_ptr.as_ref().unwrap(); // unwrap for brevity, handle error in real code
  // Do stuff with our immutable reference to Client
  client.do_stuff()
}
```

What's important here is that we stay disciplined in the following ways

1. The `Client` itself is [`Sync`](https://doc.rust-lang.org/std/marker/trait.Sync.html)
2. We access the client as an **immutable reference** (`&Client`).

In order to understand why, its important to understand that when `call_into_rust_from_go`
is called from Go, it can be:

1. scheduled by the Go scheduler on any arbitrary thread, and
2. can run concurrently with another thread also using the `Client`

##### `Send + Sync`

With these constraints in mind, we can determine that `Client` must be forced to remain `Sync`,
because the very [definition of `Sync`](https://doc.rust-lang.org/std/marker/trait.Sync.html)
is "[t]ypes for which it is safe to share references between threads."
Because the `Client` is owned by Go (meaning logically, we can only have references in Rust), and
per 1 and 2 above the references in Rust must be shared between threads, the `Client` must be safe to
do so (aka it must be `Sync`).

`Client` must also be `Send`, because we need to create it in one thread (one call from Go) and then drop it
in another (another call from an arbitrary goroutine in Go), which in Rust semantics is equivalent to passing
an owned object over thread boundaries, aka [the definition of `Send`](https://doc.rust-lang.org/std/marker/trait.Send.html).

We enforce `Send + Sync` via this little Rust compiler hack

```rust
const _: () = {
    const fn assert_send_sync<T: Send + Sync>() {}
    assert_send_sync::<Client>();
};
```

<<<<<<< HEAD
and tell the compiler the nature of `Client` in the `where` clause of each function signature at the FFI
boundary:

```rust
#[no_mangle]
pub unsafe extern "C" fn call_into_rust_from_go(client_ptr: *mut Client) -> CGOErrCode
where
    Client: Send + Sync, // Tell the compiler that `Client` is `Send + Sync`
{
    //...
}
```

=======

> > > > > > > ironrdp

##### Immutable Reference (`&Client`)

One of the foundational memory guarantees of the Rust compiler is that a mutable borrow (`&mut T`)
is an _exclusive reference_, meaning that if you have an `&mut T` there can be no other references
to the same object.

Because every call from Go into Rust is `unsafe`, and it's possible to coerce that pointer into
more or less whatever we want in `unsafe`, and you can do more with mutable references, there's
a temptation to get a `&mut Client` at the top of `call_into_rust_from_go` like

```rust
#[no_mangle]
pub unsafe extern "C" fn call_into_rust_from_go(client_ptr: *mut Client) -> CGOErrCode
where
    Client: Send + Sync,
{
    // Get immutable reference of Client
    let client: &mut Client = Box::leak(Box::from_raw(client_ptr));
    // Do stuff with our immutable reference to Client
    client.do_mutable_stuff()
}
```

Just because you can do something doesn't mean you should, and this is a thing you shouldn't do
because it breaks all sorts of typical Rust semantics downstream. If you need mutable borrows then
you'll need to use a `Mutex` and look out for deadlocks.

#### An analogy to pure Rust

Go "owns" the `Client`, and when it calls into Rust with `Client` it essentially is [locking that
call onto a thread](https://stackoverflow.com/questions/28354141/c-code-and-goroutine-scheduling/28354879#28354879)
with a `Client` ref (`&Client`) until it returns. A helpful way to reason about this is to imagine how these semantics
would be instantiated in a pure Rust program.

A naive way to try and do this is [like so](https://play.rust-lang.org/?version=stable&mode=debug&edition=2021&gist=91456b6198de6394ecc5b68f1210db70).
However this doesn't work because

```
   Compiling playground v0.0.1 (/playground)
error[E0373]: closure may outlive the current function, but it borrows `client`, which is owned by the current function
  --> src/main.rs:26:32
   |
26 |     let handle = thread::spawn(|| {
   |                                ^^ may outlive borrowed value `client`
...
31 |         println!("{}", client.get_some_field());
   |                        ------ `client` is borrowed here
   |
note: function requires argument type to outlive `'static`
```

In other words, the compiler can't guarantee that the memory underlying the `&Client` wont be freed before the thread finishes,
so it disallows this entirely.

The solution for this in Rust is to use [scoped threads](https://doc.rust-lang.org/std/thread/fn.scope.html), which ensure the compiler
that the threads are all joined by the end of the scope, allowing you to pass references into the threads. So semantically, what we should
be aiming for is [a program analogous to this](https://play.rust-lang.org/?version=stable&mode=debug&edition=2021&gist=b31ad74edfeb4e2c90551f728fcbcb32).

The key here is that everything at the top level scope of `main` must be implemented by Go; that is to say Go is responsible for ensuring that it drops
the `Client` at the end of the program, and importantly that it doesn't do so until the threads it spawns (`call_into_rust_from_go`) have returned ("joined").

---
authors: Alan Parra (alan.parra@goteleport.com)
state: implemented
---

# RFD 53 - Passwordless for FIDO2 clients

## What

Passwordless support for FIDO2 clients, including Web and CLI.

This RFD discusses "standard" FIDO2 clients - specialized/native APIs are
covered by accompanying designs.

This is a part of the [Passwordless RFD][passwordless rfd].

## Why

See the [Passwordless RFD][passwordless rfd].

## Details

### Browser-based clients

Browser-based clients are the simplest to discuss: they rely on the
[CredentialsContainer](https://developer.mozilla.org/en-US/docs/Web/API/CredentialsContainer)
API - that's it.

The flow is similar to a "regular" MFA/WebAuthn login, with the exception that
the interface doesn't need to ask for user/password; instead it should ask the
server for a passwordless challenge.

```
                                               Web authentication

                                ┌─┐
                                ║"│
                                └┬┘
                                ┌┼┐
     ┌─────────────┐             │                     ┌───────┐                                  ┌────────┐
     │authenticator│            ┌┴┐                    │browser│                                  │Teleport│
     └──────┬──────┘           user                    └───┬───┘                                  └───┬────┘
            │                   │        start login       │                                          │
            │                   │ ─────────────────────────>                                          │
            │                   │                          │                                          │
            │                   │                          │      CreateAuthenticateChallenge()       │
            │                   │                          │      (no user or password informed)      │
            │                   │                          │ ─────────────────────────────────────────>
            │                   │                          │                                          │
            │                   │                          │                                          │
            │                   │                          │ <─────────────────────────────────────────
            │                   │                          │                                          │
            │                   │                          │────┐                                     │
            │                   │                          │    │ navigator.credentials.get()         │
            │                   │                          │<───┘                                     │
            │                   │                          │                                          │
            │                   │ choose authenticator? (1)│                                          │
            │                   │ <─────────────────────────                                          │
            │                   │                          │                                          │
            │      *taps*       │                          │                                          │
            │<──────────────────│                          │                                          │
            │                   │                          │                                          │
            │                   │      enter PIN? (2)      │                                          │
            │                   │ <─────────────────────────                                          │
            │                   │                          │                                          │
            │                   │       *enters PIN*       │                                          │
            │                   │ ─────────────────────────>                                          │
            │                   │                          │                                          │
            │                   │  tap authenticator? (3)  │                                          │
            │                   │ <─────────────────────────                                          │
            │                   │                          │                                          │
            │      *taps*       │                          │                                          │
            │<──────────────────│                          │                                          │
            │                   │                          │                                          │
            │                   │  choose credential? (4)  │                                          │
            │                   │ <─────────────────────────                                          │
            │                   │                          │                                          │
            │                   │         *chooses*        │                                          │
            │                   │ ─────────────────────────>                                          │
            │                   │                          │                                          │
            │               sign challenge                 │                                          │
            │<─────────────────────────────────────────────│                                          │
            │                   │                          │                                          │
            │        user handle, signed challenge         │                                          │
            │─────────────────────────────────────────────>│                                          │
            │                   │                          │                                          │
            │                   │                          │ Authenticate(userHandle, signedChallenge)│
            │                   │                          │ ─────────────────────────────────────────>
            │                   │                          │                                          │
            │                   │                          │                    ok                    │
            │                   │                          │ <─────────────────────────────────────────
     ┌──────┴──────┐           user                    ┌───┴───┐                                  ┌───┴────┐
     │authenticator│            ┌─┐                    │browser│                                  │Teleport│
     └─────────────┘            ║"│                    └───────┘                                  └────────┘
                                └┬┘
                                ┌┼┐
                                 │
                                ┌┴┐
```

(Simplified Web authentication diagram.)

All user<->browser interaction steps depend on the browser implementation. The
observations below are meant to inform RFD readers on the status quo at the time
of writing.

(1) may be skipped if there is only a single authenticator present in the
machine. In case of ambiguity (more than one authenticator), then the user is
prompted to pick their authenticator of choice by tapping it.

(2) and (3) may be skipped for biometric authenticators - the initial tap is
sufficient for both verification and presence checks.

(4) may also be skipped if there is a single resident key for the RPID (Relying
Party ID) in the authenticator, otherwise a menu with options shows up.

### CLI clients (aka `tsh`)

There are essentially two options for CLI clients / `tsh`:

1. Leverage browsers and delegate FIDO2 authentication to them; or
2. Write CLI-native FIDO authentication.

(1) is attractive enough to merit discussion. Its main downside is that we can't
get a purely CLI-based flow, a browser needs to pop up. The upside is that we
get to benefit from the work browsers do for authentication and security. This
option means we get support for FIDO2 and native biometrics in a single stroke,
without having to deal with the delicate issues they pose ourselves.

(2) is the current approach used by Teleport, as it brings characteristics that
are important to our users. Unfortunately, it forces us to tackle some of the
difficult problems already solved by browsers, which often require
platform-specific work as well.

A compromise between both approaches is possible. For example, `tsh` could rely
on browsers in most cases (1), but implement a best-effort CLI-native solution
(2). This takes pressure out of the CLI implementation and allows it to focus in
the more impactful use-cases, instead of having to provide full functionality.

For the moment, the RFD chooses the CLI-native approach (2). A possible design
for browser-based CLI authentication was briefly considered, but discarded due
to security concerns.

#### CLI-native authentication

CLI-native passwordless relies on
[libfido2](https://github.com/Yubico/libfido2), a C library that allows
interaction with FIDO2 resident keys. (The official Go wrapper is
[github.com/keys-pub/go-libfido2](https://github.com/keys-pub/go-libfido2/).)

One of the issues for passwordless interaction is that the client (`tsh`) gets
no information about registered credentials. The usual tactic of attempting an
assertion on all hardware keys falls short, as an authenticator may hold
multiple keys for the same Relying Party - in this case, the authenticator picks
the account, not the user!

The solution is similar to the one used by browsers - we make all hardware keys
blink and ask the user to choose the desired key by touching it. Authentication
proceeds by enumerating the credentials from that key and letting the user,
again, pick the desired one. (See libfido2's [examples/select.c](
https://github.com/Yubico/libfido2/blob/master/examples/select.c).)

Authentication diagram below:

```
                                               tsh authentication

                                ┌─┐
                                ║"│
                                └┬┘
                                ┌┼┐
     ┌─────────────┐             │                   ┌───┐                                       ┌────────┐
     │authenticator│            ┌┴┐                  │tsh│                                       │Teleport│
     └──────┬──────┘           user                  └─┬─┘                                       └───┬────┘
            │                   │       tsh login      │                                             │
            │                   │ ─────────────────────>                                             │
            │                   │                      │                                             │
            │                   │                      │        CreateAuthenticateChallenge()        │
            │                   │                      │        (no user or password informed)       │
            │                   │                      │ ────────────────────────────────────────────>
            │                   │                      │                                             │
            │                   │                      │                  challenge                  │
            │                   │                      │ <────────────────────────────────────────────
            │                   │                      │                                             │
            │          wait for touch (1)              │                                             │
            │          (all authenticators)            │                                             │
            │<─────────────────────────────────────────│                                             │
            │                   │                      │                                             │
            │                   │ choose authenticator?│                                             │
            │                   │ <─────────────────────                                             │
            │                   │                      │                                             │
            │      *taps*       │                      │                                             │
            │<──────────────────│                      │                                             │
            │                   │                      │                                             │
            │                 touched                  │                                             │
            │─────────────────────────────────────────>│                                             │
            │                   │                      │                                             │
            │                   │    enter PIN? (2)    │                                             │
            │                   │ <─────────────────────                                             │
            │                   │                      │                                             │
            │                   │     *enters PIN*     │                                             │
            │                   │ ─────────────────────>                                             │
            │                   │                      │                                             │
            │       Credentials(RPID, pin) (3)         │                                             │
            │<─────────────────────────────────────────│                                             │
            │                   │                      │                                             │
            │        authenticator credentials         │                                             │
            │        (includes user handle)            │                                             │
            │─────────────────────────────────────────>│                                             │
            │                   │                      │                                             │
            │                   │  choose credential?  │                                             │
            │                   │ <─────────────────────                                             │
            │                   │                      │                                             │
            │                   │       *chooses*      │                                             │
            │                   │ ─────────────────────>                                             │
            │                   │                      │                                             │
            │   Assertion (4)   │                      │                                             │
            │   (RPID, challenge, credential, pin)     │                                             │
            │<─────────────────────────────────────────│                                             │
            │                   │                      │                                             │
            │                   │  tap authenticator?  │                                             │
            │                   │ <─────────────────────                                             │
            │                   │                      │                                             │
            │      *taps*       │                      │                                             │
            │<──────────────────│                      │                                             │
            │                   │                      │                                             │
            │            signed challenge              │                                             │
            │─────────────────────────────────────────>│                                             │
            │                   │                      │                                             │
            │                   │                      │ AuthenticateSSH(userHandle, signedChallenge)│
            │                   │                      │ ────────────────────────────────────────────>
            │                   │                      │                                             │
            │                   │                      │               SSH credentials               │
            │                   │                      │ <────────────────────────────────────────────
            │                   │                      │                                             │
            │                   │          ok          │                                             │
            │                   │ <─────────────────────                                             │
     ┌──────┴──────┐           user                  ┌─┴─┐                                       ┌───┴────┐
     │authenticator│            ┌─┐                  │tsh│                                       │Teleport│
     └─────────────┘            ║"│                  └───┘                                       └────────┘
                                └┬┘
                                ┌┼┐
                                 │
                                ┌┴┐
```

(As a side note, it's interesting to note the differences in interaction between
Web and `tsh` and how it may provide clues as to how browsers work.)

(1) is the "key selection" step explained above, requiring one touch\* from the
user to choose the desired authenticator.

(2) is the PIN-entering step. For biometric authenticators, we can "merge" steps
(1), (2) and (3) by using [Credentials](
https://github.com/keys-pub/go-libfido2/blob/9246c0bee502c5f2c2495557f07fa5f45da8cd1c/fido2.go#L770)
as the method to choose the authenticator. This technique requires one user
touch/verification and avoids the need for PINs with biometric authenticators.

(3) is when we enumerate resident keys for the {authenticator, RPID} pair. We
can skip user interaction if only a single key exists.

(4) is the assertion step, where the authenticator signs the challenge. This
step requires a final touch/verification from the user.

\* currently go-libfido2 doesn't expose operations equivalent to
[fido_dev_touch_begin](https://developers.yubico.com/libfido2/Manuals/fido_dev_get_touch_begin.html)/[
fido_dev_touch_status](https://developers.yubico.com/libfido2/Manuals/fido_dev_get_touch_status.html),
but we can make do by attempting an [Assertion](
https://github.com/keys-pub/go-libfido2/blob/9246c0bee502c5f2c2495557f07fa5f45da8cd1c/fido2.go#L639).
(We could fork it and add it ourselves; I have a local patch that does just
that, but I'd rather keep light on forking if possible.)

#### libfido2 and Teleport

[libfido2](https://github.com/Yubico/libfido2) adds a native dependency to
Teleport. Its Go wrapper,
[go-libfido2](https://github.com/keys-pub/go-libfido2/), does little to
alleviate the pains of dealing with a native dependency (in fact, as a wrapper,
there is little it could do).

libfido2 itself depends on [libcbor](https://github.com/pjk/libcbor), OpenSSL
1.1 or newer, [zlib](https://zlib.net/) and libudev (Linux only, part of
systemd).

Care must be taken to produce deterministic (and equivalent!) builds for the
various supported operating systems - as a last resort, building libcbor and
libfido2 from source may be necessary (pulling the more complex, less sensitive
libraries from package managers).

Linking for go-libfido2 varies depending on OS - it defaults to
[static for macOS](
https://github.com/keys-pub/go-libfido2/blob/master/fido2_static.go) (with an
embedded libcbor binary) and [dynamic for Linux and Windows](
https://github.com/keys-pub/go-libfido2/blob/master/fido2_other.go#L6) (with
various embedded binaries for Windows). We should consider replacing those
during build, or once again forking go-libfido2 in order to standardize building
and linking strategies. (Static linking seems to be the best option for `tsh`.)

During development, the libfido2-aware code in Teleport is to be protected by a
`libfido2` build tag - said tag could be kept in the final product if dealing
with the library proves to be a hassle.

### Security

Security considerations are largely unchanged in relation to the
[Passwordless RFD][passwordless rfd].

It's important to note that CLI applications with hardware key access are
particularly dangerous, as they are not subject to the security measures
we've come to expect from browsers (in fact, _any_ untrusted application that
interacts with hardware keys should be used with caution).
Those concerns are not exclusive to WebAuthn, but are made more delicate in face
of passwordless credentials.

`tsh` does its best to be well-behaved and mitigates hardware key access
concerns by being open source, providing signed binaries and its own Cloud
solution.

(Security considerations for the [browser-based CLI authentication](
#browser-based-cli-authentication) alternative section are contained within it.)

### UX

UX is discussed throughout the design, but here is a summary of changes:

**Browser-based clients**

The exact passwordless flow is browser-dependent, see the diagram in the
[browser-based clients](#browser-based-clients) section for a quick reference.

Web UI changes are outside of the scope of the design, but it is likely that the
user will need the means to choose between "regular" and "passwordless" flows
before the ceremony starts. A real-world example of such flow may be found at
Microsoft's https://login.live.com/.

Registration has to make the same decision: does it take a resident key slot to
allow passwordless? The decision is delegated to the Web UI, but it is likely
users will want to decide in a case-by-case basis, specially when using hardware
keys with storage limitations.

**CLI clients (aka `tsh`)**

Similarly to browser-based authentication, `tsh login` needs to know beforehand
whether to go for the "regular" or "passwordless" flow. The decision comes from
the [passwordless cluster settings](
https://github.com/gravitational/teleport/blob/master/rfd/0052-passwordless.md#cluster-settings),
with a possible user-override via the `--auth` flag.

Example of a login with multiple hardware keys, PIN, and multiple credentials
(some steps may be skipped in simpler scenarios, see the
[CLI-native authentication](#cli-native-authentication) section):

```shell
$ tsh login --proxy=example.com --auth=passwordless
> Tap your security key
*taps*
> Enter your security key PIN:
*enters PIN*
> [1] alpaca
> [2] llama
> Choose the user for login: *chooses*
> Tap your security key again to complete the login
*taps*
> > Profile URL:        https://example.com
>   Logged in as:       llama
>   Cluster:            example.com
>   Roles:              access, editor
>   Logins:             llama
>   Kubernetes:         enabled
>   Valid until:        2021-10-04 23:32:29 -0700 PDT [valid for 12h0m0s]
>   Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty
```

`tsh mfa add` must make the same decision when adding new FIDO2 keys (aka
WebAuthn in interface lingo).

Example `tsh mfa add` with passwordless / resident key creation, including
initial PIN setup:

```shell
$ tsh mfa add
> Choose device type [TOTP, WEBAUTHN]: webauthn
> Enter device name: pwdless-key
> Allow passwordless logins [YES, NO]: yes
> Tap any *registered* security key
*taps*
> Tap your *new* security key
*taps*
> Set up a new PIN for your security key:
*enters PIN*
> Confirm your new security key PIN:
*enters PIN*
> MFA device "pwdless-key" added.
```

(A possible fallback for registration is to reject hardware keys without a
configured PIN, directing the user to configure their key using vendor-specific
apps.)

## Alternatives considered

### Pure Go FIDO2 library

An alternative to the pains of libfido2 (and shortcomings of go-libfido2) is to
write our own, pure Go, FIDO2 library.

The flynn/u2f library is backed by the necessary HID interfaces and has a parked
[webauthn](https://github.com/flynn/u2f/tree/webauthn) branch that could be used
as a starting point.

This alternative seems a bit extreme, at least for the moment, so it wasn't
explored much further.

<!-- Links -->

[passwordless rfd]: https://github.com/gravitational/teleport/blob/master/rfd/0052-passwordless.md

<!-- Plant UML diagrams -->
<!--

```plantuml
@startuml

title Web authentication

participant authenticator
actor user
participant browser
participant "Teleport" as server

user -> browser: start login

browser -> server: CreateAuthenticateChallenge()\n(no user or password informed)
server -> browser:

browser -> browser: navigator.credentials.get()
browser -> user: choose authenticator? (1)
user -> authenticator: *taps*
browser -> user: enter PIN? (2)
user -> browser: *enters PIN*
browser -> user: tap authenticator? (3)
user -> authenticator: *taps*
browser -> user: choose credential? (4)
user -> browser: *chooses*
browser -> authenticator: sign challenge
authenticator -> browser: user handle, signed challenge

browser -> server: Authenticate(userHandle, signedChallenge)
server -> browser: ok

@enduml
```

```plantuml
@startuml

title tsh authentication

participant authenticator
actor user
participant tsh
participant "Teleport" as server

user -> tsh: tsh login

tsh -> server: CreateAuthenticateChallenge()\n(no user or password informed)
server -> tsh: challenge

tsh -> authenticator: wait for touch (1)\n(all authenticators)
tsh -> user: choose authenticator?
user -> authenticator: *taps*
authenticator -> tsh: touched

tsh -> user: enter PIN? (2)
user -> tsh: *enters PIN*

tsh -> authenticator: Credentials(RPID, pin) (3)
authenticator -> tsh: authenticator credentials\n(includes user handle)
tsh -> user: choose credential?
user -> tsh: *chooses*

tsh -> authenticator: Assertion (4)\n(RPID, challenge, credential, pin)
tsh -> user: tap authenticator?
user -> authenticator: *taps*
authenticator -> tsh: signed challenge

tsh -> server: AuthenticateSSH(userHandle, signedChallenge)
server -> tsh: SSH credentials

tsh -> user: ok

@enduml
```

-->

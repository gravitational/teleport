---
authors: Isaiah Becker-Mayer (isaiah@goteleport.com)
---

# RFD 120 - Desktop Access - IronRDP

## IronRDP Overview: RDP client and custom web component (`<iron-remote-gui>`)

The IronRDP project contains an RDP client written in Rust as well as a custom web component (`<iron-remote-gui>`) that uses the WASM-compiled client to enable RDP sessions in browser.

The `<iron-remote-gui>` component provides an API which allows us to listen for a `ready` event and then pass a custom websocket address that the IronRDP client will connect with under the hood.
This means that our webapp will be able to use the `<iron-remote-gui>` component directly; we will simply want to fork IronRDP and add it to our build process, similar to what we're currently doing
with `rdp-rs`.

## Audit Logging, Session Recording, and Playback

In certain ways the transition to IronRDP sends us back the drawing board, because it changes aspects of the core architecture of our desktop access system.

Currently, the system looks like the following:

```
                        Teleport Desktop Protocol                         RDP
               ------------------------------------------        ---------------------
               |                                        |        |                   |
+----------------------+     +------------------+  +------------------+     +------------------+
|                      |     |                  |  |                  |     |                  |
|  (TDP client)        |     |                  |  |    Teleport      |     |                  |
|  User's Web Browser  ------|  Teleport Proxy -----  Windows Desktop ------|  Windows Desktop |
|                      |     |                  |  |     Service      |     |                  |
+----------------------+     +------------------+  +------------------+     +------------------+
```

whereas with IronRDP, we get rid of Teleport Desktop Protocol in favor of using RDP straight through:

```
                                                 RDP
               -----------------------------------------------------------------------
               |                                                                     |
+----------------------+     +------------------+  +------------------+     +------------------+
|                      |     |                  |  |                  |     |                  |
|  (IronRDP)           |     |                  |  |    Teleport      |     |                  |
|  User's Web Browser  ------|  Teleport Proxy -----  Windows Desktop ------|  Windows Desktop |
|                      |     |                  |  |     Service      |     |                  |
+----------------------+     +------------------+  +------------------+     +------------------+
```

Importantly, audit logging and [session recording](0048-desktop-access-session-recording.md) currently take place at the Teleport Windows Desktop Service (WDS) in the form of
[Teleport Desktop Protocol (TDP)](0037-desktop-access-protocol.md) messages. This functionality will still need to take place at the WDS, however with TDP gone most of this system
will need to be rebuilt, as will the playback system.

#### Audit Logging and Session Recording

The precise details of this reworking are beyond the scope of this RFD (probably deserving of their own), but at a high level we will be converting a lot of TDP decoding logic
currently written in Go to become RDP decoding logic written in Rust, which will then make calls to Go functions (over the FFI/CGO interface) to trigger recording and audit events.
Pushing this decode logic into Rust will allow us to make use of IronRDP's existing RDP decoding logic here in the WDS, rather than rewriting a substantial chunk of it in Go.
IronRDP is already written in an asynchronous style, so it's worth considering whether audit logging and session recording logic can be pushed to an out of band asynchronous process
for async recording modes.

#### Playback

Playback will naturally need to be rewritten in order to support playing back RDP rather than TDP messages. This can be done similarly to how we are handling playback now, giving the
frontend playback component an `<iron-remote-gui>` and replaying the session through it (currently we do so with a `<TdpClientCanvas>` component). An important consideration here is
how long we continue supporting TDP session playbacks so that users are able to playback older recordings. One option is to remove our current TDP playback interface and insist users
export TDP sessions to .avi in order to view them. This will significantly decrease the maintenance required for keeping a player for both TDP and RDP in place. The video export system
will also need to be refactored to support RDP exports.

## Smartcard Authentication

We have an implementation of [[MS-RDPESC]](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpesc/0428ca28-b4dc-46a3-97c3-01887fa44a90) and a virtual smartcard, however neither of these
are integrated with [CredSSP/SSPI](https://learn.microsoft.com/en-us/windows/win32/secauthn/credential-security-support-provider). Devolutions also builds and maintains an [sspi-rs](https://github.com/Devolutions/sspi-rs)
project which is used by IronRDP to negotiate credentials before a full RDP session begins, otherwise known as NLA. However currently `sspi-rs` only supports CredSSP with NTLM, whereas smart cards require
CredSSP with Kerberos. We will first need to work with Devolutions to implement CredSSP with Kerberos, and figure out how to hook it in to our existing smart card virtual channel extension and virtual
smartcard emulator.

An additional consideration is necessary due to the fact that the virtual smartcard requires a copy of Teleport's private key to work properly. For obvious security reasons, this isn't something we
want to send to the browser client. Therefore, we will need to use a backend-level RDP decoding system (the same one as mentioned in `Audit Logging, Session Recording, and Playback` above) to identify
smart card messages and re-route them to a CredSSP client _that lives in the WDS_, rather than passing them on to the browser.

A major upside here is that NLA is a [longstanding request](https://github.com/gravitational/teleport/issues/8546) by customers which will improve security of Windows Desktop sessions, and we will have it once this is all completed.

## MFA

Currently per-session MFA is implemented at the TDP level via a special TDP message (see [RFD 0037](0037-desktop-access-protocol.md)). With TDP going away,
this will need to be redesigned. This itself is likely worthy of it's own RFD, which can explore the option of implementing the recently released
[[MS-RDPEWA]: Remote Desktop Protocol: WebAuthn Virtual Channel Protocol](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpewa/68f2df2e-7c40-4a93-9bb0-517e4283a991). If we can implement Teleport MFA over this extension, then we can also get the primary intended use case of the
extension "for free".

## Clipboard Sharing and Directory Sharing

Clipboard sharing is straight-forward at the RDP level. Our current clipboard sharing implementation should be amenable to integration with IronRDP, give or take some type changes. The primary challenge here will be finding the right abstractions for integrating
with the browser's clipboard API when WASM is targeted, which will replace our current approach that integrates with TDP calls. The Devolutions team will likely want this built in such a manner that a native, non-browser API can be hooked in as well.

Implementing directory sharing is a very similar type of project as clipboard sharing, but with added complexity due to the complexity of the directory sharing extension itself.

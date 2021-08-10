---
authors: Alan Parra (alan.parra@goteleport.com)
state: draft
---

# RFD 9999 - WebAuthn Support

## What

Provide WebAuthn support for Teleport, both server- and client-side.

## Why

(Quoting issue [#6478](https://github.com/gravitational/teleport/issues/6478).)

_"FIDO 2 and WebAuthn are intended to replace U2F. Also, WebAuthn allows using
non-USB key backends from browsers (like Touch ID or Windows Hello)._

_WebAuthn is already better supported in browsers compared to U2F. For example,
Safari supports WebAuthn but not U2F. Going forward, WebAuthn will be more
ubiquitous than U2F."_

## Details

### WebAuthn overview

WebAuthn support involves an implementation similar to U2F. There are two
different flows that need be supported: Registration and Authentication.

Registration is the process of registering a new device. A simplified explanation
is: A client, typically a browser, begins registration by asking the server (aka
Relying Party or RP) for a
[CredentialCreation](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialCreation),
containing a challenge for it to sign (among various other options). The client
signs the challenge, typically by calling
[navigator.credentials.create()](https://developer.mozilla.org/en-US/docs/Web/API/CredentialsContainer/create),
and sends a
[CredentialCreationResponse](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialCreationResponse)
to the server with the signed challenge. Assuming all is well this completes the
registration process.

```
                          Registration flow                     
                                                                
     ┌──────┐                       ┌──────┐            ┌──────┐
     │Device│                       │Client│            │Server│
     └──┬───┘                       └──┬───┘            └──┬───┘
        │                              │ BeginRegistration │    
        │                              │ ──────────────────>    
        │                              │                   │    
        │                              │ CredentialCreation│    
        │                              │ <──────────────────    
        │                              │                   │    
        │ navigator.credential.create()│                   │    
        │ <─────────────────────────────                   │    
        │                              │                   │    
        │          Credential          │                   │    
        │ ─────────────────────────────>                   │    
        │                              │                   │    
        │                              │ FinishRegistration│    
        │                              │ ──────────────────>    
        │                              │                   │    
        │                              │  success/failure  │    
        │                              │ <──────────────────    
     ┌──┴───┐                       ┌──┴───┐            ┌──┴───┐
     │Device│                       │Client│            │Server│
     └──────┘                       └──────┘            └──────┘
```

Authentication (also referred simply as Login) follows a similar "challenge ->
sign -> verify" protocol. In simple terms, the client requests a 
[CredentialAssertion](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialAssertion)
from the server, signs it by calling
[navigartor.credentials.get()](https://developer.mozilla.org/en-US/docs/Web/API/CredentialsContainer/get),
and replies with a
[CredentialAssertionResponse](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialAssertionResponse).
Once again, assuming all is well, login is complete.

See https://webauthn.io/ and https://webauthn.guide/ for starter references.

### U2F/WebAuthn compatibility

WebAuthn is backwards compatible with U2F, but it's important to examine what
this sentence actually means.

U2F, as the current register/login protocol implemented by Teleport, is not
forward compatible with WebAuthn - WebAuthn messages differ from its
predecessor, including various options that didn't exist before. Browser support
is implemented via the
[navigator.credentials](https://developer.mozilla.org/en-US/docs/Web/API/CredentialsContainer)
object, which is WebAuthn-exclusive and
[distinct from the U2F APIs in use](https://github.com/google/u2f-ref-code#reference-code-for-u2f-specifications).

WebAuthn is backwards compatible with U2F in the sense that FIDO/CTAP1 devices,
ie U2F-capable devices, are still supported. The main difference in
implementations lies in the definition of the App ID / Relying Party ID (ie,
"https://example.com:3080" vs "example.com"). The "appid" extension is the
solution for this problem and seems to be well-supported for browsers
(__TODO(codingllama):__ Verify for Firefox and Safari).

Unfortunately, the appid extension is not without its issues.
[As the standard itself says in item 2](https://www.w3.org/TR/webauthn/#sctn-appid-extension),
_"When verifying the assertion, expect that the rpIdHash MAY be the hash of the
AppID instead of the RP ID."_ This advice
[wasn't followed by github.com/duo-labs/webauthn](https://github.com/duo-labs/webauthn/blob/9f1b88ef44cc0e4f5ddf511ed12a3aa468f972d7/protocol/assertion.go#L117),
but in the short-term may be circumvented by creating a secondary
[WebAuthn object](https://pkg.go.dev/github.com/duo-labs/webauthn/webauthn#WebAuthn)
with RPID = AppID, used exclusively to validate legacy requests (identified by
the presence of the "incorrect" rpIdHash).

### UX and configuration

WebAuthn support is designed to be as seamless as the U2F experience. Users
enabling MFA for the first time may register their devices via `tsh mfa add` and
use `tsh login` as usual. Users migrating from U2F to WebAuthn retain their
previously-registered devices and keep their workflow unchanged.

Migration from U2F to WebAuthn is achieved simply by changing the second_factor
option from "u2f" to "webauthn". The WebAuthn backend sends challenges for all
previously-registered U2F devices, in addition to any WebAuthn-registered
devices, so migration is a seamless process - no re-registration required.

If the second factor setting is "on", then the backend sends both U2F and
WebAuthn challenges. In this case user-facing interfaces, such as `tsh` and the
Proxy UI, should favor replying to WebAuthn challenges (instead of U2F).

Additions to `teleport.yaml` are as follows:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: webauthn # "on" allows all MFA options
    webauthn:
      relying_party:
        # Display name of the Relying Party, defaults to "Teleport".
        display_name: "Teleport"
        # URL which resolves to an image associated with the Relying Party.
        # It must be an a priori authenticated URL of preferably 128 or less
        # bytes, as authenticators may ignore an icon if its length is greater
        # than 128 bytes.
        # TODO(codingllama): Default to Teleport's icon?
        icon: ""

      # Previously configured U2F app ID, if applicable.
      # Enables U2F-compatibility mode if set (explained below).
      # The U2F app ID is sent as the `appid` WebAuthn extension for
      # previously-registered U2F devices, to be used in place of the Relying
      # Party ID, thus keeping backwards compatibility.
      # Not required for new MFA installations.
      # Defaults to "auth_service.authentication.u2f.app_id", if present.
      u2f_app_id: "https://localhost:3080"
```

The WebAuthn configuration provided allows for some customization, but is
comprised (almost entirely) of optional parameters (u2f_app_id being the
exception, as it needs to be provided in some way).

The general philosophy for configuration is that we provide the minimum set of
configurations necessary and automatically choose the more secure/usable options
ourselves.

The following defaults are assumed, by the system, for the underlying WebAuthn
settings:

```go
	web, _ := webauthn.New(&webauthn.Config{
		// Proxy domain, inferred from the Proxy public_addr.
		RPID: "example.com",
		// Proxy public_addr.
		RPOrigin: "https://example.com:3080",
		// Either "none" or "direct" depending on attestation settings.
		AttestationPreference: "none",
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			// Unset - both "platform" and "cross-platform" allowed.
			// User presence is what we're aiming at, which both provide.
			AuthenticatorAttachment: "",
			// Not aiming at "usernameless"/"first-factor" authentication _yet_.
			RequireResidentKey: false,
			// User presence is what we're aiming at, verification is a bonus.
			UserVerification: "preferred",
		},
		// General timeout for flows, defaults to 60s.
		Timeout: 60000,
	})
```

### Attestation

Attestation support is provided in a similar manner to U2F attestation, by means
of user-provided lists of CA certificates in PEM form.

Both an allow and deny list of attestation CAs may be provided by the user.
Organizations that allow a wide variety of authenticators may use the
easier-to-maintain deny list to prohibit troublesome or unsupported
devices, while organizations that want stronger control over the supported
models may opt for the allow list instead.

If both lists are present, then the attested certificate needs to pass both
tests. This allows for a broad attestation CA to be used while removing support
for specific models or batches.

The attestation preference is set to "direct" if one of the lists is present,
otherwise it is set to "none" (ie, the backend won't require attestation data
unless it intends to make use of it).

Configuration is as follows:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: webauthn # or "on"
    webauthn:
      attestation_allowed_cas:
      - /path/to/attestation/ca.pem
      - |-
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----

      attestation_denied_cas: []
```

Note 1: WebAuthn [attestation statements come in various formats](
https://www.w3.org/TR/webauthn-2/#sctn-defined-attestation-formats), which means
this feature may require eventual work to be kept up-to-date with newer
authenticator offerings.

Note 2: duo-labs/webauthn [doesn't do much for attestation](
https://github.com/duo-labs/webauthn/blob/9f1b88ef44cc0e4f5ddf511ed12a3aa468f972d7/protocol/credential.go#L131).
As for U2F, most of the logic will be written by us. (The sources do provide
an attestation checklist that is a good starting point.)

### Proxy UI + WebAuthn

WebAuthn support is implemented through the
[navigator.credentials](https://developer.mozilla.org/en-US/docs/Web/API/CredentialsContainer)
object.

The login APIs proposed preserve much of the inputs and outputs of
[github.com/duo-labs/webauthn](https://github.com/duo-labs/webauthn), therefore
providing easy integration with the browser's CredentialsContainer.

Registration is gRPC-based and currently only supported via `tsh mfa`, but its
design also preserves much of the standard messages.

### `tsh` + WebAuthn

WebAuthn/FIDO2 support for client-side libraries is currently sparse - the main
Go client-side lib seems to be
[github.com/keys-pub/go-libfido2](https://github.com/keys-pub/go-libfido2),
which pulls a native dependency into Teleport (`libfido2`) and doesn't seem to
support Touch ID (one of the main appeals for the change). As such, the proposal
is a 2-step change in `tsh`:

1. Initially, `tsh` keeps support for only FIDO/CTAP1 (via github.com/flynn/u2f),
   but is changed to use WebAuthn-based endpoints

2. At a later stage (to be designed), support is brought in for FIDO2 / Touch
   ID.

The implications of 1. being that not all WebAuthn configuration features are
likely to be initially supported (in particular, settings related to
`authenticator_selection`).

__TODO(codingllama):__ Do more exploration on client-side libs.

### Server-side changes

#### Login API changes

This is the current flow of the login API:

```
                                                      U2F login                                                  
                                                                                                                 
          ┌─┐                                                                                                    
          ║"│                                                                                                    
          └┬┘                                                                                                    
          ┌┼┐                                                                                                    
           │                ┌───┐                             ┌─────┐                                      ┌────┐
          ┌┴┐               │tsh│                             │proxy│                                      │auth│
      codingllama           └─┬─┘                             └──┬──┘                                      └─┬──┘
           │   $ tsh login    │                                  │                                           │   
           │─────────────────>│                                  │                                           │   
           │                  │                                  │                                           │   
           │                  │  POST /webapi/u2f/signrequest    │                                           │   
           │                  │─────────────────────────────────>│                                           │   
           │                  │                                  │                                           │   
           │                  │                                  │    POST /v2/u2f/users/codingllama/sign    │   
           │                  │                                  │───────────────────────────────────────────>   
           │                  │                                  │                                           │   
           │                  │                                  │     lib.auth.MFAAuthenticateChallenge     │   
           │                  │                                  │<───────────────────────────────────────────   
           │                  │                                  │                                           │   
           │                  │lib.auth.MFAAuthenticateChallenge │                                           │   
           │                  │<─────────────────────────────────│                                           │   
           │                  │                                  │                                           │   
           │tap security key  │                                  │                                           │   
           │<─────────────────│                                  │                                           │   
           │                  │                                  │                                           │   
           ────┐              │                                  │                                           │   
               │ *taps*       │                                  │                                           │   
           <───┘              │                                  │                                           │   
           │                  │                                  │                                           │   
           │                  │     POST /webapi/u2f/certs       │                                           │   
           │                  │─────────────────────────────────>│                                           │   
           │                  │                                  │                                           │   
           │                  │                                  │POST /v2/users/codingllama/ssh/authenticate│   
           │                  │                                  │───────────────────────────────────────────>   
           │                  │                                  │                                           │   
           │                  │                                  │         lib.auth.SSHLoginResponse         │   
           │                  │                                  │<───────────────────────────────────────────   
           │                  │                                  │                                           │   
           │                  │    lib.auth.SSHLoginResponse     │                                           │   
           │                  │<─────────────────────────────────│                                           │   
           │                  │                                  │                                           │   
           │      done        │                                  │                                           │   
           │<─────────────────│                                  │                                           │   
      codingllama           ┌─┴─┐                             ┌──┴──┐                                      ┌─┴──┐
          ┌─┐               │tsh│                             │proxy│                                      │auth│
          ║"│               └───┘                             └─────┘                                      └────┘
          └┬┘                                                                                                    
          ┌┼┐                                                                                                    
           │                                                                                                     
          ┌┴┐                                                                                                    
```

The proposed changes are twofold:

1. Aliasing the current U2F-centric endpoints to more generic MFA-named
   counterparts

2. Modifying existing messages to support WebAuthn as a login method

Endpoint changes are:

* `/webapi/u2f/signrequest` is aliased to `/webapi/mfa/login/begin`
* `/v2/u2f/users/*/sign` is aliased to `/v2/mfa/users/*/login/begin`
* `/webapi/u2f/certs` is aliased to `/webapi/mfa/login/finish`
* `/v2/users/*/ssh/authenticate` remains as the final step (unchanged)

Existing messages are modified to contain the necessary WebAuthn data, as
described below:

__lib.auth.MFAAuthenticateChallenge__, the reply from `*/login/begin` endpoints,
is modified to contain a
[CredentialAssertion](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialAssertion).

A challenge is generated for each registered U2F and WebAuthn device. For U2F
devices, the system takes care to 1. inform the u2f_app_id using the `appid`
extension, 2. use the U2F key handle as the credential ID and 3. correctly
encode the device public key using the CBOR2 format (see
[github.com/fxamacker/cbor/v2](https://github.com/fxamacker/cbor/v2)).

__lib.client.CreateSSHCertWithMFAReq__ and __lib.auth.AuthenticateSSHRequest__,
the inputs for `/v2/mfa/users/*/login/begin` and `/v2/users/*/ssh/authenticate`,
are modified to contain a
[CredentialAssertionResponse](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialAssertionResponse).

__lib.auth.SSHLoginResponse__ needs no changes.

#### Registration API changes

Registration is performed by the
[AddMFADevice](https://github.com/gravitational/teleport/blob/e842fbc762e246d7c9973571b97e6d542f020f0d/api/client/proto/authservice.proto#L1118)
streaming RPC. Unlike the REST-based login flow, no interface changes are
suggested, only changes to existing messages to accommodate WebAuthn challenges
are required.

__MFAAuthenticateChallenge__ and __MFAAuthenticateResponse__ must change to
accomodate, respectivelly, a CredentialAssertion and
CredentialAssertionResponse (modeled as protos).

__MFARegisterChallenge__ and __MFARegisterResponse__, similarly to their
authenticate counterparts, must change to accomodate a
[CredentialCreation](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialCreation)
and a
[CredentialCreationResponse](https://pkg.go.dev/github.com/duo-labs/webauthn/protocol#CredentialCreationResponse)
(modeled as protos).

All relevant AuthService RPCs (or the methods they rely on) are to be modified
to support WebAuthn (AddMFADevice, DeleteMFADevice, GenerateUserSingleUseCerts).

#### Backwards compatibility

At the API level (Proxy API, Auth API), backwards compatibility is achieved by
keeping the old endpoints in place. Additions to existing messages are unknown
by legacy code, thus ignored by older Proxy and `tsh` versions.

The server version returned by `/webapi/ping` is used to determine if the Proxy
supports the new /mfa/ endpoints (with the assumption, by the Proxy, that the
Auth server also supports the new endpoints). This allows newer `tsh` versions
to safely talk to older Proxies (at least for a few releases).

gRPC backwards compatibility is not a concern.

#### WebAuthn user handle

WebAuthn requires the server to return a user handle (aka user ID) along with
both registration and login challenges. The user handle is used by the
authenticator as a way to scope the credential (along with the RPID).
Servers should rely on the user handle to identify the user as well (instead of
displayName or other information).

It is recommended for the user handle to be [a random array of 64 bytes](
https://www.w3.org/TR/webauthn-2/#sctn-user-handle-privacy), a recommendation we
shall follow.

User handles are assigned to users in the following situations:

* During new user creation
* During login, before challenges are generated, if the user lacks a handle
* During registration, before challenges are generated, if the user lacks a
  handle

User handles are stored within LocalAuthSecrets (messages below):

```proto
message LocalAuthSecrets {
  // ...
  WebAuthnSettings WebauthnSettings = 6 [
    (gogoproto.jsontag) = "webauthn_settings,omitempty"
  ];
}

message WebAuthnSettings {
  // User ID is the random WebAuthn user handle generated for the user.
  // See https://www.w3.org/TR/webauthn-2/#sctn-user-handle-privacy.
  bytes user_id = 1 [(gogoproto.jsontag) = "user_id,omitempty"];
}
```

#### WebAuthn challenge storage

Device storage uses the existing \*MFADevice\* methods in
[Identity](https://github.com/gravitational/teleport/blob/185e5fda35f3b8fb6debd46acd847cb8250b8f86/lib/services/identity.go#L124),
no changes required - the system already supports multiple
[device types](https://github.com/gravitational/teleport/blob/185e5fda35f3b8fb6debd46acd847cb8250b8f86/api/types/types.proto#L1599).
WebAuthn simply adds a WebAuthnDevice to MFADevice (taking care to notice, in
documentation, the differences in public key encoding between U2F and WebAuthn.)

WebAuthn requires the persistence of SessionData between challenge attempts
(for both login and registration). The following additions are proposed:

```go
type Identity interface {
	// UpsertWebAuthnSessionData upserts WebAuthn SessionData for the purposes
	// of verifying a later authentication challenge.
	// Upserted session data expires according to backend settings.
	UpsertWebAuthnSessionData(user, sessionID string, sd *SessionData) error

	// GetWebAuthnSessionData retrieves a previously-stored session data by ID,
	// if it exists and has not expired.
	GetWebAuthnSessionData(user, sessionID string) (*SessionData, error)

	// DeleteMFAAuthenticateChallenge deletes session data by ID,
	// if it exists and has not expired.
	DeleteWebAuthnSessionData(user, sessionID string) error
}
```

The key space is `web/users/{user_id}/webauthnsessiondata/{session_id}`.
Implementation may choose to use hard-coded session IDs for login and
registration, limiting the possibility of concurrent attempts, or pick distinct
IDs, as appropriate.

Note that registration, as presently implemented, doesn't require persistent
storage for challenges, as they may be kept in memory for the duration of the
RPC stream.


<!-- Plant UML diagrams -->
<!--

```plantuml
@startuml 9999-webauthn-support-registrationFlow

title "Registration flow"

participant Device
participant Client
participant Server

Client -> Server: BeginRegistration
Server -> Client: CredentialCreation
Client -> Device: navigator.credential.create()
Device -> Client: Credential
Client -> Server: FinishRegistration
Server -> Client: success/failure

@enduml
```

```plantuml
@startuml 9999-webauthn-support-u2fLogin

title U2F login

actor codingllama
participant tsh
participant proxy
participant auth

' BeginLogin flow
codingllama -> tsh: $ tsh login
tsh -> proxy: POST /webapi/u2f/signrequest
' input: lib.client.MFAChallengeRequest
proxy -> auth: POST /v2/u2f/users/codingllama/sign
' input: lib.auth.signInReq
auth -> proxy: lib.auth.MFAAuthenticateChallenge
proxy -> tsh: lib.auth.MFAAuthenticateChallenge
tsh -> codingllama: tap security key
codingllama -> codingllama: *taps*

' FinishLogin flow
tsh -> proxy: POST /webapi/u2f/certs
' input: lib.client.CreateSSHCertWithMFAReq
proxy -> auth: POST /v2/users/codingllama/ssh/authenticate
' input: lib.auth.AuthenticateSSHRequest
auth -> proxy: lib.auth.SSHLoginResponse
proxy -> tsh: lib.auth.SSHLoginResponse
tsh -> codingllama: done

@enduml
```

```plantuml
@startuml 9999-webauthn-support-webauthnLogin

title WebAuthn login

actor codingllama
participant tsh
participant proxy
participant auth

' BeginLogin flow
codingllama -> tsh: $ tsh login
tsh -[#green]> proxy: POST /webapi/mfa/login/begin
note left: Same handler as /webapi/u2f/signrequest
' input: lib.client.MFAChallengeRequest
proxy -[#green]> auth: POST /v2/mfa/users/codingllama/login/begin
note left: Same handler as /v2/u2f/users/*/sign
' input: lib.auth.signInReq
auth -[#blue]> proxy: lib.auth.MFAAuthenticateChallenge
proxy -[#blue]> tsh: lib.auth.MFAAuthenticateChallenge
tsh -> codingllama: tap security key
codingllama -> codingllama: *taps*

' FinishLogin flow
tsh -[#green]> proxy: POST /webapi/mfa/login/finish
note left: Same handler as /webapi/u2f/certs
' input: lib.client.CreateSSHCertWithMFAReq
proxy -[#blue]> auth: POST /v2/users/codingllama/ssh/authenticate
' input: lib.auth.AuthenticateSSHRequest
auth -[#blue]> proxy: lib.auth.SSHLoginResponse
proxy -[#blue]> tsh: lib.auth.SSHLoginResponse
tsh -> codingllama: done

@enduml
```

-->

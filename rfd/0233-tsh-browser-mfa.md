---
authors: Dan Share (daniel.share@goteleport.com)
state: draft
---

# RFD 233 - `tsh` Browser MFA

## Required Approvers

* Engineering: @zmb3 && (@codingllama || @Joerger)
* Security: @rob-picard-teleport

## What

This RFD proposes a new method for users of `tsh` to be able to solve MFA
challenges via the browser.

## Why

We encourage our users to use the strongest methods of MFA when signing up for
an account through the web UI, such as passkeys and hardware keys. However,
some types of passkeys (namely Apple TouchID) don't transfer from the browser
to `tsh`. As a result users who set up Touch ID are unable to authenticate
with `tsh` unless they first add another MFA method (like TOTP).

This RFD aims to describe how we can allow `tsh` to delegate its MFA checks to
the web UI to enable easier access to biometrics and passkeys from both browsers
and password managers. We will also be one step closer to the ultimate goal of
removing support for TOTP in Teleport.

## Details

### UX

#### User stories

**Alice logs in to their cluster using `tsh`**

Alice is a new user who has created her account with a passkey as her second
factor. She would like to log in to her cluster using `tsh`. She runs the
following command:

```
tsh login --proxy teleport.example.com --user alice
```

She is asked for her password, which is then sent to Teleport. Teleport verifies
her username and password, and checks for valid methods of second factor
authentication. Available methods of MFA are returned and `tsh` determines that
no security keys are present, so `tsh` catches the error and switches to browser
MFA, which prints a URL and attempts to open it in the default browser for her
to complete the challenge.

The browser will open to a page that contains a modal prompting her to verify it
is her by completing the MFA check. Once this is completed, `tsh` will receive
its certificates from the proxy.

Alice is now authenticated and able to interact with resources on that cluster.

**Alice explicitly uses browser authentication**

Alice has multiple MFA methods configured (TOTP and a passkey), but prefers
to use their passkey through the browser. They run the following command:

```
tsh login --proxy teleport.example.com --user alice --auth=browser
```

By explicitly specifying `--auth=browser`, Alice tells `tsh` to use browser-based
authentication from the start, skipping any checks for local hardware keys or
TOTP. `tsh` prints the MFA URL and opens Alice's default browser. If Alice isn't
already authenticated, she is prompted to login, she is then prompted to
complete the MFA check with her passkey. After successful authentication, `tsh`
will receive its certificates through a callback from the proxy.

**Alice connects to a resource that requires per-session MFA**

Alice is already authenticated with her cluster, but wants to access a resource
that requires per-session MFA. She runs the following command:

```
tsh ssh alice@node
```

`tsh` queries the cluster for available methods of MFA and checks for local
hardware keys and SSO config that can be used for MFA. If none are found, `tsh`
falls back to browser-based MFA. The MFA URL is printed and `tsh` attempts to
open her browser, to authenticate with her MFA. Upon successful MFA, `tsh`
receives short-lived, MFA-verified certificates through a callback to connect to
the resource.

### Design

#### Login MFA process

sequenceDiagram
    participant tsh
    participant browser as Browser
    participant proxy as Proxy
    participant auth as Auth

    Note over tsh: tsh login --proxy teleport.example.com<br/>--user alice --mfa-mode=browser
    Note over tsh: prompt for password

    tsh->>tsh: Start local HTTP server<br/>(127.0.0.1:random_port)<br/>Generate secret key

    tsh->>proxy: POST /webapi/mfa/login/begin<br/>w/ redirect /callback?secret_key=xxx
    proxy->>auth: Forward login request
    auth->>auth: Generate request_id<br/>Upsert SSOMFASession
    auth-->>proxy: MFA Challenge
    proxy-->>tsh: Return MFA Challenge

    tsh->>browser: Open browser to:<br/>https://teleport.example.com/web/mfa/:request_id
    activate browser

    browser->>proxy: GET /web/mfa/:request_id
    proxy-->>browser: Display WebAuthn prompt

    browser->>browser: User taps TouchID /<br/>Uses password manager passkey
    browser->>proxy: PUT /webapi/mfa/:request_id

    proxy->>auth: rpc ValidateBrowserMFAChallengeResponse
    auth->>auth: ValidateMFAAuthResponse()
    auth->>auth: Generate MFA token<br/>UpsertSSOMFASessionWithToken()
    auth->>auth: Encrypt token with secret_key
    auth-->>proxy: ValidateBrowserMFAChallengeResponse
    proxy-->>browser: HTTP 200 with redirect URL

    browser->>tsh: Redirect to callback URL<br/>http://127.0.0.1:port/callback?response={encrypted_token}
    deactivate browser
    tsh->>tsh: Decrypt response with secret_key<br/>Extract MFA token
    tsh-->>browser: Display success page

    tsh->>proxy: POST /webapi/mfa/login/finish
    proxy->>auth: AuthenticateSSHUser
    auth->>auth: VerifySSOMFASession()
    auth->>auth: Generate SSH certificates
    auth-->>proxy: SSH Login Response
    proxy-->>tsh: Return certificates
    
    tsh->>tsh: Save credentials to keyring
    Note over tsh: Login successful

##### Login MFA Flow

The flow can be broken down in to three sections:

##### `tsh` initiating a login flow

When the user performs a `tsh login` and enters their password, it will check
for either an explicit `--mfa-mode=browser` flag or it will error if there are
no other MFA methods available. The error informs the user of other mfa methods
they can try.

Upon choosing browser MFA, `tsh` will send a request to
`POST /webapi/mfa/login/begin` with a redirect URL that contains a secret key.
This request is forwarded to the auth server where `BeginSSOMFAChallenge` will
genereate a request ID and a `SSOMFASession` object that is stored on the
backend. An MFA challenge is returned back to `tsh` which contains a URL to
`/web/mfa/:request_id`.

##### The user verifying their MFA through the browser

When `tsh` receives the MFA challenge from the auth server, it will open the
user's default browser to the MFA URL that was returned.

Once in the browser, their login session will be used to connect to the auth
server. If the user is not already logged in, they will be prompted to do so.

When authenticated, the user will be prompted to verify their MFA. Once they've
done so, a request to `/webapi/mfa/:request_id` will take the challenge response
and verify it through `rpc ValidateBrowserMFAChallengeResponse`. If the response
is valid, the auth server will generate an MFA token and upsert it in to the
backend `SSOMFASession` resource, encrypt it, and append it to the callback URL
to be returned back to the browser.

##### `tsh` receiving certificates

The browser receives the redirect URL with encrypted secret and redirects to it.
`tsh`'s callback server receives the request and extracts the encrypted
response. It decrypts the MFA token with the secret key and calls
`POST /webapi/mfa/login/finish` with the MFA token. The proxy calls
`AuthenticateSSHUser` to exchange the MFA token for certificates, which `tsh`
then saves to disk.

#### Per-session MFA

For per-session MFA, the MFA verification flow is the same (verify through
browser and receive result to callback), but the initialization and certificate
retrieval is different:

1. Instead of initiating the flow by calling `POST /webapi/mfa/login/begin`,
   `tsh` will call `rpc CreateAuthenticateChallenge` with `SCOPE_USER_SESSION`.
1. Once the MFA token is receive through the callback server,
   `rpc GenerateUserCerts` is called to exchange the token for certificates,
   instead of `POST /webapi/mfa/login/finish`.

#### In-band MFA

For resources and clusters that support it, in-band per-session MFA will be
used. As of the time of writing, this is only `ssh` resources. As above, this
follows a similar flow to the login process, but with the following changes:

1. The trigger to get a MFA Challenge from the server is started by dialing an
   ssh target. If `tsh` gets an "MFA required" message, it will call
   `rpc CreateSessionChallenge` which will return an MFA Challenge.
1. Once the challenge is solved and the MFA token is sent back to `tsh`, instead
   of verifying the SSO MFA session to get certificates, the MFA token is sent
   to the MFA service using `rpc ValidateSessionChallenge`. After which, the ssh
   session is allowed to continue its connection

More context on the in-band flow can be found in
[RFD 234](0234-in-band-mfa-ssh-sessions.md#local-cluster-flow).

### Security

### Scale

### Backward Compatibility

#### Newer `tsh` client, older server

If a newer client sends a request to initiate an MFA challenge to an older
server, it won't return a `BrowserChallenge` field. If we take the approach of
enabling Browser MFA for every cluster, and the user has specifically requested
`--mfa-mode=browser`, we can show an error saying the server version is too old.

#### Older `tsh` client, newer server

If an older `tsh` client sends a request to initiate an MFA challenge, the newer
server will respond with a `BrowserChallenge` as an option for the user to MFA.
The older client won't have knowledge of this field and won't consider it as an
option for the user to MFA.

### Test Plan

Add steps to test that browser MFA works for logging in and for
per-session MFA.

### Audit Events

Audit events do not need to be modified. It will be shown that a user used
browser authentication in the `MFA Authentication Success` audit event:

```json
{
    ...

    "mfa_device": {
        "mfa_device_name": "browser",
        "mfa_device_type": "browser",
        "mfa_device_uuid": "browser"
    },
}
```

### Protobuf Definitions

```proto
package proto;

// MFAAuthenticateChallenge is a challenge for all MFA devices registered for a
// user.
message MFAAuthenticateChallenge {
  ...

  // Browser Challenge is MFA challenge that the user solves in the browser. On
  // the backend this uses the SSO flow, which is why it shares the same type.
  SSOChallenge BrowserChallenge = 6;
}

// ValidateBrowserMFAChallengeResponseRequest is used to validate an MFA response
// during a browser-based MFA authentication flow.
message ValidateBrowserMFAChallengeResponseRequest {
  string RequestID = 1 [(gogoproto.jsontag) = "request_id,omitempty"];
  MFAAuthenticateResponse MFAAuthenticateResponse = 2 [(gogoproto.jsontag) = "mfa_authenticate_response,omitempty"];
}

// ValidateBrowserMFAChallengeResponseResponse contains the redirect URL to send
// the user back to after successfully completing browser-based MFA authentication.
message ValidateBrowserMFAChallengeResponseResponse {
  string ClientRedirectURL = 1 [(gogoproto.jsontag) = "client_redirect_url,omitempty"];
}

// AuthService is authentication/authorization service implementation
service AuthService {
  ...

  // ValidateBrowserMFAChallengeResponse validates browser MFA challenge responses
  rpc ValidateBrowserMFAChallengeResponse(ValidateBrowserMFAChallengeResponseRequest) returns (ValidateBrowserMFAChallengeResponseResponse);
}
```

```proto
package teleport.mfa.v1;

// AuthenticateChallenge is a challenge for all MFA devices registered for a user.
message AuthenticateChallenge {
  ...

  // Browser challenge is a SSO challenge under the hood that allows a user to MFA in the browser,
  // to get an MFA token that is returned to the client to be used for verification.
  SSOChallenge browser_challenge = 4;
}

// AuthenticateResponse is a response to AuthenticateChallenge using one of the MFA devices registered for a user.
message AuthenticateResponse {
  ...
  oneof response {
    ...

    // Response to a browser challenge.
    SSOChallengeResponse browser = 4;
  }
}
```

### Proxy changes

#### `PUT /webapi/mfa/:request_id`

This endpoint will be called by the browser to verify the user's MFA challenge
response and, if successful, generates a callback URL with an encrypted response
containing the MFA token.

**Request Payload:**

```json
{
  "SSO": {
    "request_id": "abc123-def456-ghi789",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**Response:**

- `200 OK`: Returns redirect URL with encrypted MFA token
- `400 Bad Request`: Invalid body | Empty MFA
- `403 Forbidden`: Invalid/expired token

## Future Work

### `tsh` passwordless login

The UX of a user logging in to their cluster via `tsh` could be simplified
further by allowing them to use their browser-based passkey. The proposed flow
is described in the following user story:

**Alice uses a passwordless login**

Alice wants to login to her cluster using the passwordless flow, but she doesn't
have an MFA device registered that has passwordless capabilities (e.g. can't
verify the user using biometrics or PIN). She runs the following command to
login.

```
tsh login --proxy teleport.example.com --user alice --auth=browser
```

`tsh` detects that there are no TouchID keys, it then fallsback to FIDO2 keys
and finds none are present. These errors are caught and browser authentication
is attempted. A URL is printed and her browser opens to Teleport's login page
(if she isn't already logged in), she authenticates and is asked if she wants to
approve this `tsh` login attempt. She approves, verifies using her MFA, and her
`tsh` session is authenticated.

### Browser MFA without Re-authentication

When a user is already authenticated via `tsh` but needs to complete an MFA
challenge (such as for per-session MFA), requiring them to also be logged in
to the browser creates friction. This is especially problematic when
the user may not have an active browser session or uses different browser
profiles for different accounts.

**Alice completes per-session MFA without browser login**

Alice is already authenticated to her cluster via `tsh` and wants to access a
resource that requires per-session MFA. She runs:

```
tsh ssh alice@node
```

`tsh` determines that browser MFA is needed and makes a request to generates a
temporary, single-use MFA challenge URL. When Alice opens this URL in her
browser, instead of being redirected to the login page, she is immediately
presented with the MFA prompt. She completes the MFA challenge with her passkey,
and `tsh` receives the MFA token to continue the connection.

---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD TBD - Headless Authentication

## Required Approvers

* Engineering: @r0mant && @jakule
* Security: @reed || @jentfoo
* Product: @xinding33 || @klizhentas

## What

Support headless authentication for Teleport clients.

Headless authentication is a non-interactive form of authentication that relies on a secondary party to authenticate a user. In our case, headless authentication will be used to authenticate a user on a remote machine using their credentials and MFA device on their local machine.

## Why

Some users have valid use cases for making `tsh` requests on a remote, sometimes shared, machine.

For example:

 1. Connecting to Teleport Nodes.
 1. Performing `tsh scp` from one Teleport Node to another Teleport Node.

However, `tsh` does not currently work well on remote machines for two reasons:

 1. Client credentials are usually saved to disk, where they may be overwritten, used, or stolen by anyone with access to the remote machine.
 1. Teleport widely uses MFA to authenticate client logins and actions, which may require direct access to a WebAuthn device. For example, with Per-session MFA, each SSH connection to a Teleport Node may require MFA verification.

Headless authentication will provide a secure way to support these types of remote use cases.

## Details

### Security

Headless authentication should fulfill the following design principles to ensure that the system can not be easily exploited by attackers, including attackers with root access to the remote machine.

1. **The Client does not write any keys, certificates, or session data to disk on the remote machine**

When using headless authentication, Teleport clients will generate a new private key in memory for the lifetime of the client. This private key will not be exported anywhere. Only its public key counterpart will be shared externally to get user certificates. Likewise, the user certificates will be held in memory for the lifetime of the client.

This solves two problems: 1) multiple users with on the same remote machine will not have overlapping credentials on disk in `~/.tsh` and 2) attackers will not be able to steal a user's credentials from disk.

2. **Issued certificates through headless authentication are short lived (1 minute TTL)**

Since the user certificates are only meant to last for a single client lifetime, the Auth server should issue the certificates with a 1 minute TTL. This limits the damage potential if an attacker manages to compromise the user's certificate, whether through a faulty client or a clever attack.

3. **User must complete WebAuthn authentication for each individual headless `tsh` request (`tsh ls`, `tsh ssh`, etc.)**

Headless authentication, like any login mechanism, can be started by any unauthenticated user. To prevent phishing attacks, we must prompt the user to acknowledge and approve each headless authentication request with WebAuthn, since WebAuthn provides strong protection against phishing attacks. Legacy OTP MFA methods will not be supported.

4. **The Server must verify that the client requesting certificates is the only client that can use the certificates**

Like other login procedures, this be accomplished with a PKCE-like flow, where the client provides their public key in the login request, and the server issues certificates for the client's public key. If the client does not have access to the corresponding client private key, they cannot use the certificates.

Note: Ideally, we would ensure that the only client that can *receive* certificates is the requesting client, but ensuring the PKCE flow above is enough to defend against attacks since the certificates are useless without the client's private key. In the current design this is accomplished by encapsulating the headless login flow between the remote machine and Teleport into a single endpoint - `/webapi/login/headless`. Additionally, the certificates returned will be encrypted by the client's public key so it can only be read with the client's private key.

5. **Limit the scope of headless certificates**

When a user requests headless login certificates, we know exactly what command they are trying to complete. This means that we can scope their certificate to provide the most limited possible privileges necessary to complete said action. This would help to reduce the blast radius if an attacker manages to steal the user's headless certificates.

However, unlike Per-session MFA, which scopes certificates to a specific node connection, we will need to scope the certificates to the entire API flow for a `tsh` command. For example, `tsh ssh` needs to:

* Connect to Proxy
* List nodes
* View cluster details (Auth preference, etc.)
* Connect to Node

This means that for any given `tsh` command, the Teleport Auth server will provide different privileges to the certificates. Since each possible command will need custom logic to determine the lowest privileges necessary, we will only support a select few commands necessary for basic `tsh` workflows:

* `tsh ls`
* `tsh ssh`
* `tsh scp`

The awarded certificates should not provide access to roles usually awarded to the user, as these roles may provide more privileges than required for the requested command. However, some role provided fields should be included, such as `role.options.forward_agent` and `role.logins`. Additionally, the user should be notified of what command was requested and what permissions will be awarded to the headless session once approved.

Example:

```
Incoming headless request for command "tsh ssh server01", to connect to node <server01_uuid>. Upon approval, the request will be awarded the following permissions:

allow:
  options:
    max_session_ttl: 8h0m0s
    forward_agent: true
    port_forwarding: true
    require_session_mfa: true
  logins: ["ubuntu-user"]
  resource_ids: ["<server01_uuid>"]
  rules:
    resources: ["nodes"]
    verbs: ["read", "list"]
```

**Important note**: The server cannot guarantee that the client will actually use the resulting certificates for the `tsh` command advertised by the client. This means that a modified client could claim to need certificates for `tsh ls`, but then actually use the certificates for `tsh ssh`. In order to prevent potential permission workarounds, we will need to carefully consider how each supported command's awarded certificates may interact with other Teleport clients. For this reason, we may add more commands as they are requested, but each command may have significant security implications to consider. Each PR adding one or more new commands should include a security description and get a security review from the Teleport security team. This includes the commands listed above, as the most limited necessary permissions necessary to perform each command is not yet clear.

#### Conclusion

With the design principles above, the only possible attack would be for the attacker to issue the headless request themselves and trick the user into verifying the request with MFA, with a direct phishing attack. This phishing attack should also be mitigated by notifying the user of the command requested and the permissions that will be awarded upon approval.

### Headless authentication overview

The headless authentication flow is shown below:

```mermaid
sequenceDiagram
    participant Local Machine
    participant Remote Machine
    participant Teleport Proxy
    participant Teleport Auth
    
    Note over Remote Machine: tsh --headless ssh user@node01
    Note over Remote Machine: generate UUID for headless login<br/>request and print command / URL

    par
        Remote Machine->>Teleport Proxy: initiate headless login<br/>(POST /webapi/login/headless)
        Teleport Proxy->>Teleport Auth: save request
        Note over Teleport Auth: Request: {id, user, publicKey, state=pending}
        Teleport Proxy->>+Teleport Auth: wait for request state change
    and
        Remote Machine-->>Local Machine: user copies command / URL<br/>to local terminal / browser
        Note over Local Machine: tsh headless approve <request_id>
        opt user is not already logged in locally
            Local Machine->>Teleport Proxy: user logs in normally e.g. password+MFA
            Teleport Proxy->>Local Machine: 
        end
        Local Machine->>Teleport Proxy: get headless login request info<br/>(rpc GetHeadlessLoginRequest)
        Teleport Proxy->>Local Machine: 
        Note over Local Machine: share request details with user and<br/>prompt for confirmation (y/N)
        Local Machine->>Teleport Proxy: initiate headless request approval<br/>(rpc UpdateHeadlessLoginRequestState)
        Teleport Proxy->>Local Machine: MFA challenge
        Note over Local Machine: user taps YubiKey
        Local Machine->>Teleport Proxy: signed MFA challenge response
        Teleport Proxy->>Teleport Auth: update request state
    end

    Teleport Auth->>-Teleport Proxy: unblock on request state change
    Teleport Proxy->>Teleport Auth: Request signed user certificates
    Teleport Auth->>Teleport Proxy: Issue signed user certificates
    Teleport Proxy->>Remote Machine: user certificates<br/>(MFA-verified, 1 minute TTL)
    Note over Remote Machine: Connect to user@node01
```

This flow can be broken down into three parts: headless login initiation, local authentication, and certificate retrieval.

#### Headless login initiation

First, the client initiates headless login, providing a request UUID, the command being requested (e.g. `tsh ssh user@host`), and normal login parameters including the client public key.

```go
type HeadlessLoginRequest struct {
    SSHLogin
    // User is a teleport username
    User string `json:"user"`
    // RequestID is a uuid for the request
    RequestID string
    // Command is the client command being requested
    Command string
}

// SSHLogin contains common SSH login parameters.
type SSHLogin struct {
    // ProxyAddr is the target proxy address
    ProxyAddr string
    // PubKey is SSH public key to sign
    PubKey []byte
    ...
}
```

These request details are saved on the Auth server in a `HeadlessLoginRequest` object, with the backend prefix `/headless_login_requests/`. They will be saved with a 1 minute TTL, by which point the user should have completed the authentication flow. The request will begin in the pending state, to be later approved/denied by the user locally.

```go
type HeadlessLoginRequest struct {
    ID string
    // User is a teleport username
    User string
    // PubKey is SSH public key to sign
    PubKey []byte
    // Command is the client command being requested
    Command string
    // State is the request state (pending, approved, or denied)
    State string
}
```

#### Local authentication

In parallel to making the headless login request above, the client will generate a command and URL to initiate local authentication to approve the request: `tsh headless approve <request_id>` and `https://proxy.example.com/headless/<request_id>/approve`. The command and URL are shared with the user so they can complete local authentication for the request in a local terminal or web browser.

When the user runs the command or opens the URL locally, their local login session will be used to connect to the Teleport Auth server. If the user is not yet connected, they will be prompted to login with MFA as usual.

Once connected, the user can view the request details and either approve or deny the request:

* If the user approves the request, they will need to pass an MFA challenge to update the request to the approved state.
* If the user denies the request, the request will be updated to the denied state.

#### Certificate retrieval

After the headless login is initiated, the request will wait until local authentication is complete. This will be handled by the Teleport Proxy by using a resource watcher to wait until the `HeadlessLoginRequest` object is updated to the approved or denied state.

If the headless login request is approved, the Teleport Proxy will request MFA-verified, Single-use (1 minute TTL) user certificates from the Auth server for the initial login request details (`HeadlessLoginRequest`). The Auth server will verify the details saved in the `HeadlessLoginRequest` object before signing the requested certificates.

The resulting user certificates will then be sent to the client, and the client will complete the `tsh` request initially requested, e.g. `tsh ssh user@node01`.

### Audit log

The following actions will be tracked with audit events:

* User initiates headless login
* User approves/denies headless login request

### Server changes

Headless authentication has a unique API flow compared to other login methods.

#### `POST /webapi/login/headless`

This endpoint is used to initiate headless login. Like other login endpoints, this endpoint is not authenticated and can be called by anyone with access to the Teleport Proxy address.

```go
type HeadlessLoginRequest struct {
    SSHLogin
    // User is a teleport username
    User string `json:"user"`
    // RequestID is a uuid for the request
    RequestID string
    // Command is the client command being requested
    Command string
}

// SSHLogin contains common SSH login parameters.
type SSHLogin struct {
    // ProxyAddr is the target proxy address
    ProxyAddr string
    // PubKey is SSH public key to sign
    PubKey []byte
    ...
}

// SSHLoginResponse is a user login response
type SSHLoginResponse struct {
    // Username contains the username for the login certificates
    Username string `json:"username"`
    // Cert is a PEM encoded SSH certificate signed by SSH certificate authority
    Cert []byte `json:"cert"`
    // TLSCertPEM is a PEM encoded TLS certificate signed by TLS certificate authority
    TLSCert []byte `json:"tls_cert"`
    // HostSigners is a list of signing host public keys trusted by proxy
    HostSigners []TrustedCerts `json:"host_signers"`
}
```

#### `rpc GetHeadlessLoginRequest`

This endpoint is used by Teleport clients to retrieve headless login request details before prompting the user for approval/denial.

The endpoint is only authorized for the user who requested Headless authentication (and for server roles).

```proto
service AuthService {
  rpc GetHeadlessLoginRequest(GetHeadlessLoginRequestRequest) returns (HeadlessLoginRequest);
}

message GetHeadlessLoginRequestRequest {
  // RequestID is the headless login request uuid
  string RequestID = 1;
}

message HeadlessLoginRequest {
  // ID is the headless login request uuid
  string ID = 1;
  // User is a teleport user name
  string User = 2;
  // PubKey is SSH public key to sign
  bytes PubKey = 3;
  // Command is a `tsh` command in plain text
  string Command = 4;
  // State is the headless login request state
  State State = 5;
}

// State is a headless login request state
enum State {
  PENDING = 0;
  DENIED = 1;
  APPOVED = 2;
}
```

#### `rpc UpdateHeadlessLoginRequestState`

This endpoint is used by Teleport clients to update headless login request state to approved or denied. If the client requests to approve, the client will receive an MFA challenge and will need to reply with a valid MFA challenge response. To support this multi-step request, the rpc provides an rpc stream to both sides of the connection.

The endpoint is only authorized for the user who requested Headless authentication (and for server roles).

```proto
service AuthService {
  rpc UpdateHeadlessLoginRequestState(stream UpdateHeadlessLoginRequestStateRequest) returns (stream UpdateHeadlessLoginRequestStateResponse);
}

message UpdateHeadlessLoginRequestStateRequest {
  oneof Request {
    // Init is the initial request
    UpdateHeadlessLoginRequestStateRequestInit Init = 1;
    // MFAResponse is the client's signed MFA challenge response
    MFAAuthenticateResponse MFAResponse = 2;
  }
}

// UpdateHeadlessLoginRequestStateRequestInit is a request to update a
// headless login request's state (approve/deny). Approving requests
// requires the MFA challenge/response flow.
message UpdateHeadlessLoginRequestStateRequestInit {
  // RequestID is the headless login request ID
  string RequestID = 1;
  // NewState is the state that the request will be updated to
  State NewState = 2;
}

// State is a headless login request state
enum State {
  PENDING = 0;
  DENIED = 1;
  APPOVED = 2;
}

// MFAAuthenticateResponse is a response to MFAAuthenticateChallenge using one
// of the MFA devices registered for a user.
message MFAAuthenticateResponse {
  oneof Response {
    // Removed: U2FResponse U2F = 1;
    TOTPResponse TOTP = 2;
    webauthn.CredentialAssertionResponse Webauthn = 3;
  }
}


// UpdateHeadlessLoginRequestStateResponse is a response to update a headless
// login request's state. Denial requests should return an empty response, since
// MFA challenge/response flow is not needed.
message UpdateHeadlessLoginRequestStateResponse {
  // MFAChallenge is an MFA challenge for a user's registered MFA devices
  MFAAuthenticateChallenge MFAChallenge = 1;
}

// MFAAuthenticateChallenge is a challenge for all MFA devices registered for a user.
message MFAAuthenticateChallenge {
  reserved 1; // repeated U2FChallenge U2F
  // TOTP is a challenge for all TOTP devices registered for a user. When
  // this field is set, any TOTP device a user has registered can be used to
  // respond.
  TOTPChallenge TOTP = 2;
  // WebauthnChallenge contains a Webauthn credential assertion used for
  // login/authentication ceremonies.
  // Credential assertions hold, among other information, a list of allowed
  // credentials for the ceremony (one for each U2F or Webauthn device
  // registered by the user).
  webauthn.CredentialAssertion WebauthnChallenge = 3;
}
```

### UX

#### `tsh --headless`

We will add a new `--headless` flag to `tsh` which can be used to authenticate for a single `tsh` request. When this flag is provided, `tsh` will prompt the user to complete headless authentication on their local machine with the command `tsh headless approve <request_id>` or the URL `https://proxy.example.com/headless/<request_id>/approve`. Once the user completes local authentication, `tsh` will receive credentials to complete the request.

```console
$ tsh --headless --proxy=proxy --user=user ssh user@node01
Complete headless authentication on your local device or web browser:
 - tsh --proxy=proxy.example.com --user=dev headless approve <request_id>
 - https://proxy.example.com/headless/<request_id>/approve
// Wait for user to complete local authentication with MFA
<user@node01> $
```

#### Environment variables

In the `tsh --headless` flow, users never run `tsh login` on their remote machine. Instead, we expect the `--proxy`, `--user`, and `--headless` flags to be supplied to each command. To reduce UX friction, users can set the environment variables `TELEPORT_PROXY=<proxy_addr>`, `TELEPORT_USER=<user>`, and `TELEPORT_HEADLESS=true` instead.

We prefer setting environment variables rather than saving config to disk (`~/.tsh/proxy.example.com.yaml` and `~/.tsh/current-profile`) so that the headless flow remains stateless, preventing conflicts on shared machines.

#### `tsh headless`

When the user enters the headless authentication command or URL, the user will be prompted to login with MFA, if they are not logged in already. The user will then be notified of additional request details and asked to acknowledge that they made this request (`y/N` prompt). Finally, the user is asked to verify with MFA to approve the request.

```console
$ tsh --proxy=proxy.example.com --user=dev headless approve <request_id>
Enter password for Teleport user dev:
Tap your YubiKey
> Profile URL:        https://proxy.example.com
  Logged in as:       dev
  ...
Headless request requires authentication. Contact your administrator if you didn't initiate this request.
Additional details:
  - command: "tsh ssh user@localhost"
  - request id: <request_id>
  - ip address: <ip_address>
Approve? (y/N):
Tap your YubiKey
$ tsh headless accept <request_id> --proxy=proxy.example.com --user=dev
Headless request requires authentication. Contact your administrator if you didn't initiate this request.
Additional details:
  - command: "tsh ssh user@localhost"
  - request id: <request_id>
  - ip address: <ip_address>
Approve? (y/N):
Tap your YubiKey
```

Note: When the user has to log in for the first time, we do not reuse their MFA verification to skip the second MFA check. Although this would be better UX, we cannot retrieve additional request details to share with the user until they log in. For security reasons, we should provide an MFA check after sharing the headless request details.

#### Watch/View headless requests

To improve UX, we will also offer an option for a user to watch for and accept headless requests. This will reduce the friction of copy-pasting the command or URL for every headless request.

For `tsh`, we will add a new command, `tsh headless watch`, to watch for new requests. This will use the existing watcher API, filtering for the `HeadlessLoginRequest` resource for the user. When a request is found, its details will be shared in the user's local terminal. They will be prompted to acknowledge it (`y/N`) and then approve it with MFA.

```console
$ tsh headless watch <session_id>
Headless request requires authentication. Contact your administrator if you didn't initiate this request.
Additional details:
  - command: "tsh ssh user@localhost"
  - request id: <request_id>
  - ip address: <ip_address>
Approve? (y/N):
// y
Tap your YubiKey
...
```

Note: Unlike in the browser, `tsh headless watch` will have the ability to connect directly to the user's MFA key without additional prompts. Therefore, the `y/N` prompt is important to ensure a user knows what action they are accepting, rather than simply tapping their MFA key whenever it blinks, unsuspecting of attackers.

In the Web UI, we will create a new page to view and accept `tsh` requests for a headless session: `https://proxy.example.com/headless/<headless_session_id>/requests`. The UI may be very similar to the access request page, where a user can view requests, view additional details, and then click approve/deny. When the user clicks "approve", this will trigger a prompt for MFA verification to complete the approval.

#### Web UI

Initially, headless authentication will only be supported with `tsh`. After the initial implementation, we will expand support to the Web UI flow (`https://proxy.example.com/headless/<request_id>/approve`).

#### Teleport Connect

Teleport connect has the unique ability to detect when a `tsh --headless` command is run from its own terminal. This means that if a user connects to a remote machine using the Teleport Connect terminal and runs `tsh --headless ...`, Teleport Connect can immediately display the request details and approval/denial option, with MFA check. This can be done without the watch/view feature described above.

However, if the `tsh --headless` request is made from a non Teleport Connect terminal, we should provide a page like `https://proxy.example.com/headless/<headless_session_id>/requests` so a user can approve/deny headless requests within Teleport Connect.

Similarly to the Web UI, this may not be included in the initial implementation.

---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD TBD - Headless MFA login

## Required Approvers

* Engineering @r0mant && @jakule
* Security @reed
* Product: @xinding33 || @klizhentas

## What

Add "headless" login flow to Teleport to enable Teleport users to log in from a remote machine with MFA.

## Why

To enable Teleport users to complete MFA authentication flows securely from a remote machine. Specifically, we will enable the following flows:

1. Logging in and connecting to Teleport Nodes from a Teleport user's personal Cloud Dev Box.
2. Performing `tsh scp` from one Teleport Node to another Teleport Node.

In both cases, the user needs to 1) log in with MFA on the remote host and 2) pass Per-session MFA check to connect to Teleport Node from remote host.

## Details

### UX

The headless flow will begin when a user is connected to a remote host and performs `tsh login --headless`. At this point, nothing is assumed of the client's current environment or local login session. The user will be prompted to complete the login locally, through `tsh headless --accept` or through their web browser, with a server generated headless session ID.

Once the user completes the headless login locally, `tsh login --headless` will retrieve headless certificates from the server for succeeding requests.

```console
$ tsh login --headless --proxy=proxy --user=user
Complete headless authentication on your local device or web browser:
> tsh headless --proxy=proxy.example.com --user=dev --accept=<session_id>
  OR
https://proxy.example.com/headless/<session_id>/accept
# Wait for user action
> Profile URL:        https://proxy.example.com:3080
  Logged in as:       dev
  Cluster:            root-cluster
  Roles:              headless
  ...
```

#### Local login

If the user is not yet logged in locally, they will be prompted to login as normal through their terminal or browser. The MFA verification used to login will also be used to accept the headless login request, unblocking the `tsh login --headless` request above.

```console
$ tsh headless accept <session_id> --proxy=proxy.example.com --user=dev
Enter password for Teleport user dev:
Tap your YubiKey
> Profile URL:        https://proxy.example.com
  Logged in as:       dev
  ...
```

If the user already has a valid login session locally in their terminal or browser, they will instead be prompted to complete a single MFA check.

```console
$ tsh headless accept <session_id> --proxy=proxy.example.com --user=dev
Tap your YubiKey
```

#### Headless login session

In a headless login session, `tsh` will request new MFA-verified certificates for every request, following the same headless login flow above to complete MFA verification.

```console
$ tsh ssh user@server01
Complete headless authentication on your local device or web browser:
> tsh headless accept <session_id> --request=<request_id> --proxy=proxy.example.com --user=dev 
  OR
https://proxy.example.com/headless/<session_id>/request/<request_id>
# Wait for user action 
<user@server01> $
```

To improve UX, we will also offer an option for a user to poll for and accept headless requests. This will reduce the friction of copy-pasting for every headless request.

For `tsh`, we will add a new command, `tsh headless poll`, to poll for new requests for a specific headless session. When a request is found, its details will be shared in the user's local terminal. They will be prompted to accept it (`y/N`) and then verify with MFA.

```console
$ tsh headless accept <session_id> --poll
Incoming request "<request_id>": "tsh ssh user@server01" from "<ip_address>". Accept? (y/N):
# y + enter
Tap your YubiKey
```

Note: Unlike in the browser, `tsh headless --poll` will have the ability to connect directly to the user's MFA key without additional prompts. Therefore, the `y/N` prompt is important to ensure a user knows what action they are accepting, rather than simply tapping their MFA key whenever it blinks, unsuspecting of attackers.

In the Web UI, we will create a new page to view and accept `tsh` requests for a headless session: `https://proxy.example.com/headless/<headless_session_id>/requests`. The UI may be very similar to the access request page, where a user can view requests, view additional details, and then click approve/deny. When the user clicks "approve", this will trigger a prompt for MFA verification to complete the approval.

#### `tsh <command> --headless`

Users can also skip the initial `tsh login --headless` request and issue a headless request directly. In this case, `tsh` will request MFA-verified certificates directly from the proxy rather than requesting a headless certificate first. No keys, certificates, or profile information will be saved to disk, making this flow useful in shared hosts.

```console
$ tsh --headless --user=dev --proxy=proxy.example.com ssh user@server01
Complete headless authentication on your local device or web browser:
> tsh headless accept <session_id> --request=<request_id> --proxy=proxy.example.com --user=dev 
  OR
https://proxy.example.com/headless/<session_id>/request/<request_id>
# Wait for user action 
<user@server01> $
```

#### Browser flow

Initially, headless login will only be supported with `tsh`. After the initial implementation, we will expand support to the Web browser flow.

### Implementation details

#### Headless login proxy endpoint

#### Headless certificate

### Security

The headless login design should fulfill the following security principles to ensure that the system can not be easily exploited by attackers:

1. User must complete MFA verification for each individual action - `tsh ls`, `tsh ssh`, etc.
2. We can verify that the client requesting certificates is the same client receiving certificates.

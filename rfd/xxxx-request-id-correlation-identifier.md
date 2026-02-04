---
authors: Marek Smoliński (marek@goteleport.com)
state: draft
---

# RFD XXXX – Introducing RequestID: Reliable End-to-End Request Correlation in Teleport

## Required approvers
- Engineering: (@gus || @programmerq)
- Product: (@klizhentas || @r0mant)

# What

Introduce a universal, always-present, user-facing `request_id` that is:

Generated for every inbound request:
> Each client request or internal trigger generates a unique `request_id` at Teleport's entrance system boundaries.

Propagated through all Teleport services and components
> The `request_id` flows through the entire request path via context propagation (gRPC metadata, HTTP headers, SSH envelope) across all service boundaries.

Included in all structured logs during request flow
> Every log entry emitted while handling the request contains the `request_id` field, enabling instant log correlation across all services.

Returned in all user-facing errors
> Error messages shown to users (CLI, Web UI, API responses) include the `request_id` so users can provide it to support.

Recorded in all audit events emitted during request handling
> Audit events generated during the request handling include the `request_id` for complete audit trail correlation.

This creates a single, reliable correlation key for troubleshooting and support across Teleport Cloud and self-hosted environments.

# Why

## Teleport's is Distributed Architecture

A single user request may traverse multiple services like:
`Proxy Service, Auth Service, SSH Nodes, Database Agents, Application Agents, Kubernetes Agents, Local Proxies & Relay Services`

Today, Teleport has no correlation identifier that ties all logs for a given request together.
This creates slow issue resolution, unnecessary support loops, and poor observability.

## Industry Standard: RequestID / Correlation IDs

Modern distributed systems universally include a correlation identifier in user-facing errors.
This enables instant log retrieval and reduces support overhead.

## Example: AWS Error Response

```xml
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
  <RequestId>4442587FB7D0A2F9</RequestId>
</Error>
```

# UX
## User Story 1: Okta SSO Integration Failure
Alice creates a new Teleport Cloud tenant with Okta SAML SSO:
1. Creates tenant `corp.teleport.sh`
2. Receives invite email for `alice@corp.com` (admin account)
3. Sets up Okta SAML integration (Okta already has `alice@corp.com` as a user)
4. Attempts to login via SAML

#### Current Flow (Without request_id)

Customer sees error in the browsers during SAML SSO login flow:
```
┌──────────────────────────────────────────────┐
│  Unable to log in, please check Teleport's   │
│  logs for details.                           │
└──────────────────────────────────────────────┘
```
Returned from [internal FE flow](https://github.com/gravitational/teleport/blob/master/web/packages/teleport/src/Login/LoginFailed.tsx#L44)

**Alice creates support ticket:**
```
I configured Okta SAML integration following the docs,
but I get "Unable to log in" error when trying to login I'm getting following error:
Unable to log in, please check Teleport's log for details.

Please help.
```
Support engineer's challenge: with no timestamp or correlation ID, they must request additional details (username, timestamp) from customer and manually grep through logs looking for patterns.

#### Improved Flow (With request_id)

Customer tries to login via SAML and sees error message with request_id in the browser:
```
┌───────────────────────────────────────────────────────────────┐
│ Unable to log in, please check Teleport's log for details.    │
│                                                               │
│                                                               │
│ When contacting support, please include this request ID.      │
│ RequestID(4bf92f3577b34da6a3ce929d0e0e4736)                   │
└───────────────────────────────────────────────────────────────┘
```

**Customer creates support ticket:**
```
I get "Unable to log in" error when trying to login via Okta. My requestID is (4bf92f3577b34da6a3ce929d0e0e4736)
```

**Support engineer searches logs in Loki:**
```
{namespace="example.teleport.sh"} |= "4bf92f3577b34da6a3ce929d0e0e4736"
```

**Instant results - complete request flow across Proxy and Auth services:**

```json
{"timestamp":"2025-12-01T15:23:41Z","level":"info","component":"proxy","request_id":"4bf92f3577b34da6a3ce929d0e0e4736","message":"SAML callback request received","connector":"okta","source_ip":"203.0.113.42"}
{"timestamp":"2025-12-01T15:23:41Z","level":"info","component":"auth","request_id":"4bf92f3577b34da6a3ce929d0e0e4736","message":"processing SAML assertion","connector":"okta","attributes":{"email":"alice@corp.com","groups":["Engineering"]}}
{"timestamp":"2025-12-01T15:23:41Z","level":"info","component":"auth","request_id":"4bf92f3577b34da6a3ce929d0e0e4736","message":"validating user identity","email":"alice@corp.com"}
{"timestamp":"2025-12-01T15:23:41Z","level":"error","component":"auth","request_id":"4bf92f3577b34da6a3ce929d0e0e4736","message":"user creation blocked","error":"local user 'alice@corp.com' already exists","suggestion":"Either change NameID in SAML assertion or remove local user"}
{"timestamp":"2025-12-01T15:23:41Z","level":"error","component":"proxy","request_id":"4bf92f3577b34da6a3ce929d0e0e4736","message":"SAML authentication failed","user":"alice@corp.com","error":"user already exists"}
```

**Result:** Support engineer instantly identifies the root cause - local user conflicts with SSO user.

## User Story 2: Missing Login Permission When Connecting to an SSH Node
### Scenario
Alice attempts to connect to `node1` over SSH as `root`:

```bash
tsh ssh root@prod-node
ERROR: access denied to root connecting to node1
```

The error message provides no context and does not explain why access was denied.
Alice cannot fully diagnose the issue herself and contacts support.

With request_id included in the error message, the user receives:

```
tsh ssh root@prod-node
ERROR: access denied to root connecting to node1 [request_id: ab12cd34]
```

This gives support an immediate, searchable correlation token.
Using request_id to Trace the Full SSH Flow in Teleport self hosted logs or Teleport Cloud logs:
```
{namespace="example.teleport.sh"} |= "ab12cd34"
```

```
13:19:22.495+01:00 [request_id=ab12cd34]  DEBU [PROXY:SER] Initiating dial request cluster:ice-ice.dev dial_params:from: "127.0.0.1:63389" to: "127.0.0.1:3022" reversetunnel/local_cluster.go:311
13:19:22.495+01:00 [request_id=ab12cd34]  DEBU [HTTP:PROX] No proxy set in environment, returning direct dialer proxy/proxy.go:184
13:19:22.495+01:00 [request_id=ab12cd34]  DEBU [PROXY:SER] Succeeded dialing cluster:ice-ice.dev dial_params:from: "127.0.0.1:63389" to: "127.0.0.1:3022" reversetunnel/local_cluster.go:317
13:19:22.496+01:00 [request_id=ab12cd34]  DEBU  proceeding with connection resumption exchange resumption/server_detect.go:171
13:19:22.496+01:00 [request_id=ab12cd34]  INFO  handling new resumable SSH connection resumption/server_exchange.go:92
13:19:22.496+01:00 [request_id=ab12cd34]  INFO  handing resumable connection to the SSH server resumption/server_exchange.go:136
13:19:22.502+01:00 [request_id=ab12cd34]  DEBU [NODE]      processing auth attempt with key local_addr:127.0.0.1:3080 remote_addr:127.0.0.1:63389 user:root ...
13:19:22.503+01:00 [request_id=ab12cd34]  INFO  emitting audit event event_type:auth fields:map[...] alice] events/emitter.go:287
13:19:22.503+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.504+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.504+01:00 [request_id=ab12cd34]  DEBU [NODE]      processing auth attempt with key ...
13:19:22.504+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.504+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.504+01:00 [request_id=ab12cd34]  DEBU [NODE]      processing auth attempt ..
13:19:22.504+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.505+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.505+01:00 [request_id=ab12cd34]  DEBU [NODE]      processing auth attempt ... more cert attempts ...
13:19:22.505+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.505+01:00 [request_id=ab12cd34]  WARN [SSH:NODE]  Error occurred in handshake  "root" not in the set of valid principals for given certificate
13:19:22.505+01:00 [request_id=ab12cd34]  INFO  resumable connection completed
13:19:22.505+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
13:19:22.506+01:00 [request_id=ab12cd34]  DEBU  handling new resumable connection error: [...]
13:19:22.506+01:00 [request_id=ab12cd34]  INFO  emitting audit event ...
```

The following line clearly reveals the source of the failure:
```
"root" not in the set of valid principals for given certificate
```

Without a request_id, correlating this information across Proxy, Auth, and Node logs is tedious and error-prone.
With request_id, the entire SSH authentication flow is visible immediately, allowing support to diagnose the problem quickly.

# Current State: Teleport Distributed Tracing ([RFD 0065](https://github.com/gravitational/teleport/blob/master/rfd/0065-distributed-tracing.md#L2))
## Why trace_id is Not Sufficient ?

1. Not enabled by default - Tracing needs to be enable (Due to pref implication tracking is set to 0% by default)
2. Not User-Facing

Even when trace_id exists, it is never shown to end users.

# Security

`request_id` is purely an observability and support tool.
It must never be used for authentication, authorization, session handling, or rate limiting.
The value contains no sensitive data and is safe to include in logs, error messages, and audit events.
RequestID should not be use as a metric label since it has very hight entropy. 


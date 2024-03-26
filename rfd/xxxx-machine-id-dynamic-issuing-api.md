---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD XXXX - Machine ID Dynamic Credential API

## Required Approvers

* Engineering: @zmb3
* Security: (@reed || @jentfoo)
* Product: (@xinding33 || @klizhentas)

## What

Machine ID's `tbot` agent currently works with a static configuration.

The Machine ID Dynamic Credential API will offer an API from the `tbot` agent
by which CLI tools and customer applications will be able to request
credentials on-the-fly.

## Why

Static configuration is awkward for customers with a large number of potential
applications, Kubernetes clusters or databases as each of these targets requires
its own specific credential to include the RouteTo fields.

The ability to request credentials on-the-fly from a running `tbot` agent will
allow these more advanced use-cases.

Looking towards the future, dynamic credential issuance will be essential for
device trust implementations, where credentials will need to be issued moments
before usage in order to pass relevant device trust checks.

## Implementation

### Configuration

`tbot` must be explicitly configured with sockets to expose:

```yaml
auth_server: root.tele.example.com:443
store:
  type: memory
sockets:
- path: file:///my/tbot/socket
  # Roles on a socket limit what socket clients can request and the defaults 
  # a socket client will receive if unspecified
  roles: ["role-a", "role-b"]
```

### CLI

The `tbot` CLI should be extended with several commands that can use the API
to generate one-shot credentials on the fly:

```sh
tbot generate --socket /my/tbot/socket application --app-name foo --cluster bar --out file:///tmp/app-credentials --ttl 1m
curl --cert /tmp/app-credentials/tlscert --key /tmp/app-credentials/key https://app.bar.tele.example.com/api/users
```

A long-term goal will be to enable tools such as `tsh` to directly connect to
this socket, and be able to fetch credentials at will. This will allow seamless
use of `tsh` commands that typically require the re-issuing of certificates with
specific fields that do not work today. However, this remains out of scope for
the initial implementation.

### API

A gRPC API will be exposed over a unix socket. Because it is a unix socket,
existing security strategies around file-system permissions and ACLs can be
reused to ensure only intended consumers can connect.

```proto
// TODO: Write the Proto
```

### Client

A Go client will be produced that implements methods identical to those on the
`tbot.Bot` struct. This will allow existing implementations of destination
generation code to interact with the bot without explicit knowledge of whether
the bot is running within the same process or across a unix socket.

```go
type DestinationHost interface {
	GenerateIdentity(ctx context.Context, req IdentityRequest) (*identity.Identity, error)
	// ListenForRotation enables destinations to listen for a CA rotation
	ListenForRotation(ctx context.Context) (chan struct{}, func(), error)
	ClientForIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error)
	Logger() logrus.FieldLogger
}
```

## Security implications

The use of a unix socket rather than a TCP/IP socket restricts access to the
API to process running on the same host and as a user which has been
granted permissions to the unix socket.

However, this is still less secure than the traditional static configuration
as a malicious actor with access to the socket, but not the bots configuration
file, can request credentials which have not been explicitly configured.

To limit the scope of this, user's will be able to configure a limitation on
the roles that can be requested through the socket.
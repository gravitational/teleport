---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 34 - Clients Certificate Rotation

## What

Provide a way for long living automated clients to retrieve new User certificates as needed.

## Why

Presently, there is no good way to keep client certificates secure and valid for long durations. In order to gain access for an extended period of time, one must retrieve a set of long lived certificates. There are a few security and UX issues with this approach.

- Long lived certificates are less secure than short lived certificates.
- The User CA may be rotated due to a security breach or a scheduled rotation. When this happens, all User certificates will lose access, after a grace period, and need to be manually refreshed.
- Eventually, long lived certificates will expire after their TTL. Then they need to be manually refreshed, which puts a strain on administrators and could lead to downtime if neglected.

## Details

Cert agent will be a new standalone client with the responsibility of continuously renewing a set of certificates before they lose access. Before explaining how the agent will function, we need to create a new abstraction for certificates to enable the agent.

### Cert Store

The agent will interact with certificates through its Cert Store.

A Cert Store can be used to:
 - authenticate a client
 - store renewed certificates
 - alert when the store's certificates are altered
 - provide its certificates expiration time

```go
type Store interface {
  // extends client.Credentials to authenticate a client
  client.Credentials
  // store certs
	Store(certs) error
  // when current certs expire
	Expires() time.Time
  // used by the Auth server when signing new certificates
  PublicKey() []byte
  // signal that these certificates have been changed
  Refresh() <- chan struct{}
}
```

#### Formats

The `Store` interface can be used to support a wide variety of certificate storage formats. 

Initially, the following Cert Stores will be implemented:
 - `PathStore` (uses direct cert paths, successor of `KeyPairCredentials`)
 - `ProfileStore` (tsh profile)
 - `IdentityFileStore`

More stores can be added in the future to support:
 - db/app/kube certs in tsh profile
 - kubernetes secrets 

#### Refresh

The Cert Store will watch it's certificates for updates, whether it's a file change or something else. When this occurs, it will send a message on its `refresh` channel.

Refresh only uses a single channel, so the `Store` can only be used by a single Cert Agent. Additional Cert Agents will be prevented from using a used `Store` object.

### Cert Agent

The Agent will:
 - hold a Cert Store 
 - hold a Client that is authenticated by the Cert Store
 - watch for upcoming certificate expiration events (CA rotation or TTL expiration)
 - renew the Cert Store's certificates before expiration events

```go
type Agent struct {
  // client used to renew certificates and watch for expiration events
  Client Client
  // the agent's certificates store
  CertStore Store
  // TTL will define how long new certs generated through the Agent will 
  // live for. Note that it will be limited on the server side by the
  // user's max_session_ttl.
  TTL time.Duration
}
```

Note that the agent's Cert Store will also be used to authenticate the agent's own client, meaning the agent will act on behalf of the certificates' user.

The Agent *could* be expanded to hold a list of other stores to maintain, but this would add some complications.
 - external certificates would be signed through impersonation. This could limit the use of the certificates, and would require the agent to have a variety of impersonation rules, which could be a major security hazard.
 - external certificates could be from other clusters, meaning they can't be signed through the Agent's client. 
 - The agent would be able to renew any certificates, regardless of that certificates' user's access controls (see RBAC section)

For these reasons, this idea will be saved for a future discussion.

#### Cert Client

The agent only needs access to a few client methods in order to perform its job.

```go
type Client interface {
  NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
  GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
  RefreshConnection(ctx context.Context, creds Credentials) error
}
```

#### Watch for certificate expiration

The agent will use its `Store`'s `Expires` method to see when the certificates need to be renewed. The agent will wait until the expiration is in `1/6*agent.TTL` before attempting to renew the certificates. If the renewal fails the first time, the agent will retry 9 more times in equal intervals. This is notably arbitrary and can be improved by using more advanced techniques, such as a backoff mechanism, a jittery ticker, etc.

##### Watch CA rotation state

The agent will use `client.NewWatcher` to watch for updates to the certificate authority's rotation state. The agent will keep track of the current rotation state, and if an event is received where the rotation state is changed to `update_clients`, the agent will refresh the store's certificates and CA certificates.

##### Retrieve new certificates

The agent will use `client.GenerateUserCerts` to retrieve newly signed certificates for its `Store`. The Store's `PublicKey` and username will be provided in order for the Auth server to sign new certificates. The certificate's expiration will also be derived from the agent's `TTL` field.

##### Refresh Client connection

The agent will watch its store's `Refresh` channel in order to refresh the client connection when needed. This may be caused by the Cert Agent writing to the `Store` itself, or by an external actor.

### RBAC restriction

Certificate Agent is intended for automating procedures, not for short-cutting authentication measures already in place. For example, `tsh login` should continue to be the standard authentication method for users logging into the system.

Therefore, new certificates will only be issued if the associated user has the new `reissue_certificates` role option enabled. This option will be false by default and should only be set to true for automation user roles.

Additionally, the CA rotation state can only be watched if the Certificate Agent is allowed perform `read` or `readnosecrets` actions on `cert_authority`.

```yaml
kind: role
version: v3
metadata: 
  name: cert-agent
spec:
  options:
    reissue_certificates: true
  allow:
    rules:
      - resources: ['cert_authority']
        verbs: ['readnosecrets']
```

### Integrations

In addition to being a standalone client, Certificate Agent can be integrated into the API Client, `tctl`, and `tsh` to meet a variety of automation use cases.

#### API Client

The API Client needs a new method to allow the agent to refresh its client's connection.

```go
func (c *Client) RefreshConnection(creds Credentials) error {
  // Attempt to connect the client using the updated credentials. 
  // If it fails, fallback to the former client connection and return the error.
}
```

Now, a new client can simply start up a Cert Agent to automatically keep its credentials refreshed. This can be added to the end of the client constructor.

```go
// connect client to server
store, ok := client.creds.(Store)
if ok && config.RunCertAgent {
  agent := Agent{
    Client: client,
    Store: store,
  }
  // agent will automatically refresh the store with updated certificates,
  // refresh the client connection, and close as soon as the client is closed.
  go agent.Run()
}
```

#### tsh and tctl

If desired, `tsh` and `tctl` could integrate Cert Agent functionality. This could be useful for Teleport users who orchestrate `tsh` and `tctl` to run automated processes.

`tctl auth sign --certagent` could be used to generate new certificates and automatically start up a new Certificate Agent Service using the certificates as a Store. All `--format` options would be supported. 

`tsh login` and `tsh [db|app|kube] login` could also support the `--certagent` flag for all available formats.

Note that the `reissue_certificates` role option would need to be enabled, so normal users won't be able to run a Cert Agent to refresh their `tsh login` credentials.

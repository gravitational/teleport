---
authors: Tiago Silva (tiago.silva@goteleport.com)
state: draft
---

# RFD 117 - Forward User Identity between Teleport services

## Required Approvers

- Engineering: `@r0mant`
- Security: `@reedloden`

## What

This RFD proposes a way to forward user identity from the Teleport Proxy to
Teleport Kubernetes Service and remote Teleport proxy.

### Related issues

- [#21609](https://github.com/gravitational/teleport/issues/21609)

## Why

When a user connects to a Teleport proxy, the proxy will authenticate the user
using his X.509 certificate. Kubernetes Proxy will then forward the request to the
Teleport service responsible for serving the Kubernetes cluster (Kube Service) or
to the remote Teleport proxy when the Kubernetes Cluster belongs to a trusted cluster.
For the proxy to be able to forward the user's identity to the correct service,
it needs to generate a new X.509 key-pair with the user identity embedded that
will be used during the mTLS authentication.
The upstream service will then use the user identity to authorize the request and to
impersonate the configured Kubernetes principals when forwarding the request to
the Kubernetes API server.

To avoid having to generate a new certificate for each request, the proxy will
generate a new certificate for each user's identity and cache it in memory for
`min(3h, certExpiration)`. This certificate cache must be keyed with a key that
uniquely identifies the user's identity so that the proxy can retrieve the certificate
for the correct identity when forwarding the request. In the past, we had situations
where the cache was not properly keyed and the proxy would forward the request
with the wrong certificate ending up with authorization errors.

Currently, the key used to cache the certificate is the user's identity has the
following format:

```go
func (c *authContext) key() string {
	// it is important that the context key contains user, kubernetes groups and certificate expiry,
	// so that new logins with different parameters will not reuse this context
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", c.teleportCluster.name, c.User.GetName(), c.kubeUsers, c.kubeGroups, c.kubeClusterName, c.certExpires.Unix(), c.Identity.GetIdentity().ActiveRequests)
}
```

Each time we introduce a new field to the user's identity, we need to make sure
that the key is updated to include the new field if the field is used by Kubernetes
Access. This process is error-prone and it's easy to forget to update the key, and
eventually introduce a bug or a security vulnerability.

This RFD proposes a way to forward the user's identity from the proxy to the
Kube Service and remote Teleport proxy that does not require the proxy to generate
a new certificate for each user's identity.

## Details

The Proxy will authenticate to the Kube Service and any remote Teleport proxy using its
certificate. The certificate used by proxy does not contain any user identification
and it's signed by the Teleport CA. The proxy will then forward the user's identity
using HTTP headers or gRPC metadata instead of generating a new certificate on
the behalf of the user. Since the proxy is authenticated using its certificate,
the upstream service can trust the identity received in the request headers.

This change will remove the requirement for the proxy to generate a new certificate
and thus call auth's `ProcessKubeCSR` endpoint to sign the key-pair containing the
user's identity.


### Impersonation: Forward user identity using HTTP headers

As opposed to the majority of protocols supported by Teleport, the Kubernetes protocol
is based on HTTP 1/2. This allows us to forward the user's identity using HTTP headers
for the specific case of Kubernetes.

The proposed solution is to use the Proxy certificate to authenticate to the upstream
service and to forward the user's identity using HTTP headers or gRPC metadata
for later Impersonation.

Once the upstream service receives a request, it will check the certificate provided
to validate the request's provenance and its role - making sure it's a proxy - for
authenticating the request. Once authenticated, it will impersonate the identity
contained in the request headers. This impersonated identity will be available in
`authz.Context` and services won't be able to distinguish between a request forwarded
by the proxy or a request made directly by the user.

This approach is similar to what Kubernetes API does when
forwarding requests to the API server using Impersonation. In the Teleport case, we must
forward the full user's identity instead of just the username/roles.

Since the proxy has full access to the HTTP request, it can add the user's identity
to the request headers before forwarding it to the upstream service.

```go
headers["TELEPORT_IMPERSONATE_IDENTITY"] = json(clientCert.Subject)
```

In order to prevent the user from tampering with the headers, the authorization
layer will delete the headers after checking if the request originated in a proxy,
so the user cannot send the headers directly and the proxy forwards them
to the upstream service.

Besides the security benefits, this option also has the advantage of allowing us to
reuse the same connection to forward multiple requests from different users. It happens
because the HTTP headers are sent with each request and we do not require a new
connection per request such as in the Proxy Protocol option. This will improve
performance by reducing the number of connections that need to be established and
the time it takes to forward a new request upstream. Currently, the proxy creates a new
`http.Transport` per request which means that a new connection is established
every time a request is forwarded. This is not ideal because it increases the
latency of the request and it also increases the load on the upstream service.

This option requires sending the user's IP address to the upstream service
using a header instead of the protocol extension implemented by [TLS IP Pinning RFD](https://github.com/gravitational/teleport/pull/22481).
This is because the connection is reused and the IP address is not available
in the connection prelude since the connection does not belong to a specific user/request.

## Rollout plan

To avoid breaking changes, we will implement this feature in two phases.

### Phase 1: Add support for receiving user identity in Kube Service/Proxy from the Kube Proxy

- Teleport 13.0

We will add support for receiving user identity in Kube Service/Proxy from the
Kube Proxy. Teleport heartbeats contain the agent version and the proxy will
only forward the user's identity to the Kube Service/Proxy if the agent version
is >= 13.0. For older agents, the proxy will continue to generate a new certificate
for each user's identity and call the `ProcessKubeCSR` endpoint. This will allow
us to roll out this feature without breaking changes to Teleport clusters running
older versions (<= 12.0).

When the proxy uses its X.509 certificate to authenticate to the upstream service,
the upstream service will expect the user's identity to be
forwarded using HTTP headers or Proxy Protocol extensions. If the user's identity
is not forwarded, the upstream service will reject the request.

### Phase 2: Enable the proxy forwarding of user identity to Kube Service/Proxy

- Teleport 14.0

We will enable the proxy to forward user identity to Kube Service/Proxy.
At this point, Teleport supports clients running Teleport 13.0 and newer and we can
remove the code that generates a new certificate for each user's identity and calls
`ProcessKubeCSR` endpoint because the upstream services already support receiving
user identity from the proxy without the need for the proxy to generate a new certificate.

## Enterprise considerations

Currently, when the Teleport proxy calls `ProcessKubeCSR` endpoint and Auth server
is configured with an Enterprise license, the auth server checks if the license
has the Kubernetes feature enabled. If the Auth server isn't licensed to Kubernetes usage,
Auth server will return an error to the proxy and the request is rejected.

Since we no longer need to call `ProcessKubeCSR` endpoint, we will need to verify
that the cluster is licensed to Kubernetes before forwarding the request to the Kube
Service. Auth server will forbid agents to register their `KubeServers` during
heartbeats if Auth isn't licensed to Kubernetes. Users will also receive an error
when listing Kubernetes clusters using `tsh kube ls` informing them the cluster
isn't properly licensed for Kubernetes Access. This would allow us to
automatically fix the error message returned to the user when the license is
invalid [teleport.e#661](https://github.com/gravitational/teleport.e/issues/661).

This change only affects enterprise users that are using the Kubernetes Access feature
without a valid license.

## Security

Besides the performance improvements, this change will also improve security by
removing the certificate cache and removing the possible reuse of certificates
with the wrong identity. This happened in the past when the cache was not properly
keyed and resulting in the proxy forwarding the wrong certificate to the upstream service.
The bug was fixed by adding extra fields to the cache key. However, this is still error-prone and
it's easy to introduce major security vulnerabilities.

Since the proxy will send the same Identity to the upstream service that it
received from the client, the upstream service will be able to trust the user's identity
and there is no possibility of the proxy forwarding the wrong identity.


## Other solutions considered

This section describes other solutions considered for forwarding the user's identity
to the upstream service but were discarded because they either didn't provide
the same security guarantees.

#### Forward user identity using Proxy Protocol extensions

Similarly to the [TLS IP Pinning RFD](https://github.com/gravitational/teleport/pull/22481),
Teleport Proxy will forward the user's identity using Proxy Protocol extensions. This
means that the Teleport proxy will send the user's identity to the upstream service when dialing
the connection. The upstream service will then be able to extract the user's identity
from the connection prelude and use it to authorize the request.

As opposed to the HTTP header option, this option doesn't require sending the user's IP
because it's already included in another Proxy Protocol extension.

This option requires a new connection per request because the Proxy Protocol
extensions are sent when the connection is established. This means that
we cannot reuse the connection to forward multiple requests from different users.
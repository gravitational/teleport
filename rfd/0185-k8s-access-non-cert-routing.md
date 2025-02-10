---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 185 - Kubernetes Access without Certificate-based Routing

## Required Approvers

* Engineering: @tigrato && @rosstimothy

## What

Switch from using certificate attribute-based routing for Kubernetes access to
a new form of routing that does not require the issuance of a new-certificate
for each Kubernetes cluster.

## Why

Today, Machine ID output can only generate a Kubeconfig for a single cluster
that has been named ahead of time. This has proved problematic for a number of
use-cases where the user wishes to authenticate a machine to connect to many
Kubernetes clusters. At smaller scales, it is not a problem to configure an
output per Kubernetes cluster, but at larger scales, this becomes a problem.

The root of this behaviour in `tbot` comes from the fact that a distinct
client certificate must be issued for each Kubernetes cluster that you wish to
connect to. This is because an attribute is encoded within the certificate to
assist with routing.

The need to issue a certificate per cluster becomes a problem when wishing to 
connect to a large number of clusters for the following reasons:

- Increased pressure on the Auth Server. Certificate signing is an expensive
  operation and requesting the signing of hundreds of certificates within a 
  short period of time threatens to overwhelm the Auth Server.
- Less flexibility. If a short-lived Kubernetes cluster is spun up, then a new
  certificate must be issued for that cluster. This means that `tbot` must be
  reconfigured or re-invoked.
- Increased pressure on `tbot`. If `tbot` were to issue certificates for a large
  number of clusters, then it would require a larger amount of resources to make
  the increased number of requests.
- Decreased reliability. The more certificates that must be issued to
  successfully generate a Kubeconfig, the more likely that one of the set will
  fail for some ephemeral reason.

We're currently talking to a user who has the need to connect to 100s of
Kubernetes clusters from a single-host to enable the Kubernetes platform team
to manage and monitor clusters across the organization.

For users who do not leverage Machine ID, there are still some benefits of 
undertaking this work. For example, using `tsh` to generate a Kubeconfig will
be faster as it will not need to request the signing of a new certificate as 
part of the process.

Additionally, from an idealistic design perspective, the current approach
compromises the intended use of certificates. Certificates are designed to
encode attributes of an identity - information about which cluster you wish to
connect to is not an attribute of identity!

## Details

### Today

Today, we encode the intended target cluster within the user X.509 certificate
within the `KubernetesCluster` attribute (OID 1.9999.1.3).  

This attribute is primarily used by the Proxy and Kubernetes Agent to determine
where to route requests intended for the Kubernetes cluster.

When the KubernetesCluster attribute is requested by a client using the 
`GenerateUserCerts` RPC, the certificate request is considered "Kubernetes"
for analytics purposes. If we were to replace this attribute, we would need to 
find an alternative way to determine if a certificate request is intended for
Kubernetes.

The target Kubernetes agent performs authorization checks based on the
user's identity (e.g roleset, attributes) rather than relying on this attribute.

### Proposal

Rather than leverage the `KubernetesCluster` attribute within the X509
certificate for routing requests to a specific Kubernetes cluster, we propose to
pre-pend the request path with this information.

For example, today, to make a request, the client may use the following URL:

```shell
GET https://example.teleport.sh/apis/apps/v1/namespaces/my-namespace/deployments
```

Following this proposal, the client would use the following URL:

```shell
GET https://example.teleport.sh/v1/teleport/base64-teleport-cluster-name/base64-k8s-cluster-name/apis/apps/v1/namespaces/my-namespace/deployments
```

To avoid potential problems with URL encoding/escaping, the name of the Teleport
and Kubernetes cluster will be base64 encoded.

Within the Kube Forwarder, new route handlers would be added for all HTTP
methods for `/v1/teleport/:teleportCluster/:kubeCluster/*`. These route handlers
would parse the `teleportCluster` and `kubeCluster` from the URL, strip off the
prefix, and then pass the request through to the original handler - with the
additional context of the desired Teleport and Kubernetes cluster. This would
take higher precedence than attributes contained within the certificate.

Teleport clients would no longer need to call `GenerateUserCerts` with a
`KubernetesCluster` to produce a certificate explicitly for access to
Kubernetes. When producing a kubeconfig, the client should use the user's 
standard identity.

To configure Kubernetes clients to work with this, the Kubeconfig generated by
Teleport would be updated to include this base path within the
`clusters[name].cluster.server` field:

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ...omitted... # No-change
    # Before:
    # server: https://example.teleport.sh/
    # After:
    server: https://example.teleport.sh/v1/teleport/base64-teleport-cluster-name/base64-k8s-cluster-name
    tls-server-name: kube-teleport-proxy-alpn.example.teleport.sh # No-change
  name: example-example
```

#### Backwards/Forwards Compatibility

By retaining the existing behaviour and endpoints within the Kube Forwarder,
older clients using a kubeconfig and certificate generated with the
`KubernetesCluster` attribute will continue to function.

- VX.Y.0:
  - Introduce new Kube Forwarder behaviour.
  - Introduce opt-in `kubernetes/v2` `tbot` output. This will allow
    users to opt-in to the new behaviour once they are satisfied that their
    Auth Server, Proxy and Kubernetes Agents have been upgraded to the required
    version.
- V(X+1).0.0:
  - Switch `tsh` to new behaviour. No longer requests the signing of a
    certificate with the `KubernetesCluster` attribute and uses standard user
    identity to authenticate.
  - Switch `tbot` `kubernetes/v1` output to use `kubernetes/v2` implementation
    under the hood.
- V(X+2).0.0:
  - It is now "safe" to remove the old attribute based routing from the Kube
    Forwarder. We may wish to delay this further if we have concerns about
    older client compatability.

#### Client Compatability

Kubernetes API clients have been known to vary greatly in their support for 
more unusual configuration. We have witnessed this in the past when it came to
supporting the ability to explicitly provide the SNI to use when making
requests.

Therefore, we should ensure that Kubernetes clients and SDKs do support
providing a base path as part of the server address (e.g they do not strip off
the path and only use the hostname/port).

The key customer we are working with has specified that they use the following
Kubernetes clients:

- `kubectl`
- Python Kubernetes SDK

Both of these clients correctly handle a base-path as configured in Kubeconfig:

```yaml
clusters:
  - cluster:
      server: http://127.0.0.1:8888/example/base/path
    name: test-base-path
```

#### Analytics

It would no longer be possible to track usage of Kubernetes Access for analytics
purposes via the certificate generation event. However, we have other events
that indicate Kubernetes Access usage (e.g an event per request), that are
better suited for this purpose - so this seems acceptable.

#### TBot UX

The `tbot` configuration will be extended with a new `kubernetes/v2` service
type that will allow you to specify multiple kubernetes clusters to generate
within a single Kubeconfig file.

To select which clusters to include within the Kubeconfig, the name or labels
of the clusters can be specified.

```yaml
- type: kubernetes/v2
  destination:
    type: directory
    path: /opt/machine-id
  # Each selector can either be a name or a set of labels. If multiple selectors
  # are provided, then the output will contain the union of the results of the
  # selectors.
  selectors:
    - name: my-cluster
    - labels:
        "env": "production"
```

### Alternatives

#### Leverage SNI

Rather than encoding the target Kubernetes cluster into the URL, we could
instead encode it into the SNI.

This has the following limitations:

- We've already witnessed limited support for specifying SNIs in Kubernetes
  clients.
- We already overload the SNI and ALPN with a few meanings that makes routing
  more difficult. Adding another level to the SNI (e.g a subdomain) would not be
  valid with the certificates we issue today.

#### Custom Protocol

Rather than encoding the target Kubernetes cluster into the URL, we could 
leverage HTTP headers, or, a custom header protocol which we inject prior to the
request.

This has the following limitations:

- Limited support for customisation across the various Kubernetes SDKs.
- Injecting a custom protocol header would require the use of a local proxy,
  which is unwieldy to manage.

## Security Considerations

### Per-session MFA

TODO

### Auditing

No new audit log events would be introduced as the result of this change.

Users would no longer expect to see the `cert.create` audit event in correlation
with Kubernetes Access occurring. This should be noted in the release notes.
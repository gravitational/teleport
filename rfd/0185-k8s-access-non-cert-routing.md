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

Additionally, from an idealistic design perspective, the current approach
compromises the intended use of certificates. Information about which cluster
you wish to connect to is not an attribute of a user or machines identity.

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

Rather than leverage the `KubernetesCluster` attribute for routing requests to
a specific Kubernetes cluster, we propose to pre-pend the URL with this 
information.

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

To configure clients to work with this, the Kubeconfig would be updated to
include this base path within the clusters[name].server field appended to the
existing hostname it uses today:

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

Within the Kube Forwarder, new route handlers would be added for all HTTP 
methods for `/v1/teleport/:teleportCluster/:kubeCluster/*`. These route handlers
would parse the `teleportCluster` and `kubeCluster` from the URL, strip off the
prefix, and then pass the request through to the original handler - with the
additional context of the desired Teleport and Kubernetes cluster. This would
take higher precedence than attributes contained within the certificate.

Clients would no longer need to call `GenerateUserCerts` with a
`KubernetesCluster` requested - instead - the normal identity of the user or 
machine can be used to authenticate.

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

The key user we are working with has specified that they use the following
Kubernetes clients:

- `kubectl`

#### Audit and Analytics

It will be worth including in the release notes that there will no longer 
necessarily be a Kubernetes certificate generation event emitted in the audit
log prior to Kubernetes access occurring.

It would no longer be possible to track usage of Kubernetes Access for analytics
purposes via the certificate generation event. However, we have other events
that indicate Kubernetes Access usage (e.g an event per request), that are
better suited for this purpose - so this seems acceptable.

### Alternatives

#### Leverage SNI

#### Custom Protocol

## Security Considerations

### Per-session MFA
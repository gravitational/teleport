---
author: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 175 - SPIFFE Federation and SPIFFE Join Method

## Required Approvers

- Engineering: @timothyb89
- Product: @thedevelopnik

## What

Implement support for SPIFFE Federation within Teleport Workload Identity,
including support for a SPIFFE based join method based on the federation
mechanism.

See https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md
for the SPIFFE Federation specification. This will be referred to as
"the specification" throughout this document.

## Why

Implementing support for SPIFFE Federation has the following benefits:

- Improves the feature parity of Workload Identity with competing solutions,
  increasing appeal to prospects.
- Allows customers to adopt Teleport Workload Identity gradually, by federating
  with existing SPIFFE based systems, rather than needing to perform a complete
  migration.
- Supports an Active/Active and Active/Passive high availability model.
- Support segmented environments using multiple Teleport clusters which
  federate with one another.
- Enables the implementation of a SPIFFE-based join method.

Implementing support for a SPIFFE join method:

- Secure alternative to the `token` based join-method for OnPrem environments.
- Allows for more advanced join implementations based on Teleport Workload 
  Identity, e.g distributing SPIFFE SVIDs using an out of band mechanism and
  then joining using the SPIFFE SVIDs.

## Implementation

### Federation

SPIFFE federation relationships are one-way. One trust domain can be configured
to trust identities issued by another trust domain, but the reverse may not 
be true. However, it is typical that two federation relationships will be
formed, one in each direction, to allow workloads in both trust domains to
verify one another.

The federation relies on the exchange of trust bundles. Trust domains which
wish to support another trust domain trusting identities it has issued must
publish an accessible Bundle Endpoint. This endpoint provides the trust bundle
necessary to verify the identities issued by the trust domain.

The structure of this trust bundle is defined in the specification as a JWKS
(JSON Web Key Set).

#### Teleport's Bundle Endpoint

A new endpoint will be exposed on the Teleport Proxy API, which will be a 
SPIFFE Federation specification compliant Bundle Endpoint. This will allow
other SPIFFE trust domains to trust identities issued by Teleport Workload
Identity.

The endpoint will implement the `https_web` profile, meaning that:

- It will be served using the TLS certificates that the Teleport Proxy has been
  configured with. These must be certificates issued by a CA trusted by the
  trust domain control plane which will be consuming the bundle.
- It must not require client authentication.

The specification does not require that this endpoint is present at a specific
path. The endpoint will be accessible at the path `/webapi/spiffe/bundle.json`.

The endpoint will return the format required by the specification.

#### Trusting Other Trust Domains

A new resource, `SPIFFEFederation`, will be introduced to allow configuration
of federation relationships. This resource will implement the RFD153 Resource
Guidelines.

```protobuf
syntax = "proto3";

package teleport.machineid.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/duration.proto";
import "teleport/header/v1/metadata.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1;machineidv1";

// SPIFFEFederation is a resource that represents the configuration of a trust
// domain federation.
message SPIFFEFederation {
  // The kind of resource represented.
  string kind = 1;
  // Differentiates variations of the same kind. All resources should
  // contain one, even if it is never populated.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources share.
  // Importantly, the name MUST match the name of the trust domain you federate
  // with.
  teleport.header.v1.Metadata metadata = 4;
  // The configured properties of the trust domain federation
  SPIFFEFederationSpec spec = 5;
  // Fields that are set by the server as results of operations. These should
  // not be modified by users.
  SPIFFEFederationStatus status = 6;
}

// SPIFFEFederationBundleSourceStatic is a static bundle source. It should be an
// option of last resort, as it requires manual updates.
message SPIFFEFederationBundleSourceStatic {
  // The SPIFFE JWKS bundle.
  string bundle = 1;
}

// SPIFFEFederationBundleSourceHTTPSWeb is a bundle source that fetches the bundle
// from a HTTPS endpoint that is protected by a Web PKI certificate.
message SPIFFEFederationBundleSourceHTTPSWeb {
  // The URL of the SPIFFE Bundle Endpoint.
  string bundle_endpoint_url = 1;
}

// SPIFFEFederationBundleSourceHTTPSSPIFFE is a bundle source that fetches the bundle
// from a HTTPS endpoint that is protected by a SPIFFE certificate that is
// "self-served" (i.e. the SPIFFE certificate is issued by the same trust domain
// that the bundle is for).
message SPIFFEFederationBundleSourceHTTPSSPIFFE {
  // The URL of the SPIFFE Bundle Endpoint.
  string bundle_endpoint_url = 1;
  // The initial SPIFFE bundle that is used to bootstrap the connection to the
  // bundle endpoint. After the first sync, this field will no longer be used.
  string bundle_bootstrap = 2;
}

// SPIFFEFederationBundleSource configures how the federation bundle is sourced.
// Only one field can be set.
message SPIFFEFederationBundleSource {
  SPIFFEFederationBundleSourceStatic static = 1;
  SPIFFEFederationBundleSourceHTTPSWeb https_web = 2;
  SPIFFEFederationBundleSourceHTTPSSPIFFE https_spiffe = 3;
}

// SPIFFEFederationSpec is the configuration of a trust domain federation.
message SPIFFEFederationSpec {
  // The source of the federation bundle.
  SPIFFEFederationBundleSource bundle_source = 1;
}

// FederationStatus is the status of a trust domain federation.
message FederationStatus {
  // The most recently fetched bundle from the federated trust domain.
  string current_bundle = 1;
  // The time that the most recently fetched bundle was obtained.
  google.protobuf.Timestamp current_bundle_synced_at = 2;
  // The duration that the current bundle suggests the next bundle should be 
  // refresh after.
  google.protobuf.Duration current_bundle_refresh_hint = 3;
}
```

A background task will run on the Teleport Auth Server to periodically fetch
the trust bundle from the configured Bundle Endpoint. This will be written into
`status.current_bundle`. If specified, the `current_bundle_refresh_hint` will
be used to determine when the next fetch should occur. If unspecified, it will
be assumed that the bundle should be refreshed every 60 minutes.

### SPIFFE Joining

SPIFFE joining will be implemented as a new join method.

SPIFFE defines two forms of SVID:

- X.509 SVIDs
- JWT SVIDs

One option would be to validate the identity of the joining node by requiring
that it offer the X.509 SVID as a client certificate. Unfortunately, this
has the following problems:

- Traversing TLS-terminating loadbalancers is challenging.
- Incompatibility with renewal mechanisms that rely on an X.509 client
  certificate being presented (e.g BotInstance renewals) as only one client
  certificate can be presented.

JWT SVIDs also come with their own challenges:

- TTL is determined by SPIFFE implementation and could be longer than we'd
  prefer when it comes to avoiding re-use. This could be mitigated by a 
  challenge-response nonce based flow using the audience field as we do with
  other join methods.
- JWT SVIDs are less common and may not be supported by all SPIFFE
  implementations.

Instead, we'll leverage a combination. The joining node will not use the X.509
certificate as a client certificate, but will instead present it as part of
the join payload, with the server returning a challenge nonce. The client will
sign the nonce with the private key corresponding to the X.509 certificate and
return it to the server. The server will then verify the signature using the
public key from the X.509 certificate.

```protobuf

```

The join method will be configured using the ProvisionTokenV2 resource:

```protobuf

```

## UX

### Federation

The `SPIFFEFederation` resource will be configurable using `tctl`, Terraform and
the Kubernetes operator.

Example configuration:

```yaml
kind: spiffe_federation
version: v1
metadata:
  # name must match the name of the trust domain you are federating with.
  name: example.com
spec:
  bundle_source:
    https_web:
      bundle_endpoint_url: https://example.com/webapi/spiffe/bundle.json
```

### Joining

Like other join methods, the SPIFFE join method will be configurable using 
the ProvisionTokenV2 resource. This will be configured using `tctl`, Terraform
or the Kubernetes Operator.

Example configuration:

```yaml

```

## Security Considerations

### Audit Events

The following audit events will be added or modified:

- `bot.join` and `agent.join` will be leveraged by SPIFFE joining.
- `spiffe.federation.create` upon creation of a `SPIFFEFederation` resource.
- `spiffe.federation.update` upon update of a `SPIFFEFederation` resource.
- `spiffe.federation.delete` upon deletion of a `SPIFFEFederation` resource.
- `spiffe.federation.rotation` upon the trust bundle returned by the Bundle
  Endpoint changing.

### Interception of Federation Trust Bundle Sync

One of the greatest risks of SPIFFE Federation is the interception of the
federation bundle fetch. If an attacker is able to instead return an 
illegitimate bundle, they could issue identities that would be trusted by
workloads in the trust domain.

This is primarily mitigated by requiring that the Bundle Endpoint is protected
by TLS. In the case of the `https_web` profile, the certificate must be issued
by a CA trusted by Teleport (typically Web PKI). In the case of the self-serving
`https_spiffe` profile, the certificate must be issued by the trust domain
itself.

Additionally, this is mitigated by the inclusion of the
`spiffe.federation.rotation` audit event. This event will be output whenever
Teleport detects the trust bundle has changed, and will allow operators to
investigate the change.

In future versions, we can improve the `https_web` profile by allowing
pinning of the certificate used to protect the Bundle Endpoint. This would
require manual intervention if the web PKI certificate is rotated, but
mitigates the risk of an attacker illegitimately obtaining a valid certificate
for the Bundle Endpoint.

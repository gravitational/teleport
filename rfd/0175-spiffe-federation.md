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

SPIFFE Federation is a standardized mechanism for exchanging trust bundles 
between trust domains. This enables workloads in one trust domain to validate
the identity of workloads in another trust domain. See
https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md
for the SPIFFE Federation specification. This will be referred to as
"the specification" throughout this document.

This feature will be gated behind a Teleport Enterprise license.

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
message SPIFFEFederationStatus {
  // The most recently fetched bundle from the federated trust domain.
  string current_bundle = 1;
  // The time that the most recently fetched bundle was obtained.
  google.protobuf.Timestamp current_bundle_synced_at = 2;
  // The duration that the current bundle suggests the next bundle should be 
  // refresh after.
  google.protobuf.Duration current_bundle_refresh_hint = 3;
}
```

The `SPIFFEFederation` resource will be accessed via the standard RFD153
gRPC CRUD APIs, and will be guarded by RBAC. We will grant the verbs to view,
modify and create `SPIFFEFederation` resources to the preset `editor` role.

For security reasons, we will guard the `status` field of the `SPIFFEFederation`
resource from modification by the user. This field will be set by the Teleport
Auth Server and there is no need for the user to modify it.

#### Syncing Trust Bundles.

A background task will be introduced to the Teleport Auth Server:
`sync-spiffe-federation`. This will run on an elected Teleport Auth Server,
and periodically fetch the trust bundle from the configured Bundle Endpoint.
This will be written into `status.current_bundle`.

The `current_bundle_refresh_hint` and `current_bundle_synced_at` fields will be
used to determine when the next fetch should occur. If the `current_bundle` is
older than the `current_bundle_refresh_hint` from the `current_bundle_synced_at`,
a new fetch will be initiated.  If unspecified, it will be assumed that the
bundle should be refreshed every 5 minutes (as per 
https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md#41-adding-and-removing-keys).

In addition, this task will watch the `SPIFFEFederation` resources and
immediately fetch the trust bundle when a new resource is created or updated.

#### `tbot`

Some minor changes will need to be made to `tbot` to support federation.

When the SPIFFE service starts, it will additionally need to fetch and watch
the `SPIFFEFederation` resources. When a resource is updated, added or deleted,
the service will need to update its trust bundle cache and distribute the
updated trust bundles to any subscribed workloads.

### SPIFFE Joining

SPIFFE joining will be implemented as a new join method.

SPIFFE defines two forms of SVID that are issued to workloads:

- X.509 SVIDs
- JWT SVIDs

One option would be to validate the identity of the joining node by requiring
that it offer the X.509 SVID as a client certificate. Unfortunately, this
has the following problems:

- Building the ability to validate X.509 certificates issued by arbitrary
  CAs into our TLS servers is complex and error-prone. We risk a compromised
  CA being able to issue user certificates that would be trusted by Teleport.
- Traversing TLS-terminating load balancers is challenging.
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

Instead, we'll use a third option. The joining node will not use the X.509
certificate as a client certificate, but will instead present it as part of
the join payload, with the server returning a challenge nonce. The client will
sign the nonce with the private key corresponding to the X.509 certificate and
return it to the server. The server will then verify the signature using the
public key from the X.509 certificate.

The join will flow as follows:

1. The Client opens a BiDi stream to the Auth Server and submits the initial
   join request, including the name of the Join Token they wish to use and the
   X.509 SVID.
2. The Server sends the Client a challenge nonce. This will be at least 32 bytes
   of random data. We do not validate the Join Token exists at this point as to
   not leak information about the existence of the Join Token.
3. The Client signs the nonce with the private key corresponding to the X.509
   SVID. It sends this signature in a message back to the server.
4. The Server now has all the information needed to validate the join:
   a. Verifies the signature, and basic fields of the X.509 SVID (e.g expiry).
   b. Finds the specified join token
   c. Searches the configured SPIFFEFederations for a trust bundle that matches
   the trust domain of the SPIFFE ID within X.509 SVID.
   d. Verifies the X.509 SVID's signature against the trust bundle.
   e. Verifies that the SPIFFE ID contained with the SVID matches one of the
      allow rules.
5. The Server signs the user or host certificate and returns it to the Client.

The use of a BiDi gRPC stream allows the server to keep the expected nonce in-
memory during the join process.

```protobuf
service JoinService {
  // .. Existing Methods ..
  // RegisterUsingSPIFFEMethod is used to register a Bot or Agent using a SPIFFE
  // SVID.
  rpc RegisterUsingSPIFFEMethod(stream RegisterUsingSPIFFEMethodRequest) returns (stream RegisterUsingSPIFFEMethodResponse);
}

// The initial information sent from the client to the server.
message RegisterUsingSPIFFEMethodInitialRequest {
  // Holds the registration parameters shared by all join methods.
  types.RegisterUsingTokenRequest join_request = 1;
  // The X509 SVID encoded in ASN.1 DER format.
  bytes x509_svid = 2;
}

// Payload of RegisterUsingSPIFFEMethodRequest, which is the solution to the
// challenge provided by the server.
message RegisterUsingSPIFFEMethodChallengeSolution {
  // The signature of the challenge nonce using the private key corresponding to
  // the X.509 SVID.
  bytes signed_nonce = 1;
}

message RegisterUsingSPIFFEMethodRequest {
  oneof payload {
    // Initial information sent from the client to the server.
    RegisterUsingSPIFFEMethodInitialRequest init = 1;
    // The challenge response required to complete the SPIFFE join process.
    // This is sent in response to the servers challenge.
    RegisterUsingSPIFFEMethodChallengeSolution challenge_solution = 2;
  }
}

// The RegisterUsingSPIFFEMethodResponse payload used by the server to issue the
// challenge nonce to the client.
message RegisterUsingSPIFFEMethodChallenge{
  // The nonce that the client must sign with the private key corresponding to
  // the X.509 SVID.
  bytes nonce = 1;
}

message RegisterUsingSPIFFEMethodResponse {
  oneof payload {
    // The challenge required to complete the SPIFFE join process. This is sent
    // to the client in response to the initial request.
    RegisterUsingSPIFFEMethodChallenge challenge = 1;
    // The signed certificates resulting from the join process.
    Certs certs = 2;
  }
}
```

The join method will be configured using the ProvisionTokenV2 resource:

```protobuf
syntax = "proto3";

message ProvisionTokenSpecV2SPIFFE{
  // Rule is a set of properties the SPIFFE SVID must have to be allowed to use
  // this ProvisionToken
  message Rule {
    // SPIFFEID matches against the full SPIFFE ID of the joining client's SVID.
    // It should be prefixed with spiffe:// and glob-like patterns are 
    // supported in the path element.
    // Example: spiffe://example.com/foo/*
    string SPIFFEID = 1;
  }

  // Allow is a list of Rules, clients using this token must match one
  // allow rule to use this token.
  repeated Rule Allow = 1;
}
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

Once configured, the administrator can delete the resource to stop federating.

### Joining

Like other join methods, the SPIFFE join method will be configurable using 
the ProvisionTokenV2 resource. This will be configured using `tctl`, Terraform
or the Kubernetes Operator.

Example configuration:

```yaml
kind: token
version: v2
metadata:
  name: spiffe-join-token
spec:
  roles: [Bot]
  bot_name: my-bot
  join_method: spiffe
  spiffe:
    allow:
      - spiffe_id: spiffe://example.com/foo/*
      - spiffe_id: spiffe://second.example.com/bar/foo/fizz
```

## Security Considerations

### Audit Events

The following audit events will be added or modified:

- `bot.join` and `agent.join` will be leveraged by SPIFFE joining.
  - The Subject, SANs, Serial and Issuer of the X.509 SVID will be included in
    the audit event.
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
by a CA trusted by Teleport (typically Web PKI).

In the case of the self-serving `https_spiffe` profile, the certificate must be
issued by the trust domain itself. The user must configure out of band the
initial value of the trust bundle, and from this point onwards the fetched
trust bundles will be used to validate future fetches. This allows for rotation
of the trust domain CA. This profile reduces reliance on the security of Web
PKI, but requires manual out-of-band steps during initial configuration.

Additionally, this is mitigated by the inclusion of the
`spiffe.federation.rotation` audit event. This event will be output whenever
Teleport detects the trust bundle has changed, and will allow operators to
investigate the change.

In future versions, we can improve the `https_web` profile by allowing
pinning of the certificate used to protect the Bundle Endpoint. This would
require manual intervention if the web PKI certificate is rotated, but
mitigates the risk of an attacker illegitimately obtaining a valid certificate
for the Bundle Endpoint.

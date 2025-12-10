---
authors: Alan Parra (alan.parra@goteleport.com)
state: draft
---

# RFD 0237 - Sub CA support

## Required Approvers

* Engineering: @rosstimothy && @espadolini && @greedy52 && @nklaassen
* Security: (@rob-picard-teleport || @klizhentas)
* Product: @klizhentas

## What

Add support for Teleport CAs to operate as a sub CA of an external root.

While the feature design is generic, support is (initially) only available for a
subset of Teleport CAs that are commonly visible to external trusted root
configurations: namely the "DB client" and "Desktop Access" CAs.

Sub CA support is an Enterprise feature.

## Why

Teleport operates multiple distinct CAs, each used to mint certificates for its
supported protocols. Historically those CAs all operate as self-signed roots, a
simple and effective solution.

As Teleport's customer base grows it becomes more common to encounter
corporations that want a larger degree of control over those CAs. The sub CA
feature addresses internal security policies and allows organizations to chain
the more "visible" Teleport CAs to their own self-managed roots.

## UX

<a id="ux1"></a>
### Alice configures "db-client" as a sub CA

1. First, Alice issues CSRs for the desired CA / cluster.

    ```shell
    $ tctl auth sub-ca create-csr --type=db-client
    > -----BEGIN CERTIFICATE REQUEST-----
    > ...
    > -----END CERTIFICATE REQUEST-----
    ```

    Note: if HSMs are configured then `tctl auth sub-ca create-csr` must be
    executed locally on each Auth server.

1. Alice sends the CSRs to the parent CA, acquiring the certificates as a
   result.

1. Alice configures the databases to trust the parent CA, if not already
   trusted.

1. Finally, Alice writes the certificates back to Teleport. The new certificates
   take effect immediately.

    `tctl auth sub-ca create-override --type=db-client cert.pem [chain.pem...]`

### Alice configures "windows" as a sub CA

Alice begins with the following command, then [continues as the first
example](#ux1).

`tctl auth sub-ca create-csr --type=windows

### Alice customizes the sub CA Subject

Alice begins with the following command, then [continues as the first
example](#ux1).

```shell
tctl auth sub-ca create-csr --type=db-client \
    --subject 'O=mycluster,OU=Llama Unit,CN=Llama Teleport DB client CA'
```

The Subject string is an RFC 2253-like string. The specifics will be documented
in public docs.

### Alice configures the cert_authority_override resource directly

Alice creates and signs the CSR, then does:

```shell
$ tctl create <<EOF
type: cert_authority_override
version: v1
metadata:
  name: db-client # matches CA name
spec:
  certificate_overrides:
  - certificate: |-
      ----- BEGIN CERTIFICATE -----
      (...)
      ----- END CERTIFICATE -----
    chain:
    - |-
      ----- BEGIN CERTIFICATE -----
      (...)
      ----- END CERTIFICATE -----
    # Informative, must match certificate if present.
    public_key: 8D:53:F4:BF:54:63:B9:B6:C8:C1:84:A4:08:5B:B1:F1:07:67:96:DF
    # Informative, must match certificate if present.
    cluster_name: zarquon2
EOF
```

### Alice disables the "db-client" override

Disabling an override makes it inactive, falling back to the corresponding
Teleport self-signed certificate, but retains the configuration. Disables take
effect immediately.

A disabled override is still considered during CA rotation.

`tctl auth sub-ca disable-override --type=db-client

### Alice deletes the "db-client" override

Deleting an override removes it completely, making Teleport use its self-signed
certificate. Deletes take effect immediately.

`tctl auth sub-ca delete-override --type=db-client

### Alice performs a "db-client" CA rotation

1. Alice starts a rotation and attempts to advance to the `update_clients` step:

    ```shell
    $ tctl auth rotate --manual --type=db-client --phase=init
    > Updated rotation phase to "init". To check status use 'tctl status'
    >
    > There are active overrides for CA "db-client". You must either supply an
    > override for public key "AB:CD:EF:..." or disable the override.
    >
    > tctl auth sub-ca create-csr --type=db-client --public-key='AB:CD:EF:...'
    > or
    > tctl auth sub-ca disable-override --type=db-client --public-key='AB:CD:EF:...'

    $ tctl auth rotate --manual --type=db-client --phase=update_clients
    > ERROR: Found CA overrides for authority "db-client". You must either
    > supply an override for public key "AB:CD:EF:..." or disable the override.
    >
    > tctl auth sub-ca create-csr --type=db-client --public-key='AB:CD:EF:...'
    > or
    > tctl auth sub-ca disable-override --type=db-client --public-key='AB:CD:EF:...'
    ```

    Note: the interactive rotation wizard will print similar messages to above.
    Users must perform the commands in a separate shell and then acknowledge the
    manual steps, as usual.

1. Alice updates the CA override for "db-client":

    ```shell
    $ tctl auth sub-ca create-csr --type=db-client --public-key='AB:CD:EF:...'
    > (CSR PEM)

    # Alice issues certificate from CSR.

    tctl auth sub-ca create-override --type=db-client cert.pem [chain.pem...]
    ```

1. Alice advances the rotation to the `update_clients` step:

    ```shell
    $ tctl auth rotate --manual --type=db-client --phase=update_clients`
    # OK, all overrides are addressed.
    ```

## Details

The design works similarly to [RFD 0194 - SPIFFE issuer override][rfd0194]: we
retain the Teleport generated private key but "override" the self-signed
certificate with an externally-signed one. Retaining the private key and
overriding only the self-signed certificate simplifies the implementation and
allows overrides to take effect without a CA rotation.

Unlike RFD 0194, the override is conceptually generic (it may apply to any
Teleport CA) and works at a more fundamental system layer. For the initial
release only the "db-client" and "windows" CAs may be targeted, but support
could be seamlessly expanded in the future. It could even be applied to the
SPIFFE TLS CA, replacing a workload_identity_x509_issuer_override.

Overrides for client facing CAs, like "db-client" and "windows", take effect
immediately. The customer holds all necessary certificates before creating the
override, so they may take the necessary steps (like updating trusted roots).

[rfd0194]: https://github.com/gravitational/teleport/blob/master/rfd/0194-spiffe-x509-override.md

### The cert_authority_override resource

A new Terraform-friendly resource is added to configure CA overrides. The new
resource avoids direct changes to cert_authority, a sensitive Teleport-managed
resource.

Creating a new override automatically generates the corresponding empty CRL.

```proto
package teleport.subca.v1;

message CertAuthorityOverride {
  // Kind is "cert_authority_override"
  string kind = 1;
  // Sub kind not supported.
  string sub_kind = 2;
  // Version is "v1".
  string version = 3;
  // Metadata for the resource.
  teleport.header.v1.Metadata metadata = 4;
  // Spec for the resource.
  CertAuthorityOverrideSpec spec = 5;
  // Dynamic state for the resource.
  CertAuthorityOverrideStatus status = 7;
}

message CertAuthorityOverrideSpec {
  repeated CertificateOverride certificate_overrides = 1;
}

message CertificateOverride {
  // SHA256 of the certificate's DER-encoded SubjectPublicKeyInfo
  // (aka RawSubjectPublicKeyInfo).
  // Informative if certificate is present.
  string public_key = 1;

  // Informative. Cluster of the CA.
  string cluster_name = 2;

  // Certificate to present, in PEM form.
  //
  // The public key must match an existing (CA,cluster) pair.
  // It must also match the public_key field, if present.
  //
  // The Subject's "O=" field must match the CA cluster.
  // It must also match the cluster_name field, if present.
  string certificate = 3;

  // Certificate chain, in PEM form.
  //
  // The chain must be sorted from leaf to root.
  //
  // If present Teleport may supply the chain along with the certificate in
  // appropriate situations.
  repeated string chain = 4;

  // TBD.
  // bool exclude_sub_ca_from_client_chains = n;

  // If true disables the override.
  // A disabled override may exist simply to mark a certain public key as not
  // overridden. In this case the certificate may be absent.
  bool disabled = 5;
}

message CertAuthorityOverrideStatus {
  map<string, CertificateRevocationList> public_key_to_crls = 1;
}

message CertificateRevocationList {
  string pem = 1;
}
```

The following changes are done to [TLSKeyPair](
https://github.com/gravitational/teleport/blob/d7b212d617003992fab4420f87fbdb0b63c761cb/api/proto/teleport/legacy/types/types.proto#L1288)
(a component of CertAuthority) in order to supply new override information:

```diff
 package types; // api/proto/teleport/legacy/types/

 message TLSKeyPair {
   bytes Cert = 1;
   bytes Key = 2;
   PrivateKeyType KeyType = 3;
   bytes CRL = 4;
+
+  // If true a certificate override (via cert_authority_override) is active.
+  // "Cert" and "CRL" and replaced by the override.
+  bool CertOverrideActive = 5;
+
+  // Certificate trust chain, in PEM form.
+  //
+  // Sorted from leaf to root.
+  //
+  // Absent for self-signed certificates, but may be present if a cert override
+  // is active.
+  repeated X509Certificate TrustChain = 6;
 }
+
+message X509Certificate {
+  string pem = 1;
+}
```

Existing certificate generation RPCs are also modified to carry a certificate
chain. The Teleport Sub CA is presented as part of the chain, allowing
downstream systems to only know about/trust the external root CA.

The exception to the above is Windows PKI / smart card authentication, which
requires the issuing CA to be directly known by the NTAuth store (see
[Guidelines for smart card logon with third-party CAs][windows-smartcard-logon],
_"must be issued from a CA that is in the NTAuth store"_). Therefore, the chain
is not provided for the "windows" CA, as the Sub CA must be known. (Note: a
similar restriction likely applies to DB access for MSSQL Server with PKINIT
authn - [public docs][mssql-pub] and [sources][mssql-sources].)

```diff
 package proto // api/proto/teleport/legacy/client/proto

 message DatabaseCertResponse {
   bytes Cert = 1;
   repeated bytes CACerts = 2;
+
+  // Certificate trust chain, in PEM form.
+  //
+  // Sorted from leaf to root.
+  //
+  // If present should be presented along with Cert to form its trust chain.
+  repeated types.X509Certificate TrustChain = 3;
 }
```

[windows-smartcard-logon]: https://learn.microsoft.com/en-us/troubleshoot/windows-server/certificates-and-public-key-infrastructure-pki/enabling-smart-card-logon-third-party-certification-authorities
[mssql-pub]: https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/sql-server-ad-pkinit/
[mssql-sources]: https://github.com/gravitational/teleport/blob/99843ebb0ed5ddc4f4e9e34d1cb23e008afac0f8/lib/auth/db.go#L160

### Subject customization and CA restrictions

Teleport makes a single demand of CA certificates: the "O=" field [must contain
the cluster name](
https://github.com/gravitational/teleport/blob/d7b212d617003992fab4420f87fbdb0b63c761cb/lib/tlsca/parsegen.go#L41-L47).

If customization of the O= field is desired then cluster name is recorded using
OID "1.3.9999.4.1". If any Subject customization is at play then the system
favors the alternate OID to the O= field.

In an attempt to make CA certificate requirements clearer and more maintainable
in code, the tlsca.ClusterName() function is to be removed and replaced by the
following:

```go
package tlsca // lib/tlsca

func CAInfoFromSubject(subject pkix.Name) (*CAInfo, error) {
	// (...)
}

type CAInfo struct {
	ClusterName string
}
```

Managed Subjects may have, at the system's discretion, the certificate serial
number added to their Subject. Customized Subjects are not changed in this
regard.

### The "windows" CA

The "windows" CA is used to issue per-user RDP (Remote Desktop Protocol)
certificates for Desktop Access. It does not exist in Teleport as a CA on its
own, and instead is backed by the "tls-user" CA. It appears, sometimes with
special treatment, in endpoints like `/webapi/auth/export?type=windows` and
commands like `tctl auth export --type=windows`.

The "windows" CA is to be lifted to a proper CA, separate from "tls-user". The
details of the split are considered out of scope for this RFD and are tracked
outside of it.

The CA split is a prerequisite for the Sub CA support feature.

TODO(codingllama): Link to PR and/or RFD for the Windows CA split.

### CA rotation

CA rotations will take into account existing certificate overrides, forbidding
advances from "init" to "update_clients" if a new or active private key for an
otherwise overridden (CA,cluster) pair lacks an override.

A CA rotation may be triggered if there is a desire for the overridden
certificates to be backed by a distinct private key. One has simply to create
the necessary overrides during the "init" phase of the rotation.

(CA,cluster) pairs without existing overrides are unaffected.

### SPIFFE issuer overrides

SPIFFE issuer overrides ([RFD 0194][rfd0194]) could be represented as a
cert_authority_override for the "spiffe-tls" virtual CA.

Unifying the features is considered a stretch goal.

### Auth service changes

A new Auth-bound gRPC service is responsible for managing
cert_authority_override resources and Sub CA operations:

```proto
package teleport.subca.v1;

service SubCAService {
  rpc CreateCSR(CreateCSRRequest) returns (CreateCSRResponse);

  // Implementation note: used by `tctl create` and Terraform.
  rpc UpsertCertAuthorityOverride(UpsertCertAuthorityOverrideRequest)
    returns (UpsertCertAuthorityOverrideResponse);

  // Implementation note: used by `tctl auth sub-ca
  // create-override|disable-override`.
  rpc AddCertificateOverride(AddCertificateOverrideRequest)
    returns (AddCertificateOverrideResponse);

  // Implementation note: used by `tctl auth sub-ca delete-override`.
  rpc RemoveCertificateOverride(RemoveCertificateOverrideRequest)
    returns (RemoveCertificateOverrideResponse);

  rpc GetCertAuthorityOverride(GetCertAuthorityOverrideRequest)
    returns (GetCertAuthorityOverrideResponse);
  rpc ListCertAuthorityOverride(ListCertAuthorityOverrideRequest)
    returns (ListCertAuthorityOverrideResponse);
  rpc DeleteCertAuthorityOverride(DeleteCertAuthorityOverrideRequest)
    returns (DeleteCertAuthorityOverrideResponse);
}

message CreateCSRRequest {
  // CA type per api/types.CertAuthType.
  // Required.
  string ca_type = 1;

  // Optional. Targets all clusters if empty.
  string cluster_name = 2;

  // Targets a (CA,cluster) pair by its public key.
  // Optional.
  string public_key = 3;

  // Customized certificate Subject.
  // Eg: `O=mycluster,OU=Llama Unit,CN=Llama Teleport DB client CA`.
  //
  // Teleport CA Subject restrictions still apply. The system may modify the
  // Subject or reject the request if restrictions cannot be fulfilled.
  //
  // Optional. If present the request must target a single cluster.
  DistinguishedName custom_subject = 4;
}

message DistinguishedName {
  repeated AttributeTypeAndValue names = 1;
}

message AttributeTypeAndValue {
  repeated int oid = 1;
  // Note: Go only allows strings as the value for a pkix.AttributeTypeAndValue.
  // See
  // https://cs.opensource.google/go/go/+/refs/tags/go1.25.5:src/crypto/x509/pkix/pkix.go;l=152.
  string value = 2;
}

message CreateCSRResponse {
  repeated CertificateSigningRequest csrs = 1;
}

message CertificateSigningRequest {
  // CSR in PEM form.
  string pem = 1;
}

message AddCertificateOverrideRequest {
  CertificateOverrideTarget target = 1;

  // Value to add or modify.
  // Patches are always additive.
  CertificateOverride certificate_override = 2;
}

message AddCertificateOverrideResponse {
  CertificateOverride certificate_override = 1;
}

message RemoveCertificateOverrideRequest {
  // Certificate override to delete.
  CertificateOverrideTarget target = 1;
}

message RemoveCertificateOverrideResponse {}

message CertificateOverrideTarget {
  // CA type per api/types.CertAuthType.
  // Required.
  string ca_type = 1;

  // Targets a (CA,cluster) override by name.
  string cluster_name = 2;

  // Targets a (CA,cluster) override by its public key.
  string public_key = 3;
}

// Upsert/Get/List/Delete requests/responses per RFD 0153.
// Upsert is unmasked.
// List is paginated.
```

The storage key space for cert_authority_override resources is
`/cert_authority_overrides/{ca_type}`.

### Cache and event stream

The new cert_authority_override resource is both cached and supported by event
streams. Streaming events to cert_authority_override resources invalidate the
cache for the corresponding cert_authority.

## Security

The external root must be properly managed by customer, as it is outside of
Teleport's scope.

The introduction of Sub CAs doesn't change the security properties or trust
relationships within Teleport, particularly for Sub CAs of "client" CAs.

## Backward Compatibility

Rollbacks of clusters with active cert_authority_override resources, to versions
that predate the introduction of such resources, will cause Teleport to suddenly
start minting certificates with its old self-signed certificates. That is likely
to cause problems in downstream systems that do not trust Teleport's self-signed
roots.

A possible mitigation is to backport a minimal knowledge of
cert_authority_override to the N-1 version such that it, at least, warns the
user on startup. Whether this is an acceptable or desirable mitigation is TBD.

## Audit Events

New audit events are added to track the cert_authority_override life cycle. Only
successful interactions are written to audit.

```proto
package events; // api/proto/teleport/legacy/types/events

message CertAuthorityOverrideEvent {
  Metadata metadata = 1;
  UserMetadata user = 2;
  ResourceMetadata resource = 3; // Note: name=ca_type
  CertAuthorityOverrideMetadata cert_authority_override = 4;
}

message CertAuthorityOverrideMetadata {
  repeated CertAuthorityCertificateOverrideMetadata certificate_overrides = 1;
}

message CertAuthorityCertificateOverrideMetadata {
  CertificateOverrideMetadata certificate = 1;
  repeated CertificateOverrideMetadata chain = 2;
  string cluster_name = 3;
  bool disabled = 4;
  // Note: entry delete tracked by code.
}

message CertificateOverrideMetadata {
  string issuer = 1;
  string subject = 2;
  string serial_number = 3;
  string public_key = 4;
}
```

Event types:

* `cert_auth_override.upsert`
* `cert_auth_override.delete`

Codes:

* `TCO01I` - CertAuthorityOverrideUpsertCode
* `TCO02I` - CertAuthorityOverrideCertificatesUpsertCode
  * Special case of upsert: added/updated certificate
* `TCO03I` - CertAuthorityOverrideCertificatesDeleteCode
  * Special case of upsert: deleted certificate
* `TCO04I` - CertAuthorityOverrideDeleteCode

## Observability

Automated metrics are used around the new RPCs.

A custom metric is added to track the latency impact of fetching and calculating
certificate overrides.

* `teleport_ca_certificate_override_latency_seconds` -
  histogram -
  {ca_type string, num_overrides int}

## Test Plan

Sub CA support is added to the test plan.

**Sub CAs**

Features must be tested and functional before and after the override, ie, it's
possible to connect to DBs, Desktops, etc.

<!--
TODO: Include brief instructions on how to create a self-signed external CA and
how to mint certificates from the CSR.
-->

- [ ] Create an override for the "db-client" CA
  - [ ] "db-client" override works even if the database only knows about the
        external root CA (ie, clients correctly pass the TLS chain)
- [ ] Create an override for the "windows" CA
  - [ ] Verify that overriding the "windows" CA does not affect the "tls-user" CA
- [ ] Verify overridden CA certificates using `tctl auth export`
- [ ] Perform a CA rotation, reconfigure trust roots if necessary, and re-verify
     access.
- [ ] Exercise tctl commands, verify that audit events are issued
  - [ ] `tctl auth sub-ca create-override`
  - [ ] `tctl auth sub-ca disable-override`
  - [ ] `tctl auth sub-ca delete-override`
  - [ ] `tctl create` (kind:cert_authority_override)
  - [ ] `tctl edit`   (kind:cert_authority_override)
  - [ ] `tctl delete` (kind:cert_authority_override)
  - [ ] `tctl get`    (kind:cert_authority_override, doesn't write to audit)

## Alternatives considered

### Custom CSR payloads

The design offers only Subject customization via the `tctl auth sub-ca
create-csr`, as that is understood to be sufficient. A CSR signing command could
be provided to offer a higher degree customization:

`tctl auth sub-ca sign-csr --type=db-client cert-request.pem`

The sign-csr command validates the request, similarly to the creation/update of
a cert_authority_override resource, ensuring it fulfils the requirements of a
Teleport Sub CA certificate.

### Internal CA overrides

Overrides for internal CAs, meaning those that need to be trusted by Teleport
services themselves, are presently not supported.

An override for such a CA requires the distribution of the new certificate to
all relevant Teleport processes. An update to an internal CA could either
require a full rotation to be effective (noted in the "status" of the
cert_authority_override resource), or a "light" rotation could be attempted via
[CertAuthority.SetRotation](
https://github.com/gravitational/teleport/blob/d7b212d617003992fab4420f87fbdb0b63c761cb/api/types/authority.go#L72-L73).

### PKCS#11 interface

A PKCS#11 interface, paired with configuration, could be used to implement
automated issuance of overridden certificates. An Auth node with adequate access
can mint the certificates and create/update the corresponding
cert_authority_override resource as needed.

tbot could be used as the integration point, monitoring CA rotation and
interacting with a local script to issue new certificates.

PKCS#11, while convenient, is not desirable when interacting with offline CAs,
and is therefore considered out of scope.

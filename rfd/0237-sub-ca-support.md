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

All existing and future Teleport CAs will be overridable. Support will be added
one CA at a time due to the number of CAs. The first supported CAs will be the
subset of Teleport CAs that are commonly visible to external trusted root
configurations: namely the "DB client" and "Windows Desktop Access" CAs. After
that the SPIFFE CA will be next, followed by all other CAs.

Sub CA support is an Enterprise feature.

## Why

Teleport operates multiple distinct CAs, each used to mint certificates for its
supported protocols. Historically those CAs all operate as self-signed roots, a
simple and effective solution.

As Teleport's customer base grows it becomes more common to encounter
corporations that want a larger degree of control over those CAs. The sub CA
feature addresses internal security policies and allows organizations to chain
Teleport CAs to their own self-managed roots.

## UX

### Auth server prerequisites

All Auth servers should be upgraded to a version that supports Sub CAs /
certificate overrides before any override is created.

It's recommended to backup the cluster state database, prior to creating an
override, in case a Teleport downgrade is ever required.

<a id="ux1"></a>
### Alice configures "db_client" as a sub CA

1. First, Alice issues CSRs for the desired CA

    ```shell
    $ tctl auth create-override-csr --type=db_client
    > (Writes "db_client-${public_key}.pem".)
    ```

    Note: if HSMs are configured then `tctl auth create-override-csr` must be
    executed locally on each Auth server.

1. Alice sends the CSRs to the parent CA, acquiring the certificates as a
   result.

1. Alice configures the databases to trust the parent CA, if not already
   trusted.

1. Alice writes the certificates back to Teleport. The new certificates take
   effect immediately (certificates issued from this moment onwards are chained
   to the overridden certificate).

    `tctl auth create-override --type=db_client cert.pem [chain.pem...]`

1. Alice is now free to remove trust from the old self-signed certificate from
   the databases.

### Alice configures "windows" as a sub CA

Alice begins with the following command, then [continues as the first
example](#ux1).

`tctl auth create-override-csr --type=windows`

### Alice customizes the sub CA Subject

Alice begins with the following command, then [continues as the first
example](#ux1).

```shell
tctl auth create-override-csr --type=db_client \
    --subject='O=mycluster,OU=Llama Unit,CN=Llama Teleport DB client CA'
```

The Subject string is an RFC 2253-like string. The specifics will be documented
in public docs.

### Alice configures the cert_authority_override resource directly

Alice creates and signs the CSR, then does:

```shell
$ tctl create <<EOF
kind: cert_authority_override
sub_kind: db_client
version: v1
metadata:
  name: zarquon # cluster name
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
EOF
```

### Alice configures a SPIFFE TLS override

Alice begins with the following command, then [continues as the first
example](#ux1).

`tctl auth create-override-csr --type=spiffe-tls`

### Alice disables the "db_client" override

Disabling an override makes it inactive, falling back to the corresponding
Teleport self-signed certificate, but retains the configuration. Disables take
effect immediately.

`tctl auth update-override --set-disabled=true --type=db_client`

Disables are only allowed for keys in the [AdditionalTrustedKeys set](
https://github.com/gravitational/teleport/blob/3121f066a27a4c24cb330452416a7261147eb2fa/api/proto/teleport/legacy/types/types.proto#L1398),
meaning it can only happen for new keys during the "init" CA rotation phase. A
disable may be forced via the `--force` flag.

### Alice deletes the "db_client" override

Deleting an override removes it completely, making Teleport use its self-signed
certificate. Deletes take effect immediately.

`tctl auth delete-override --type=db_client`

Deletes are only allowed for keys absent from the CA key sets, as a fallback. A
delete may be forced with the `--force` flag.

### Alice performs a "db_client" CA rotation

1. Alice starts a rotation and attempts to advance to the `update_clients` step:

    ```shell
    $ tctl auth rotate --manual --type=db_client --phase=init
    > Updated rotation phase to "init". To check status use 'tctl status'
    >
    > There are active overrides for CA "db_client". You must either supply an
    > override for public key "AB:CD:EF:..." or disable the override.
    >
    > tctl auth create-override-csr --type=db_client --public-key='AB:CD:EF:...'
    > tctl auth create-override --type=db_client cert.pem
    > or
    > tctl auth create-override --set-disabled=true --type=db_client --public-key='AB:CD:EF:...'

    $ tctl auth rotate --manual --type=db_client --phase=update_clients
    > ERROR: Found CA overrides for authority "db_client". You must either
    > supply an override for public key "AB:CD:EF:..." or disable the override.
    >
    > tctl auth create-override-csr --type=db_client --public-key='AB:CD:EF:...'
    > tctl auth create-override --type=db_client cert.pem
    > or
    > tctl auth create-override --set-disabled=true --type=db_client --public-key='AB:CD:EF:...'
    ```

    Note: the interactive rotation wizard will print similar messages to above.
    Users must perform the commands in a separate shell and then acknowledge the
    manual steps, as usual.

1. Alice updates the CA override for "db_client":

    ```shell
    $ tctl auth create-override-csr --type=db_client --public-key='AB:CD:EF:...'
    > (Writes "db_client-${public_key}.pem".)

    # Alice issues certificate from CSR.

    tctl auth create-override --type=db_client cert.pem [chain.pem...]
    ```

1. Alice advances the rotation to the `update_clients` step:

    ```shell
    $ tctl auth rotate --manual --type=db_client --phase=update_clients
    # OK, all overrides are addressed.
    ```

    Once a private key is removed from a CA, Teleport will also remove its
    corresponding overrides.

#### Exemplified key/certificate life cycle during rotation

* K*n* = private keys
* C*n* = self-signed certificates
* O*n* = overridden certificates

Before rotation and before override:
* K1, C1 -> (K1,C1) used to mint certificates

Before rotation, override created:
* K1, C1, O1 -> (K1,O1) used to mint certificates

Rotation phase=init:
* K1, C1, O1 -> (K1,O1) used to mint certificates
* New K2, C2 -> exists in [AdditionalTrustedKeys][], not used to mint certs

Rotation phase=init, override created:
* K1, C1, O1 -> (K1,O1) used to mint certificates
* New K2, C2 -> exists in [AdditionalTrustedKeys][], not used to mint certs

Rotation phase=update_clients:
* K1, C1, O1 -> moved to AdditionalTrustedKeys, now unused
* K2, C2, O2 -> moved to [ActiveKeys][], (K2,O2) used to mint certificates

[AdditionalTrustedKeys]: https://github.com/gravitational/teleport/blob/840b60d05896bd34ab7cc57f9527d36b32f909a5/api/proto/teleport/legacy/types/types.proto#L1395-L1398
[ActiveKeys]: https://github.com/gravitational/teleport/blob/840b60d05896bd34ab7cc57f9527d36b32f909a5/api/proto/teleport/legacy/types/types.proto#L1390-L1391

### Alice creates a disabled override, then enables it

Creating a disabled override is useful for long migration processes, where the
override is prepared long before downstream systems have their trust updated.

Note that the downstream system should trust both the self-signed Teleport CA
and the external CA, so that enabling the override won't require tight
coordination. Once the override is enabled then downstream systems could remove
trust of the self-signed Teleport CA.

A rotation may be optionally initialized prior to creating the disabled
override. The example shows how to perform the initial override along with a CA
rotation.

```shell
# 1. Start a rotation.
tctl auth rotate --type=db_client --phase=init

# 2. Create the CSR.
tctl auth create-override-csr --type=db_client \
  --subject='OU=Llama Unit,CN=Llama Teleport DB client CA'
> (Writes CSR file for NEW key.)

# 3. Sign the CSR for the NEW key using the external CA.

# 4. Create the disabled override.
tctl auth create-override \
  --type=db_client \
  --set-disabled=true cert.pem
> Created override for db_client, public key '2B:CD:EF:...'

# Time passes until downstream trust is configured for both OLD and NEW
# (overridden) certificates.

# 5. Enable the override.
tctl auth update-override
  --type=db_client \
  --public-key='2B:CD:EF:...' \
  --set-disabled=false

# 6. Advance rotation.
# NEW, overridden certificate is now used to sign client certificates.
tctl auth rotate --type=db_client --phase=update_clients
```

Using a declarative resource:

```shell
# Steps 1, 2 and 3 as above.

# 4. Create the disabled override.
cat >db_client_override.yaml <<EOF
kind: cert_authority_override
sub_kind: db_client
version: v1
metadata:
  name: zarquon # cluster name
spec:
  certificate_overrides:
  - disabled: true
    certificate: |-
      ----- BEGIN CERTIFICATE -----
      (...)
      ----- END CERTIFICATE -----
EOF
tctl create db_client_override.yaml

# Time passes.

# 5. Enable the override when ready. Takes effect immediately.
yq eval '.spec.certificate_overrides[0].disabled=false' -i db_client_override.yaml
tctl create -f db_client_override.yaml

# Step 6 as above.

# `cat` and `yq` used as an example, replace with your favorite editor.
```

## Details

The design works similarly to [RFD 0194 - SPIFFE issuer override][rfd0194]: we
retain the Teleport generated private key but "override" the self-signed
certificate with an externally-signed one. Retaining the private key and
overriding only the self-signed certificate simplifies the implementation and
allows overrides to take effect without a CA rotation.

Unlike RFD 0194, the override is conceptually generic (it may apply to any
Teleport CA) and works at a more fundamental system layer. For the initial
release only the "db_client", "windows" and "spiffe-tls" CAs may be targeted,
but support could be seamlessly expanded in the future.

Overrides for client facing CAs (the only ones supported so far) take effect
immediately. The customer holds all necessary certificates before creating the
override, so they may take the necessary steps (like updating trusted roots).

[rfd0194]: https://github.com/gravitational/teleport/blob/master/rfd/0194-spiffe-x509-override.md

<a id="ca_override_resource"></a>
### The cert_authority_override resource

A new Terraform-friendly resource is added to configure CA overrides. The new
resource avoids direct changes to cert_authority, a sensitive Teleport-managed
resource.

Certificates supplied to an override are validated to make sure they can
function as CA certificates and fulfill Teleport's requirements. This includes
[Subject requirements][#subject-customization] and that the certificate's
expiration is not after than the corresponding self-signed CA certificate
(typically valid for 10y).

Creating an override automatically generates the corresponding empty CRL.

```proto
package teleport.subca.v1;

message CertAuthorityOverride {
  // Kind is "cert_authority_override".
  string kind = 1;
  // Sub kind is the CA type.
  // Eg: "db_client", "spiffe-tls", "windows".
  string sub_kind = 2;
  // Version is "v1".
  string version = 3;
  // Metadata for the resource.
  // The name of the resource is the cluster name.
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

  // Certificate to present, in PEM form.
  //
  // The public key must match an existing CA certificate.
  // It must also match the public_key field, if present.
  //
  // The Subject's "O=" field must match the CA cluster.
  string certificate = 2;

  // Certificate chain, in PEM form.
  //
  // The chain must be sorted from leaf to root.
  //
  // If present Teleport may supply the chain along with the certificate in
  // appropriate situations.
  //
  // The chain is limited to a generous (but sensible) server-defined length.
  repeated string chain = 3;

  // TBD.
  // bool exclude_sub_ca_from_client_chains = n;

  // If true disables the override.
  // A disabled override may exist simply to mark a certain public key as not
  // overridden. In this case the certificate may be absent.
  bool disabled = 4;
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

<a id="subject-customization"></a>
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

### The "db_client" CA and JWT

The Database Access / Snowflake integration doesn't use TLS, but instead relies
on JWTs signed by the db_client CA. Trust is established via a known public key.

Sub CA overrides have no effect or impact on this integration. The exceptional
behavior is noted here for completeness only.

References:

* [Database Access with Snowflake](https://goteleport.com/docs/enroll-resources/database-access/enrollment/managed/snowflake/#step-35-export-a-public-key)
* [lib/auth.Server.GenerateSnowflakeJWT](https://github.com/gravitational/teleport/blob/fcc6b798afa5f39f9353129ee8c0dafcc3611593/lib/auth/db.go#L328-L380)

### The "windows" CA

The "windows" CA is used to issue per-user RDP (Remote Desktop Protocol)
certificates for Desktop Access. It does not exist in Teleport as a CA on its
own, and instead is backed by the "tls-user" CA. It appears, sometimes with
special treatment, in endpoints like `/webapi/auth/export?type=windows` and
commands like `tctl auth export --type=windows`.

The "windows" CA is to be lifted to a proper CA, separate from "tls-user".
See [RFD 0239 - Windows CA split][rfd0239].

The CA split is a prerequisite for the Sub CA support feature.

[rfd0239]: https://github.com/gravitational/teleport/blob/master/rfd/0239-windows-ca-split.md

### CA rotation

CA rotations will take into account existing certificate overrides, forbidding
advances from "init" to "update_clients" if a new or active private key for an
otherwise overridden (CA,cluster) pair lacks an override.

A CA rotation may be triggered if there is a desire for the overridden
certificates to be backed by a distinct private key. One has simply to create
the necessary overrides during the "init" phase of the rotation.

(CA,cluster) pairs without existing overrides are unaffected.

<a id="spiffe-override"></a>
### SPIFFE issuer overrides

SPIFFE issuer overrides are consolidated under the cert_authority_override
entity. The CA name, unlike other CAs, includes the protocol: "spiffe-tls". This
calls attention to the nature of the override and leaves design space for other
types of overrides.

[Workload overrides][rfd0194] are considered deprecated and cannot be created
anew. Existing workload overrides are transparently migrated into the
corresponding cert_authority_override entity.

Workload override commands will remain for one full release, during which they
will error and direct the user to the Sub CA feature. On release N+1 the
commands are removed from the CLI.

Workload override API endpoints, similarly, will error for a full release and be
removed on N+1.

<a id="client-agent-compat-validation"></a>
### Client/Agent compatibility validation

Client/Agent compatibility is required to apply certain override extensions,
such as the ability to provide a certificate chain.

Compatibility can be tested by making clients explicitly assert feature support
in their requests. This lets Auth fail requests from old clients/agents cleanly,
instead of replying with a response that is destined to failure (for example, a
certificate chain that is going to be ignored).

There will be no client/agent compatibility validation for the initial feature
release, as the first set of CAs do not call for such a feature:

* Databases may be configured to trust the overridden Teleport certificate, so
  there is no need to break due to lack of certificate chain support.
* Windows Desktop Access needs the immediate CA to be trusted (there is no chain
  support, as explained in the [cert_authority_override resource
  section](#ca_override_resource))
* SPIFFE already has similar controls built into the workload override
  implementation.

### Expiration alerts

The following cluster alerts are created automatically, based on the remaining
validity of the CA certificate. Works for both overrides (preferred if present)
and self-signed certificates.

* min(365d, 1/2 remaining lifetime) = low priority alert
* min(180d, 1/4 remaining lifetime) = medium priority alert
* min(90d, 1/8 remaining lifetime)  = high priority alert

For example:

| Total validity | Remaining lifetime | Alert level |
| -              | -                  | -           |
| 10y            | 365d               | low         |
| 10y            | 180d               | medium      |
| 10y            |  90d               | high        |
|  1y            | 183d               | low         |
|  1y            |  92d               | medium      |
|  1y            |  46d               | high        |

Alerts target users that have either "cert_authority:update" or "token:create"
permissions. The "token:create" permission is used as a proxy for a powerful
user, for example someone with the "editor" role (which lacks direct
cert_authority permissions).

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

  // Implementation note: used by `tctl auth create-override|update-override`.
  rpc AddCertificateOverride(AddCertificateOverrideRequest)
    returns (AddCertificateOverrideResponse);

  // Implementation note: used by `tctl auth delete-override`.
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

  // Targets a CA certificate by its public key.
  // Optional.
  string public_key = 2;

  // Customized certificate Subject.
  // Eg: `O=mycluster,OU=Llama Unit,CN=Llama Teleport DB client CA`.
  //
  // Teleport CA Subject restrictions still apply. The system may modify the
  // Subject or reject the request if restrictions cannot be fulfilled.
  //
  // Optional. If present the request must target a single certificate.
  DistinguishedName custom_subject = 3;
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
  string ca_type = 1;

  // Value to add or modify.
  // Patches are always additive.
  CertificateOverride certificate_override = 2;

  bool force_immediate_disable = 3;
}

message AddCertificateOverrideResponse {
  CertificateOverride certificate_override = 1;
}

message RemoveCertificateOverrideRequest {
  // Certificate override to delete.
  string ca_type = 1;
  string public_key = 2;

  bool force_immediate_delete = 3;
}

message RemoveCertificateOverrideResponse {}

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

See also the [client/agent compatibility validation
section](#client-agent-compat-validation).

## Audit Events

New audit events are added to track the cert_authority_override life cycle.

```proto
package events; // api/proto/teleport/legacy/types/events

message CertAuthorityOverrideEvent {
  Metadata metadata = 1;
  UserMetadata user = 2;
  ResourceMetadata resource = 3; // name=cluster name
  Status status = 4;             // Distinguishes successes and failures.
  CertAuthorityOverrideMetadata cert_authority_override = 4;
}

message CertAuthorityOverrideMetadata {
  string ca_type = 1;
  repeated CertAuthorityCertificateOverrideMetadata certificate_overrides = 2;
}

message CertAuthorityCertificateOverrideMetadata {
  CertificateOverrideMetadata certificate = 1;
  repeated CertificateOverrideMetadata chain = 2;
  bool disabled = 3;
  // Note: entry delete tracked by the event code.
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

- [ ] Create an override for the "db_client" CA
  - [ ] "db_client" override works even if the database only knows about the
        external root CA (ie, clients correctly pass the TLS chain)
- [ ] Create an override for the "windows" CA
  - [ ] Verify that overriding the "windows" CA does not affect the "tls-user" CA
- [ ] Create an override for the "spiffe-tls" CA
- [ ] (N-1 upgrade) Create SPIFFE TLS workload overrides, verify migration to
      cert_authority_override
- [ ] Create override in a leaf cluster, verify "propagation" and access via
      root cluster
- [ ] Verify overridden CA certificates using `tctl auth export`
- [ ] Perform a CA rotation, reconfigure trust roots if necessary, and re-verify
      access.
- [ ] Test plan executed against software keys
- [ ] Test plan executed against PKCS#11 HSM
- [ ] Test plan executed against AWS KMS
- [ ] Test plan executed against AWS KMS (multi-region)
- [ ] Test plan executed against GCP KMS
- [ ] Verify that expiration alerts fire appropriately
- [ ] Exercise tctl commands, verify that audit events are issued
  - [ ] `tctl auth create-override`
  - [ ] `tctl auth update-override`
  - [ ] `tctl auth delete-override`
  - [ ] `tctl auth pub-key-hash` (client-side only, doesn't write to audit)
  - [ ] `tctl create` (kind:cert_authority_override)
  - [ ] `tctl edit`   (kind:cert_authority_override)
  - [ ] `tctl delete` (kind:cert_authority_override)
  - [ ] `tctl get`    (kind:cert_authority_override, doesn't write to audit)

## Alternatives considered

### Custom CSR payloads

The design offers only Subject customization via the `tctl auth
create-override-csr`, as that is understood to be sufficient. A CSR signing
command could be provided to offer a higher degree customization:

`tctl auth sign-override-csr --type=db_client cert-request.pem`

The sign-override-csr command validates the request, similarly to the
creation/update of a cert_authority_override resource, ensuring it fulfils the
requirements of a Teleport Sub CA certificate.

### Internal CA overrides

Overrides for internal CAs, meaning those that need to be trusted by Teleport
services themselves, are presently not supported.

An override for such a CA requires the distribution of the new certificate to
all relevant Teleport processes. An update to an internal CA could either
require a full rotation to be effective (noted in the "status" of the
cert_authority_override resource), or a "light" rotation could be attempted via
[CertAuthority.SetRotation](
https://github.com/gravitational/teleport/blob/d7b212d617003992fab4420f87fbdb0b63c761cb/api/types/authority.go#L72-L73).

<a id="pkcs11"></a>
### PKCS#11 interface

A PKCS#11 interface, paired with configuration, could be used to implement
automated issuance of overridden certificates. An Auth node with adequate access
can mint the certificates and create/update the corresponding
cert_authority_override resource as needed.

tbot could be used as the integration point, monitoring CA rotation and
interacting with a local script to issue new certificates.

PKCS#11, while convenient, is not desirable when interacting with offline CAs,
and is therefore considered out of scope.

### ACME Sub CA rotation

The ACME protocol could be used for automatic Sub CA rotation, allowing for a
simpler and less error-prone process. It is considered out of scope of the
current design for reasons similar to [PKCS#11](#pkcs11).

### Linked workload overrides

Instead of fully replacing workload_identity_x509_issuer_override, both
cert_authority_override and workload override entity could coexist.

The concept of a "linked" workload override is introduced to
cert_authority_override:

```diff
 message CertificateOverride {
   // (...)
+
+  // Name of the linked workload identity override.
+  // Eg: "default".
+  string linked_workload_identity_override = n;
 }
```

The workload override entity is the de-facto implementation of the feature for
SPIFFE. The feature can be interacted with either via workload override or Sub
CA commands - the latter will cause the corresponding changes to a workload
override entity.

Create operations on workload override the corresponding "linked"
cert_authority_override. Deletes on a workload override "cascade" in a similar
manner.

The linked overrides concept was discarded in favor of the more straightforward
[replacement with cert_authority_override](#spiffe-override), which promotes a
simpler product and simpler UX long-term.

---
authors: Noah Stride (noah@goteleport.com), Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 0194 - Upstream PKI Support

## Required Approvers

* Engineering: ???
* Product: ???? 

## What

Today, all certificate authorities within Teleport are root certificate
authorities. This RFD proposes adding support for a certificate authority
within Teleport to be an intermediate certificate authority within an external
PKI hierarchy owned by a user.

This RFD primarily focuses on the "SPIFFE" certificate authority which issues
credentials for Teleport Workload Identity. However, decisions made within this
RFD should consider the broader implications.

## Why

There are a number of reasons why an organization may desire, or require, that
Teleport Workload Identity credentials are issued by an intermediate to a 
root certificate authority that is controlled by the organization.

For example:

- It may be required as part of compliance policy. 
- Ability to revoke certificates issued by Teleport Workload Identity, without
  using Teleport Workload Identity.
- Ability to rotate the Teleport Workload Identity certificate authority without
  needing to distribute the new certificate authority to validators working with
  workload identities.

## Details

The project is largely divided into two phases:

- Building out the underlying support for Teleport CAs being intermediate CAs
  within an external PKI hierarchy, with a manual process for rotating.
- Building out integrations with specific PKI solutions to automate the process
  of rotating the Teleport CA.

### Phase 1: Intermediate CA Support

#### Overview

A new `pre-init` phase will be added to the Teleport CA rotation process.

The pre-init phase may only be entered from the `standby` phase.

Upon entering the `pre-init` phase, a new set of key pairs
(but not certificates) will be generated and stored in the
CertificateAuthorityV2 resource. These will be stored in a new field,
`spec.NextKeys` of the existing CAKeySet type.

If the user requests that the CA return to the `standby` phase from `pre-init`,
then the `spec.NextKeys` field will be cleared. No further action is required.

Once in the `pre-init` phase, the user may generate a CSR for the new CA key-pair
using the `tctl auth generate-ca-csr` command.

Out of band, the user then takes the CSR, and obtains a signed certificate from
their upstream PKI hierarchy. They must also obtain a copy of the root CA and
any intermediate CAs between the root CA and the leaf CA issued for Teleport.

The user then use `tctl auth import-ca` to provide the signed certificate,
intermediates and root CAs to Teleport:

- Teleport will verify the signed certificate and chain:
  - The given leaf CA validates against the given chain.
  - The given leaf CA's key matches the generated key in the `spec.NextKeys` field.
- The CertificateAuthority resource will then be updated:
  - The given leaf CA will be written into `spec.NextKeys.TLS.Cert`.
  - The given intermediate CAs will be written into `spec.NextKeys.TLS.IntermediateCerts`.
  - The given root CAs will be written into `spec.NextKeys.TLS.RootCerts`.

With the certificates loaded into `spec.NextKeys`, the user can now transition
the CA to the `init` phase sing the standard `tctl auth rotate` command. This
will:

- Swap the `NextKeys` field into the `AdditionalTrustedKeys` field.
- Transition the CA to the `init` phase.

From this point, the CA rotation continues as normal.

#### Resource Changes

The following new fields will be added to the `CertificateAuthorityV2` protobuf
message:

- `NextKeys` (CAKeySet): The keys generated upon entering the `pre-init` phase
  and for which a CSR can be generated.

To accommodate storing the intermediate and root CAs related to a Teleport
CA, two new fields will be introduced to the `TLSKeyPair` protobuf message:

- `IntermediateCerts` (bytes) - PEM-encoded intermediate certs linking the
  CA in the `Cert` field to one of the root CAs in the `RootCerts` field.
- `RootCerts` (bytes) - PEM-encoded root certs. Multiple root certs may be
  stored in this field.

#### RPC Changes

A new RPC should be introduced to the TrustService for uploading the signed
certificate and chain:

```protobuf
service TrustService {
  // ImportCA imports a signed certificate and intermediate/roots into a
  // Teleport CA which is in the pre-init phase.
  //
  // Requires Create/Update permissions for the CertificateAuthority resource.
  rpc ImportCA(ImportCARequest) returns (ImportCAResponse) {}
}


message ImportCARequest {
  // The type of the CA (e.g `spiffe`)
  string type = 1;
  // The signed CA certificate (encoded in PEM), generated from the CSR, for
  // Teleport to use. 
  bytes leaf_ca_certificate = 2;
  // Any intermediate CA certificates (encoded in PEM and appended to one
  // another) necessary to chain leaf_ca_certificate to a certificate in 
  // root_ca_certificates.
  bytes intermediate_ca_certificates = 3;
  // The root CA certificates (encoded in PEM and appended to one another) at
  // the top of the PKI hierarchy in which leaf_ca_certificate was issued.
  bytes root_ca_certificates = 4;
}

message ImportCAResponse {}
```

The RotateCertAuthority RPC will need to be modified to support the new
`pre-init` phase. Initially, this phase will only be accepted for the `spiffe`
certificate authority.

#### Expiry Warnings

The expiry of a CA would have significant impact for a Teleport cluster and
therefore we must provide notification of an impending expiry.

The Teleport cluster alerts mechanism will be used to alert the user when any CA
passes:

- 15% of the remaining lifetime of the CA (e.g 13.5 days for a 90-day CA).
- 10% of the remaining lifetime of the CA (e.g 9 days for a 90-day CA).
- 5% of the remaining lifetime of the CA (e.g 4.5 days for a 90-day CA).

#### UX

```shell
# User indicates they wish to enter the pre-init phase.
$ tctl auth rotate --manual --type spiffe --phase pre-init
Updated rotation phase to pre-init. New keys have been generated, and you can
now provide a certificate to move to the next phase.
# User can now generate a CSR for the new CA key-pair.
$ tctl auth generate-ca-csr --type spiffe -o spiffe.csr
A CSR for the SPIFFE CA has been generated and written to spiffe.csr.
# User provides the signed certificate and chain:
$ tctl auth import-ca --type spiffe --chain spiffe-chain.crt
# User progresses to the `init` phase of a normal rotation.
$ tctl auth rotate --manual --type spiffe --phase init 
Updated rotation phase to "init". To check status use 'tctl status'
```

#### Teleport Workload Identity

The behaviour of Teleport Workload Identity will need to be adjusted to support
the SPIFFE CA being an intermediate CA. Specifically, we must decide how the 
CAs above the Teleport SPIFFE CA are treated and distributed.

If no changes were made, then, the PKI hierarchy above Teleport would
effectively be superficial. Teleport Workload Identity would continue to
distribute its intermediate CA as if it were a root CA. This would mean that 
workloads using Teleport Workload Identity as a source of a trust bundle would
not be able to validate X509 SVIDs that were issued elsewhere in the PKI 
hierarchy. This provides a level of isolation for the trust domain and workloads
can expect that any signed certificate is going to abide by the X509 SVID
specification.

Unfortunately, a number of TLS/X509 validators refuse to accept an intermediate 
certificate (e.g not self-signed) as a root CA. This is not per the
specification, but, would pose significant concerns around compatability.

This means that Teleport Workload Identity must instead:

- Distribute the root CA in place of the leaf SPIFFE CA in trust bundles.
- Include the leaf CA and any intermediate CA with any X509 SVID.

The ramifications of this for the Teleport Workload Identity security model are
significant and must be included in the documentation:

- Other CAs within the PKI hierarchy may be able to be used to issue X509
  certificates that appear to be X509 SVIDs issued by Teleport Workload Identity
  for the trust domain - without the attestation configured within Teleport
  Workload Identity having taken place. It is the sole responsibility of the
  operator to ensure that this is not possible or does not pose a risk.
- Validators MUST take additional care when processing X509 certificates. There
  is no longer a guarantee that a certificate that passes signature validation is
  a valid X509 SVID that has been issued in line with the SPIFFE specification.
- When a trust domain federates with a Teleport Workload Identity trust domain,
  the federating trust domain will be trusting X509 certificates issued anywhere
  within the PKI hierarchy rather than solely X509 SVIDs issued by the trust 
  domain.

Additionally, end-users should be made aware of other implications such as:

- Whilst X509 SVIDs will be trusted across the entire PKI hierarchy, JWT SVIDs
  will only be trusted within the Teleport Workload Identity trust domain. This
  creates a disparity between X509 and JWT SVIDs that would not usually exist.

### Phase 2: Automation & Integration

https://ztpki-staging.venafi.com/api/v2/swagger/
---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0194 - SPIFFE X.509 issuer override for external PKI support

## Required Approvers

* Engineering: @strideynet || @timothyb89

## What

Teleport manages a certificate authority to issue SPIFFE credentials, consisting of one or more keys used for JWT-SVID issuance and one or more keys and self-signed CA X.509 certificates for X509-SVID issuance. This RFD proposes adding support for overriding the X.509 certificates used as issuers for X509-SVID credentials.

## Why

There are a number of reasons why an organization may desire, or require, that Teleport Workload Identity credentials are issued by an intermediate to a root certificate authority that is controlled by the organization.

For example:

- It may be required as part of compliance policy.
- Ability to revoke certificates issued by Teleport Workload Identity, without using Teleport Workload Identity.
- Ability to rotate the Teleport Workload Identity certificate authority without needing to distribute the new certificate authority to validators working with workload identities.

## Issuer override

Currently, X509-SVID issuance in the Auth Service selects the first key usable by the Auth and signs a certificate for the client's public key using the X.509 certificate associated with the key as the issuer. We will introduce a new resource with kind `workload_identity_x509_issuer_override`:

```yaml
kind: workload_identity_x509_issuer_override
version: v1
metadata:
  name: default
spec:
  overrides:
    - issuer: base64 DER certificate
      chain:
        - base64 DER certificate
        - base64 DER certificate
    - issuer: base64 DER certificate
      chain:
        - base64 DER certificate
        - base64 DER certificate
```

The name `default` will be the only allowed name for now, to allow supporting multiple independent overrides in the future, specified in each `workload_identity` (or maybe in a tunable at the SPIFFE "trust domain" level once that story is more fleshed out).

During the X509-SVID issuance, if the client signals support for using issuer overrides (`tbot` and `tsh` will be updated to always signal support, and to store the certificate chain appropriately), the Auth will search for an issuer certificate listed in the `.spec.overrides` list that has the same public key as the key selected by the Auth, and will use said certificate as the issuer for the new certificate in lieu of the internal X.509 self-signed certificate. The new certificate is returned to the client as usual, and the certificate chain associated to the issuer is included in the response.

This format was chosen to allow for maximum flexibility in exchange for some potential duplication (it's likely that the issuer certificate will also be part of the certificate chain), as it makes it possible to have no certificate chain - for example, if the issuer certificates are alternate self-signed certificates, or if they are the very same self-signed certificates as in the X.509 CA, which allows for an alternate "override" that doesn't actually override anything, to avoid breaking workloads that were in place and haven't been updated to trust the new PKI.

If no issuer is found for the key selected by the Auth at issuance time, issuance will fail, because failing to issue credentials is better than silently succeeding in issuing credentials that no validator is intended to trust. For reliable operations, it's important that issuer overrides are kept up to date with any new keys added during a CA rotation - the `init` phase of the CA rotation is a good time to update them, and the CA rotation interactive helper will be updated to warn against advancing phases if issuer overrides are detected with missing keys.

A new RPC for signing CSRs will be added to the Auth, to support users and automation in getting intermediate certificates issued by external PKIs which often require a CSR to be signed and not just a public key (which can be read by any client). Depending on the requirements of certificate issuance services we'll end up supporting, we'll have different "modes" for the CSR signature, including two that are respectively selecting a "blank" CSR and a CSR with the same subject as the Teleport X.509 issuer. Existing X.509 certificate issuers have a nonce in the `serialNumber` field in their subject, which is not necessarily supported as one that moves the content of the `serialNumber` in the subject to a more commonly supported `organizationalUnit`.

### API changes

The new `workload_identity_x509_issuer_override` resource will be managed in the new `teleport.workloadidentity.v1.X509OverridesService` gRPC service in the auth, with the usual CRUD RPCs for Get, List, Create, Update, Upsert, Delete. Interacting with issuer overrides will require assigning resource permissions with kind `workload_identity_x509_issuer_override` and verbs `read`,`list`,`create`,`update`,`delete` in the usual combinations (`read`+`list` for List, `create`+`update` for Upsert). MFA for admin actions will be required for human users.

The `teleport.workloadidentity.v1.X509OverridesService/SignX509IssuerCSR` RPC will take the self-signed issuer certificate (in DER format) and will receive in response a CSR signed with its respective key. The initial implementation of this RPC will only support Teleport clusters where all Auth Service agents have access to all the keys in the SPIFFE certificate authority, but the API will be extended and/or replaced to support asynchronous operations - writing a request in the cluster state storage and letting the client poll for the request being filled. The API will require `create` permissions for the `workload_identity_x509_issuer_override_csr` pseudo-kind.

The `teleport.workloadidentity.v1.WorkloadIdentityIssuanceService/IssueWorkloadIdentity` and `IssueWorkloadIdentities` request and response messages will have additional fields:

- in `IssueWorkloadIdentityRequest` and `IssueWorkloadIdentitiesRequest`, a boolean field `.x509_svid_params.use_issuer_overrides` will let the client signal support for overrides (i.e., support for certificate chains);
- in `Credential` (used in both `IssueWorkloadIdentityResponse` and `IssueWorkloadIdentitiesResponse`), a new repeated bytes field `.x509_svid.chain` will include the certificates in the chain in the same format as `.x509_svid.cert`, i.e. ASN.1 DER-encoded X.509 certificates in order from closest to the end-entity cert to closest to the root cert.

### Client tooling

For manual operations, a `tctl workload-identity x509-issuer-override sign-x509-issuer-csrs` command will fetch all X.509 issuers from the SPIFFE CA and will sign a CSR for each, outputting them to standard output as a sequence of PEM "CERTIFICATE REQUEST" blocks, and a `tctl workload-identity x509-issuer-override create chain1.pem [chain2.pem...]` command will create (or upsert, if `--force` is passed) an issuer override with the given certificate chains, whose first certificates must replace the SPIFFE CA X.509 issuers.

A `tctl workload-identity x509-issuer-override status` command will fetch both CA and default override, and display a summary of the configured overrides and their expiration date, highlighting CA keys that are missing an override or overrides that are no longer necessary.

The intended automated way to set up and renew a certificate override will be through a new `tbot` service, usable as both a long-running service and as a "oneshot" service. The service will support select services that issue intermediate certificate authority certificates, and it will issue new intermediates following CA rotations of the Teleport SPIFFE CA and renew existing ones as they're about to expire.

### Audit log

New `workload_identity_x509_issuer_override.{create,update,delete}` events will be emitted to keep track of changes to the issuer override, containing resource metadata with the override name (for now, always `default`) and user metadata with the identity that caused the event.

## Root override (out of scope)

In the future we could consider also overriding the roots exported as part of the SPIFFE trust bundle, either as part of cluster configuration or as an option in client tooling. The former option reduces the security benefits of the alternate PKI, since a misconfigured or malicious cluster will be able to change the roots of trust that validators will potentially be configured to follow, but the latter can be useful, especially when using `tbot`'s `workload-identity-api` service.

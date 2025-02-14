---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0194 - SPIFFE X.509 override for external PKI support

## Required Approvers

* Engineering: @strideynet && TBD
* Product: TBD

## What

Teleport manages a certificate authority to issue SPIFFE credentials, consisting of one or more keys used for JWT-SVID issuance and one or more keys and self-signed CA X.509 certificates for X509-SVID issuance. This RFD proposes adding support for overriding the X.509 certificates used as issuers for X509-SVID and the X.509 certificates exported as part of the SPIFFE bundle through the Proxy Service endpoint and through the `tbot` Workload Identity services.

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

The name `default` is significant, as all `workload_identity`s that don't specify an issuer override will use the one called `default` if it exists, but the cluster administrator can create more than one override, and each `workload_identity` can be configured to use a specific one, to allow for more than one external PKI. A new field will be added to `workload_identity` for this purpose:

```yaml
kind: workload_identity
version: v1
metadata:
  name: my-workload
spec:
  rules:
    # ...
  spiffe:
    # ...
    x509:
      # ...
      issuer_override: nondefault-override-name
```

During the X509-SVID issuance, if an override is specified (or if no override is specified and the `default` one exists) and the client signals support for using issuer overrides (`tbot` and `tsh` will be updated to always signal support, and to store the certificate chain appropriately), the Auth will search for an issuer certificate listed in the `.spec.overrides` list that has the same public key as the key selected by the Auth, and will use said certificate as the issuer for the new certificate in lieu of the internal X.509 self-signed certificate. The new certificate is returned to the client as usual, and the certificate chain associated to the issuer is included in the response.

This format was chosen to allow for maximum flexibility in exchange for some potential duplication (it's likely that the issuer certificate will also be part of the certificate chain), as it makes it possible to have no certificate chain - for example, if the issuer certificates are alternate self-signed certificates, or if they are the very same self-signed certificates as in the X.509 CA, which allows for an alternate "override" that doesn't actually override anything, to avoid breaking workloads that were in place and haven't been updated to trust the new PKI.

If no issuer is found for the key selected by the Auth at issuance time, issuance will fail, because failing to issue credentials is better than silently succeeding in issuing credentials that no validator is intended to trust. For reliable operations, it's important that issuer overrides are kept up to date with any new keys added during a CA rotation - the `init` phase of the CA rotation is a good time to update them, and the CA rotation interactive helper will be updated to warn against advancing phases if issuer overrides are detected with missing keys.

A new RPC for signing CSRs will be added to the Auth, to support users and automation in getting intermediate certificates issued by external PKIs which often require a CSR to be signed and not just a public key (which can be read by any client).

TBD: how flexible should the CSR generation be? ACME wants subject and SANs to be in the CSR, but Venafi's Zero-Touch PKI is fine with a blank CSR since everything about the cert is specified out of band.

### API changes

The new `workload_identity_x509_issuer_override` resource will be managed in the new `teleport.workloadidentity.v1.X509OverridesService` gRPC service in the auth, with the usual CRUD RPCs for Get, List, Create, Update, Upsert, Delete. Appropriate permissions should be granted to whoever or whatever needs to interact with issuer overrides, with kind `workload_identity_x509_issuer_override` and verbs `read`,`create`,`update`,`delete`. MFA for admin actions will be required for human users.

The `teleport.workloadidentity.v1.X509OverridesService/SignX509IssuerCSR` RPC will take the self-signed issuer certificate (in DER format) and will receive in response a CSR signed with its respective key. The initial implementation of this RPC will only support Teleport clusters where all Auth Service agents have access to all the keys in the SPIFFE certificate authority, but the API will be extended and/or replaced to support asynchronous operations - writing a request in the cluster state storage and letting the client poll for the request being filled.

TBD: do we tie `SignX509IssuerCSR` to `create`+`update` permissions for `workload_identity_x509_issuer_override`? It seems like what you'd always want I think, but maybe there's a point to using a separate verb and pseudokind? On the other hand, the operation isn't particularly sensitive to begin with

The `workload_identity` resource will have a new string field in `.spec.spiffe.x509.issuer_override` which will be ignored by Auth Services that wouldn't know how to use the overrides to begin with.

The `teleport.workloadidentity.v1.WorkloadIdentityIssuanceService/IssueWorkloadIdentity` and `IssueWorkloadIdentities` request and response messages will have additional fields:

- in `IssueWorkloadIdentityRequest` and `IssueWorkloadIdentitiesRequest`, a boolean field `.x509_svid_params.use_issuer_overrides` will let the client signal support for overrides (i.e., support for certificate chains);
- in `Credential` (used in both `IssueWorkloadIdentityResponse` and `IssueWorkloadIdentitiesResponse`), a new repeated bytes field `.x509_svid.chain` will include the certificates in the chain in the same format as `.x509_svid.cert`, i.e. ASN.1 DER-encoded X.509 certificates in order from closest to the end-entity cert to closest to the root cert.

### Client tooling

TBD: `tctl workload-identity x509-issuer-override sign-x509-issuer-csr`? How should the user pass the original issuer in?

TBD: do we want a command to check if an override is "healthy" (i.e. if it has one issuer for each issuer in the Teleport CA and if the issuers are not expired or about to expire)? do we need management beyond `tctl get`/`create`/`rm`/`edit`?

TBD: a `tbot` service that can be configured to create and update an override (either `default` or custom) by issuing intermediate CAs through an external PKI service, updating the override resource when a new issuer appears in the `spiffe` CA (as a result of a rotation) or when the alternate issuers are about to expire;

TBD: do people generally use IaC for certificate issuance? we can support the `workload_identity_x509_issuer_override` resource in IaC but that will likely only work for long-lived intermediate CAs issued by hand, not for anything that can follow rotations and cert expirations unless we also support the CSR generation somehow

### Audit log

WIP: new `workload_identity_x509_issuer_override.create` (`write`?)/`.delete` event with user metadata for who/what did it

## Root override

TBD: using an external PKI but letting Teleport distribute its roots to validators partially defeats the purpose of using an external PKI, since a compromised Teleport cluster can just lie to validators about which roots should be trusted, and distributing pins for the roots is about as onerous as distributing the roots in the first place - do we really need this?

The SPIFFE bundle exported by Teleport from either the Proxy Service's https endpoint or by `tbot`'s various services currently includes all and only the trusted issuers in the `spiffe` certificate authority.

We will introduce a new resource with kind `workload_identity_x509_root_override`:

```yaml
kind: workload_identity_x509_root_override
version: v1
metadata:
  name: default
spec:
  roots:
    - base64 DER certificate
    - base64 DER certificate
    - base64 DER certificate
```

When a `default` root override exists, or when the client is configured to use a specific override, the produced SPIFFE bundle will include the certificates specified in the override instead of the X.509 certificates listed as trusted in the `spiffe` certificate authority. If a gradual transition is needed while existing SPIFFE credentials are still being issued by the internal hierarchy, the internal issuers should be included in the override. Likewise, a no-op override can be created by listing all and only the internal hierarchy's certificates.

### API changes

The new `workload_identity_x509_root_override` resource will also be managed in the new `teleport.workloadidentity.v1.X509OverridesService` gRPC service, with the usual CRUD RPCs for Get, List, Create, Update, Upsert, Delete. Appropriate permissions should be granted to whoever or whatever needs to interact with root overrides, with kind `workload_identity_x509_root_override` and verbs `read`,`create`,`update`,`delete`. The `read` verb will be granted as part of implicit permissions, just like `read` on `cert_authority`. MFA for admin actions will be required for human users.

For the MVP, the Get RPC will be used by the Proxy and by clients (`tbot`, `tsh`) when generating the SPIFFE bundle (which is currently done by reading the `spiffe` cert authority). In the future we could consider a dedicated RPC that returns the public material needed for a SPIFFE bundle all at once, taking into account any configured override.

### Client tooling

TBD: do we need anything other than general `tctl get`/`create`/`rm`/`edit` and IaC resource management?

TBD: should the interactive CA rotation tool check and warn for overrides that include internal issuers as roots?

### Audit log

WIP: new `workload_identity_x509_root_override.create` (`write`?)/`.delete` event with user metadata for who/what did it

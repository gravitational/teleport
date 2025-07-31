---
authors: Zac Bergquist <zac.bergquist@goteleport.com>
state: released (v18.1.0, v17.7.0)
---

# RFD 210 - HSM support for Active Directory environments

## What

Ensure that Teleport works correctly in Active Directory environments
when configured to use HSM-backed private key material.

## Why

Teleport supports certificate-based login in Active Directory environments for
both Windows desktop access (RDP), and MS SQL database access.

Windows requires that these certificates include metadata which contains a link
to a certificate revocation list (CRL), which is validated as part of the login
process.

Teleport's current implementation assumes that there is a single CA issuing
these certificates, so all certificates that Teleport generates point to the
same CRL distribution point (CDP).

The assumption that there is a single CA is invalid when Teleport is configured
to use HSMs, as each auth server will have its own certificate and HSM-backed
private key. Windows requires that the CRL pointed to by the certificate is
signed by the same CA as the certificate itself. As a result, Teleport logins
only work 1 out of N times (where N is the number of auth servers in the
cluster) when configured for HSMs, because the auth server that issued the
cert for the session or may not be the same as the auth server that generated
the CRL.

## Required Approvers

* Engineering: @nklaassen && (@gabrielcorado || @greedy52)
* Security: (@rjones || @klizhentas)

## Details

Windows supports a variety of different CRL distribution points:

- local paths: `file://...`
- remote file share paths: /`\\server\share\...`
- LDAP objects: `ldap://CN=Teleport,CN=CDP,...`
- HTTP/HTTPS endpoints: `https://...`

Teleport has always published its CRLs to LDAP, as it's guaranteed to be
available in the environments where Teleport runs and Teleport already has an
LDAP requirement. While it would be possible to make the Teleport proxy server
host CRLs over HTTPS, we aim to avoid such a change as we can't be sure that all
of the Windows components have network access to the Teleport proxy.

> [!NOTE]
> Teleport has its own locking system and does not use CRLs for revocation.
> Teleport only implements CRLs to satisfy Windows requirements, and its CRLs
> are always empty and always valid for 1 year from time of issuance.

In order to support HSM-enabled clusters with multiple active signers, we need
to consider two aspects:

1. Certificate generation: how does Teleport generate the CRL distribution point
2. CRL publishing: how does the CRL actually get to the distribution point referenced in the cert

### Certificate Generation

The certificate generation step is largely the same for Windows desktop access
and MS SQL database access. In both cases, Teleport generates an LDAP URL based
on the configured Active Directory domain. It takes the form:

```
ldap://CN=CLUSTER,CN=SIGNER,CN=CDP,CN=Public Key Services,CN=Configuration,DC=example,DC=com
```

Where:
- `CLUSTER` is the name of the Teleport cluster
- `SIGNER` is `Teleport` (for Windows desktop certs) or `TeleportDB` for database certs
- `example.com` is the AD domain (`DC=example,DC=com` in LDAP-speak)

### CRL Publishing

CRL publishing works differently for each Teleport feature.

#### MS SQL database access

For database access, our setup instructions require that users generate the CRL
with `tctl auth crl`, copy it over to the Windows environment, and publish it to
LDAP using `certutil -dspublish`.

It is worth noting that `tctl auth crl` generates empty CRLs that are valid for _1 year_,
so database access will require manual intervention in case:

- the CRL expires after 1 year
- a CA rotation is performed

#### Windows desktop access

For Windows desktop access, Teleport's `windows_desktop_service` generates a CRL
on startup and publishes it to LDAP directly - no `certutil -dspublish` step is
necessary. This process is repeated every time the agent restarts, as well as
every time the agent refreshes its credentials (typically every 4 hours or so).

As a result, desktop access requires manual intervention in the case of a CA rotation,
but does not suffer from the CRL expiry issue, since the CRL is constantly getting
pushed back by 1 year.

### Proposed Changes

In order to support HSMs, which result in multiple active signers being present in the cluster,
we will need to:

1. Generate certs with a CDP that references the specific cert issuer
2. Publish multiple CRLs to LDAP instead of a single CRL

#### Cert Generation: use SKID of the issuer

When generating certificates, Teleport will use the subject key ID of the signer to
generate a CDP that is unique to that signer.

```
ldap://CN=IV4GC3LQNRSSAVDFNRSXA33SORJUWSKE_CLUSTER,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=example,DC=com
```

In this case, `IV4GC3LQNRSSAVDFNRSXA33SORJUWSKE` is the base32-encoded subject
key identifier of the issuing certificate, and `CLUSTER` is the name of the
Teleport cluster.


#### Publishing: `tctl auth crl --out`

For `tctl` we will follow the precedent set in
[#51298](https://github.com/gravitational/teleport/pull/51298) for `tctl auth
export`, and add a `--out` flag to `tctl auth crl`. This flag will cause `tctl`
to write the CRL(s) (whether there is one or many) to disk rather than to emit a
single CRL to standard out.

Teleport users setting up MS SQL access will then be able to perform a setup
procedure very similar to what they already do, they will only have to repeat
the `certutil -dspublish` command  once for each individual CRL.

#### Publishing: add CRLs to the backend `certificate_authority`

In order to maintain the behavior where Teleport's `windows_desktop_service`
publishes CRLs to LDAP on a periodic basis, we need to change the way the agent
requests a CRL from the auth server.

Today, the agent calls an auth RPC that generates a CRL on-demand. The agent has
no control over which signer is used to generate the CRL, since it is totally
dependent on which auth server handles the RPC.

Instead of calling an RPC to get a single CRL, the agent needs to be able to get
a set of CRLs (one for each active signer in the cluster). This means that the
CRLs must be present somewhere in Teleport's cluster state backend and not
generated on-demand.

Since these CRLs are small (they are always empty), we will store the CRL
directly in the `cert_authority` resource. This approach has several advantages:

- Agents already set up a watch to monitor CA resources in order to implement CA rotation,
  so the entire resource (including the CRL) should always be present in the agent's cache
  and very cheap to look up.
- It avoids a lot of boilerplate code required to add a new resource and set up RBAC, caching, etc.

Instead of calling an RPC to get a CRL, the agent will attempt to pull CRLs from
the `cert_authority`. For backwards compatibility, the agent will fall back to
the original `Teleport` CDP if the CRL is not present in the `cert_authority`
resource. This allows newer agents to maintain today's behavior even when running
against older auth servers.

### UX

The goal of this proposal is to fix Teleport logins on HSM-enabled clusters without making
significant changes to the user experience.

#### Database access

The `tctl auth crl` command will behave exactly like `tctl auth export`, printing a warning
that asks the user to add the `--out` flag if it detects more than one active signer.

We will also update the docs for this feature to ensure that it covers how to
import multiple CRLs, and advise users to repeat the setup process when CA
rotations are performed.

#### Desktop access

From a desktop access perspective, the user experience should not change at all.

Users will have to update both their auth servers and their agents in order to get
HSM support working, but we will take care to ensure that agents and auth can be
updated independently and don't need to update in any particular order.

### Proto Specification

New field will be added to `TLSKeyPair` resource:

```protobuf
bytes CRL = 4 [(gogoproto.jsontag) = "crl"];
```

This field will store empty DER-encoded revocation list for `cert_authorities` that require it.

### Audit Events

A new field will be added to the `certificate.create` audit event which will
hold information about the authority used to create certificate:

```protobuf
message CertificateCreate {
  // ...

  // CertificateAuthority holds information about creator of certificate
  CertificateAuthority CertificateAuthority = 5 [(gogoproto.jsontag) = "certificate_authority"];
}

// CertificateAuthority holds information about creator of certificate
message CertificateAuthority {
  // ID is identifier of the cert authority (type and domain)
  string ID = 1 [(gogoproto.jsontag) = "id,omitempty"];
  // SubjectKeyID is BASE32-encoded subject key ID from authority certificate
  string SubjectKeyID = 2 [(gogoproto.jsontag) = "subject_key_id,omitempty"];
}
```

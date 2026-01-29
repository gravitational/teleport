---
authors: Alan Parra (alan.parra@goteleport.com)
state: draft
---

# RFD 0239 - Windows CA split

## Required Approvers

* Engineering: @rosstimothy && @zmb3
* Security: (@rob-picard-teleport || @klizhentas)
* Product: @klizhentas

## What

Split the "windows" (aka Desktop Access) CA from the User CA.

## Why

The "windows" CA is used to generate user certificates for Desktop Access. Those
certificates are used to perform smart card authentication on RDP (Remote
Desktop Protocol).

Currently the User CA issues those certificates, as they are end-user certs.
Splitting the CAs improves Teleport's security posture by introducing a more
specialized CA (the "windows" CA needs to be trusted by downstream Windows
systems) and lets both CAs be rotated independently.

This is a task we've been long meaning to address (see [#10111](
https://github.com/gravitational/teleport/issues/10111)).

## UX

The CA split borrows its methodology and UX from the previous [db and db_client
split][docs-db-ca-migration].

The new CA is called "windows", as that term is already used in the product's UX
(for example, `/webapi/auth/export?type=windows` and `tctl auth export
--type=windows`).

Honorable mention to [RFD 0168][rfd0168].

[docs-db-ca-migration]: https://goteleport.com/docs/zero-trust-access/management/operations/db-ca-migrations/#teleport-db_client-ca-migration
[rfd0168]: https://github.com/gravitational/teleport/blob/master/rfd/0168-database-ca-split.md

### New Teleport cluster

New Teleport clusters are created with distinct User and Windows CAs. No further
action is needed from the user.

### Existing Teleport cluster

Existing Teleport clusters have the User CA copied into Windows CA on startup. A
CA rotation is required to create distinct key material.

Neither User or Windows CA is rotated automatically, nor is there any forced
rotation relationship between them. This follows our [database CA
migration][docs-db-ca-migration] advice.

Old Windows Desktop service instances (prior to the introduction of the Windows
CA) will remain issuing RDP certificates using the User CA. Only new Windows
Desktop service instances are capable of using the Windows CA. This is
transparent while the Windows CA is a copy of the User CA, but the issued
certificates will differ once a rotation happens.

Attempts to rotate the Windows CA will [issue a warning][ca-warning-example]
calling attention to this fact that only new agents are capable of utilizing it,
thus all agents should be upgraded first.

[ca-warning-example]: https://github.com/gravitational/teleport/blob/acc3b793502c352489a49d8ff59d17bae6a38fc9/tool/tctl/common/auth_rotate_command.go#L1197-L1200

## Details

The Windows CA, if it doesn't exist, is created during Teleport initialization:

1. If the UserCA exists: as a copy of the UserCA (existing cluster)
    * Implemented after [init migrations][init-migrations]
1. If the UserCA does not exist: as a distinct CA (new cluster)
    * Implemented by [initializeAuthorities][init-authorities]

Cluster [initialization already runs under a lock][init-lock], so no further
precautions against Auth server races are required.

[init-migrations]: https://github.com/gravitational/teleport/blob/30b4bdcfe6b18d6c9c6075b393c0528d9c1082f1/lib/auth/init.go#L644
[init-authorities]: https://github.com/gravitational/teleport/blob/30b4bdcfe6b18d6c9c6075b393c0528d9c1082f1/lib/auth/init.go#L707
[init-lock]: https://github.com/gravitational/teleport/blob/30b4bdcfe6b18d6c9c6075b393c0528d9c1082f1/lib/auth/init.go#L456

### Agent compatibility check

In order to ensure correct functioning a new [GenerateWindowsDesktopCert][] RPC
request parameter is added to detect old clients/agents.

Old agents (prior to Windows CA) get certificates issued using the User CA. New
agents use the Windows CA.

```diff
 message WindowsDesktopCertRequest {
   // (Existing fields omitted.)
+
+  // Set by callers that fully support the new Windows CA.
+  //
+  // Auth issues certificates differently according to this parameter:
+  // - false issues certificates using the UserCA
+  //   (Teleport agent versions < 18.x, where 18.x marks the initial release of
+  //   the Windows CA split).
+  // - true issues certificates using the WindowsCA
+  //   (Teleport agent version >= 18.x)
+  //
+  // Transitional. To be removed on Teleport 20, when all agents are expected
+  // to support the Windows CA.
+  bool SupportsWindowsCA = n;
 }
```

[GenerateWindowsDesktopCert]: https://github.com/gravitational/teleport/blob/269f876060350f785f45ca87f1653d30cb5fbcbe/api/proto/teleport/legacy/client/proto/authservice.proto#L3657-L3659

### Client compatibility check

Incompatible tctl versions are detected per [User-Agent][tctl-ua] plus
[version][tctl-version] and stopped from querying the User CA, due to the
inherent ambiguity of such requests. (Ie, does it want the Windows CA instead?)

This stops commands such as `tctl auth export --type=windows` from exporting the
wrong CA.

The check is only applied to [GetCertAuthority][] requests and is itself less
than perfect, but is considered sufficient (and better than a more severe
breakage, such as a MinClientVersion increase).

[tctl-ua]: https://github.com/gravitational/teleport/blob/306b50a648f2245d3a35e603cb7a9eb684bbaa59/api/metadata/metadata.go#L126
[tctl-version]: https://github.com/gravitational/teleport/blob/306b50a648f2245d3a35e603cb7a9eb684bbaa59/api/metadata/metadata.go#L100
[GetCertAuthority]: https://github.com/gravitational/teleport/blob/306b50a648f2245d3a35e603cb7a9eb684bbaa59/api/proto/teleport/trust/v1/trust_service.proto#L29

### Trust architecture

**Connect, tbot and tsh**

* No direct interaction with the Windows CA.

**tctl**

* tctl has various commands that interact with CAs.
* No direct trust of the Windows CA or special interactions, other than the ones
  above.

**Proxy**

* No direct interaction with the Windows CA.
* The Proxy connects to the Windows Desktop Service using a [UserCA
  certificate][proxy-to-wda-cert] with the appropriate RouteToWindowsDesktop.

**Windows Desktop Service**

* The Windows Desktop Service connects to the Windows host using a [Windows CA
  issued certificate][wda-to-host-cert]

[proxy-to-wda-cert]: https://github.com/gravitational/teleport/blob/840b60d05896bd34ab7cc57f9527d36b32f909a5/lib/web/desktop.go#L281-L291
[wda-to-host-cert]: https://github.com/gravitational/teleport/blob/840b60d05896bd34ab7cc57f9527d36b32f909a5/lib/srv/desktop/windows_server.go#L1250-L1258

### Product surface

The Windows CA split has direct impact in the following user-visible surface:

* Desktop Access feature
* `tctl status`
* `tctl auth export`
* `tctl auth sign`
* `tctl auth rotate`
* `tctl auth crl`
* /webapi/auth/export endpoint

### Release targets

The Windows CA split targets:

* (N-1).x.0 - A minor release in the current version (eg, 18.7.0)
* N         - The next major version (eg, 19.0.0)

Any deprecation related actions only take effect on N+1 (eg, 20.0.0), to allow
for at least a full release cycle.

## Backward compatibility

Agent and client compatibility are discussed in their respective design
sections.

Rollbacks to versions prior to the split have the system revert to using the
User CA for Windows Desktop Access. That is harmless without a rotation, but may
break downstream trust if a rotation happened. The mitigation is that downstream
systems should retain trust to the User CA certificate for a safety period,
after which a downgrade becomes unlikely.

In the case of a rollback the split Windows CA remains in the backend,
effectively dormant until a following upgrade makes use of it again.

## Test Plan

**Windows CA split**

Features must be tested before split, after split (or on new cluster), and after
CA rotation.

- [ ] New cluster creates a distinct Windows CA
- [ ] Existing cluster splits the User CA
  - [ ] CAs are identical on split
  - [ ] OLD windows_desktop_service works with NEW Auth
        (partial upgrade, mints with UserCA)
  - [ ] NEW windows_desktop_service works with NEW Auth
        (complete upgrade, mints with WindowsCA)
  - [ ] NEW windows_desktop_service works with OLD Auth
        (partial downgrade, mints with UserCA)
  - [ ] OLD windows_desktop_service works with OLD Auth
        (complete downgrade, mints with UserCA)
  - [ ] CAs are distinct after rotation (NEW Auth)
  - [ ] OLD windows_desktop_service works with NEW Auth
        (after rotation, still mints with UserCA).
- [ ] Desktop Access works as expected
  - [ ] AD-based
  - [ ] static hosts
- [ ] CRL publishing reacts to CA rotations, publishes the WindowsCA
- [ ] `teleport start --bootstrap` can create a WindowsCA
- [ ] tctl commands work as expected
  - [ ] `tctl status`
  - [ ] `tctl auth export`
  - [ ] `tctl auth sign`
  - [ ] `tctl auth rotate`
  - [ ] `tctl auth crl`
- [ ] /webapi/auth/export endpoint
- [ ] Test plan executed against software keys
- [ ] Test plan executed against Cloud/HSM-backed keys

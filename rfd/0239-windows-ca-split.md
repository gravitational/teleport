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

It is recommended to allow a safety evaluation period of the new release before
User or Windows CA rotations, to avoid potential complications due to version
rollbacks.

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

Client-side tools that equate `"windows"` to `UserCA` will request the User CA
directly to APIs (instead of the now-correct Windows CA). That is harmless
before a rotation, but incorrect behavior as soon as rotation happens. The
mitigation is to increase the [MinClientVersion][] to match the earliest feature
release (ie, (N-1).x.0), accompanied by the appropriate release note warnings
and documentation.

Rollbacks to versions prior to the split suffer a similar problem: the system
will revert to using the User CA for Windows Desktop Access. Harmless without a
rotation, but very likely to break downstream trust if a rotation happened. The
mitigation is that downstream systems should retain trust to the User CA
certificate for a safety period, after which a downgrade becomes unlikely.

In the case of a rollback the split Windows CA remains in the backend,
effectively dormant until a following upgrade makes use of it again.

[MinClientVersion]: https://github.com/gravitational/teleport/blob/30b4bdcfe6b18d6c9c6075b393c0528d9c1082f1/version.go#L39

## Test Plan

**Windows CA split**

Features must be tested before split, after split (or on new cluster), and after
CA rotation.

- [ ] New cluster creates a distinct Windows CA
- [ ] Existing cluster splits the User CA
  - [ ] CAs are identical on split
  - [ ] OLD windows_desktop_service works with NEW Auth (partial upgrade)
  - [ ] NEW windows_desktop_service works with NEW Auth (complete upgrade)
  - [ ] NEW windows_desktop_service works with OLD Auth (partial downgrade)
  - [ ] OLD windows_desktop_service works with OLD Auth (complete downgrade)
  - [ ] CAs are distinct after rotation (NEW Auth)
  - [ ] OLD windows_desktop_service works with NEW Auth (after rotation).

    TODO(codingllama): Verify if the item above is possible. If yes keep it in
    the plan. If no see where/how we can improve a potential error message and
    update the plan accordingly.

- [ ] Desktop Access works as expected
- [ ] tctl commands work as expected
  - [ ] `tctl status`
  - [ ] `tctl auth export`
  - [ ] `tctl auth sign`
  - [ ] `tctl auth rotate`
  - [ ] `tctl auth crl`
- [ ] /webapi/auth/export endpoint
- [ ] Test plan executed against software keys
- [ ] Test plan executed against Cloud/HSM-backed keys

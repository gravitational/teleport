---
title: RFD 0168 - Split Database CA
authors: Gavin Frazar (gavin.frazar@goteleport.com)
state: implemented
---



# Required Approvals

-   Engineering: @r0mant @smallinsky
-   Product: @xinding33 || @klizhentas
-   Security: @reedloden || @jentfoo


# Related issues

-   <https://github.com/gravitational/teleport-private/issues/782>


# What

This RFD proposes to split the current DatabaseCA into a "user" database CA
and a "host" database CA.


# Why

The DatabaseCA is used as both a client and host CA currently.
Self-hosted databases are configured to trust the DatabaseCA and have a
cert/key pair that is signed by the DatabaseCA.
A compromised self-hosted database cert/key pair can be used to connect to
other self-hosted databases.

Some protocols are not vulnerable because they check that the CN subject
matches the db username.
Other protocols (Redis, Cassandra, ScyllaDB), don't check the CN subject, and
security is degraded for these protocols.

Teleport is opposed to shared secrets, so while these protocols are still
protected by a user/password secret, this should not be relied upon.

If we split the DatabaseCA, we can configure self-hosted databases to have a
"host" database cert/key pair and only trust client certs signed by the "user"
database CA, mitigating the vulnerability.


# Current trust architecture


## tsh and tbot

-   Database certs are signed by the UserCA with database "route" info
    populated.
-   The HostCA is trusted, because connections flow through the Teleport
    Proxy.


## Teleport Proxy

-   The proxy presents a HostCA cert to clients, and trusts the UserCA for
    client certs.
-   When the proxy dials a db agent over reverse tunnel, it trusts the HostCA
    and presents a DatabaseCA client cert to the db agent.


## Database Agent

-   Database agents trust the DatabaseCA for client (the proxy service) certs
    and host (self-hosted databases) certs.
-   When the database agent dials a self-hosted database, it presents a
    DatabaseCA client cert.


## Self-Hosted Databases

-   Databases are configured with a cert/key pair and trusted .cas file via
    `tctl auth sign`.
-   The DatabaseCA is trusted by the self-hosted database and is also the
    issuer for the self-hosted database's cert.
-   When a client (db agent) connects, a self-hosted database will present a
    DatabaseCA host cert and trust a DatabaseCA client cert.


# How to split the DatabaseCA

We can make a new DBHostCA, a new DBUserCA, or both.
Options:

1.  Make both a new DBHostCA and a new DBUserCA.
2.  Make a new DBUserCA, and keep the DatabaseCA as a "host" CA.
3.  Make a new DBHostCA, and keep the DatabaseCA as a "user" CA.

Regardless of what we do, the proxy, agent, and self-hosted database will all
be affected, not just db agent <-> self-hosted database.

Option 1 adds complexity for no discernible benefit.
We can just re-use the existing DatabaseCA as a "host" CA or a "user" CA and
create the other.

The backend cert\_authority type of the existing DatabaseCA is `db`.

With option 2:

-   A new cert\_authority type `db_user` is added.
-   The proxy **could be changed** (it does not have to be changed, see below) to
    present a client cert signed by the DBUserCA to the db agent over reverse
    tunnel connection.
-   The proxy will continue to trust a host (db agent) cert signed by the HostCA
    when it dials the db agent over reverse tunnel.
-   The db agent will continue to present a HostCA cert to a client
    (the proxy service).
-   The db agent **will be changed** to present a client cert signed by the
    DBUserCA to the self-hosted database.
-   The db agent will continue to trust a host (self-hosted database) cert
    signed by the DatabaseCA.
-   The self-hosted database **will be changed** to trust client (the db agent)
    certs signed by the DBUserCA.
-   The self-hosted database will continue to present a DatabaseCA cert to a
    client (the db agent).

With option 3:

-   A new cert\_authority type `db_host` is added.
-   The proxy will continue to present a client cert signed by the DatabaseCA to
    the db agent over reverse tunnel connection.
-   The proxy will continue to trust host (the db agent) certs signed by the
    HostCA when it dials a db agent over reverse tunnel connection.
-   The db agent will continue to present a host cert signed by the HostCA to a
    client (the proxy service).
-   The db agent will continue to present a client cert signed by the DatabaseCA
    to the self-hosted database.
-   The db agent **will be changed** to trust host (the self-hosted database)
    certs signed by the DBHostCA when it dials the self-hosted database.
-   The self-hosted database will continue to trust client (the db agent) certs
    signed by the DatabaseCA.
-   The self-hosted database **will be changed** to present a host cert signed by
    the DBHostCA to a client (the db agent).

With option 2, the DatabaseCA and DBUserCA more accurately describe the
purpose of each CA, but changing the client cert presented by the proxy
service adds complexity.
Since proxy<->agent communication is not vulnerable to the exploit that this
RFD aims to resolve, we could go with option 2 except not change the
proxy<->agent> trust architecture at all.
This would mean the DatabaseCA would still act as a client cert authority
internally when proxy dials agent.
However, it would be just as simple as option 3, and there is no security
issue with internal communication.

Option 3 is potentially simpler than option 2, because it does not require a
change in the proxy service.

We should go with option 2, because it makes more sense to create a backend
cert\_authority type `db_user` rather than a type `db_host` alongside the `db`
cert\_authority type.
It's clear what the difference is between the CA types: `db_user` for client
certs, `db` for database certs.

We should not change the proxy to get a client cert signed by the DBUserCA,
because it is not necessary for the security fix and it adds complexity.


# Implementation Overview


## Creating the new CA

We will add a new cert auth type: `db_user` for the DBUserCA.

If a new cluster is created, then the DBUserCA will be created normally as
part of auth first init CA creation.

If this is not the first time auth starts, then during auth init the
DatabaseCA will already exist. If the DBUserCA does not exist yet, we will
create the DBUserCA as a copy of the DatabaseCA.

We will also add a bool field to the CertAuthorityV2 spec: `is_copy`.
`is_copy` will be set to `true` when the copy is made.
When the CA is rotated, we will set `is_copy`: `false`.

As a special condition when the DBUserCA already exists during auth init, we
will check to see if `is_copy` is `true`.
If `is_copy` is true, but DBUserCA != DatabaseCA, we will delete and
recreate the DBUserCA as a copy of DatabaseCA.
This is to handle an edge case scenario like this:

1.  customer upgrades to a new version and DBUserCA = DatabaseCA is created.
2.  customer does not rotate any CAs yet.
3.  customer downgrades to a version that has no notion of DBUserCA.
4.  customer rotates DatabaseCA, and now DatabaseCA != DBUserCA.
5.  customer upgrades to a new version again.

In step 4 since that version of Teleport is unaware of the DBUserCA and
the customer will have reconfigured their databases, they would lose
self-hosted database access upon upgrading to a version with the DBUserCA,
since the DBUserCA has essentially stuck around as a copy of a CA they
rotated away.

I considered an alternative solution where we just check the CA rotation
status, and recreate the DBUserCA if it was never rotated, but this would
break db access if a new cluster is created, databases are configured to
trust the DBUserCA, and then the above scenario occurs.


## CA Rotation

If DBUserCA = DatabaseCA (customer upgraded an existing cluster), then we
we will handle the rotation of either CA rotation as a rotation of both.

A rotation of either CA requires customers reconfigure their self-hosted
databases to maintain access, so after rotating either DatabaseCA or
DBUserCA the security vulnerability will be resolved.

If we did not rotate both CAs when they are equal to each other, then
rotating just the DatabaseCA would be pointless and pose a security risk:

Imagine you are a customer and have determined that a cert/key signed
by the DatabaseCA may have been compromised.
You decide to rotate your DatabaseCA and reconfigure your databases.
You think you are now safe, but actually DBUserCA = old DatabaseCA, and your
databases are still vulnerable.

There's no downside to rotating both of the CAs, and if we do it then
customers are not potentially exposed to a security risk AND they aren't
annoyed with performing two rotations.

There is one special case we should handle: if HostCA = DatabaseCA =
DBUserCA.
Remember that the DatabaseCA was introduced in v10 as a copy of the HostCA.

We advised customers of this detail, but it is still quite likely that there
are customers who never rotated their DatabaseCA since v10.

Automatically rotating database CAs when HostCA is rotated, or HostCA when a
database CA is rotated, would be quite surprising behavior and has the
potential for things to break in unexpected ways. For one example, customers
would be surprised to find that rotating their HostCA breaks self-hosted
database access.

Rather than trigger an automatic rotation of all three CAs in this case,
we should create a cluster alert that the other CA(s) should be rotated as
well.


## API update

There are three Teleport components that need to work with the new CA:

1.  database agent
2.  tctl
3.  web ui resource enrollment (it generates a curl command to hit the
    `/webapi/sites/<cluster>/sign/db` endpoint)

All of these components call the `GenerateDatabaseCert` API, which returns a
signed cert and trusted CAs in the response.
In the request, the `requester_name` field is set to either `unspecified` or
`tctl` (and the web ui is also using `requester_name: tctl`).

We can leverage this behavior to only change the auth server implementation
of `GenerateDatabaseCert`.

When requester is `unspecified`, this indicates the agent is requesting a
client cert, which it will present to the self-hosted database for mTLS
handshake.

When requester is `tctl`, this indicates that the requester wants to
configure a self-hosted database with a cert and trusted cas.

For the agent, the response will be changed to:

    Cert: cert signed by the DBUserCA
    CACerts: DatabaseCA cert(s).

For tctl and `/webapi/sites/<cluster>/sign/db` endpoint, the response will
be changed to:

    Cert: cert signed by DatabaseCA
    CACerts: DBUserCA cert(s)

This will enable unpatched agents and tctl to still function properly after
the auth server is upgraded.

We should also mark `GenerateDatabaseCert` as deprecated starting with v15.
In v15 we can update the agent, tctl, and web ui to instead use one of
`GenerateDatabaseClientCert` or `GenerateDatabaseHostCert`, which will
supercede the old API func.
For compatibility we will keep `GenerateDatabaseCert` around for
at least one major version.
This deprecation is optional, we don't need to do it.
It would just make the code easier to understand at the call site, and
eliminate some branching in the auth server.


# Backwards compatibility

If a customer upgrades to a version that includes this CA split, their
database access will not be broken at all thanks to DatabaseCA = DBUserCA.

Once a customer rotates their db CAs, they will lose some backwards
compatibility.
If they downgrade their auth server below a version which introduces the
DBUserCA, then they will lose self-hosted database access.

Older versions of the proxy, agent, tctl, tsh, tbot will continue to work
even after a CA rotation, which maintains our version compatibility guarantee.


# Release process

We will include these changes in v15 and backport these changes to v12, v13,
v14.

We should release the backport changes as a minor release in v12/13/14.

We will then advise customers to rotate their CAs as soon as possible, but
we should communicate the backwards compatibility limitations that come with
it.

For customers who want to resolve the vulnerability immediately, they can
do a full rotation and then reconfigure their databases.

For customers who are more concerned about backwards compatibility, they
can delay rotating their CAs until they are comfortable with stability.
These customers might prefer to wait to rotate their CAs after upgrading to
another version, so that they know they can downgrade to a version that was
stable for them.

Ideally, the minor releases in v12/13/14 should contain only this CA split to
reduce the odds of an unrelated issue that forces a customer to downgrade.

Here is why:
If a customer rotates and reconfigures their databases to trust the new
DBUserCA, then downgrades to a version without DBUserCA support, there are
only two ways they can restore access to self-hosted databases:

1.  reconfigure their self-hosted databases (again) to trust the DatabaseCA
2.  OR immediately upgrade to a patched version

If they can’t downgrade to a supported version, nor can they immediately
upgrade to a supported version (2), then they have to reconfigure databases
(1).

If they reconfigure databases (1), then later upgrade to a patched version,
their db access will be broken again because DatabaseCA != DBUserCA and the
patched version will be signing agent client cert with DBUserCA, which forces
them to reconfigure databases yet again.

Therefore, we should try as much as possible to avoid a scenario where the
customer has to downgrade to a version that doesn't support the DBUserCA.


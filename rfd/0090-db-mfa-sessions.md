---
author: Gavin Frazar (gavin.frazar@goteleport.com)
state: implemented (v12.1.0)
---
 
# RFD 90 - Database MFA Sessions

## Required Approvers
- Engineering: Marek `@smallinsky`, Roman `@r0mant`
- Product: Sasha `@klizhentas`, Xin `@xinding33`

## What
Use a local proxy tunnel to define the lifetime of a database MFA session, so that database access users are not required to tap an MFA device every minute when per-session-MFA is enabled.

## Why
Database access via GUI client such as DataGrip requires the user to start a local proxy to connect to: `tsh proxy db --tunnel <db>`.

To support per-session-MFA for the local proxy, we are adding a callback function called on each new connection to the local proxy.
This callback function will check if the database certs need to be reissued to proxy the connection, and prompt for credentials/MFA if needed.

Currently, database certs are issued with 1 minute ttl if per-session-MFA is enabled as described in [RFD 14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md).
This causes poor UX when the GUI client can establish a new connection at any time,
and a user may be prompt once per minute for their MFA.

We should remove the 1 minute restriction on database cert TTL in this case to
improve UX.

## Details
When per-session-MFA is enabled, we should not always restrict database cert TTL to
1 minute.

Instead, database cert TTL should be restricted to `max_session_ttl`, and the cert
should be kept in-memory by a local proxy tunnel if possible.

"Doesn't this just disable per-session-mfa for database access?" (My initial thinking)

- No, not quite. Sessions are still limited by the lifetime of a local proxy process and an MFA prompt is always required to start these proxies when per-session-mfa is enabled.
- This can "weaken" per-session-mfa security though. If a user finds that they would prefer the old behavior of 1-minute TTL certs, they can still achieve that through configuration using `max_session_ttl`.

### API

We should change the auth service to issue database certs without capping the
cert TTL to 1 minute for per-session-MFA when the certs are requested for a
db local proxy tunnel, which will only keep the database certs in-memory.

### Local Proxy With Tunnel

Make the local proxy tunnel keep its database certs in-memory only.
This way, the certs are ephemeral and their lifetime is narrowed by the
lifetime of the proxy.

The local proxy can refresh its certs as needed by invoking a callback function
on start and on each new connection which checks the local proxy's db cert is
valid.
When it refreshes its certs, the local proxy tunnel will identify itself as the
cert requester and therefore per-session-mfa 1-minute TTL will not be applied.

### Local Proxy Without Tunnel

`tsh proxy db <database>` will not request certs on each new connection.
If this command triggers database login, the certs will still have 1-minute TTL.

### `tsh db connect`

Currently, `tsh db connect` starts a local proxy if any of the following are true:
1. TLS routing is enabled.
2. Proxying connection to a Microsoft SQL server
3. Proxying connection to a Snowflake database. 

The local proxy it starts only uses a tunnel for Snowflake and Elastisearch.

`tsh db connect` should connect via a local proxy with a tunnel if per-session-mfa is required.
This way database certs can be held in-memory.

This should not change the UX of starting `tsh db connect <db>`; there are basically two cases:
1. per-session-MFA is required for access to `<db>`:
Current behavior will always prompt the user for MFA - this is the same UX after the proposed change.

2. per-session-MFA is not required for access to `<db>`:
Current behavior will reissue the certs and only prompt the user for credentials if they need to relogin,
which is also the same UX as before. 

As an extra UX benefit after `tsh db connect <db>` has been started, this change allows the local proxy tunnel 
to re-establish connectivity if the connection dies.
For example, if the `mysql` cli is used and left idle for too long, the connection may die and require the user 
to establish a new connection.
Right now, the only way to establish a new connection is to kill the cli session <ctrl-d> and re-run
`tsh db connect` (and we do not currently hint the user to try this).
If the cli was instead connecting to a local proxy tunnel, the local proxy could just establish a new 
connection and prompt the user for credentials if needed.

Example of current behavior when connection dies:
```
mysql> select user();
No connection. Trying to reconnect...
ERROR 2013 (HY000): Lost connection to MySQL server during query
ERROR 2026 (HY000): SSL connection error: error:14094412:SSL routines:ssl3_read_bytes:sslv3 alert bad certificate
ERROR: 
Can't connect to the server

mysql> select user();
No connection. Trying to reconnect...
ERROR 2026 (HY000): SSL connection error: error:14094412:SSL routines:ssl3_read_bytes:sslv3 alert bad certificate
ERROR: 
Can't connect to the server

mysql> 
```

Example of new behavior when connection dies and credentials are required to establish a new connection:
(The "DEV: ..." messages are just dev print statements to visualize what is happening in the logic-flow)

```
mysql> select user();
No connection. Trying to reconnect...
ERROR 2013 (HY000): Lost connection to MySQL server during query
DEV: invoking local proxy on-new-conn callback
DEV: db cert expire time:  2022-09-20 18:51:37 +0000 UTC
DEV: time now:  2022-09-20 18:58:26.616242 +0000 UTC
DEV: cert is expired, so we should request cert reissue
DEV: prompting for MFA to reissue cert
Tap any security key
DEV: updating local proxy certs
Connection id:    10018
Current database: *** NONE ***

+---------------------------+
| user()                    |
+---------------------------+
| teleport50@108.81.245.225 |
+---------------------------+
1 row in set (2.98 sec)
```

### Teleport Connect

Teleport Connect will start a local proxy tunnel when per-session-mfa is enabled.
It will configure the local proxy callback function to notify the Electron
app when MFA is needed to reissue certificates; the app window can be brought to the top to prompt the user for
MFA.
This stands in contrast with a cli-based prompt, which may not raise any indicator that an MFA tap is required
when the cli is not visible on the user's screen.

The full details of how Teleport Connect will implement such a callback triggered MFA prompt are outside the
scope of this RFD, but a local proxy tunnel, with a callback function that is invoked on each new connection, 
is sufficient to enable this capability.

### Preserving Prior Behavior
We should be sure to preserve the behavior where db certs are saved to the filesystem when per-session-MFA is 
not required.
If we do not preserve this behavior, then `tsh db login` and `tsh proxy db` would be unusable commands.

### Security
This proposal only affects database access so we will focus on attack surface changes only for database access.

In proposing this change, we should address the security concerns that motivated [RFD 14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md):

1. Without per-session-mfa:
    - Current behavior:
    An attacker can use the client machine's secrets to access Teleport resources for `max_session_ttl` time if per-session-mfa is not required, by default 12 hours.
    - New behavior:
    No change.

2. With per-session-mfa:
    - Current behavior:
    An attacker only gains access to a database for up to 1 minute immediately after a user completes an MFA challenge.
    - New behavior:
    An attacker can gain access to a database for `max_session_ttl` time, narrowed by the lifetime of the local proxy process.
    The local proxy process itself represents a "session".
        - An attacker may still be able to extract the db cert from memory of the running local proxy, or simply intercept the cert sent by the local proxy when it dials the database.
        - Cert exfiltration is not needed though - anyone who can connect to the local proxy tunnel can connect to the database it proxies.
            - Note: tsh db connect will always open a local proxy tunnel too, whereas before it was only sometimes the case.
        - This was already a concern with `tsh proxy db --tunnel` or if `tsh db connect` starts a local proxy tunnel.

### Integrating with PIV hardware private keys for security improvements
[yubikey PIV integration](https://github.com/gravitational/teleport/blob/master/rfd/0080-piv-private-key.md)
brings Personal Identity Verification (PIV) support. This allows for hardware-stored private keys.

The local proxy tunnel can integrate with PIV yubikeys via the keystore interface.
It should work like any other private key except that the yubikey must be connected and the user may need
to pass a presence check to allow the local proxy to access the private key.
Consider the cases where these additional requirements for key usage are not met:
1. Problem: the hardware key is not inserted.
   Expected behavior: the local proxy tunnel callback function will prompt the user to insert their PIV key
   with a timeout. If the key is not inserted before the timeout, then the connection attempt fails but
   the local proxy continues to listen for new connections.
2. Problem: the user does not tap the key when a "presence check" requires them to.
   Expected behavior: the same timeout applies, and the connection is simply dropped if they do not
   tap their yubikey.

In both cases, the prompt will be issued via cli prompt as usual or via a pop-up-style notification
with Teleport Connect.

#### PIV security benefits
The main concern with removing the 1 minute TTL for database certificates is that exfiltration of the user's
certificate and private key would allow an attacker to access database resources for a much longer duration 
than 1 minute.
Keeping db certificates in memory will make exfiltration harder; it would effectively require that the user's 
machine be totally compromised by an attacker.
Even with this additional layer of defense, a user may be concerned to know that per-session-mfa can be
bypassed by exfiltration.

PIV hardware private keys prevent private key exfiltration from being possible.

That leaves session hijacking as an attack vector, and local proxy tunnel session hijacking in particular.
The RFD mentions two mitigation options for for this:

 1. Enable [per-session MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa/), which requires you to pass an MFA check (touch) to start a new Teleport Service session (SSH/Kube/etc.)
 2. Require Touch to access hardware private keys, which can be done with PIV-compatible hardware keys. In this case, touch is required for every Teleport request, not just new Teleport Service sessions

The first option will not help us here, since we are extending the lifetime of a session to be
min(`max_session_ttl`, local proxy tunnel lifetime) instead of 1 minute. It also won't help because
the local proxy tunnel accepts local tcp connections without TLS.

The second option, which requires a presence check to use the private key, will not completely prevent session 
hijacking either.
The local proxy tunnel listens for tcp connections without TLS; if a connection closes or dies,
then an attacker trying to connect via the local proxy tunnel will fail to pass the presence check needed
to perform the upstream TLS handshake for a new connection.
However, active tcp connections to the local proxy tunnel could still be hijacked.
Furthermore, an attacker capable of connecting to the local proxy could in theory attempt to connect to the
local proxy while the user is actually present, and trick them into tapping their yubikey to pass the presence 
check.
A presence check required for each new connection will also eliminate any UX improvement we would have gained 
by removing the 1 minute cert ttl, because it's even more restrictive than per-session-mfa TTL limited
certificates.

Since a presence check does not completely prevent local proxy tunnel session hijacking and it eliminates
the UX improvement of this RFD, I would not recommend users enable presence checking for database
access, but they at least have the option to do so anyway.

In summary, there are two main security benefits of yubikey PIV integration for per-session-mfa database-access:
1. Exfiltration of the user's private key is not possible.
2. If the user removes their hardware key from the machine, then certificates cannot be reused for *new*
   connections by the local proxy tunnel.
   (active connections have already negotiated mTLS, so these sessions are still vulnerable after the key is 
   removed by hijacking the tcp connection to the local proxy)

An attacker capable of hijacking local connections on a user's machine cannot be *fully* mitigated;
this is simply unavoidable. But with PIV we can prevent key exfiltration and narrow the attack surface area
to only active local proxy tunnel connections to specific resources.

## Alternatives

Here are some alternatives we can do. They are not necessarily mutually-exclusive options:
1. We do nothing.
   A user may be prompted up to once per minute to refresh their db cert by the local proxy tunnel, or they can disable `require_session_mfa`.
2. We only make this change specific to Microsoft SQL Server.
   In practice, this has been the protocol causing the most excessive MFA prompts for our local proxy tunnel, because SQL server db clients frequently try to establish new connections.
3. We add a per-session-mfa TTL to cluster auth preference. This way, a Teleport admin can specify that any resource which requires per-session-mfa should be hard-limited to a shorter TTL.
   - This is not necessary since `max_session_ttl` can be adjusted for specific resources, but it could be easier for the user to configure a cluster-wide "set the cert TTL to X for all resources that require per-session-mfa".
4. We don't bother with keeping certs in memory. Pros/cons for keeping certs in memory:
   - Pro: Keeping certs in memory does make cert exfiltration harder (though not impossible, especially in a garbage collected language where hardening against memory dumping is extremely difficult)
   - Pro: Keeping certs in memory makes it clear that a "session" is limited by a user starting a local proxy, so users can't reuse certs for many sessions - essentially we are considering a local proxy process as a single session.
   - Con: Even if we could rely on memory to prevent exfiltration, a cert is not a secret! It is necessarily sent unencrypted to do the TLS handshake - a MITM attack can steal the cert when client tries to connect to server.
   - Con: If the key itself is still on disk, then it's actually easier to exfiltrate the private key than it is to steal the public cert from memory.
     - This is mitigated by work on keeping private keys in yubikeys, in which case the private key would be safe and stealing a cert won't help an attacker.
   - Con: Keeping certs in memory requires a local proxy tunnel, and if an attacker can already read the client ~/.tsh keys, they can almost certainly just connect to this running proxy and gain access.
     - IP pinning doesn't help against a remote attacker either.
5. We keep the private key in memory as well (or possibly only keep the private key in memory).
   - Pro: stealing the private key is as hard or harder than stealing the public cert, which makes more sense.
     - Not useful if we can have the private key stored on a yubikey though.
   - Con: Worse UX. The user will have to relogin every time they `tsh db connect` or `tsh proxy db --tunnel`, since we keep the private key issued to them in memory and it is thrown away when they end a session.

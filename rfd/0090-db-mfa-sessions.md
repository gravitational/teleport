---
author: Gavin Frazar (gavin.frazar@goteleport.com)
state: draft
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

We should remove the 1 minute restriction on database cert TTL to improve UX.

## Details
When per-session-MFA is enabled, we should not restrict database cert TTL to 1 minute.

Instead, database cert TTL should be restricted to `max_session_ttl`, and the cert
should be kept in-memory by a local proxy tunnel.

"Doesn't this just disable per-session-mfa for database access?" (My initial thinking)

- No, not quite. Sessions are still limited by the lifetime of a local proxy process and an MFA prompt is always required to start these proxies when per-session-mfa is enabled.
- This can "weaken" per-session-mfa security though. If a user finds that they would prefer the old behavior of 1-minute TTL certs, they can still achieve that through configuration using `max_session_ttl`.

### API

We should change the auth service to always issue database certs without capping the cert TTL to 1 minute for per-session-MFA.

### Local Proxy With Tunnel

Make the local proxy tunnel keep its database certs in-memory only, and to always request for cert reissue when the local proxy starts.
This way, the certs are ephemeral and their lifetime is narrowed by the lifetime of the proxy.

Right now, we load certs from the filesystem when using `tsh proxy db --tunnel`.
So this change will affect per-session-MFA UX flow in that `tsh proxy db --tunnel` will always prompt for MFA when the local proxy starts.

The local proxy can also refresh its certs as needed by invoking a callback function on each new connection which checks the local proxy's db cert expiry.

### Local Proxy Without Tunnel

If per-session-MFA is required, then we should forbid `tsh proxy db <database>` and give the user an error message telling them to add the `--tunnel` flag since per-session-MFA is required.

This prevents per-session-MFA database certs from being saved to the filesystem and directs users towards the better UX of using a local proxy tunnel.

### `tsh db login`

When per-session-MFA is required, we should have behavior like that of `tsh proxy db <database>`: Error message and tell the user to simply connect with `tsh db connect` instead.
This is for the same reason: prevent database certs from being saved to the filesystem when per-session-MFA is required.

It's also better UX, because if we allowed them to use `tsh db login` for per-session-MFA then they would immediately be re-prompted for MFA by `tsh db connect`, which is confusing and annoying behavior.

### `tsh db connect`

Currently, `tsh db connect` starts a local proxy if any of the following are true:
1. TLS routing is enabled.
2. Proxying connection to a Microsoft SQL server
3. Proxying connection to a Snowflake database. 

The local proxy it starts only uses a tunnel for Snowflake and Elastisearch.

Instead, we should always use a local proxy and always with a tunnel.
This way database certs can be held in-memory.

This should not change the UX of starting `tsh db connect <db>`; there are basically two cases:
1. per-session-MFA is required for access to `<db>`: Current behavior will always prompt the user for MFA - this is the same UX after the proposed change.
2. per-session-MFA is not required for access to `<db>`: Current behavior will reissue the certs and only prompt the user for credentials if they need to relogin - again, same UX.

As an extra UX benefit after `tsh db connect <db>` has been started, this change allows the local proxy tunnel to re-establish connectivity if the connection dies.
For example, if the `mysql` cli is used and left idle for too long, the connection may die and require the user to establish a new connection.
Right now, the only way to establish a new connection is to kill the cli session <ctrl-d> and re-run `tsh db connect` (and we do not currently hint the user to try this).
If the cli was instead connecting to a local proxy tunnel, the local proxy could just establish a new connection and prompt the user for credentials if needed.

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

### Preserving Prior Behavior
We should be sure to preserve the behavior where db certs are saved to the filesystem when per-session-MFA is not required.
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
        - This was already a concern with `tsh proxy db --tunnel`, although that command effectively does not work with per-session-mfa after 1 minute.
            If we implement this proposal, then `tsh proxy db --tunnel` will work with per-session-mfa and will allow anyone who can connect to the local proxy to connect to the database it proxies until the cert or session expires.

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
     - Not useful if can have the private key stored on a yubikey though.
   - Con: Worse UX. The user will have to relogin every time they `tsh db connect` or `tsh proxy db --tunnel`, since we keep the private key issued to them in memory and it is thrown away when they end a session.

---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 33 - Clients Certificate Rotation

## What

Clients should be able to rotate their own certificates once they expire. Clients use
User certificates, which can expire either due to reaching their EOL (determined by TTL),
or due to the Auth server rotating its User CA. 

## Why

Enabling clients to rotate certificates allows several key security features.

**Short lived certificates:**

Short lived certificates are more secure than long lived certificates due to the time
window granted for access. There are many real life scenarios that may occur where
the system is compromised due to a long lived certificate (point to example or docs).

However, short lived certificates must be frequently reissued. Currently, the only way
to do this for user certificates is to manually reissue them by logging in or running
the auth server command `tctl auth sign`.

**Lock out a user by revoking rotation rights:**

Currently, the only way to lock out a user is to rotate the CA. However, this revokes
all certificates rather than just the targeted one.

If users are expected to rotate their own CAs on a short schedule (~30 min), then we 
can revoke access for that user by revoking their right to rotate certificates.

## Details

### CA rotation API

The Auth API must provide a way to discover the CA's certificate rotation state
in order to enable client certificate rotation.

This can be made into a new RPC, or a wrapper function around stream watcher (if this is possible).

```go
client.WatchCARotationState()
```

### Client Credentials auto rotate

#### Credentials update certificates before expiration

Client watches for upcoming expiration events
 - When expiration from TTL is approaching
 - When the CA is in RotatePhaseUpdateClients 

Call the following RPC when the certs are going to expire:

```go
client.GenerateUserCerts()
```

This RPC uses an existing Key. Rather than moving Key generation code to the client,
it will be made into an RPC as well

```go
client.GenerateNewKey()
// Alternatively generate a new key and certs in the same RPC
client.GenerateUserCertsWithNewKey()
```

##### TTL concerns

Client certificates TTLs must be forced to be short to ensure this functionality is secure.
There are two options I have in mind:
 1. TTL/CA state watching occurs within the same command as the original certificate issue.
    - ex. `tsh login --autorotate` which will set TTL to a value defined by auth and start
      a service to watch both the CA rotation state and the TTL using the certificates generated. 
 2. The client refuses to auto rotate certs if the TTL defined is too long, or future certs
    are just given a shorter TTL.

#### Credentials notify the client to reload

The client's credentials must detect changes to its certificates and notify the client to
update them. This can be done by watching the file's metadata or contents for changes. The 
credentials will also watch their own certificates' TTL and notify the client before they expire.

When a change is detected, the client will follow this decision tree:
 - are they the active credentials?
  - (yes) are they still valid?
    - (yes) are they about to expire?
      - (yes) retrieve new certs and replace the old ones (in disk), update client connection
      - (no) certificates were updated by an external source, update client connection
    - (no) Reinitialize client connection trying other credentials. Update the original
      active credentials if possible.
  - (no) are they about to expire?
    - (yes) retrieve new certs and replace the old ones (in disk)
    - (no) continue as is

#### Subscription model

This can also be done through a subscription model for better separation of concerns. 
The client will subscribe to credential change events, and the credentials will subscribe 
to auth server state changes from the client. The latter will use the client via closure
to reissue certs.
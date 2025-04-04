---
authors: Alan Parra (alan.parra@goteleport.com), Erik Tate (erik.tate@goteleport.com) 
state: draft
---

# RFD 127 - Encrypted Session Recordings

## Required Approvers
* Engineering: @rosstimothy, @zmb3, @espadolini, @nklaassen
* Security: doyensec

## What

This document proposes an approach to encrypting session recording data before
writing to disk or any long term storage.

## Why

Recordings temporarily stored to disk can be easily tampered with by users with
enough access. This could even occur within the session being recorded if the
host user has root access.

Encrypting session recordings at rest can help prevent exposure of credentials
or other secrets that might be visible within the recording. 

## Details

This document should fulfill the following requirements:
- Ability to encrypt session recording data at rest in long term storage and
during any intermediate disk writes.
- Ability to replay encrypted sessions using the web UI.
- Ability to source key material from an HSM or other supported keystore.
- An encryption algorithm suitable for this workload.

### Encryption Algorithm

This RFD assumes the usage of [age](https://github.com/FiloSottile/age), which
was chosen for its provenance, simplicity, and focus on strong cryptography
defaults without requiring customization. The formal spec can be found
[here](https://age-encryption.org/v1). Officially supported key algorithms are
limited to X25519 (recommended by the spec), Ed25519, and RSA. Support for
other algorithms would either have to be requested from the upstream or
manually implemented as a custom plugin.

### Config Changes

Encrypted session recording is a feature of the auth service and would be
enabled by setting `session_recording_encryption: on` within the `auth_service`
file config.
```yaml
# teleport.yaml
version: v3
auth_service:
  session_recording_encryption: on
```
HSM integration is facilitated through the existing configuration
options for setting up an HSM backed CA keystore through pkcs#11. Example
configuration found [here](https://goteleport.com/docs/admin-guides/deploy-a-cluster/hsm/#step-25-configure-teleport).


### Session Recording Modes

There are four session recording modes that describe where the recordings are
captured and how they're shipped to long term storage.
- `proxy-sync`
- `proxy`
- `node-sync`
- `node`
Where the recordings are collected is largely unimportant to the encryption
strategy, but whether or not they are handled async or sync has different
requirements.

In sync modes the session recording data is written immediately to long term
storage without intermediate disk writes. This means we can simply instantiate
an age encryptor at the start of the session and encrypt the recording data as
it's sent to long term storage.

In async modes the session recording data is written to intermediate `.part`
files. These files are collected until they're ready for upload and are then
combined into a single `.tar` file. In order to encrypt individual parts, we
will build a special `io.Writer` that contains a single instance of the age
encryptor that can proxy writes across multiple files. This will require
maintaining a concurrency-safe mapping of in-progress uploads to encrypted
writers which incurs a bit of added complexity. However it intentionally avoids
any sort of intermediate key management which seems a worthwhile tradeoff.

### Protocols

We record sessions for multiple protocols, including ssh, k8s, database
sessions and more. Because this approach encrypts at the point of writing
without modifying the recording structure, the strategy for encryption is
expected to be the same across all protocols.

### Encryption

At a high level, `age` encryption works by generating a per-file symmetric key
used for data encryption. That key is wrapped using an asymmetric keypair and
included in the header section of the encrypted file as a stanza. Plugins
implementing different key algorithms only affect the crypto involved with
wrapping and unwrapping data encryption keys.

### Key Generation

In order to generate keys, a new session encryption CA will be added. The CA
will be configured through the existing `auth_service.ca_key_params` block.
A new `Ed25519` keypair will be generated and added to the CA's active keyset.
For keystores that support wrapping, including HSMs, the private key will be
wrapped before being added. The public key will be used by `age` to wrap data
encryption keys at the point of encryption, and the private key will be used
within the auth service to decrypt during playback.

It's suggested by `age` that `X25519` keypairs be used in cases where new key
material is being generated. `Ed25519` was chosen for this implementation
because it is already officially supported by both Teleport and `age`.

### Decryption and Replay

Because decryption will happen in the auth service before streaming to the
client, the UX of replaying encrypted recordings is no different from
unencrypted recordings. The auth server will pull the active primary key from
the session encryption CA, unwrap it if necessary, and decrypt on
the fly as data is streamed back to the client. This should be compatible with
all replay clients, including web.

### Key Rotation

Key rotation of the wrapping keypair is possible by leveraging the active
keyset and and additional trusted keyset already present on CAs generated by
Teleport. A new `Ed25519` keypair would be generated, wrapped if necessary, and
stored in the active keyset. The original pair can then be moved to the
additional trusted keyset which will be kept indefinitely. After propagation of
the CA changes, all new session recordings will be encrypted with the new
active key. Historical recordings remain decryptable by their original key
found in the additional trusted keyset. The per-session data encryption keys
used to perform the actual encryption will not be rotated.

### Security

Protection of key material invovled with encrypting session recordings is
largely managed by our existing CA and keystore features. The one exception
being the private keys used by `age` to decrypt files during playback. When
possible, those keys will be wrapped by the backing keystore such that
decryption related secrets are not directly accessible.

One of the primary concerns outside of key management is ensuring that session
recording data is always encrypted before landing on disk or in long term
storage. In order to help enforce this, all session recording interactions
should be gated behind a standard interface that can be implemented as either
plaintext or encrypted. This will help ensure that once the encrypted writer
has been selected, any interactions with session recordings are encrypted by
default.

## UX Examples

For the most part, the user experience of encrypted session recordings is
identical to non-encrypted session recordings. The only differences come into
play when configuring keystores backing the CA which are already established
features.

### Teleport admin replaying encrypted session using tsh
```bash
tsh play 49608fad-7fe3-44a7-b3b5-fab0e0bd34d1
```

### Test Plan
- Sessions are recorded when `auth_service.session_recording_encryption: on`.
- Encrypted sessions can be played back in both web and `tsh`.
- Encrypted sessions can be recorded and played back with or without a backing
keystore.


---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# Local Encryption of Session Data

## What

Provide encrypted-at-rest session recordings that do not rely on a cloud provider's server-side encryption.

## Why

Most, if not all, cloud storage providers offer an encrypted storage solution where data is automatically encrypted on the server side during upload and decrypted again during download. By necessity, the key(s) used to encrypt the data on the server side _must_ be known to cloud storage provider. 

Some clients, for whatever reason, do not feel comfortable sharing their encryption key(s) with a cloud provider and would prefer a client-side solution where the encryption key(s) do not leave their possession.

This RFD gives proposes a solution based on envelope encryption with a locally-supplied master key. It fits neatly over the existing session recording mechanism outlined in [RFD-002](./0002-streaming.md).

## Details

### Proposed Solution

 1. Create a new `V2` slice format that includes
    1. a slice flagset to indicate whether the payload is encrypted
    2. an optional encrypted envelope around the gzipped slice payload
 2. Adding configuration items to to the teleport config file to supply keys.

#### Cipher selection

Both content and content-encryption keys will be encrypted with AES256/GCM. No user selection of the cipher suite will be permitted. 

#### Configuration

Keys will be configured in the Teleport config file, under the `teleport/storage` block. The new `audit_sessions_local_keys` item will hold a list of keys. 

The key format is modelled on the token format. Each key will be a colon-separated string, containing in-order
1. An optional comma-separated list of tags
2. An arbitrary text key ID
3. a base64-encoded string holding the key bits, or the path to a file containing the same. Paths will be identified by a leading `/`.

Only one tag is defined: `main`. Keys tagged with `main` may be used as Key-Encryption Keys to encrypt recordings. Untagged keys may only be used for decryption. There must be exactly one `main`-tagged key in the keyset.

Example YAML:

```yaml
teleport:
  storage:
    audit_sessions_uri: s3://teleport/recordings?region=ap-southeast-2
    audit_sessions_local_keys:
      - some-key:main:AgqHAQf+tym2eV9iSKWS5TMvcaKYFFX5F3DQxICWvKk=
      - another-key:GhBT5ktJiXGaxnFyAmzRkt0K
      - key-in-a-file:/path/to/some/key
```
If a `audit_sessions_local_keys` entry is not present, the system will fall back to the existing gzipped protobuf format.

If the `audit_sessions_local_keys` is malformed in any way, including any key files it references, Teleport will fail to start and alert the user with an error message.

#### Encryption & Decryption

As per [RFD-002](./0002-streaming.md), the recording system gathers session events and compresses them into _slices_. Once the compressed slice reaches a minimum size (currently 5 MiB) the chunk is wrapped with a slice header and stored, before moving on to the next slice.  This finalization step provides a natural place to encrypt the slice. When operating in encrypted mode, the slice writer will 
 1. Envelope encrypt the gzipped event records using the active encryption key
 2. Rewrite the slice record to treat the resulting ciphertext as the slice payload, and 
 3. Pass it though to the existing storage machinery

The same process happens in reverse. When the recording reader encounters a new slice record, it will
 1. Examine the slice to see if the slice is encrypted
 2. If so, extract the Key-Encryption Key ID from the envelope header (see below)
 3. Map the key ID to a key supplied in the config file, and use that key to decrypt the envelope
 4. Pass the resulting plaintext (i.e. a gzipped series of protobuf records) along as per the existing recording system.

Each slice may be encrypted with a different master key, even in the same file. This allows multiple nodes with different encryption keys to write to the same session. The session will be played back as long as the node managing the playback has access to all the keys used for the session.

Reusing the existing recording machinery also allows session files in transit to be recorded at rest whenever they are stored locally on a node (e.g. async recording).

#### V2 Proto Recording Format

[RFD-002](./0002-streaming.md), shows that a recording file is a sequence of semi-independent _slices_. Each slice contains a gzipped sequence of session events serialised into the protobuf wire format. Each slice is at least (and probably close to) 5MiB for all but the _last_ slice in a recording, which may be any size. No upper bound on slice size is currently enforced.

The V2 format will add a `flags` field after the version header. For consistency's sake, this will be another Uint64. The new header will look this:

| Field Size (bytes) | Encoding |         Interpretation              |
|--------------------|----------|-------------------------------------|
|                  8 |BE Uint64 | Slice format version. Currently `1` |
|                  8 |BE Uint64 | Flags                               |
|                  8 |BE Uint64 | Payload size (Ss), in bytes         |
|                  8 |BE Uint64 | Padding size (Sp), in bytes         |
|                 Ss |   []Byte | Payload                             |
|                 Sp |   []Byte | Padding (ignore)                    |

If the `Encrypted` flag is ***not*** set (see "Flags", below), the payload treated as a regular, gzipped payload as in RFD-0002. 

If the `Encrypted` flag ***is*** set, the payload contains an envelope header and encrypted data. The Key ID in the envelope header identifies the master key used to encrypt the envelope. The remainder of the payload is the encrypted, gzipped session events.

The envelope header (see "envelope header", below) is treated as part of the payload, and included in the payload byte count.

The `Flags` field is left open for future expansion.

##### Defined Flags
|                  Flag |      Name | Interpretation                              |
|-----------------------|-----------|---------------------------------------------|
| `0x00000000000000001` | Encrypted | Payload is wrapped in an encrypted envelope |

##### Envelope header 
| Field Size (in bytes) | Encoding  | Interpretation                              |
|-----------------------|-----------|---------------------------------------------|
|                     4 |    Uint32 | Key ID Length (Sk)                          |
|                    Sk |    []Byte | Key ID Length                               |

The key ID is an arbitrary string used to identify which key was used to encrypt the slice Date Encryption Key. Tink uses the notion of a "Key URI" across its various keystore integrations, so that the same mechanism can be used to identify a key regardless of how it is stored. We could use a simple scheme like `teleport://$KEY_ID_FROM_CONFIG_FILE`.

#### Envelope Encryption Implementation

NOTE: for the most part, this is an implementation detail. I'm assuming we will make a decision and delete this section before this RFD is adopted. In terms of implementation risk, I have a working proof of concept implemented with Tink.

There are two competing options for implementing the envelope encryption:

 * Defining our own envelope & key management system, or
 * Using [Google Tink](https://developers.google.com/tink)

##### Defining our own
- **Pro:** Can be implemented using the tools in the standard `crypto` and `crypto/cipher` packages
- **Pro:** Easier to verify for FIPS compliance as we don't have to wade through 3rd party code
- **Con:** Have to write our own envelope encrypter, envelope format de-/serialiser & keystore. 

None of this is particularly *difficult* (and we can gain a ***lot*** of useful inspiration from Tink), but it is worth noting that it needs to be done.

##### Tink
 - **Pro:** Integrated key management solution, should we later want to use a cloud-based key manager (e.g. Cloud Key Manager, AWS KMS) to manage master keys
 - **Pro:** Pre-defined & implemented envelope format (uses protobuf under the hood, adds ~130B per slice)
 - **Pro:** Plumbing between envelope decrypter and keystore is provided
 - **Pro:** More intuitive API than `crypto` & `crypto/cipher`
 - **Con:** Managing transitive third-party dependencies. Getting the PoC running was a challenge due to differing updated GRPC 
 - **Con:** Does not support a locally-provided key out of the box. We still have to write our own keystore integration.
 - **Con:** Harder to assert FIPS compliance. Unknown how it will react to `boringcrypto` builds.

I'm personally leaning towards the Tink implementation, but I can be convinced either way.

## Outstanding security issues

### Encrypting Compressed Data

Encrypting compressed data [has been exploited before](https://en.wikipedia.org/wiki/CRIME) with a chosen plaintext attack. I'm not sure how relevant this is to the Teleport use case, but in any case it needs review.

The tradeoff  is against a slightly more invasive change to the recording system, and much larger session files. 

### Playback

#### Decompressed Playback Files
When a session is played back from a recording the session recording is filtered and rewritten into a format optimized for playback (plain text JSON). This subset of the session is then streamed to the viewer via GRPC API. A cleanup process that runs every 5 minutes automatically deletes uncompressed recordings that have been unwatched for more than an hour.  This leaves a subset of the session saved to in plain text on the node for some small amount of time. 

#### Viewers Themselves
Once the session is streamed to the viewer for playback it is beyond our control, and the viewer can potentially distribute it however they wish.

## FAQs

### Why not just use AWS client-side encryption?

Ideally we would be able to flip a switch on the `S3` client and use client-side encryption. This is not a feasible solution for Teleport, for several reasons:

 1. The AWS SDK for Go [does not directly support client-side encryption using purely local keys](https://github.com/aws/aws-sdk-go/issues/1241), which is counter to the stated goal of not having the encryption keys leave the client's possession. The SDK _does_ provide the appropriate hooks so we could implement implement it ourselves, but is a nontrivial amount of work.

 2. The  encrypted clients in the AWS SDK for Go do not support [Chunked Transfer Encoding](https://en.wikipedia.org/wiki/Chunked_transfer_encoding). The entire session streaming system in Teleport is built on chunked transfer, so any solution that does _not_ support chunked transfer not fit for purpose.
 
 3. The  encrypted clients in the AWS SDK for Go [use the Go standard library AEAD implementation](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3crypto/#AESGCMContentCipherBuilderV2). This requires that the entire object be loaded into memory before en- and decryption. Given that session recordings can run into the Gigabytes, this is not a a restriction that we can tolerate.

The other neat thing is that this provides a common solution to all storage back ends, even simple files. 

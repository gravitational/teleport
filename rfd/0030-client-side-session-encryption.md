---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# Local Encryption of Session Data on S3

## What

Provide encrypted-at-rest session recordings that do not rely on AWS' server-side encryption.

## Why

When Teleport uploads a session recording to S3 it is (optionally) encrypted at rest by S3 server-side encryption. 

For whatever reason, some Teleport users do not want to share encryption keys with AWS. To this end, S3 offers the option of encrypting the data on the client side during the upload process, using client-managed keys. We can use this service to provide at-rest encryption on S3, without sharing keys with Amazon.

In a perfect world we would be able to flip a switch on the S3 Client to enable this. Unfortunately, several limitations in the AWS SDK for Go (discussed below) make this change less trivial that we had hoped.

## Details
### Background

There are two keys involved in storing an object on S3 with client-side encryption

 1. The _Master Key_, also known as the _Key Encryption Key_ or _KEK_. This is the ultimate secret that is required to decrypt an object.
 2. The _Session Key_, also known as the _Content Encryption Key_, or _CEK_. This is a one-off key used to encrypt the object content. This key is in turn encrypted using the KEK and embedded into the stored object.

When an object is downloaded the encrypted CEK is extracted from the object and decrypted using the master key, and then the CEK is then used to decode the actual content.

The overall process looks something like this:


```
                                        ┌─────────\
                                        │ Content │
                                        └────┬────┘
                             Content         │         Encrypted
               ┌───────────┐   Key   ┌───────▼──────┐    Object
     ┌─────────►    Key    ├─────────►  Encryption  ├────────────────┐
     │         │  Wrapper  │         │    Client    │                │
┌────┴────\    └───────────┘         └──────────────┘           ┌────▼───┐
│  Master │                 Metadata                            │   S3   │
│   Key   │           ┌─────────────────┐                       │ Bucket │
└────┬────┘    ┌──────▼────┐         ┌──┴───────────┐           └────┬───┘
     └─────────►    Key    │         │  Decryption  ◄────────────────┘
               │ Unwrapper │         │    Client    │   Encrypted 
               └──────┬────┘         └──▲────┬──────┘  Object & S3
                      └─────────────────┘    │          Metadata
                           Content Key       │
                                       ┌─────▼─────\
                                       | Decrypted │
                                       │  Content  │
                                       └───────────┘
```

### Key Management

AWS provides two options for managing the master encryption key:
 1. The Amazon Key Management Service, where the master key is store remotely and retrieved from KMS prior to upload, and 
 2. A locally-managed key that never leaves the client side.

Given that one the driving goals of using client-side encryption is to avoid having to share the master encryption keys, this document will concern itself with option _2_.

Unfortunately, the AWS SDK for Go [does not support client-side encryption using purely local keys](https://github.com/aws/aws-sdk-go/issues/1241). It does, however, provide the appropriate hooks so we can implement it ourselves. For more details, see the "Design & Implementation" section below.

### Performance concerns

#### Memory Usage

The  encryption clients in the AWS SDK for Go [use the Go standard library AEAD implementation](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3crypto/#AESGCMContentCipherBuilderV2). This requires that the entire object be loaded into memory before en- and decryption. Uploading or fetching a sufficiently large session file may result in degraded performance, or could be used as a DoS attack vector. We may have to impose a limit on the recording size to mitigate this risk.

There _are_ [streaming AEAD alternatives available](https://github.com/google/tink/tree/master/go/streamingaead). Wrapping this in a custom [`ContentCipher`](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3crypto/#ContentCipher) in order to make it available for the AWS SDK to use, _can_  definitely be done. It may, however, be better to implement the smallest required change (i.e. the key wrapper) first and then assess the need later.

#### Tempfile usage

The encryption machinery uses tempfiles to store partial results between encryption & upload, as well as download & decryption.
#### Upload efficiency

The current session uploader uses the `s3manager` type to manage the efficient,, concurrent upload of large objects. The `s3manager` creates its own upload and download clients internally, and does not appear to allow us to substitute the `EncryptionClient` and `DecryptionClient` required for client-side encryption.

This means that the implementation for the encrypted uploader and downloader will have to be more involved than the current handlers that use the `s3manager`, and that there may be a performance hit when using client-side encryption beyond the obvious cost of the en- & decryption itself.

## Design and Implementation

### Cypher selection

The AWS client code supports both symmetric and asymmetric encryption.

Using an asymmetric cypher would be more secure, in that the client would not have to distribute a private decryption key to the nodes doing the encryption. Using an asymmetric cypher would, however, totally break the session playback system. Teleport would have no way of decrypting the sessions in order to play them back.

For that reason, and for general simplicity, we will proceed under the assumption that only symmetric keys will be supported.

The default Key-encryption algorithm for symmetric keys is AES-256, which is the appropriate choice for our application.
### Configuration

In order to activate client-side encryption, the user will specify 256-bit encryption key in the teleport config file. 

The key will be set in the `teleport/storage` block. The value may be either a hex string, or a path to a file _containing_ a hex encoded key.

```yaml
teleport:
  storage:
    audit_sessions_uri: s3://teleport/recordings?region=ap-southeast-2
    audit_sessions_local_key: "020a870107feb729b6795f6248a592e5332f71a2981455f91770d0c48096bca9"
```
    
If the key is supplied, Teleport will use client-side encryption, with the supplied key acting as the master key. 

If no key is supplied, Teleport will use the `s3manager` upload  & download clients. Any server-side encryption is largely dependant on the target bucket settings and is outside the scope of this document. 

### Key Wrapping Algorithm

The AWS SDK uses interfaces defined in the `s3crypto` package to define key wrapping algorithms. If we provide a compatible key wrapping and unwrapping mechanism, we _should_ be able to use the existing SDK machinery to create and retrieve client-side-encrypted object in a manner compatible with other clients.

The Java SDK uses the well-defined [RFC3394 AESWrap](https://datatracker.ietf.org/doc/html/rfc3394) key wrapping algorithm. [Several](https://pkg.go.dev/github.com/chiefbrain/ios/crypto/aeswrap) [implementations](https://pkg.go.dev/github.com/dunhamsteve/ios/crypto/aeswrap) [of RFC3394](https://pkg.go.dev/github.com/gwatts/ios/crypto/aeswrap) in Go already exist.

Integrating the key wrapping algorithm would require
 1. Selecting an implementation (or implementing our own, but probably not) of RFC3394
 2. Wrapping the RFC in a type 
   * that knows the master key, and 
   * implements the `s3crypto.ContentCipherBuilder` and `s3crypto.CipherDataDecrypter` interfaces
 3. Registering the `CipherDataDecrypter` in an [`s3crypto.CryptoRegistry`](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3crypto/#CryptoRegistry) so that the S3 SDK can use it to decrypt objects on download.

### Integration

The actual integration build down to a straightforward exercise in abstraction:

1. Define a common interface for session uploading and downloading
2. Wrap the Key wrapper, Crypto registry, the en- & decryption clients, etc into something that exposes that interface
3. Wrap the existing `s3manager` in something that exposes the same interface
4. Swap out the implementation based on the existence of the master key in the config file

## Alternative Implementations

### Custom KeyPadder

If we are not interested in preserving compatibility with the AWS SDK for other languages, we can easily define a custom key wrapeer using the PCKS7 padder in the AWS SDK. This would remove the need for the AESWrap implementation, but would also make the encryption format Teleport-specific. Other AWS clients would not be able to interpret the at all.

All of the other changes would still need to happen.

If we went down this path, it would be straightforward to create a bulk download & decrypt tool, should this become a pressing issue.

### Manual encryption of session recordings

A second alternative is to bypass the whole idea of encryption via AWS, and define an encrypted file format for recordings. This is probably more complicated than plugging into AWS, but it does have a few advantages:

 * With careful cipher selection we can remove the requirement to fit the whole file in memory, temp file usage, etc, imposed by using AWS
 * It's a more general solution: all of the other storage system - including local disk - would inherit the functionality

More work would be needed to define a useful file format, but there are plenty of container formats to use as models.

## Further work
### Streaming AEAD Implementation

Beyond the scope of this RFD

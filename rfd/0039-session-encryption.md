---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 38 - Session encryption

## What

Support encryption of Parquet files and session recordings when stored on cloud services [#6311](https://github.com/gravitational/teleport/issues/6311)

## Why

Customers are are concerned about sensitive data stored within recorded sessions and rightfully so. Security is currently completely in the hands of the session storage provider and their at-rest encryption. Customers want additional security by encrypting the session data before upload.

### Details

#### Client encryption or cloud provided (AWS KMS)

I evaluated some cloud provided encryption solutions like AWS KMS and compared them to plain client side encryption. Interoperability with cloud specific tools is a major benefit of hooking into cloud solutions like AWS KMS but it comes with the burden of `M * N` implementation cost. In essence, we need to provide a custom implementation for each storage backend which will quickly add up to an immense amount of code and complexity. A client side fallback is necessary anyway since we support all S3 compatible providers but most S3 providers do not support any means of native encryption like AWS does. For that reason that leaves using client side encryption as the only viable alternative currently.

This does hurt interoperability slightly since it locks users out of the AWS KMS hooks in other AWS tools but I believe we can provide a good experience anyway and adding some lightweight options to `tctl` for messing with encrypted files if needed.

#### Encryption algorithms

The selected encryption algorithms must be considered industry standard and secure but also must comply with FIPS PUB 140-3 and thus the algorithms must be included in SP 800-140C (as of March 2020) in order to comply with government regulations.

The symmetric cipher that will be used for bulk encryption shall thus be AES-CBC-HMACSHA256 mode (provides authentication) and any eventual checksumming that has to be performed in the implementation should be done with a newer algorithm in the SHA3 family such as SHA3-256.

AES-CBC-HMACSHA256 was chosen because it is a popular AEAD construct supported by various libraries including Google Tink which can be built to rely on BoringCrypto primitives.

This is the default cipher used for encrypting session recordings specifically. Parquet natively only supports AES-GCM for authenticated encryption which forces us to use that there. AES-GCM is not suitable as a default cipher construct since it does not have the ability to decrypt and encrypt streams which is crucial.

The asymmetric cipher that will be used is RSA-2048 as it is widely compliant and has trusted implementations.

#### Encryption tools

Ideally we would use a well support and pre-build solution that allows encryption file chunks programmatically and via a CLI and provides things like algorithm selection and other low level stuff automatically. Unfortunately I am not aware of such a solution and thus we will have to roll our own. Suggestions for such projects are welcome.

#### Key management

Since customers may have tools that are designed to work specifically with different cloud services and their encryption and key management schemes we may need to slightly tailor the key management approach based on the cloud service the deployment is connected to. Described below is the default key management implementation that works with all supported storage backends.

When using symmetric encryption algorithms to encrypt and decrypt data on auth servers a strategy is needed for managing these keys and making sure all auth servers in a high availability deployment can decrypt data from all other auth servers.

Broadly, there are two options available:

1. Sharing a single encryption key across all authentication servers.
2. Taking a card from the AWS Encryption SDK playbook and using a two layer key system.

Option 1 has multiple significant drawbacks which make it unsuitable for production use. The major issue is that all data is fundamentally tied to the only layer of security meaning that if a malicous party potentially acquired access hold to the key, all data would have to be reincrypted. This makes key rotation prohibitive. Another major issue is that of prolonged key reuse which contradicts industry best practices.

Option 2 is doing something very similar to what the AWS Encryption SDK does. Ideally using that library would be ideal since it is a very nice encryption toolkit but unfortunately it isn't available for Go. The strategy involves using unique ephemeral data keys for each encrypted object (rotated when reincrypted/modified) that are encrypted and bundled with each encrypted object in a defined format. A central PEM encoded master keypair stored in a new resource is used to encrypt these per object keys.

Option 2 here seems like a far more robust solution for production use and that is thus what we should employ.

For facilitating key rotation the encrypted objects should be stored in a seperate file from it's data key bundle. This is due to modification restricts on objects imposed by services such as AWS S3.

All encrypted objects should be suffixed by a `.enc` extension to communicate that they are encrypted. For simplicity no other data should be bundled in the same file.

The file that stores the encrypted data key should be a file with the name `$objectName.key` stored next to the encrypted object.

Each encrypted object has an accompanying file named `$objectName.key` that stores a JSON document of this format.

```json
{
    // Contains one or more base64 encoded AES-CBC-HMACSHA256 data key encrypted with the master key
    // together with a unique identifier for the related master key.
    "dataKey": "[]{
        key: string,
        masterKey: string
    }",

    // An integer starting at 1 that communicates the schema used for encryption.
    // May be changed in the future.
    "version": "number",
}
```

This format allows a decrypting auth server to easily fetch and decrypt the data key using the master key. It can then fetch and decrypt the object itself.
Key rotation in event of a security incident can be performed by fetching and rewriting every key file to reencrypt the data key.

The master key resource stores multiple copies of the master key in the following form where the map key is a unique per-auth identifier and the value is a base64 encoded encrypted master key. The master key is encrypted with a asymmetric keypair that never leaves the auth server. This integrates well with HSM and provides good security.

```json
{
    "masterKeys": "map<string, string>"
}
```

#### High-availability

HA support is implemented by storing multiple copies of the encrypted master key, one for each auth server. When a new auth server joins a cluster it sends it's public key to an established auth server. The established auth server decrypts the master key and reencrypts it with the public key of the new auth server. That copy is then stored in the master key resource and the new auth server is now initiated and has the ability to decrypt data.

#### HSM Support

HSM support is built into the design via the per auth server private keypair. An interface to this keypair is supplied to internal routines which provides encrypt and decrypt options along with keypair generation. This interface is then simply implemented for HSM's and enabled in a config with the standard HSM configuration. The non-HSM variant works by essentially emulating a HSM in software.

#### Session recording format

A couple of prototype tests were conducted with storing session recordings in a Parquet format and querying via AWS Athena and downloading the recording locally. Unfortunately storing session recordings in Parquet turned out to be highly suboptimal as the columnar database nature of the Parquet format does not work well with the iterative stream of state-folding events of session recordings. Session recordings must support streaming since downloading entire recordings is unpractical and that is something that is not possible with Parquet. For that reason, I've opted to not change the recording format.

There definitely are better formats our there but introducing a format change without a major benefit was deemed to be not worth it. The primary reason

Session recordings are encrypted at the envelope level and stored in the existing archive format. This allows for a minimal format change reducing in less complexity.

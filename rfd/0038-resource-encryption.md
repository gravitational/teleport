---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 38 - Resource Encryption

## What

This document specifies a concrete system for encrypting resources such as Parquet audit logs and session recordings at rest.

## Why

Currently most resources we store locally and remotely on services like AWS S3 are not encrypted. This is not sufficient for many security guidelines and regulations. Teleport needs to support encrypting these resources at rest with access to encryption keys restricted to auth nodes.

One alternative is to enable built-in at rest encryption within services such as AWS S3. This does mitigate the problem but still bears the risk of not being in full control of the encryption keys. This approach shifts the security of a Teleport cluster onto the security of a third party vendor more than necssary. This means that Teleport needs to support client-side encryption.

## Details

### Encryption algorithms

The selected encryption algorithms must be considered industry standard and secure but also must comply with FIPS PUB 140-3 and thus the algorithms must be included in SP 800-140C (as of March 2020) in order to comply with government regulations.

The symmetric cipher that will be used for bulk encryption shall thus be AES-CBC-HMACSHA256 mode (provides authentication) and any eventual checksumming that has to be performed in the implementation should be done with a newer algorithm in the SHA3 family such as SHA3-256.

AES-CBC-HMACSHA256 was chosen because it is a popular AEAD construct supported by various libraries including Google Tink which can be built to rely on BoringCrypto primitives.

The asymmetric cipher that will be used is RSA-2048 as it is widely compliant and has trusted implementations.

### Local and Remote

Currently, encrypting resources at-rest on remote services is of the highest priority.
When these need to be accessed by an auth server on request of a node or these resources must be fetched from the remote services and decrypted and cached locally for at least the duration of the access.

Because this is a form of client side encryption we do not rely on the service itself and thus we should strive to support encryption on all remote storage services Teleport supports where possible including both AWS and GCP.

### Key management

When using symmetric encryption algorithms to encrypt and decrypt data on auth servers a strategy is needed for managing these keys and making sure all auth servers in a high availability deployment can decrypt data from all other auth servers.

Broadly, there are two options available:

1. Sharing a single encryption key across all authentication servers.
2. Taking a card from the AWS Encryption SDK playbook and using a two layer key system.

Option 1 has multiple significant drawbacks which make it unsuitable for production use. The major issue is that all data is fundamentally tied to the only layer of security meaning that if a malicous party potentially acquired access hold to the key, all data would have to be reincrypted. This makes key rotation prohibitive. Another major issue is that of prolonged key reuse which contradicts industry best practices.

Option 2 is doing something very similar to what the AWS Encryption SDK does. Ideally using that library would be ideal since it is a very nice encryption toolkit but unfortunately it isn't available for Go. The strategy involves using unique ephemeral data keys for each encrypted object (rotated when reincrypted/modified) that are encrypted and bundled with each encrypted object in a defined format. Each auth server has it's own asymmetric master key pair consisting of a public and private RSA-2048 key. Each auth server must only have access to it's own private key but all auth servers should have access to the public keys of all other auth servers. The symmetric data encryption key is encrypted once with the public key of every auth server and bundled with the encrypted object when uploading.

Option 2 here seems like a far more robust solution for production use and that is thus what we should employ.

#### Encryption format

For facilitating key rotation the encrypted objects should be stored in a seperate file from it's data key bundle. This is due to modification restricts on objects imposed by services such as AWS S3.

All encrypted objects should be suffixed by a `.enc` extension to communicate that they are encrypted. For simplicity no other data should be bundled in the same file.

The bundle that stores the encrypted data key for all master key pairs should be a JSON file with the name `$objectName.keys`. The JSON file has a root dict object with a property for every key ID. The value of the property should be a base64 encoded version of the encrypted symmetric data key.

This format allows a decrypting auth server to easily fetch the key bundle and decrypt the data key using it's private key. It can then fetch and decrypt the object itself.

Key rotation can be performed by with any active public/private keypair and the procedure is to fetch and rewrite every key bundle to remove the obsolete encrypted data key property and add a new property containing the encrypted data key for the new keypair.

The format detailed above should be used wherever it makes sense from a security standpoint. Devitations from how the data object is encrypted should be made wherever necessary but the key bundle format should never be deviated from. A notable deviation that will be made is for Parquet based files where the encryption will be handled by giving Parquet control of the symmetric data key. The reason for this is that flat-file encryption with Parquet is vulnerable to security attacks similar to CRIME.

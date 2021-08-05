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

### Local and remote

Currently, encrypting resources at-rest on remote services is of the highest priority.
When these need to be accessed by an auth server on request of a node or these resources must be fetched from the remote services and decrypted and cached locally for at least the duration of the access.

Because this is a form of client side encryption we do not rely on the service itself and thus we should strive to support encryption on all remote storage services Teleport supports where possible including both AWS and GCP.

### Key management

When using symmetric encryption algorithms to encrypt and decrypt data on auth servers a strategy is needed for managing these keys and making sure all auth servers in a high availability deployment can decrypt data from all other auth servers.

Broadly, there are two options available:

1. Sharing a single encryption key across all authentication servers.
2. Taking a card from the AWS Encryption SDK playbook and using a two layer key system.

Option 1 has multiple significant drawbacks which make it unsuitable for production use. The major issue is that all data is fundamentally tied to the only layer of security meaning that if a malicous party potentially acquired access hold to the key, all data would have to be reincrypted. This makes key rotation prohibitive. Another major issue is that of prolonged key reuse which contradicts industry best practices.

Option 2 is doing something very similar to what the AWS Encryption SDK does. Ideally using that library would be ideal since it is a very nice encryption toolkit but unfortunately it isn't available for Go. The strategy involves using unique ephemeral data keys for each encrypted object (rotated when reincrypted/modified) that are encrypted and bundled with each encrypted object in a defined format. A central PEM encoded master keypair stored in a new resource is used to encrypt these per object keys.

Option 2 here seems like a far more robust solution for production use and that is thus what we should employ.

#### Encryption format

For facilitating key rotation the encrypted objects should be stored in a seperate file from it's data key bundle. This is due to modification restricts on objects imposed by services such as AWS S3.

All encrypted objects should be suffixed by a `.enc` extension to communicate that they are encrypted. For simplicity no other data should be bundled in the same file.

The file that stores the encrypted data key should be a file with the name `$objectName.keys` stored next to the encrypted object.

This format allows a decrypting auth server to easily fetch and decrypt the data key using the master key. It can then fetch and decrypt the object itself.

Key rotation in event of a security incident can be performed by fetching and rewriting every key file to reencrypt the data key.

Both the data and key file should have a short 8 byte header declaring the version. The header is to be interpreted as a little-endian 64 bit signed integer and the version described in this RFD is represented as 1.

The format detailed above should be used wherever it makes sense from a security standpoint. Devitations from how the data object is encrypted should be made wherever necessary but the key bundle format should never be deviated from. A notable deviation that will be made is for Parquet based files where the encryption will be handled by giving Parquet control of the symmetric data key. The reason for this is that flat-file encryption with Parquet is vulnerable to security attacks similar to CRIME.

#### Key configuration

The encryption and key management schema detailed above calls of a central store containing a PEM RSA-2048 keypair used to encrypt the data keys. This will be implemented by introducing a new sensitive resource that is only ever present on auth servers containing the key.

If the resource isn't present on auth server startup, a new key is randomly generated via an OS CSPRNG and the resource is set via a compare-and-swap. If the compare-and-swap fails another auth server already created the resource and we fetch that instead.

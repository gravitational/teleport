---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 38 - Resource Encryption

## What

This document specifies an abtract system for encrypting resources such as Parquet audit logs session recordings at rest. Details are included for an implementation of session recording encryption.

## Why

Currently most resources we store locally and remotely on services like AWS S3 are not encrypted. This is not sufficient for many security guidelines and regulations. Teleport needs to support encrypting these resources at rest with access to encryption keys restricted to auth nodes.

One alternative is to enable built-in at rest encryption within services such as AWS S3. This does mitigate the problem but still bears the risk of not being in full control of the encryption keys. This approach shifts the security of a Teleport cluster onto the security of a third party vendor more than necssary. This means that Teleport needs to support client-side encryption.

## Details

### Encryption algorithms

The selected encryption algorithms must be considered industry standard and secure but also must comply with FIPS PUB 140-3 and thus the algorithms must be included in SP 800-140C (as of March 2020) in order to comply with government regulations.

Because only the auth servers or someone with full access to them should only ever be able to access encrypted resources I propose that for the common case, only symmetric encryption is used for resources. The encryption scheme must also provide authentication to comply with industry standard practices. This means utilizing a cipher with built-in authentication such as AES-256-GCM or including a HMAC code with a non-authentication cipher.

The symmetric cipher that will be used for bulk encryption shall thus be AES-256 in GCM mode (provides authentication) and any eventual checksumming that has to be performed in the implementation should be done with a newer algorithm in the SHA3 family such as SHA3-256.

### Local and Remote

Currently, encrypting resources at-rest on remote services is of the highest priority.
When these need to be accessed by an auth server on request of a node or these resources must be fetched from the remote services and decrypted and cached locally for at least the duration of the access.

Because this is a form of client side encryption we do not rely on the service itself and thus we should strive to support encryption on all remote storage services Teleport supports where possible.

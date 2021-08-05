---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 38 - Resource Encryption

## What

This document specifies a system for encrypting resources such as Parquet audit logs session recordings at rest.

## Why

Currently most resources we store locally and remotely on services like AWS S3 are not encrypted. This is not sufficient for many security guidelines and regulations. Teleport needs to support encrypting these resources at rest with access to encryption keys restricted to auth nodes.

One alternative is to enable built-in at rest encryption within services such as AWS S3. This does mitigate the problem but still bears the risk of not being in full control of the encryption keys. This approach shifts the security of a Teleport cluster onto the security of a third party vendor more than necssary. This means that Teleport needs to support client-side encryption.

## Details

---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 38 - Parquet and a new event architecture

## What

An overhaul/extension of the current way we handle events to accomodate new requirements
- Natively export to Parquet on-the-fly [#6160](https://github.com/gravitational/teleport/issues/6160)
- Support encryption of Parquet files and session recordings when stored on cloud services [#6311](https://github.com/gravitational/teleport/issues/6311)
- Perform advanced queries in real-time on events in an efficient manner [#7724](https://github.com/gravitational/teleport/issues/7724)

## Why

Our requirements have changed and customers want these capabilities to improve integration into their systems and to comply with security regulations.

[#6160](https://github.com/gravitational/teleport/issues/6160) will make us compatible with SIEM and analytics tools which many of our customers use. This is crucial to make events useful on a larger scale.

[#6311](https://github.com/gravitational/teleport/issues/6311) is required because many do not trust the built-in at-rest encryption of S3 want additional security in the form of client side or AWS KMS encryption.

[#7714](https://github.com/gravitational/teleport/issues/7724) is crucial to enable new views into data from the Teleport webapp and improve the efficiency of existing types of queries.

## Details

This section will go through the requirements, the different architectural and technical solutions I've evaluated, and finally what a sustainable path forward is that fulfills these requirements.

Each section attempts to address one of the concerns outlined above.

### Parquet export [#6160](https://github.com/gravitational/teleport/issues/6160)

This requirement calls for a way to continously export events into Parquet formatted columnar files and periodically uploaded for storage on cloud storage platforms such as AWS S3 and GCP Cloud Storage.

The current events system is structured as the event driver interface `IAuditLog`, implemented for each backend with standardized operations for upload, and query. The simplest way to implement the above feature would be to utilize the `MultiLog` splitter which allows fan-out to multiple backends for upload operations. A new backend implementing `IAuditLog` is to be implemented for Parquet that only supports upload/similar operations and is to be used in tandem with a backend that supports queries such as `DynamoDB`.

When enabled, events will fan-out to this backend and be saved into Parquet files and periodically uploaded after a short timeframe or an event threshold is reached. When AWS S3 is used as the backend cloud storage, AWS Kinesis Firehose should be used to improve the perfromance and reliability of this system by streaming the events from the driver onto a Firehose stream which will take care of the batching and writing to Parquet files.

Customers and services can then read these Parquet files using SIEM and analysis tools such as Snowflake and AWS Athena to ingest and query this data as they wish.

#### Format

To preserve consistency and ease of use along with interoperability the Parquet format will be very similar to how events are stored in JSON format today. The fields will be flattened to columns to allow detailed queries and better ingestion as storing JSON in Parquet leads to difficulty for tools to understand the data.

### Parquet and session encryption [#6311](https://github.com/gravitational/teleport/issues/6311)

Encryption for Parquet event exports and session recordings are a crucial feature for security concerned customers. We need to support encryption of session recording and Parquet formats across a variety of cloud services.

#### Encryption algorithms

The selected encryption algorithms must be considered industry standard and secure but also must comply with FIPS PUB 140-3 and thus the algorithms must be included in SP 800-140C (as of March 2020) in order to comply with government regulations.

The symmetric cipher that will be used for bulk encryption shall thus be AES-CBC-HMACSHA256 mode (provides authentication) and any eventual checksumming that has to be performed in the implementation should be done with a newer algorithm in the SHA3 family such as SHA3-256.

AES-CBC-HMACSHA256 was chosen because it is a popular AEAD construct supported by various libraries including Google Tink which can be built to rely on BoringCrypto primitives.

This is the default cipher used for encrypting session recordings specifically. Parquet natively only supports AES-GCM for authenticated encryption which forces us to use that there. AES-GCM is not suitable as a default cipher construct since it does not have the ability to decrypt and encrypt streams which is crucial.

The asymmetric cipher that will be used is RSA-2048 as it is widely compliant and has trusted implementations.

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

    // An integer starting at 1 one that communicates the schema used for encryption.
    // May be changed in the future.
    "version": "number",
}
```

This format allows a decrypting auth server to easily fetch and decrypt the data key using the master key. It can then fetch and decrypt the object itself.

Key rotation in event of a security incident can be performed by fetching and rewriting every key file to reencrypt the data key.

#### Encryption compatability

To make Parquet encryption compatible with SIEM and data analysis tools when optionally enabled, data keys are optionally stored in AWS KMS and are tagged with an ID that is also included in the KEYID metadata of the Parquet files. This allows automatic key lookup and decryption.

#### HSM Support

HSM support will be implemented by loading master keys into the HSM for encrypting and decrypting data keys. They will also be used to securely generate those data keys and encrypt the data.

#### Parquet format

Parquet natively supports FIPS 140-3 compliant encryption algorithms and handles encryption internally. The symmetric algorithm that will be used is AES_GCM_V1 as per Parquet specifications which is included in the NIST SP 800-38D specification. Parquet will use the data keys generated and managed by the above process.

#### Session recording format

A couple of prototype tests were conducted with storing session recordings in a Parquet format and querying via AWS Athena and downloading the recording locally. Unfortunately storing session recordings in Parquet turned out to be highly suboptimal as the columnar database nature of the Parquet format does not work well with the iterative stream of state-folding events of session recordings. Session recordings must support streaming since downloading entire recordings is unpractical and that is something that is not possible with Parquet. For that reason, I've opted to not change the recording format.

There definitely are better formats our there but introducing a format change without a major benefit was deemed to be not worth it. The primary reason Parquet was investigated for session recordings is a unified events format with good encryption support.

Encrypted session recordings will encrypted at the envelope level and bundled together in the same archive format that is in current use.

### Advanced queries [#7714](https://github.com/gravitational/teleport/issues/7724)

Teleport has reached a point where audit events contain a lot of high cardinality data across a myriad of different types of events. This is currently managed by using a dynamic schema where each event has different fields and is stored as a JSON blob with no schema enforced.

We now want to perform more advanced queries on these high cardinality fields on some event types such as the one outlined in the above issue. I evaluated multiple solutions for hot event storage and querying to find what suits Teleport's usage patterns best.

One of the original ideas that we could use Parquet for was ingesting it into some kind of tool that could be used for querying said data. These were tools like AWS Athena, DuckDB, and Apache Drill which analyze Parquet formatted columnar data in a remote location. I evaluated some of thse tools and observed high latency numbers for the queries. These tools have the capabilities to handle complex queries on big sets of data but are so called "Big Data" tools. They are designed to ingest and provide analytics in an "offline" environment where data is aggregated and ingested for processing and report generation on some schedule.

This is unfortunately not the current workload of most of the Teleport event queries. The current usage patterns within Teleport are fast real-time queries from an interface designed to show events to admins as they happen. Due to the design decisions made by these tools they have >1s latencies for even the 50th percentile of common queries which renders them unusable for this task nor do they handle a hig amount of concurrent queries either.

Since Parqet export will be implemented regardless, future non real-time analytics features could potentially be built on these tools but they are not the solution to the primary problem of complex real-time queries.

After significant evaluation and comparison the workload we're seeking of rich real-time queries on an evolving dataset seems to always land on databases supporting SQL like queries dynamic data such as JSON. In the industry, this is generally solved by specialized timeseries databases or databases that support SQL queries. We currently rely heavily on serverless offerings such as DynamoDB and in the near term, migrating to something else for these types of queries is not doable.

After discussing how this has bee solved at other companies, the solution that seems doable for Teleport is to collect all requirements on types of queries now and in the future and use a mix of global and local indexes on DynamoDB or other K/V esque databases to filter down the search set, then it boils down to writing filtering logic to perform final filtering on the Teleport auth node itself. This could be expanded into a more rigorous framework inside of the event drivers for better structure.

---
authors: Michael McAllister (michael.mcallister@goteleport.com)
state: canceled
---

# RFD 60 - gRPC Backend

## What

A new [backend](https://github.com/gravitational/teleport/tree/v8.3.1/lib/backend) that sends requests to a user defined gRPC server for persistence. Initially targeting cluster state with audit events to come later (in a subsequent RFD)

## Why

As Cloud-Hosted Teleport continues to grow, so does our need to have greater flexibility on not only what persistence layer we use (be it currently supported backends like DynamoDB, or otherwise) but _how_ these are implemented.

Take for instance, the current implementation of the [DynamoDB backend](https://github.com/gravitational/teleport/tree/v8.3.1/lib/backend/dynamo) which allows in its configuration to specify the table name, but does not presently have the ability to customize the Partition key. This means that it's not possible to co-locate multiple installations of Teleport within the same DynamoDB table, and as a result are bounded by [AWS account limits](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ServiceQuotas.html#limits-tables) when deploying multiple installations.

Other backend solutions have their own challenges regarding authentication and authorization. For instance, most databases have limits on the number of users/roles that you can define which we'd need to leverage to ensure strong data isolation between each deployment. Using an authentication method that would allow one tenant to access another tenants data would be a non-option.

Implementing a gRPC client within Teleport allows not only the Cloud team, but external developers to implement:
 - A custom persistence layer that may not be supported by Teleport natively
 - Custom caching logic, connection pooling, etc.
 - Custom database schema, partition scheme, etc.
 - Custom authentication and authorization (mTLS, LDAP, etc.)

## Details

### Scope

It is proposed that the Teleport core codebase hold the protocol buffer definitions and the corresponding client.

To keep this RFD brief the Cloud teams (aspirational) implementation of the corresponding gRPC server is out of scope of this documentation and not discussed in detail, it will also be wholly owned by the Cloud team and not apart of the core codebase.

Additionally audit events are initially outside of scope, but will be implemented at a later date.

### Protobuf Definition

The proposed protobuf definition aims to mirror the interface that is defined in [backend.go](https://github.com/gravitational/teleport/blob/cf162af679f3c136b0cc5a7c5bfcd8bba14afdaa/lib/backend/backend.go#L41-L91)

```protobuf
message Item {
    // Key is a key of the key value item
    bytes key = 1;

    // Value is a value of the key value item
    bytes value = 2;

    // Expires is an optional record expiry time
    google.protobuf.Timestamp expires = 3 [
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = false
    ];

    // ID is a record ID, newer records have newer ids
    sfixed64 id = 4;

    // LeaseID is a lease ID, could be set on objects with TTL
    sfixed64 lease_id = 5;
}

message Lease {
    // Key is a key of the key value item
    bytes key = 1;

    // ID is a record ID, newer records have newer ids
    sfixed64 id = 2;
}

message Watch {
    // resume_last_id is the last item index we saw, and we want to resume from any event after that point
    sfixed64 resume_last_id = 1;
}

message CompareAndSwapRequest {
    // Expected is the existing item
    Item expected = 1;

    // replace_with is the new item to swap
    Item replace_with = 2;
}

message GetRequest {
    // Key is a key of the key value item
    bytes key = 1;
}

message GetRangeRequest {
    // start_key is the starting key to request in the range request
    bytes start_key = 1;

    // end_key is the ending key to request in the range request
    bytes end_key = 2;

    // limit is the maximum number of results to return
    int32 limit = 3;
}

message GetRangeResult {
    repeated Item items = 1;
}

message DeleteRequest {
    // Key is a key of the key value item
    bytes key = 1;
}

message DeleteRangeRequest {
    // start_key is the starting key to request in the range request
    bytes start_key = 1;

    // end_key is the ending key to request in the range request
    bytes end_key = 2;
}

message KeepAliveRequest {
    Lease lease = 1;
    google.protobuf.Timestamp expires = 2 [
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = false
    ];
}


message Event {
   // OpType specifies operation type
   enum OpType {
      UNKNOWN = 0;
      UNRELIABLE = 1;
      INVALID = 2;
      INIT = 3;
      PUT = 4;
      DELETE = 5;
      GET = 6;
   }
   OpType type = 1;
   Item item = 2;
}

service BackendService {
    // Create creates item if it does not exist
    rpc Create(Item) returns (Lease);

    // Put upserts value into backend
    rpc Put(Item) returns (Lease);

    // CompareAndSwap compares item with existing item and replaces is with replace_with item
    rpc CompareAndSwap(CompareAndSwapRequest) returns (Lease);

    // Update updates value in the backend
    rpc Update(Item) returns (Lease);

    // Get returns a single item or not found error
    rpc Get(GetRequest) returns (Item);

    // GetRange returns query range
    rpc GetRange(GetRangeRequest) returns (GetRangeResult);

    // Delete deletes item by key, returns NotFound error if item does not exist
    rpc Delete(DeleteRequest) returns (google.protobuf.Empty);

    // DeleteRange deletes range of items with keys between startKey and endKey
    rpc DeleteRange(DeleteRangeRequest) returns (google.protobuf.Empty);

    // KeepAlive keeps object from expiring, updates lease on the existing object,
	 // expires contains the new expiry to set on the lease,
	 // some backends may ignore expires based on the implementation
	 // in case if the lease managed server side
    rpc KeepAlive(KeepAliveRequest) returns (google.protobuf.Empty);

    // NewWatcher returns a new event watcher
    rpc NewWatcher(Watch) returns (stream Event);
}
```

### Configuration

In order for the gRPC client (the Teleport side) to be able to talk to a server, at minimum the address of the server providing the gRPC service will be required, additionally the following existing configuration properties will be reused:

- buffer size for client size buffering
- mTLS details (CA, Cert, Key) (currently used for etcd)

Representing this in yaml the storage stanza of the configuration could potentially look like:
```yaml
storage:
   type: grpc
   server: endpoint.example.com:1992
   tls_ca_file: /secrets/grpc/ca.crt
   tls_cert_file: /secrets/grpc/tls.crt
   tls_key_file:  /secrets/tls.key
   audit_events_uri: ['dynamodb://example-ddb-table.events'] # Future iteration will support grpc://endpoint.example.com:1992
   audit_sessions_uri: s3://example-bucket/sessions
   continuous_backups: true
   auto_scaling: true
   read_min_capacity: 20
   read_max_capacity: 100
   read_target_value: 50.0
   write_min_capacity: 10
   write_max_capacity: 100
   write_target_value: 70.0
```

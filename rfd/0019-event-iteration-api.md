---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 19 - Event Fetch API with Pagination

## What 1

Implement a new API based on the current API for fetching events but with full pagination support in form of a `StartKey: string` parameter that allows the client to search from a specific point in the event stream instead of starting at the first event of the start date.

## Why 2

This RFC was sparked from issue [#5435](https://github.com/gravitational/teleport/issues/5435).
The primary motivation for this RFD is that clients are at the moment not able to fetch an event stream between two dates incrementally with multiple API calls.

Such functionality requires the client to be able to specify a point in the event stream from where the server will start searching forward from. Currently no such parameter exists which means the server will always start returning events from the start of the stream.

## Details 3

The current API for event fetch and search is over (https://github.com/gravitational/teleport/blob/master/lib/auth/apiserver.go#L246) plain HTTPS and not GRPC and runs on the authentication server.

I propose implementing a new GRPC API that replaces and deprecates the old HTTP API endpoints. The old API endpoints should be kept as is but documentation should mention that it is deprecated and advise clients to use the new API.

### GRPC Messages 3.1

Out-of-session event request.
This replaces and deprecates the `GET /:version/events` HTTP endpoint.

These should be defined in [api/client/proto/authservice.proto](https://github.com/gravitational/teleport/blob/master/api/client/proto/authservice.proto).

```protobuf
message GetEventsRequest {
   // Namespace, if not set, defaults to 'default'
   Namespace string
   // Oldest date of returned events
   StartDate Timestamp
   // Newest date of returned events
   EndDate Timestamp
   // EventType is optional, if not set, returns all events
   EventType string
   // Maximum amount of events returned
   Limit int64
   // When supplied the search will resume from the last key
   StartKey string
}
```

In-session event request.
This replaces and deprecates the `GET /:version/events` HTTP endpoint.

```protobuf
message GetSessionEventsRequest {
   // SessionID is a required valid session ID
   SessionID string
   EventType string
   Limit int64
   StartKey string
}
```

Event response from both endpoints.

```protobuf
message Events { 
   Items repeated oneof Event
   // the key of the last event if the returned set did not contain all events found i.e limit < actual amount. this is the key clients can supply in another API request to continue fetching events from the previous last position
   LastKey string
}
```

#### Notes on the EventType parameter 3.1.1

Currently the DynamoDB implementation is incorrect in the HTTP API as described in [#5435](https://github.com/gravitational/teleport/issues/5435). This should be fixed so that event filtering works correctly in the new GRPC API although I'd like some discussion on if it should be fixed in the current API or should be left as is.

### GRPC Endpoints 3.2

This RFD calls for two new endpoints. One endpoint for out-of-session event requests and one for in-session event requests.

These endpoints will live on the `AuthService` GRPC service in [api/client/proto/authservice.proto](https://github.com/gravitational/teleport/blob/master/api/client/proto/authservice.proto) and are defined as follows:

```proto
rpc GetEvents(GetEventsRequest) returns (Events);
```

```proto
rpc GetSessionEvents(GetSessionEventsRequest) returns (Events);
```

### Implementation details 3.3

#### Auth service 3.3.1

Handling of these new GRPC endpoints will be implemented in [lib/auth/grpcserver.go](https://github.com/gravitational/teleport/blob/master/lib/auth/grpcserver.go). `auth.Client.SearchEvents` and `auth.Client.SearchSessionEvents` will be refactored with pagination support implemented with the same startkey/lastkey interface as seen in the GRPC API. These methods are what the GRPC handlers call. The current HTTP API implementation will be slightly altered to deal with these changes but will remain identical in functionality, simply not exposing the new internal pagination support.

#### Backends 3.3.2

Performed some investigative work on how we can implement this pagination support in the different backends. The standard solution that works with the current backend interface is to simply query all events in a given date range from the backend and then filter out the paginated section using the given start key and limit. This is however something we want to avoid as it involves a lot of uneeded processing on every request by the authentication server.

Pagination support will needed to be added to the `IAuditLog` interface defined in `lib/events/api.go`. This involves adding a start key function parameter and an optional last key parameter in the return value of the `SearchEvents` and `SearchSessionEvents` methods.

Pagination in DynamoDB is possible to implement via `ExclusiveStartkey` and `LastEvaluatedKey`. We already use it's pagination internally to work with the maximum response sizes of the DynamoDB API.

Below is an example DynamoDB query in Go based on the current implementation that the new backend APIs may generate to fetch a paginated section of events from an event stream between a start and end date.

```go
query := "EventNamespace = :eventNamespace AND CreatedAt BETWEEN :start and :end"
attributes := map[string]interface{}{
   ":eventNamespace": defaults.Namespace,
   ":start":          fromUTC.Unix(),
   ":end":            toUTC.Unix(),
}
attributeValues, err := dynamodbattribute.MarshalMap(attributes)
// handle err
input := dynamodb.QueryInput{
   KeyConditionExpression:    aws.String(query),
   TableName:                 aws.String(l.Tablename),
   ExpressionAttributeValues: attributeValues,
   IndexName:                 aws.String(indexTimeSearch),
   //                         supplied as a parameter via the new gRPC API
   ExclusiveStartKey:         startKey,
}
```

Not an expert on the format used for filelog but since it is a plain sequence of JSON documents, implementing efficient pagination seems difficult. On this backend we might need to fall back to loading all events into memory and then sorting out the events that fall within the range of events requested.

Firestore seems to have good support for pagination with its query cursors and document snapshots which allow us to define a range between startkey and startkey + limit. This means we can efficiently query a subset of events.

#### Client library

The Go client library has to be updated to work with this new API. It currently uses the HTTP API and should be switched over completely to the new GRPC API as we deprecate the HTTP API. Adding pagination support to the existing methods in the library is not possible without a breaking change and unless that is acceptable, then adding variants of the event fetch methods with the `Paginated` suffix seems like the best option.

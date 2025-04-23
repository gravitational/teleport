---
authors: Tim Ross (tim.ross@goteleport.com)
state: draft
---

# RFD 153 - Resource Guidelines

## Required approvers

- Engineering: `@zmb3 && codingllama`

## What

Guidelines for creating backend resources and APIs to interact with them.

## Why

To date resources follow slightly different patterns and have inconsistent APIs for creating, updating and deleting
them, see [#29234](https://github.com/gravitational/teleport/issues/29234). In addition to reducing confusion and
fostering a better developer experience, this will also remove complexity from the terraform provider and teleport
operator that have to accommodate for every subtle API and resource difference that exist today.

## Details

### Project structure

All protos are defined in the `api` module under the proto directory. When adding a new resource and gRPC API a new folder that matches the desired package name of the proto should be created, which should be the domain that represents that resource and any other associated resources. Inside it, there should be a directory for each version of the API should exist. The actual RPC service should exist in its own file `foo_service.proto` which has the service defined first and all request/response messages defined after. This allows to quickly discover the API without having to scroll through a bunch of boilerplate and standard messages. Important types, like the resource definition, or any supporting types should exist in their own file. This makes discovering them easier and reduces the amount of things required to be imported.

An example of the file layout for the resource used as an example in this RFD is included below.

```bash
api/proto
├── README.md
├── buf-legacy.yaml
├── buf.lock
├── buf.yaml
└── teleport
    ├── foo
    │   └── v1
    │       ├── foo.proto
    │       └── foo_service.proto
    ├── legacy
    │   ├── client
    │   │   └── proto
    │   │     ├── authservice.proto
    │   │     ├── certs.proto
    │   │     ├── event.proto
    │   │     ├── joinservice.proto
    │   │     └── proxyservice.proto
    │   └── types
    │         ├── device.proto
    │         ├── events
    │         │    ├── athena.proto
    │         │    └── events.proto
    │         ├── types.proto
    │         ├── webauthn
    │         │    └── webauthn.proto
    │         └── wrappers
    │              └── wrappers.proto
```

The legacy directory contains resources and API definitions that were defined prior to our shift to user smaller,
localized services per resource. Adding new resources and APIs to the giant monolithic `proto.AuthService` should be
avoided if possible.

### New resource checklist

Use the non-exhaustive list below as a guide when adding a new resource. Further sections in the RFD dive into more
detail about what is needed to complete a particular item.

- [ ] Create proto spec for resource and RPC service
- [ ] Create backend service
- [ ] Add resource client to `api` client
- [ ] Implement gRPC service
- [ ] Add resource parser to event stream mechanism
- [ ] Add resource support to tctl (get, create, edit)
- [ ] Optional: Add resource to cache
- [ ] Add support for bootstrapping the resource
- [ ] Add support for resource to Teleport Operator
- [ ] Add support for resource to Teleport Terraform Provider

### Defining a resource

A resource MUST include a kind, version, `teleport.header.v1.Metadata`, a `spec` and a `status` message. While the kind and version may seem like they would be easy to derive from the message definition itself, they need to be defined so that anything processing a generic resource can identify which resource is being processed. For example, `tctl` interacts with resources in their in raw yaml text form and leverages `services.UnknownResource` to identify the resource and act appropriately.

All properties defined in the `spec` of a resource MUST only be modified by the
owner/creator of the resource. For example, if a resource is created via
`tctl create`, then any fields within the `spec` MUST not be altered dynamically
by the Teleport process. When Teleport automatically modifies the `spec` during
runtime it causes drift between what is in the Teleport backend and the state stored by external IaC tools. If a resource has properties that are required to be modified dynamically by Teleport, a separate `status` field should be added to the resource to contain them. These fields will be ignored by IaC tools during their reconciliation.

```protobuf
import "teleport/header/v1/metadata.proto";

// Foo is a resource that does foo.
message Foo {
  // The kind of resource represented.
  string kind = 1;
  // Differentiates variations of the same kind. All resources should
  // contain one, even if it is never populated.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources share.
  teleport.header.v1.Metadata metadata = 4;
  // The specific properties of a Foo. These should only be modified by
  // the creator/owner of the resource and not dynamically altered or updated.
  FooSpec spec = 5;
  // Any dynamic state of Foo that is modified during runtime of the
  // Teleport process.
  FooStatus status = 6;
}

// FooSpec contains specific properties of a Foo that MUST only
// be modified by the owner of the resource. These properties should
// not be automatically adjusted by Teleport during runtime.
message FooSpec {
  string bar = 1;
  int32 baz = 2;
  bool qux = 3;
}

// FooStatus contains dynamic properties of a Foo. These properties are
// modified during runtime of a Teleport process. They should not be exposed
// to end users and ignored by external infrastructure as code(IaC) tools like terraform.
message FooStatus {
  google.protobuf.Timestamp next_audit = 1;
  string teleport_host = 2;
}
```

This differs from existing resources because legacy resources make heavy use of features provided
by [gogoprotobuf](https://github.com/gogo/protobuf). Since that project has long been abandoned, we're striving to
migrate away from it as described in [RFD-0139](https://github.com/gravitational/teleport/pull/28386).
The `teleport.header.v1.Metadata` is a clone of `types.Metadata` which doesn't use any of the gogoproto features.
Legacy resources also had a `types.ResourceHeader` that used gogo magic to embed the type in the resource message. To
get around this, the required fields from the header MUST be included in the message itself. A non-gogo clone does exist
`teleport.header.v1.ResourceHeader`, however, to get the fields, embedded custom marshalling must be manually written.

If a resource has associated secrets (password, private key, jwt, mfa device, etc.) they should be defined in a separate
resource and stored in a separate key range in the backend. The traditional pattern of defining secrets inline and only
returning them if a `with_secrets` flag is provided causes a variety of problems and introduces opportunity for human
error to accidentally include secrets when they should not have been. It would then be the responsibility of the caller
to get both the base resource and the corresponding secret resource if required.

There are many things to consider, and many ways to design a resource
specification. To provide consistency and uniformity there are a few
[standards](https://cloud.google.com/apis/design/standard_fields)
and [design patterns](https://cloud.google.com/apis/design/design_patterns)
that should be followed when possible. The most notable of the design patterns
is
[Bool vs. Enum vs. String](https://google.aip.dev/126). There have been several occasions in the past where a particular field
was not flexible enough which prevented behavior from being easily extended to
support a new feature.

### API

All APIs should follow the conventions listed below that are largely based on
the [Google API style guide](https://cloud.google.com/apis/design/standard_methods).

#### Create

The `Create` RPC takes a resource to be created and must also return the newly created resource so that any fields that
are populated server side are provided to clients without requiring an additional call to `Get`.

The request MUST fail and return a `trace.AlreadyExists` error if a matching resource is already present in the backend.

```protobuf
// Creates a new Foo resource in the backend.
    rpc CreateFoo(CreateFooRequest) returns (Foo);

message CreateFooRequest {
  // The desired Foo to be created.
  Foo foo = 1;
}
```

#### Update

The `Update` RPC takes a resource to be updated and must also return the updated resource so that any fields that are
populated server side are provided to clients without requiring an additional call to `Get`. If partial updates of a
resource are desired, the request may contain
a [FieldMask](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#field-mask).

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Updates an existing Foo in the backend.
    rpc UpdateFoo(UpdateFooRequest) returns (Foo);

message UpdateFooRequest {
  // The full Foo resource to update in the backend.
  Foo foo = 1;
  // A partial update for an existing Foo resource.
  FieldMask update_mask = 2;
}
```

#### Upsert

> The `Create` and `Update` RPCs should be preferred over `Upsert` for normal operations,
> see [#1326](https://github.com/gravitational/teleport/issues/1326) for more details.

The `Upsert` RPC takes a resource that will overwrite a matching existing resource or create a new resource if one does
not exist. The upserted resource is returned so that any fields that are populated server side are provided to clients
without requiring a call to `Get`. If `Upsert` is not consumed it may be omitted from the API in favor of `Create` and
`Update`.

```protobuf
// Creates a new Foo or replaces an existing Foo in the backend.
    rpc UpsertFoo(UpsertFooRequest) returns (Foo);

message UpsertFooRequest {
  // The full Foo resource to persist in the backend.
  Foo foo = 1;
}
```

#### Get

The `Get` RPC takes the parameters required to match a resource (usually the resource name should suffice), and returns
the matched resource.

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Returns a single Foo matching the request
    rpc GetFoo(GetFooRequest) returns (Foo);

message GetFooRequest {
  // A filter to match the Foo by. Some resource may require more parameters to match and
  // may not use the name at all.
  string foo_id = 1;
}
```

#### List

The `List` RPC takes the requested page size and starting point and returns a list of resources that match. If there are
additional resources, the response MUST also include a token that indicates where the next page of results begins.

Most legacy APIs do not provide a paginated way to retrieve resources and instead offer some kind of `GetAllFoos` RPC
which either returns all `Foo` in a single message or leverages a server side stream to send each `Foo` one at a time.
Returning all items in a single message causes problems when the number of resources scales beyond gRPC message size
limits. To provide parity with this legacy API if needed, a helper method should be implemented on the client which
builds the entire resource set by repeatedly calling `List` until all pages have been consumed.

```protobuf
// Returns a page of Foo and the token to find the next page of items.
    rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);

message ListFoosRequest {
  // The maximum number of items to return.
  // The server may impose a different page size at its discretion.
  int32 page_size = 1;
  // The next_page_token value returned from a previous List request, if any.
  string page_token = 2;
}

message ListFoosResponse {
  // The page of Foo that matched the request.
  repeated Foo foos = 1;
  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}
```

A listing operation should not abort entirely if a single item cannot be (un)marshalled, it should instead be logged,
and the rest of the page should be processed. Aborting an entire page when a single entry is invalid causes the cache
to be permanently unhealthy since it is never able to initialize loading the affected resource.

#### Delete

The `Delete` RPC takes the parameters required to match a resource and performs a hard delete of the specified resource
from the backend and returns a `google.protobuf.Empty`.

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Remove a matching Foo resource
    rpc DeleteFoo(DeleteFooRequest) returns (google.protobuf.Empty);

message DeleteFooRequest {
  // Name of the foo to remove. Some resource may require more parameters to match and
  // may not use the name at all.
  string foo_id = 1;
}
```

### Backend Storage

A backend service to handle persisting and retrieving a resource from the backend is typically defined in
`lib/services/local/foo.go`. An accompanying interface which mirrors the service is defined in `lib/services/foo.go`.
Continuing on with the example above, the sections below show how the backend service for the `Foo` resource might look like.

The sections also contain a reference example for how to interact with the backend to perform common operations on a
resource. For most cases, when adding a new resource, it is preferred to create a service that wraps
the [generic.Service](https://github.com/gravitational/teleport/blob/7f3c58df1fd675a813dc2992c10b2796b9b5c6bf/lib/services/local/generic/generic.go#L73-L81)
over implementing everything from scratch. If custom behavior is required for a subset of backend operations, they may
be implemented directly while all the other operations still make use of the generic service.

#### Resource validation
The strictest validation of a resource should be performed prior to write operations. Any resource persisted in the
backend should be guaranteed to be valid. Read operations should not perform resource validations, doing so could
prevent a resource being read if validations are modified to be more restrictive after a resource had already been
written. This allows for resilient reads which ensure that any resource stored is always allowed to be retrieved from
the backend.

#### Create

When creating a new resource, the `backend.Backend.Create` method should be used to persist the resource. It is also
imperative that the revision generated by the backend is set on the returned resource.

```go
func (s *FooService) CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	value, err := convertFooToValue(foo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key("foo", foo.GetName()),
		Value:   value,
		Expires: foo.Expiry(),
	}

	lease, err := s.backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Update the foo with the revision generated by the backend during the write operation.
	foo.GetMetadata().SetRevision(lease.Revision)
	return foo, nil
}
```

#### Update

All update operations should prefer `backend.Backend.ConditionalUpdate` over the `backend.Backend.Update` method to
prevent blindly overwriting an existing item. When using conditional update, the backend write will only succeed if the
revision of the resource in the update request matches the revision of the item in the backend. Conditional updates
should also be preferred over traditional `CompareAndSwap` operations.

```go
func (s *FooService) UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	// The revision is cached prior to converting to the value because
	// conversion functions may set the revision to "" if MarshalConfig.PreserveResourceID
	// is not set.
	rev := foo.GetMetadata().GetRevision()
	value, err := convertFooToValue(foo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key("foo", foo.GetName()),
		Value:   value,
		Expires: foo.Expiry(),
		Revision: rev,
	}

	lease, err := s.backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Update the foo with the revision generated by the backend during the write operation.
	foo.GetMetadata().SetRevision(lease.Revision)
	return foo, nil
}
```

#### Upsert

When upserting a resource, the `backend.Backend.Put` method should be used to persist the resource. It is also
imperative that the revision generated by the backend is set on the returned resource.

A resource may expose an upsert method from the backend layer even if the gRPC API does not expose an `Upsert` RPC. This
may occur if a resource is cached, since the
[cache collections](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/cache/collections.go#L60)
require an upsert mechanism,
see [`services.DynamicAccessExt`](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/services/access_request.go#L260-L278)
for an example.

```go
func (s *FooService) UpsertFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	value, err := convertFooToValue(foo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key("foo", foo.GetName()),
		Value:   value,
		Expires: foo.Expiry(),
	}

	lease, err := s.backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Update the foo with the revision generated by the backend during the write operation.
	foo.GetMetadata().SetRevision(lease.Revision)
	return foo, nil
}
```

#### Get

To retrieve a resource the `backend.Backend.Get` method should be provided a key built from the match parameters of the
request. Note the rewrapping of the `trace.NotFound` error below. This results in a much friendly error being provided
to the user and prevents the backend key from leaking into other layers.

```go
func (s *FooService) GetFoo(ctx context.Context, id string) (*Foo, error) {
	if id == "" {
		return nil, trace.BadParameter("missing foo id")
	}

	item, err := s.backend.Get(ctx, backend.Key("foo", id))
	if err != nil {
		// Wrap the error to prevent leaking the backend key.
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("foo %v is not found", id)
		}
		return nil, trace.Wrap(err)
	}
	foo, err := convertItemToFoo(item)
	return foo, trace.Wrap(err)
}
```

#### List

Listing can either be done via calling `backend.Backend.GetRange` manually in a loop or by making use of the functional
helpers in the
[stream](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/api/internalutils/stream/stream.go)
package to do the heavy lifting.

A listing operation should not abort entirely if a single item cannot be converted from a `backend.Item`, it should
instead be logged, and the rest of the page should be processed. Aborting an entire page when a single entry is invalid,
causes Teleport to be permanently unhealthy since it is never able to load or cache the affected resource(s).

```go
func (s *FooService) ListFoos(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
	rangeStart := backend.Key("foo", pageToken)
	rangeEnd := backend.RangeEnd(backend.ExactKey("foo"))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	// Increase the page size by one to detect if another page is available if
	// a full page match is retrieved without having to fetch the next page.
	pagSize++

	fooStream := stream.MapWhile(
		backend.StreamRange(ctx, s.backend, rangeStart, rangeEnd, limit),
		func (item backend.Item) (types.User, bool) {
			foo, err := convertItemToFoo(item)

			// Warn if an item cannot be converted but don't prevent the entire page from being processed.
			if err != nil {
				s.log.Warnf("Skipping foo at %s because conversion from backend item failed: %v", item.Key, err)
				return nil, true
			}
			return foo, true
	})

	foos, more := stream.Take(userStream, pageSize)
	var nextToken string
	if more && fooStream.Next() {
		nextToken = backend.NextPaginationKey(foos[len(foos)-1])
	}

	return foos, nextToken, trace.NewAggregate(err, fooStream.Done())
}
```

### Cache

One thing to consider when creating a resource is whether it will need to be cached. As mentioned above, any resource
that is cached must have its backend layer implement the specific set of operations required by the cache collections
[executor](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/cache/collections.go#L54-L76).
While `Upsert` and `DeleteAll` semantics are required by the cache it is preferred that the two methods are not directly
exposed in the gRPC API. Several existing resources include a `DeleteAll` purely for the cache that always returns a
`trace.NotImplemented` error. To avoid exposing the methods in the gRPC API at all, a local variant of the backend
service similar
to [`services.DynamicAccessExt`](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/services/access_request.go#L260-L278)
should be used.

Caching is most important for resources that are accessed frequently and in a "hot path" (i.e., during the process of
performing normal day-to-day operations). For example, resources like cluster networking config, session recording
config, CAs, roles, etc. which are retrieved per connection should be cached to reduce latency. Resources which are
accessed infrequently, or which scale linearly with cluster size are good examples of resources that should NOT be
cached.

If a resource is to be cached, it must be added to
the [Auth cache](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/cache/cache.go#L95-L154)
and the cache of any service that requires it. To add the `Foo` resource to the cache its executor would look similar
to the following:

```go
type fooExecutor struct{}

func (fooExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*foov1.Foo, error) {
	var (
		startKey string
		allFoos []*foov1.Foo
	)
	for {
		foos, nextKey, err := cache.Foo.ListFoos(ctx, 0, startKey, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allFoos = append(allFoos, foos...)

		if nextKey == "" {
			break
		}
		startKey = nextKey
	}
	return allFoos, nil
}

func (fooExecutor) upsert(ctx context.Context, cache *Cache, resource foov1.Foo) error {
	return cache.Foo.UpsertFoo(ctx, resource)
}

func (fooExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.FooLocal.DeleteAllFoos(ctx)
}

func (fooExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.Foo.DeleteFoo(ctx, &foov1.DeleteFoo{Name: resource.GetName()})
}
```

#### Event stream mechanism

The event stream mechanism allows events (e.g creation, updates, delete) regarding your resource to be subscribed to
by consumers.

In order to add your resource to the event stream mechanism, you must write a "parser" which will allow your resource
to be decoded from the event. This can be found in `lib/services/local/events.go`.

For example, to add a parser for `foo`:

```go
func newFooParser() *fooParser {
	return &fooParser{
		baseParser: newBaseParser(backend.Key(fooPrefix)),
	}
}

type fooParser struct {
	baseParser
}

func (p *fooParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindFoo, types.V1, 0)
	case types.OpPut:
		foo, err := services.UnmarshalFoo(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(foo), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
```

In addition, you will need to add support for instantiating this parser to the
switch in the `NewWatcher` function in `lib/services/local/events.go`:

```go
		case types.KindFoo:
			parser = newFooParser()
```

### api client

For users to interact with the new Foo RPC service the client in the `api`
module needs to be updated to provide the functionality. To do so a new method
should be added to `client.Client` that exposes the gRPC client for the service
as shown below.

```go
func (c *Client) FooClient() foopb.FooServiceClient {
 	return foopb.NewFooServiceClient(c.conn)
}
```

### Bootstrap

Teleport allows a fresh cluster to be created with a set of resources via the `--bootstrap` flag. This is primarily used
when creating a new cluster from a backup of another, or migrating an existing cluster from one storage backend to
another. Typically resources are retrieved from an existing cluster
via `tctl get all --with-secrets > /some/path/to/resources/yaml` and then spawning a new instance of with the bootstrap
flag: `teleport start --bootstrap=/some/path/to/resources/yaml`.

For a resource to be supported it must be added to the list of items retrieved with `tctl get all` and to the auth
initialization code responsible for parsing resources during
the [bootstrap process](https://github.com/gravitational/teleport/blob/d0f2b4406bfacc895f796b665d07c5d740280e38/lib/auth/init.go#L321-L335).

### Backward Compatibility

Changing existing resources which do not follow the guidelines laid out in this RFD may lead to breaking changes. It is
not recommended to change existing resources for change’s sake. Migrating APIs which do not conform to the
recommendations in this RFD can be made in a backward compatible manner. This can be achieved by adding new APIs that
conform with the advice above and falling back to the existing APIs if a `trace.NotImplemented` error is received. Once
all compatible versions of Teleport are using the new version of the API, the old API may be cleaned up.


### Proto Specification

Below is the entire specification for the examples above.

<details open><summary>Foo Proto</summary>

```protobuf
syntax = "proto3";

package teleport.foo.v1;

import "teleport/header/v1/metadata.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1;foov1";

// Foo is a resource that does foo.
message Foo {
  // The kind of resource represented.
  string kind = 1;
  // An optional subkind to differentiate variations of the same kind.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources shared.
  teleport.header.v1.Metadata metadata = 4;
  // The specific properties of a Foo. These should only be modified by
  // the creator/owner of the resource and not dynamically altered or updated.
  FooSpec spec = 5;
  // Any dynamic state of Foo that is modified during runtime of the
  // Teleport process.
  FooStatus status = 6;
}

// FooSpec contains specific properties of a Foo that MUST only
// be modified by the owner of the resource. These properties should
// not be automatically adjusted by Teleport during runtime.
message FooSpec {
  string bar = 1;
  int32 baz = 2;
  bool qux = 3;
}

// FooStatus contains dynamic properties of a Foo. These properties are
// modified during runtime of a Teleport process. They should not be exposed
// to end users and ignored by external IaC tools.
message FooStatus {
  google.protobuf.Timestamp next_audit = 1;
  string teleport_host = 2;
}
```

</details>

<details open><summary>Foo Service</summary>

```protobuf
syntax = "proto3";

package teleport.foo.v1;

import "teleport/foo/v1/foo.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1;foov1";

// FooService provides an API to manage Foos.
service FooService {
  // GetFoo returns the specified Foo resource.
  rpc GetFoo(GetFooRequest) returns (Foo);

  // ListFoos returns a page of Foo resources.
  rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);

  // CreateFoo creates a new Foo resource.
  rpc CreateFoo(CreateFooRequest) returns (Foo);

  // UpdateFoo updates an existing Foo resource.
  rpc UpdateFoo(UpdateFooRequest) returns (Foo);

  // UpsertFoo creates or replaces a Foo resource.
  rpc UpsertFoo(UpsertFooRequest) returns (Foo);

  // DeleteFoo hard deletes the specified Foo resource.
  rpc DeleteFoo(DeleteFooRequest) returns (google.protobuf.Empty);
}

// Request for GetFoo.
message GetFooRequest {
  // The id of the Foo resource to retrieve.
  string foo_id = 1;
}


// Request for ListFoos.
//
// Follows the pagination semantics of
// https://cloud.google.com/apis/design/standard_methods#list.
message ListFoosRequest {
  // The maximum number of items to return.
  // The server may impose a different page size at its discretion.
  int32 page_size = 1;

  // The page_token value returned from a previous ListFoo request, if any.
  string page_token = 2;
}

// Response for ListFoos.
message ListFoosResponse {
  // Foo that matched the search.
  repeated Foo foos = 1;

  // Token to retrieve the next page of results, or empty if there are no
  // more results exist.
  string next_page_token = 2;
}

// Request for CreateFoo.
message CreateFooRequest {
  // The foo resource to create.
  Foo foo = 1;
}

// Request for UpdateFoo.
message UpdateFooRequest {
  // The foo resource to update.
  Foo foo = 1;

  // The update mask applied to a Foo.
  // Fields are masked according to their proto name.
  FieldMask update_mask = 2;
}

// Request for UpsertFoo.
message UpsertFooRequest {
  // The foo resource to upsert.
  Foo foo = 2;
}

// Request for DeleteFoo.
message DeleteFooRequest {
  // Name of the foo to remove.
  string foo_id = 1;
}
```

</details>

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
avoided.

### New resource checklist

Use the non-exhaustive list below as a guide when adding a new resource. Further sections in the RFD dive into more
detail about what is needed to complete a particular item.

- [ ] Create proto spec for resource and RPC service
- [ ] Create backend service
- [ ] Implement gRPC service with authz
- [ ] Add resource client to `api` client
- [ ] Add resource support to tctl (get, create, edit)
- [ ] Add resource parser to event stream mechanism
- [ ] Optional: Add resource to cache
- [ ] Add support for bootstrapping the resource
- [ ] Add support for resource to Teleport Operator
- [ ] Add support for resource to Teleport Terraform Provider

### Defining a resource

At the top level, all new resources MUST include:

- `kind`: a string indicating the kind of resource
- `sub_kind`: a string indicating the resource sub_kind, the field must be defined and validated even if it is unused.
- `version`: a string indicating the resource version
- `metadata`: a `teleport.header.v1.Metadata` including the resource name, labels, description, expiration, and revision.
- `spec`: a message including all other user-specified resource fields.

Most new resources SHOULD include:

- `scope`: a string indicating the administrative scope of the resource, see RFD 229.

New resources MAY include:

- `status`: a message including all resource state that is computed server-side and not user-provided.


While the kind and version may seem like they would be easy to derive from the message definition itself, they need to be defined so that anything processing a generic resource can identify which resource is being processed. For example, `tctl` interacts with resources in their in raw yaml text form and leverages `services.UnknownResource` to identify the resource and act appropriately.

All properties defined outside the `status` of a resource MUST only be modified by the
owner/creator of the resource. For example, if a resource is created via
`tctl create`, then any fields within the `spec` MUST not be altered dynamically
by the Teleport process. When Teleport automatically modifies the `spec` during
runtime it causes drift between what is in the Teleport backend and the state stored by external IaC tools.
If a resource has properties that must be modified by Teleport, a separate `status` field should
be added to the resource to contain them. These fields will be ignored by IaC tools during their reconciliation.

Do NOT define a `CheckAndSetDefaults()` method for new resource kinds, default
values outside of the resource `status` must not be set.
Resources MUST have a `Validate` or `StrongValidate` function that applies
strict validation to a resource whenever it is accepted from an untrusted
source and before it is persisted to storage.
Resources MAY have a `WeakValidate` function that applies a weaker form of
validation to a resource when it is accepted from a trusted source such as
backend storage.

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
  // The scope of the resource
  string scope = 5;
  // The specific properties of a Foo. These should only be modified by
  // the creator/owner of the resource and not dynamically altered or updated.
  FooSpec spec = 6;
  // Any dynamic state of Foo that is modified during runtime of the
  // Teleport process.
  FooStatus status = 7;
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

Prefer a string over an enum when users will be expected to read or write the
field in a resource YAML or IaC definition.
If you use an enum, the terraform provider will only accept integer values.
Enums may be used when users will not interact with the field.

### API

All APIs should follow the conventions listed below that are largely based on
the [Google API style guide](https://cloud.google.com/apis/design/standard_methods) and the [Buf
Protobuf style guide](https://buf.build/docs/best-practices/style-guide/)

#### Create

The `Create` RPC takes a resource to be created and must also return the newly created resource so that any fields that
are populated server side are provided to clients without requiring an additional call to `Get`.

The request MUST fail and return a `trace.AlreadyExists` error if a matching resource is already present in the backend.

```protobuf
// Creates a new Foo resource in the backend.
rpc CreateFoo(CreateFooRequest) returns (CreateFooResponse);

message CreateFooRequest {
  // The desired Foo to be created.
  Foo foo = 1;
}

message CreateFooResponse {
  Foo foo = 1;
}
```

#### Update

The `Update` RPC takes a resource to be updated and must also return the updated resource so that any fields that are
populated server side are provided to clients without requiring an additional call to `Get`. If partial updates of a
resource are desired, the request MAY contain
a [FieldMask](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#field-mask).

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Updates an existing Foo in the backend.
rpc UpdateFoo(UpdateFooRequest) returns (UpdateFooResponse);

message UpdateFooRequest {
  // The full Foo resource to update in the backend.
  Foo foo = 1;
  // A partial update for an existing Foo resource.
  FieldMask update_mask = 2;
}

message UpdateFooResponse {
  Foo foo = 1;
}
```

#### Upsert

> The `Create` and `Update` RPCs should be preferred over `Upsert` for user-driven interactions,
> see [#1326](https://github.com/gravitational/teleport/issues/1326) for more details.
> The `Upsert` method is necessary for IaC integrations that define the source of truth for the resource.

The `Upsert` RPC takes a resource that will overwrite a matching existing resource or create a new resource if one does
not exist. The upserted resource is returned so that any fields that are populated server side are provided to clients
without requiring a call to `Get`.

```protobuf
// Creates a new Foo or replaces an existing Foo in the backend.
rpc UpsertFoo(UpsertFooRequest) returns (UpsertFooResponse);

message UpsertFooRequest {
  // The full Foo resource to persist in the backend.
  Foo foo = 1;
}

message UpsertFooResponse {
  Foo foo = 1;
}
```

#### Get

The `Get` RPC takes the parameters required to match a resource (usually the resource name and scope should suffice), and returns
the matched resource.

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Returns a single Foo matching the request
rpc GetFoo(GetFooRequest) returns (GetFooResponse);

message GetFooRequest {
  // A name to match the Foo by. Some resource may require more parameters to match and
  // may not use the name at all.
  string name = 1;
  // The scope of the resource.
  string scope = 2;
}

message GetFooResponse {
  Foo foo = 1;
}
```

#### List

The `List` RPC takes the requested page size and starting point and returns a list of resources that match. If there are
additional resources, the response MUST also include a token that indicates where the next page of results begins.

`List` RPC can optionally provide a `Filter` message and/or a `SortMode` field where supported.
It MUST accept a scope filter if the resource is scoped.

Do not provide an unpaginated `GetAllFoos` RPC.

```protobuf
// Returns a page of Foo and the token to find the next page of items.
rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);

message ListFoosRequest {
  // The maximum number of items to return.
  // The server may impose a different page size at its discretion.
  int32 page_size = 1;
  // The next_page_token value returned from a previous List request, if any.
  string page_token = 2;

  enum SortMode {
    SORT_MODE_UNSPECIFIED = 0;
    SORT_MODE_FIELD_EXAMPLE = 1;
  }
  // sort_mode specifies the sorting type for the results.
  SortMode sort_mode = 3;
  // is_sort_descending specifies sort direction
  bool is_sort_descending = 4;

  // filter is a collection of fields to filter Foos
  ListFoosFilter filter = 5;

  // Filters foos by scope.
  teleport.scopes.v1.Filter scope_filter = 6;
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

##### Pagination Stability

As of time of writing 2025-11-24 the backend does not support snapshotting. Meaning every `List` query executed
against the backend has a potentially different view of the data. It is possible for:
* Items to be deleted or created between `List` calls.
* Mutable fields to be modified causing items to be reordered when sorting resulting in duplicate entries.

#### Sorting Support

Note that as of time of writing (2025-11-24), advanced sorting is possible in Cache only.

This is achieved via the [sortcache](https://github.com/gravitational/teleport/blob/96f222c00624e3f7ad3cbcd1859936420b438725/lib/utils/sortcache/sortcache.go#L47) package.

Each index requires a dedicated key function that returns lexicographically sortable key, for example:

```go
func (u *inventoryInstance) getAlphabeticalKey() bytestring {
	var name, id string
	if u.isInstance() {
		name = u.instance.GetHostname()
		id = u.instance.GetName()
	} else {
		name = u.bot.GetSpec().GetBotName()
		id = u.bot.GetSpec().GetInstanceId()
	}

	return bytestring(ordered.Encode(name, id, u.getKind()))
}
```

With the pagination token using a base32hex representation of that key:
```go
nextPageToken = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(rawKey))
```

base32hex is the chosen format for tokens which maintains the order between keys, which can be useful for debugging and for future optimizations.

1. Default sort mode must be the same and use the same key format as the pagination in the backend.
2. Compound keys should use make use of the `rsc.io/ordered` package.
3. Sorting options must remain unchanged for subsequent `List`.
4. The backend should return a `*trace.CompareFailedError` if the sort mode is unsupported by the backend.

##### Page Token contract

The backend only enforces a single contract for the token:
* Empty token (`""`) indicates no more items.
* Any other string value indicates further results are available.

If possible the structure of the token should be opaque to the client to prevent tampering and reduce the implicit API surface.

##### Page Size

Clients should make use of [clientutils](https://github.com/gravitational/teleport/blob/ae1c0890bccf304b0bb40b9a2436501b4ec2a966/api/utils/clientutils/resources.go#L19) to automatically adjust the page size when fetching resources:
```go
foos, err := clientutils.Resources(ctx, client.ListFoos)
```

#### Delete

The `Delete` RPC takes the parameters required to match a resource and performs a hard delete of the specified resource
from the backend and returns an empty response message. 

The request MUST fail and return a `trace.NotFound` error if there is no matching resource in the backend.

```protobuf
// Remove a matching Foo resource
rpc DeleteFoo(DeleteFooRequest) returns (DeleteFooResponse);

message DeleteFooRequest {
  // Name of the foo to remove. Some resource may require more parameters to match and
  // may not use the name at all.
  string name = 1;
  // Scope of the foo to remove.
  string scope = 2;
}

message DeleteFooResponse {}
```

### Backend Storage

A backend service to handle persisting and retrieving a resource from the backend is typically defined in
`lib/services/local/foo.go`. An accompanying interface which mirrors the service is defined in `lib/services/foo.go`.
Continuing on with the example above, the sections below show how the backend service for the `Foo` resource might look like.

For most cases, when adding a new resource, it is preferred to create a service
that wraps the `generic.ServiceWrapper` or `generic.ScopeAwareServiceWrapper`
over implementing everything from scratch.
If custom behavior is required for a subset of backend operations, they may be
implemented directly while all the other operations still make use of the
generic service.

If the resource has a scope, it should be namespace by scope, meaning that each
resource should be uniquely identified by a `(name, scope)` pair with a unique
backend key.
In this case, the backend service should wrap `generic.ScopeAwareServiceWrapper`.

```go
// FooService is a storage service for Foos.
type FooService struct {
	service *generic.ScopeAwareServiceWrapper[*foov1.Foo]
}

// NewFooService returns a new service for Foos.
func NewFooService(bk backend.Backend) (*FooService, error) {
	service, err := generic.NewScopeAwareServiceWrapper(generic.ScopeAwareServiceWrapperConfig[*foov1.Foo]{
		Backend:               bk,
		ResourceKind:          foos.Kind,
		UnscopedBackendPrefix: backend.NewKey("foo"),
		ScopedBackendPrefix:   backend.NewKey("scoped", "foo"),
		MarshalFunc:           services.MarshalProtoResource[*foov1.Foo],
		UnmarshalFunc:         services.UnmarshalProtoResource[*foov1.Foo],
		ValidateFunc:          foos.StrongValidate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FooService{
		service: service,
	}, nil
}
```

#### Resource validation

The strictest validation of a resource should be performed prior to write operations. Any resource persisted in the
backend should be guaranteed to be valid. Read operations should not perform resource validations, doing so could
prevent a resource being read if validations are modified to be more restrictive after a resource had already been
written. This allows for resilient reads which ensure that any resource stored is always allowed to be retrieved from
the backend.

#### Create

When creating a new resource, the `backend.Backend.Create` method should be used to persist the resource.
It is also imperative that the revision generated by the backend is set on the
returned resource.
`generic.(ScopeAware)ServiceWrapper` handles this.

```go
func (s *FooService) CreateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.CreateFoo(ctx, foo)
}
```

#### Update

All update operations should prefer `backend.Backend.ConditionalUpdate` over the `backend.Backend.Update` method to
prevent blindly overwriting an existing item. When using conditional update, the backend write will only succeed if the
revision of the resource in the update request matches the revision of the item
in the backend.
Conditional updates should also be preferred over `CompareAndSwap` operations.

```go
func (s *FooService) UpdateFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.ConditionalUpdateResource(ctx, foo)
}
```

#### Upsert

When upserting a resource, the `backend.Backend.Put` method should be used to persist the resource. It is also
imperative that the revision generated by the backend is set on the returned resource.

```go
func (s *FooService) UpsertFoo(ctx context.Context, foo *foov1.Foo) (*foov1.Foo, error) {
	return s.service.UpsertResource(ctx, foo)
}
```

#### Get

The Get method should accept the protobuf `Get<Resource>Request` message as a
parameter, so that options can be added to the request in a backward-compatible
manner.

`trace.NotFound` errors from the backend should be re-wrapped to include the resource name instead of the backend key for friendlier user-facing errors.
`generic.(ScopeAware)ServiceWrapper` handles this.

```go
// GetFoo returns a single Foo matching the request
func (s *FooService) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error) {
	return s.service.GetResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}
```

#### Delete

The Delete method should accept the protobuf `Delete<Resource>Request` message as a
parameter, so that options can be added to the request in a backward-compatible
manner.

`trace.NotFound` errors from the backend should be re-wrapped to include the resource name instead of the backend key for friendlier user-facing errors.
`generic.(ScopeAware)ServiceWrapper` handles this.

```go
// DeleteFoo removes a matching Foo resource
func (s *FooService) DeleteFoo(ctx context.Context, req *foov1.DeleteFooRequest) error {
	return s.service.DeleteResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}
```

#### List

Listing can be done via collecting a stream returned by
`backend.Backend.Items`, making use of the functional helpers in the
[stream](https://github.com/gravitational/teleport/blob/a32c69b581ff0ceef55b83a601cfb54c6d52d710/lib/itertools/stream/stream.go) package.

The List method should accept the protobuf `List<Resource>Request` message as a parameter, so that options including filters can be added to the request in a backward-compatible manner.

A listing operation should not abort entirely if a single item cannot be
converted from a `backend.Item`, it should instead be logged, and the rest of
the page should be processed.
Aborting an entire page when a single entry is invalid causes Teleport to be
permanently unhealthy since it is never able to load or cache the affected
resource(s).
`generic.(ScopeAware)ServiceWrapper` handles this.

```go
// ListFoos returns a page of Foos and the token to find the next page of items.
func (s *FooService) ListFoos(ctx context.Context, req *foov1.ListFoosRequest) ([]*foov1.Foo, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	return s.service.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), func(foo *foov1.Foo) bool {
		return scopes.MatchScope(scopeFilter, foo.GetScope())
	})
}
```

#### Range

The Range method should accept the protobuf `List<Resource>Request` message as a parameter, so that options including filters can be added to the request in a backward-compatible manner.

A range operation should not abort entirely if a single item cannot be
converted from a `backend.Item`, it should instead be logged, and the rest of
the range should be processed.
Aborting an entire range when a single entry is invalid causes Teleport to be
permanently unhealthy since it is never able to load or cache the affected
resource(s).
`generic.(ScopeAware)ServiceWrapper` handles this.

```go
// RangeFoos ranges over all foos matching any scope filter specified in the
// request, between startKey and endKey interpreted as scoped resource cursors.
func (s *FooService) RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error] {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[*foov1.Foo](trace.Wrap(err))
	}
	return stream.FilterMap(s.service.Resources(ctx, startKey, endKey), func(foo *foov1.Foo) (*foov1.Foo, bool) {
		return foo, scopes.MatchScope(scopeFilter, foo.GetScope())
	})
}
```

### RBAC

Making use of the `stream.FilterMap` pattern can be used to verify access to a resource when listing:

```go
// ListFoos returns a page of Foo resources.
func (s *Service) ListFoos(ctx context.Context, req *foov1.ListFoosRequest) (*foov1.ListFoosResponse, error) {
	authzContext, err := s.cfg.ScopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// do a pre-check to weed out requests that definitely won't be authorized.
	ruleCtx := authzContext.RuleContext()
	if err := authzContext.CheckerContext.CheckMaybeHasAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// list method scope filters must use identity-based defaults per RFD 0229i
	req.SetScopeFilter(authzContext.CheckerContext.ResolveScopeFilter(req.GetScopeFilter()))

	stream := stream.FilterMap(
		s.cfg.Reader.RangeFoos(ctx, req, req.GetPageToken(), ""),
		func(foo *foov1.Foo) (*foov1.Foo, bool) {
			// Skip foos the caller is not authorized to see
			ruleCtx := authzContext.RuleContext()
			ruleCtx.Resource153 = foo
			if err := authzContext.CheckerContext.Decision(ctx, foo.GetScope(), func(checker *services.ScopedAccessChecker) error {
				return checker.CheckAccessToRules(&ruleCtx, foos.Kind, types.VerbReadNoSecrets, types.VerbList)
			}); err != nil {
				return nil, false
			}
			return foo, true
		},
	)
	page, nextPageToken, err := generic.CollectPageAndCursor(
		stream,
		int(req.GetPageSize()),
		foos.MakeCursor,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return foov1.ListFoosResponse_builder{
		Foos:          page,
		NextPageToken: nextPageToken,
	}.Build(), nil
}
```

### Cache

Caching is most important for resources that are accessed frequently and in a "hot path" (i.e., during the process of
performing normal day-to-day operations). For example, resources like cluster networking config, session recording
config, CAs, roles, etc. which are retrieved per connection should be cached to reduce latency. Resources which are
accessed infrequently, or which scale linearly with cluster size are good examples of resources that should NOT be
cached.

If a resource is to be cached, it must be added to the
[Auth cache](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/cache/cache.go#L95-L154)
and the cache of any service that requires it. To add the `Foo` resource to the cache, a new `lib/cache/foo.go` file should be
added which houses a collection and any `(Cache)` receiver methods. The collection MUST be
[registered](https://github.com/gravitational/teleport/blob/4c71ad634b3564ad3234f1b3e46f00faabdbcbef/lib/cache/collections.go#L164-L712)
and added to the list of cache
[collections](https://github.com/gravitational/teleport/blob/4c71ad634b3564ad3234f1b3e46f00faabdbcbef/lib/cache/collections.go#L65-L135).

> [!NOTE]
> Some resources may not need to be stored in the cache, but still need to be registered with the cache to allow
watchers for said resource to be created. These resources do NOT need a collection and can instead be [registered](https://github.com/gravitational/teleport/blob/cc9712c0444ee16e07adc75c21cb6b0a6ebd1af8/lib/cache/collections.go#L137-L147)
with the unique set of resources to indicate as such to the cache.

<details open><summary>lib/cache/foo.go</summary>

```go
package cache

import (
	"context"
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type fooIndex string

const (
	fooNameIndex fooIndex = "name"
)

func newFooCollection(upstream services.FooUpstream, w types.WatchKind) (*collection[*foov1.Foo, fooIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter FooUpstream")
	}

	return &collection[*foov1.Foo, fooIndex]{
		store: newStore(
			foos.Kind,
			proto.CloneOf[*foov1.Foo],
			map[fooIndex]func(*foov1.Foo) string{
				// sorted by name
				fooNameIndex: foos.MakeCursor,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*foov1.Foo, error) {
			return stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
				return upstream.ListFoos(ctx, foov1.ListFoosRequest_builder{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
					// TODO: propagate filter from WatchKind.
					ScopeFilter: nil,
				}.Build())
			}))
		},
		watch: w,
	}, nil
}

func (c *Cache) GetFoo(ctx context.Context, req *foov1.GetFooRequest) (*foov1.Foo, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetFoo")
	defer span.End()

	getter := genericGetter[*foov1.Foo, fooIndex]{
		cache:      c,
		collection: c.collections.foos,
		index:      fooNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*foov1.Foo, error) {
			return c.FooUpstream.GetFoo(ctx, req)
		},
	}

	fooCursor := scopes.MakeResourceCursor(req.GetScope(), req.GetName())
	out, err := getter.get(ctx, fooCursor)
	return out, trace.Wrap(err)
}

func (c *Cache) RangeFoos(ctx context.Context, req *foov1.ListFoosRequest, startKey, endKey string) iter.Seq2[*foov1.Foo, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/RangeFoos")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[*foov1.Foo](trace.Wrap(err))
	}

	lister := genericLister[*foov1.Foo, fooIndex]{
		cache:      c,
		collection: c.collections.foos,
		index:      fooNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*foov1.Foo, string, error) {
			return c.FooUpstream.ListFoos(ctx, foov1.ListFoosRequest_builder{
				PageSize:    int32(pageSize),
				PageToken:   pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		filter: func(foo *foov1.Foo) bool {
			return scopes.MatchScope(scopeFilter, foo.GetScope())
		},
		nextToken: foos.MakeCursor,
	}

	return lister.Range(ctx, startKey, endKey)
}
```

</details>

#### Event stream mechanism

The event stream mechanism allows events (e.g creation, updates, delete) regarding your resource to be subscribed to
by consumers.

In order to add your resource to the event stream mechanism, you must write a "parser" which will allow your resource
to be decoded from the event. This can be found in `lib/services/local/events.go`.

You should NOT return a `ResourceHeader` for delete events, instead return a skeleton of the proper resource type with only the fields that can be derived from the key set.

For example, to add a parser for `foo`:

```go
func newFooParser() *fooParser {
	return &fooParser{
		baseParser: newBaseParser(
			fooUnscopedWatchPrefix(),
			fooScopedWatchPrefix(),
		),
	}
}

type fooParser struct {
	baseParser
}

func (p *fooParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		sqn, err := fooNameFromKey(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		foo := foov1.Foo_builder{
			Kind:    foos.Kind,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: sqn.Name,
			}.Build(),
			Scope: sqn.Scope,
		}.Build()
		return types.Resource153ToLegacy(foo), nil
	case types.OpPut:
		foo, err := services.UnmarshalProtoResource[*foov1.Foo](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(foo), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func fooNameFromKey(key backend.Key) (scopes.QualifiedName, error) {
	switch {
	case key.HasPrefix(fooScopedWatchPrefix()):
		components := key.TrimPrefix(fooScopedWatchPrefix()).Components()
		if len(components) != 2 {
			return scopes.QualifiedName{}, trace.NotFound("failed parsing %v", key.String())
		}
		encodedScope, name := components[0], components[1]
		scope, err := scopes.DecodeFromKey(encodedScope)
		if err != nil {
			return scopes.QualifiedName{}, trace.Wrap(err)
		}
		return scopes.QualifiedName{
			Scope: scope,
			Name:  name,
		}, nil
	case key.HasPrefix(fooUnscopedWatchPrefix()):
		components := key.TrimPrefix(fooUnscopedWatchPrefix()).Components()
		if len(components) != 1 {
			return scopes.QualifiedName{}, trace.NotFound("failed parsing %v", key.String())
		}
		return scopes.QualifiedName{
			Name: components[0],
		}, nil
	default:
		return scopes.QualifiedName{}, trace.NotFound("failed parsing %v", key.String())
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


### Reference Implementation

A branch with a complete implementation of a Foo service including the proto specification, backend storage support, a gRPC API layer with authorization, `tctl` support, events support, and cache support can be found at https://github.com/gravitational/teleport/pull/68358

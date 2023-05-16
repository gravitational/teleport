---
authors: Andrey Bulgakov (andrey.bulgakov@goteleport.com)
state: implemented
---

# RFD 0114 - Partial cache healthiness

## Required Approvers

* Engineering: @fspmarshall && @espadolini && @rosstimothy


## What

The caching subsystem should be improved to gracefully handle situations in which a downstream teleport service
wants to cache a resource unsupported by the auth service it's connected to.


## Why

This situation can happen because of a version mismatch between teleport services within a cluster or
within a group of trusted clusters.

Currently, it is treated as a global cache error by the downstream service: the entire cache gets stuck in an error
loop and all requests get routed to the upstream API as if caching was disabled.

Currently, because of this, we only add new resources to the cache of downstream services in major versions which makes
development less flexible.

It would be much more desirable to handle this gracefully so that cache would still work for unaffected resources
and only requests for the resources unavailable for caching would be routed to the upstream API.

The upgraded cache implementation would improve the behaviour in two particular scenarios:

1. An existing but uncached resource gets added to cache in a new teleport version. Improved cache would route requests
   for that resource to Auth API where they would likely succeed.
2. A totally new resource gets added to cache in a new teleport version. Requests forwarded to Auth API
   would fail with `NotImplemented`, but the remaining cache would not be affected. The component responsible
   for the failed request would have a chance to handle the error and gracefully degrade its functionality.

Relevant issue: [#21586](https://github.com/gravitational/teleport/issues/21586)


## Details

### How cache currently works

`Cache` interface shares numerous methods with Auth API Client interface `ClientI` with the intention that an instance
of Cache could be used transparently instead of the client. Cache has a reference to the actual API client that
it can be used as a source of truth.

Internally, the implementation of Cache includes implementations of all relevant upstream services
running off of an in-memory storage backend. It makes use of the `Events` interface implemented by the API client
to subscribe to *relevant* changes happening on the actual Auth service backend, and replay them into the memory backend.
Each downstream service has a specific set of resource kinds that are relevant to it and events for which it wants
to follow. Caches in the Auth service itself use the same approach, but they use the event stream from the backend directly.

```go
type Events interface {
    NewWatcher(ctx context.Context, watch Watch) (Watcher, error)
}
```

For synchronization purposes, a special `OpInit` event with no payload always comes first on the stream.

Upon receipt of `OpInit`, cache fetches all relevant resources using the API client, puts them into the in-memory storage
and starts replaying the event stream. Only at this point cache transitions into a healthy state and can be used
by the readers. While being unhealthy, cache routes all read requests to the original API Client. It can become unhealthy
again if the event stream gets interrupted.

Currently, if some of the requested kinds are unsupported by Auth API, cache never becomes healthy.
The goal of this RFD is to propose a way to avoid that.

Auth API client is not the only thing that implements the `Events` interface and can produce a stream of events.
It's implemented by multiple types, including `local.EventsService` which generates events from the storage backend and
`Cache` itself which helps to fan out an event stream to multiple consumers. There are also a few implementations
of `Events` that wrap each other to provide validation.


### General idea

When requesting a stream of events from Auth service, the cache should set a new flag on the request to opt in for
the new partial success mode. In this mode, the request won't fail if some of the requested kinds cannot be watched.
Instead, it will succeed and the first `OpInit` event on the resulting stream will contain the list of kinds that
will be included in the event stream. The list will be attached to a new special resource type associated with the event.

Cache will use the list of confirmed kinds for controlling initial cache synchronization, read request routing and
propagation of the kind availability to fanout event readers.

During the initial synchronization, cache will delete data for all configured resource kinds from the in-memory storage.
Then, only resources of confirmed kinds will be fetched and upserted into the storage.

The routing logic implemented in the `readGuard` component will also need to be updated. Currently, it's binary:
if the cache is healthy, all reads are routed to the local services powered by the in-memory backend,
or to the API client otherwise. Read guard should become aware of the resource kinds that were confirmed by the event source
and route only requests related to those resource kinds to the in-memory storage, assuming that cache is in a healthy state.
Requests related to kinds not included in the confirmed list should always be routed to the API client.

Throughout its lifecycle, cache can encounter disconnections and other errors interrupting the event stream. This brings
it back to unhealthy state. Every time it becomes healthy again, it's considered a new *generation* of cache. Since every
time it reconnects to the Auth service, it might be of different versions, the list of confirmed kinds should be considered
specific to the current generation and refreshed during each re-initialization. When deleting data from the in-memory
storage during the initial synchronization, resource kinds from the current and the previous generation must be deleted
to avoid storing stale data for resource kinds that aren't included anymore.

Since cache can also act as an event source, it should propagate the availability status if a client tries to use the cache
to subscribe to events of a resource kind that wasn't confirmed to the cache itself. Such client will also receive a
list of confirmed kinds, omitting unavailable ones, on the first `OpInit` event or a failure depending on the supplied
AllowPartialSuccess flag. 


### Changes to the Events interface

Because the `Events` interface is central to the caching subsystem, it will require some changes. Also, because it has
so many implementations and is used in so many contexts, some rules need to be established and all the implementations
must be updated to behave in a consistent way.

None of the proposed changes to the interface are direct though. They all happen through related types and behaviour.


#### types.Watch
```diff
type Watch struct {
	Name string
	Kinds []WatchKind
	QueueSize int
	MetricComponent string
+
+	// AllowPartialSuccess enables partial success mode in which the request won't fail if some of the requested Kinds
+	// are not available for watching.
+	AllowPartialSuccess bool
}
```

#### types.WatchStatus

The `Event` struct has fields for a type and an associated resource. Currently, events with type `OpInit` have
the associated resource field set to `nil`. We should add a new resource type that can be attached to `OpInit` events
and carry the list of resource kinds what weren't available for watching:

```protobuf
message WatchStatusV1 {
  string Kind = 1;
  string SubKind = 2;
  string Version = 3;
  Metadata Metadata = 4;
  WatchStatusSpecV1 Spec = 5;
}

message WatchStatusSpecV1 {
  repeated WatchKind Kinds = 1;
}
```


#### Rules for `Events` / `NewWatcher()` implementations 

If, during validation, an implementation of `NewWatcher()` determines that a requested resource kind cannot be watched,
it should remove it from `w.Kinds`. To avoid concurrency issues, when making changes to `w.Kinds`, a new slice should
be allocated instead of modifying the original one. If `AllowPartialSuccess` is not set or `w.Kinds` becomes empty,
an error must be returned. To avoid breaking error handling in old clients, it should be the exact error value that led
to the rejection and would've been immediately returned by the current version of code. 


#### Implementation details: cache.Cache and cache.Fanout

`cache.Cache` implements the `Events` interface so that it could fan out events to other consumers that want to watch
a subset of its event stream.

When `NewWatcher()` is called on a Cache instance that's already in a healthy state - it knows which resource kinds
have been confirmed and are included in its event stream. Based on that, `NewWatcher()` can fail immediately
if some of the requested resource kinds aren't available but `AllowPartialSuccess` isn't set. Otherwise, it can partially
succeed and pass a list of confirmed resource kinds on the `OpInit` event, omitting the unavailable ones.

When `NewWatcher()` is called on an unhealthy cache though, it's only possible to perform partial validation and make sure
that Cache is at least configured to watch the requested kinds. In this situation Cache will have to return a `fanoutWatcher`
that might fail later. When Cache receives its `OpInit` event and propagates that to fanout watchers, each will have to
re-check if the request conditions are still met. Some might need to close with errors.


#### Implementation details: client.streamWatcher

`streamWatcher` allows watching events on the Auth Service over a GRPC connection. To add partial success mode
to it, we'll need to make a small update to the API protocol and add a field that will be used to pass 
the `AllowPartialSuccess` flag.

```diff
message Watch {
   repeated WatchKind Kinds = 1;
+   bool AllowPartialSuccess = 2;
}
```

On the server side this method is implemented by several `NewWatcher()` implementations wrapping each other and adding
validations on each level. The last implementation in the chain is `Cache` which is already covered above.   


#### Implementation details: local.watcher

This one just follows the general rules:
- checks each requested kind, build a list of successful ones
- if some kinds were rejected and `AllowPartialSuccess` isn't set, returns the first original validation error
- otherwise, generates `OpInit` with `WatchStatus` resource attached and the list of confirmed kinds on it


### Backward compatibility

An older client will send a `WatchEvents` request without the `AllowPartialSuccess` flag and expect all requested kinds
to be recognized. Therefore, updated Auth server should fail such requests if any of the requested kinds is unavailable. 
In a reverse situation, if an older Auth service sends an `OpInit` event without a resource to an updated client,
the client should assume that all requested resources will be included in the event stream, otherwise the old `WatchEvents()`
API method would have simply failed.


### Generic readGuard[RD]

Read guard should now make the routing decision based on the resource kind that's being accessed.

As suggested in the original issue [#21586](https://github.com/gravitational/teleport/issues/21586), there's a simple
solution in which we would just pass the kind to `(*Cache).read()` and based on the cache health and whether the kind
was confirmed we'd get a `readGuard` routing all calls either to cache or to the API client. This option doesn't protect
from programming errors where somebody would request a guard for one kind and then would call a method associated with
another one.

Another option suggested in that issue is to leverage generics to make `readGuard` only provide methods specific to
the requested kind. Here's how that could look:

```go
// collectionReader extends collection interface which can return an appropriate reader interface,
// e.g. AppGetter, implemented by either cache or API client, depending on the passed in cacheOK.
// The same instances of genericCollection would satisfy collection and collectionReader, but can't modify the original interface
// because we'd still need to have a map[resourceKind]collection and go generics don't support type variance.
type collectionReader[RD any] interface {
	collection
	getReader(cacheOK bool) RD
}

// executor gets updated with a new type parameter and a method.
// e.g., executor[types.Application, services.AppGetter]
type executor[R types.Resource, RD any] interface {
   getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]R, error)
   upsert(ctx context.Context, cache *Cache, value R) error
   deleteAll(ctx context.Context, cache *Cache) error
   delete(ctx context.Context, cache *Cache, resource types.Resource) error
   isSingleton() bool

   // *NEW* getReader will return a cached implementation RD or one powered by the API client.
   // cacheOK here indicates health of the specific collection.
   getReader(cache *Cache, cacheOK bool) RD
}

// getReader is an example of an implementation of getReader in a resource executor
func (appExecutor) getReader(cache *Cache, cacheOK bool) services.AppGetter {
   if cacheOK {
       return cache.appsCache
   }
   return cache.Config.Apps
}


// genericCollection also gets a new type parameter for the reader type and an extra method to satisfy collectionReader
type genericCollection[R types.Resource, RD any, E executor[R, RD]] struct {
   cache *Cache
   watch types.WatchKind
   exec  E
}

// getReader is a simple method that satisfies collectionReader and delegates the decision to the executor.
func (c *genericCollection[_, RD, _]) getReader(cacheOK bool) RD {
    return c.exec.getReader(c.cache, cacheOK)
}

// readCache is a replacement of (*Cache).read(). It has to become a package function because Go doesn't support
// generic methods with new type variables. collectionReader[RD] provides watchKind() for routing decisions and the reader
// type RD for constructing a generic readGuard[RD].
// The value of cacheOK passed to collection.getReader() will depend on both overall cache health and whether
// the resource kind was confirmed during the initialization.  
func readCache[RD any](cache *Cache, collection collectionReader[RD]) (readGuard[RD], error) { ... }

// readGuard now has only one generic field instead of listing all possible services.  
type readGuard[RD any] struct {
   reader   RD
   release  func()
   released bool
}

// cacheCollections is a type an instance of which will replace the c.collections map on Cache
// This struct will be returned by setupCollections() instead of the map.
type cacheCollections struct {
    // byKind is the former c.collections map
   byKind       map[resourceKind]collection
   // apps is a typed collectionReader reference that can be passed to readCache()
   apps         collectionReader[services.AppGetter]
   // kubeClusters is another one
   kubeClusters collectionReader[services.KubernetesGetter]
   // ... long list of other typed collectionReaders ...
}

// GetApps is finally an example of how all of this would work together
func (c *Cache) GetApps(ctx context.Context) ([]types.Application, error) {
   rg, err := readCache(c, c.collections.apps)
   if err != nil {
       return nil, trace.Wrap(err)
   }
   defer rg.Release()
   return rg.reader.GetApps(ctx)
}
```

This seems like a nicer solution, but involving a lot of code changes, so the final implementation might fall back
to the first option `(*Cache).read(kind types.WatchKind)`.

---
authors: Michael Myers (michael.myers@goteleport.com)
state: implemented v14.0
---

# RFD 137 - Unified Resource view for the web UI
### Required Approvers
* @zmb3 && @rosstimothy

**note: this is a post-implementation RFD to capture implementation details**

## What

A new view on the web UI (and eventually Connect) that will display all available resources such as Nodes, Databases, Apps, etc, in a single unified way.

![Screenshot 2023-07-25 at 4 18 08 PM](https://github.com/gravitational/teleport/assets/5201977/6953289d-ec3e-4eae-bdb9-10f3cfebfd7f)

## Why

Rather than separating all resources into their own individual pages, a Unified Resource page would help consolidate every kind of resource into a single, searchable view. This will help with discoverability and make searching across resources a lot better. This will also lay the foundation for a much more helpful "Dashboard" type page where we can have things such as Pinned Resources (coming right after this).

## Details

In order to get a searchable and filterable list of all resource kinds, we will create a new, in-memory store on the auth server
that will initialize on auth service start and only live as long as the service is up. The watcher will update the store any time 
it receives an event for any of the watched kinds. We will use a very similar setup for this as our in-memory backend, 
but instead of storing the resource in its textual representation required by the backend, we can store resources as 
their concrete type(e.g. types.ServerV2).

The key is in the format `name/type` to prevent name collisions across types. However, this make the default sort a
little weird where resource with UUID names will come first, even though they'll be displayed with a "friendly" name. We
can combat this by storing `Servers` specifically a `name+UUID/type`. This will require a separate map of `resourceKey -> UUID`
because the `OpDelete` event only returns a `ResourceHeader`, rather than the resource itself (no hostname in the header).

### The UnifiedResourceCache
Most of the magic will happen within this new cache. It's a pretty close copy of our memory backend implementation with a few changes. The important types are below

```go
// UnifiedResourceCache contains a representation of all resources that are displayable in the UI
type UnifiedResourceCache struct {
	mu  sync.Mutex
	log *log.Entry
	cfg UnifiedResourceCacheConfig
	// tree is a BTree with items
	tree            *btree.BTreeG[*item]
	initializationC chan struct{}
	stale           bool
	once            sync.Once
	cache           *utils.FnCache
	ResourceGetter
}

type resource interface {
	types.ResourceWithLabels
	CloneResource() types.ResourceWithLabels
}

```

The cache will initialize with a `resourceWatcher` that watches every type currently available in the UI. As of this RFD,
those types include server, database servers, app servers, kube servers, saml IdP service providers, and Windows desktops. 
This can be easily updated to include (or not include) any type by removing its type from the watcher, and the 
getResource call associated with that type. 

The cache initializes by getting every current resource of the above types and adding them to the internal bTree, and then starts a watcher to put/delete those types based on events. If the resources are requested from an uninitialized or stale UnifiedResourceCache, we will resort to getting all the current resources again, and storing the result in a `utils.FnCache` until the watcher is back online. This `fnCache` has a TTL of 15 seconds.

We created a new interface `resource` that extends `ResourceWithLabels` with a new method `CloneResource()`. This is only used to have a common "copy" method when getting resources to prevent concurrent memory modification. 

The underlying resource watcher is fundamentally the same as usual, except we'd added a `QueueSize` of `8192`, to match other watchers such as auth and proxy.

### ListUnifiedResources grpc endpoint
We will add a new grpc endpoint, `ListUnifiedResources`. It will function pretty close to how `ListResources` works, except without needing to differentiate between `RequestType`, as every request is a unified resource request. Originally, the thought of just extending `ListResources`, but because of the many new filters that will be added to the request, we don't want to muddy up the `ListResourcesRequest`. Some of the new filters include which types to include (can be multiple), subtypes, etc etc. 

```protobuf
// ListUnifiedResourcesRequest is a request to receive a paginated list of unified resources
message ListUnifiedResourcesRequest {
  // Kinds is a list of kinds to match against a resource's kind. This can be used in a
  // unified resource request that can include multiple types.
  repeated string Kinds = 1 [(gogoproto.jsontag) = "kinds,omitempty"];
  // Limit is the maximum amount of resources to retrieve.
  int32 Limit = 2 [(gogoproto.jsontag) = "limit,omitempty"];
  // StartKey is used to start listing resources from a specific spot. It
  // should be set to the previous NextKey value if using pagination, or
  // left empty.
  string StartKey = 3 [(gogoproto.jsontag) = "start_key,omitempty"];
  // Labels is a label-based matcher if non-empty.
  map<string, string> Labels = 4 [(gogoproto.jsontag) = "labels,omitempty"];
  // PredicateExpression defines boolean conditions that will be matched against the resource.
  string PredicateExpression = 5 [(gogoproto.jsontag) = "predicate_expression,omitempty"];
  // SearchKeywords is a list of search keywords to match against resource field values.
  repeated string SearchKeywords = 6 [(gogoproto.jsontag) = "search_keywords,omitempty"];
  // SortBy describes which resource field and which direction to sort by.
  types.SortBy SortBy = 7 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "sort_by,omitempty"
  ];
  // WindowsDesktopFilter specifies windows desktop specific filters.
  types.WindowsDesktopFilter WindowsDesktopFilter = 8 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "windows_desktop_filter,omitempty"
  ];
  // UseSearchAsRoles indicates that the response should include all resources
  // the caller is able to request access to using search_as_roles
  bool UseSearchAsRoles = 9 [(gogoproto.jsontag) = "use_search_as_roles,omitempty"];
  // UsePreviewAsRoles indicates that the response should include all resources
  // the caller would be able to access with their preview_as_roles
  bool UsePreviewAsRoles = 10 [(gogoproto.jsontag) = "use_preview_as_roles,omitempty"];
}

// ListUnifiedResourceResponse response of ListUnifiedResources.
message ListUnifiedResourcesResponse {
  // Resources is a list of resource.
  repeated PaginatedResource Resources = 1 [(gogoproto.jsontag) = "resources,omitempty"];
  // NextKey is the next Key to use as StartKey in a ListResourcesRequest to
  // continue retrieving pages of resource. If NextKey is empty, there are no
  // more pages.
  string NextKey = 2 [(gogoproto.jsontag) = "next_key,omitempty"];
}
```

This endpoint will pull resources from our unified resource cache and run through the same filtering/rbac as `ListResources`

### Performance Limitations
As of the current implementation, `ListUnifiedResources` is not very performant for large clusters. It uses `FakePaginate` under the hood, similar to the rest of the legacy Web UI resources, which will load the entire resource set into memory to then handle sorting, RBAC checks, and pagination. This becomes very taxing as every request has to load the entire set before getting the "next page". This is a limitation due to not being able to preserve a stream across RPCs (for example, how `ListNodes` works with `IterateResources`.  A possible solution for the next iteration might be storing resources in a map and using the bTree per sort order to store indexes but this needs to be verified. 

### Backward Compatibility

We will leave `ListResources` alone and keep the old individual resource endpoints available until we've vetted the unified resource view. We will remove the individual navigation items from the web ui, but leave the routes/api endpoints until we've gained confidence in the new flow. 

Any leaf cluster that does not support Unified Resources will gracefully fallback to a "legacy" view, with the navigation and tables we currently have.

### UX
The UI for the web will be changing drastically given the fact that all resources will be on the same page and we will be using a Card layout rather than the current Table. However, besides the visual changes, the actual resources listed will still be interacted with the same way. You can still click "Connect" on servers and databases, or Login for apps. The individual functionality for each "Item" will remain the same. Labels will be truncated but viewable with an expand button.

This example of a `ResourceCard` is a general view of what a resource will look like. The checkboxes, while not included in the initial release of Unified Resources, will be made available shortly after to perform bulk actions such as lock, search audit logs, etc. These bulk actions will be an ever growing list of actions as time goes on.

![Screenshot 2023-07-25 at 4 18 19 PM](https://user-images.githubusercontent.com/43280172/256302298-2d853296-faff-4e58-8a6b-102f8c42ff01.png)



Also, filters will be added. Some filters will just be a "pseudo-filter" in the sense that it'll just help populate an advanced search string, (example, clicking a label), but a few new actual filters will be present in the requests. The biggest of them is the `Kinds` array, which will contain a list of kinds that you want to be returned. ["node","app"] will only return nodes and apps. If left empty (the default), all resource kinds will be returned

![Screenshot 2023-07-25 at 4 18 19 PM](https://github.com/gravitational/teleport/assets/5201977/0a58e6bc-b94f-44aa-9a63-56d4991a395c)


This page will also use "infinite scrolling" rather than pagination. We will employ the use of a loading skeleton as we wait for the data to come in and then at a certain scroll distance from the bottom (not the exact bottom, but maybe 200 or so pixels up) we will fetch more. [example of skeleton component](https://codepen.io/JCLee/pen/dyPejGV)

We will not offload previous requests and only append resources as the user scrolls. On paper, this could cause issues if someone tries to load too many resources onto the page, but we do not expect any customers to casually browse through 10,000 items. If this becomes a pain point, we can revisit. 

### Test Plan

The test plan will be updated reflect all the resources are in a single view, but generally speaking the functionality that we are testing should largely remain unchanged.

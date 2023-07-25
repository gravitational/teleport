

---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 137 - Unified Resource view for the web UI

## What

A new view on the web UI (and eventually Connect) that will display all available resources such as Nodes, Databases, Apps, etc, in a single unified way.

![Screenshot 2023-07-25 at 4 18 08 PM](https://github.com/gravitational/teleport/assets/5201977/6953289d-ec3e-4eae-bdb9-10f3cfebfd7f)

## Why

Rather than separating all resources into their own individual pages, a Unified Resource page would help consolidate every kind of resource into a single, searchable view. This will help with discoverability and make searching across resources a lot better. This will also lay the foundation for a much more helpful "Dashboard" type page where we can have things such as Pinned Resources (coming right after this).

## Details

In order to get a searchable and filterable list of all resource kinds, we will create a new, in-memory store that will initialize on auth service start and only live as long as the service is up. The watcher will update the store any time it receives an event for any of the watched kinds. We will use a very similar setup for this as our in-memory backend, but instead of storing the marshaled (unmarshaled? i forget which. TODO LEARN THE RIGHT WORD BEFORE PUBLISH) resource, we can store the actual resource. 

Once that exists, we can add a new Kind `KindUnifiedResource`. This isn't an actual Kind but more of a "meta" kind that will be used to distinguish request types in `ListResources`. Using the existing pathways that `ListResources` uses will give us almost all the functionality we already need such as pagination, filtering, etc. 

### New Types
`KindUnifiedResource` will be created and used as the `RequestType` for Unified Resource requests coming from the web. This will inform `ListResources` to pull from our Unified Resource collection.

Most of the core implementation is reusing logic from existing components with slight changes where needed. Our new Unified Resource store will be a b-tree similar to the backend memory structure, but the item is slightly modified to have it's `Value` be a common interface for the resources we want to track.
```go
type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value represents a resource that is available for a unified resource request
	Value types.ResourceWithLabels
}
```
The key is in the format `name/type` to prevent name collisions across types. However, this makes the default sort a little weird where resource with UUID names will come first, even though they'll be displayed with a "friendly" name. Such as `Servers` being stored by a UUID but making the `Hostname` visible. This can change if we find a better way "on the fly" since it's only created when the auth service is started so, we aren't locked into anything in this regard. For now, we'll implement with `GetName()/GetKind()`

For the existing resource endpoints, we return a "ui" version of the resource. Because our list will be of multiple types, we will add the `Kind` field to our existing resource structs. In a vacuum, it's redundant. When listed together, this will allow our UI to differentiate between the different kinds
```diff
type Server struct {
+	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
+	Kind string `json:"kind"`
	// Tunnel indicates of this server is connected over a reverse tunnel.
	Tunnel bool `json:"tunnel"`
	// Name is this server name
	Name string `json:"id"`
	// ...
}
```

### The Watcher and Collector

We will add `UnifiedResourceWatcher` to the auth `Server` 

The UnifiedResourceWatcher isn't really different than any other watcher in `lib/services/watcher.go`. The underlying watcher is still just a `resourceWatcher` and the collector is the same as something like `nodeCollector`, but uses a btree as it's `current` instead of a map. We can also ignore any of the expiration/event related features. We will create the `resourceWatcher` with a list of Kinds we plan to watch. To do this, we have to extend the `resourceKind` method that collectors have.

The one biggish change is, currently, `collector.resourceKind()` returns a single type as a string and when the watcher runs `resourceWatcher.watch()` it creates a slice containing the single Kind returned. We can make `resourceKind()` take ownership of supplying the slice instead of `watch`, which would allow us to return the slice of all desired kinds to watch
```go
func (u *unifiedResourceCollector) resourceKind() []types.WatchKind {
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindWindowsDesktop},
	}
}
```
The above example can be expanded to include any Kinds we may need/want to add in our Unified Resource view.

When the watcher inits, we populate the btree with all the available items on startup
```go
func (u *uiResourceCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	err := u.getAndUpdateNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	//...

	u.defineCollectorAsInitialized()
	return nil
}
```

### ListUnifiedResources grpc endppint
We will add a new grpc endpoint, `ListUnifiedResources`. It will function pretty closes to how `ListResources` works, except without needing to differentiate between `RequestType`, as every request is a unified resource request. Originally, the thought of just extending `ListResources`, but because of the many new filters that will be added to the request, we don't want to muddy up the `ListResourcesRequest`. Some of the new filters include which types to include (can be multiple), subtypes, etc etc. 

This endpoint will pull resources from our unified resource cache and run through the same filtering/rbac as `ListResources`

### Backward Compatibility

We will leave `ListResources` alone and keep the old individual resource endpoints available until we've vetted the unified resource view. We will remove the individual navigation items from the web ui, but leave the routes/api endpoints until we've gained confidence in the new flow. 

### UX
The UI for the web will be changing drastically given the fact that all resources will be on the same page and we will be using a Card layout rather than the current Table. However, besides the visual changes, the actual resources listed will still be interacted with the same way. You can still click "Connect" on servers and databases, or Login for apps. The individual functionality for each "Item" will remain the same. Labels will be truncated but viewable with an expand button.



https://github.com/gravitational/teleport/assets/5201977/9c859a2a-784d-4805-9ba7-9e37a8a4ce77



Also, filters will be added. Some filters will just be a "pseudo-filter" in the sense that it'll just help populate an advanced search string, (example, clicking a label), but a few new actual filters will be present in the requests. The biggest of them is the `Kinds` array, which will contain a list of kinds that you want to be returned. ["node","app"] will only return nodes and apps. If left empty (the default), all resource kinds will be returned

![Screenshot 2023-07-25 at 4 18 19 PM](https://github.com/gravitational/teleport/assets/5201977/0a58e6bc-b94f-44aa-9a63-56d4991a395c)


This page will also use "infinite scrolling" rather than pagination. We will employ the use of a loading skeleton as we wait for the data to come in and then at a certain scroll distance from the bottom (not the exact bottom, but maybe 200 or so pixels up) we will fetch more. [example of skeleton component](https://codepen.io/JCLee/pen/dyPejGV)

### Test Plan

The test plan will be updated reflect all the resources are in a single view, but generally speaking the functionality that we are testing should largely remain unchanged.

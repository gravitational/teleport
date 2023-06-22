
---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 137 - Unified Resource view for the web UI

## What

A new view on the web UI (and eventually Connect) that will display all available resources such as Nodes, Databases, Apps, etc, in a single unified way. [INSERT IMAGE HERE]

## Why

Rather than separating all resources into their own individual pages, a Unified Resource page would help consolidate every kind of resource into a single, searchable view. This will help with discoverability and make searching across resources a lot better. This will also lay the foundation for a much more helpful "Dashboard" type page where we can have things such as Pinned Resources (coming right after this).

## Details

A couple notes and definitions to keep in mind throughout this RFD. 
- If speaking about "ALL" resource kinds, it's meant to say "All resource kinds that are displayed in the web ui resource tables". There will be a section that talks about _actually all_ kinds, but I'll make it explicit that I mean that there.
- I'll try to use the word Kinds instead of Types but may accidentally slip up. TODO, REMOVE THIS BULLET WHEN IVE CLEARED ALL
- TODO (add images)

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

`lib/web/ui.UnifiedResource` will be the shape of ALL returned resources from a unified resource request. Normally, every specific request type would use it's own `MakeServer`-esque struct, but with unified resources, it can be any kind so we have to accommodate. We could have used an empty interface slice (this is the way the items are sent over the wire anyway), but this provides a bit more safety and documentation so we'll roll with it. 
```go
// Unified Resource describes a unified resource for webapp
type UnifiedResource struct {
	// Kind is the resource kind
	Kind string `json:"kind"`
	// Name is this server name
	Name string `json:"name"`
	// Labels is this server list of labels
	Labels []Label `json:"tags"`

	// The fields below are supplied on for specific resources.

	// Addr is the address of the Server and Desktop
	Addr string `json:"addr"`
	// SSHLogins is the list of logins this user can use on this server. This exists for Databases and Servers
	SSHLogins []string `json:"sshLogins"`
	// Logins is the list of logins this user can use on this desktop.
	Logins []string `json:"logins"`
}
```
(not exhaustive, just an example).
We will include any field that is needed per resource and omit the rest. A small subset of fields will exist on every resource returned but due to the nature of the individual resource kinds, certain fields will only exist on it's relevant resource (such as `Addr`).  In reality, the UI doesn't really need many of the fields that are currently available on these resources for a unified view. When they are needed, they will still exists from their individual request types in `ListResources`. 

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

### Extending ListResources
The benefit of extending `ListResources` over a new grpc endpoint is that we get all the sorting/filtering/pagination built in (mostly). The web always uses `listResourcesWithSort` . We can add another case to the switch statement below
```go
	switch req.ResourceType {
	case types.KindUnifiedResource:
		unifiedResources, err := a.GetUnifiedResources(ctx, req.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = uiResources
	case //...
	case //...
```
`GetUnifiedResources` above performs RBAC on each type just like the existing cases. 
```go
	for _, resource := range unifiedResources {
		switch r := resource.(type) {
		case types.Server:
			{
				if err := a.checkAccessToNode(r); err != nil {
					if trace.IsAccessDenied(err) {
						continue
					}

					return nil, trace.Wrap(err)
				}

				filteredResources = append(filteredResources, resource)
			}
		case //...
		case //...
```
`ListResourceRequest` will have a few new fields added. 

### Backward Compatibility

Because we are only extending the capabilities of of `ListResources`, any older calls to `ListResources` should still work exactly the same. We can incrementally adopt a unified resource response anywhere we may need to, or not at all. 

### UX
The UI for the web will be changing drastically given the fact that all resources will be on the same page and we will be using a Card layout rather than the current Table. However, besides the visual changes, the actual resources listed will still be interacted with the same way. You can still click "Connect" on servers and databases, or Login for apps. The individual functionality for each "Item" will remain the same. Labels will be truncated but viewable with an expand button.

[insert gif of expanding cards here] 

Also, filters will be added. Some filters will just be a "pseudo-filter" in the sense that it'll just help populate an advanced search string, (example, clicking a label), but a few new actual filters will be present in the requests. The biggest of them is the `Kinds` array, which will contain a list of kinds that you want to be returned. ["node","app"] will only return nodes and apps. If left empty (the default), all resource kinds will be returned

[insert image of filters]


### Test Plan

The test plan will be updated reflect all the resources are in a single view, but generally speaking the functionality that we are testing should largely remain unchanged.
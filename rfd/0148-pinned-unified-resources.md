
---
authors: Michael Myers (michael.myers@goteleport.com)
state: partially implemented (phase 1 - Teleport 14.1)
---

# RFD 0148 - Pinned Unified Resources in the web UI

## Required approvers

engineering: @rosstimothy || @zmb3

## What

This RFD discusses the method of "pinning" resources in the web UI. Pinning will allow users to have
specific resources in a tab that they want to have easier access to. Pinning is analogous to "favoriting"

![unnamed](https://github.com/gravitational/teleport/assets/5201977/affe68b9-323f-4aa0-948c-9d8fb53f8c01)
![unnamed-1](https://github.com/gravitational/teleport/assets/5201977/1f9c5915-4cde-478c-b788-cd49b04edcd3)


## Why

Generally, most users access some resources much more than others. We reduced friction of search/discovery by
adding in the unified resource view. Pinning takes this view one step further by allowing them to keep their
favorite resources always within a click or two away without needing to search and filter for the same resource
every day.

## Details

### User Preferences

Pinned resources will be stored in the `ClusterPreferences` object as
`PinnedResources`, an array of resource IDs (that match the [name sortKey](https://github.com/gravitational/teleport/blob/master/lib/services/unified_resource.go#L389-L394) used in our
unified resources caches, ex: `theservername/node`, `grafana/app`, etc).


```diff
// UserPreferencesResponse is the JSON response for the user preferences.
type UserPreferencesResponse struct {
	Assist                     AssistUserPreferencesResponse      `json:"assist"`
	Theme                      userpreferencesv1.Theme            `json:"theme"`
	Onboard                    OnboardUserPreferencesResponse     `json:"onboard"`
+	UnifiedResourcePreferences UnifiedResourcePreferencesResponse `json:"unifiedResourcePreferences"`
+	ClusterPreferences         ClusterUserPreferencesResponse     `json:"clusterPreferences,omitempty"`
}
```

Defined in protobuf as below:

```protobuf
// PinnedResourcesUserPreferences is a collection of resource IDs that will be
// displayed in the user's pinned resources tab in the Web UI.
message PinnedResourcesUserPreferences {
  // resource_ids is a list of unified resource name sort keys.
  repeated string resource_ids = 1;
}

// ClusterUserPreferences are user preferences saved per cluster.
message ClusterUserPreferences {
  // pinned_resources is a list of pinned resources.
  PinnedResourcesUserPreferences pinned_resources = 1;
}
```

User preferences are generally fetched from the root cluster but apply across
both root and leaf clusters. For example, if the user preferences specify that
the user prefers the light theme, then we always use the light theme, even if
the user switches to a leaf cluster in the UI.

The new `ClusterPreferences` field contains settings that should be stored
and applied per-cluster. Pinned resources are the first such example.

There are two new web API endpoints to retrieve only the cluster preferences. We
use the same backend storage as the user preferences because these fields are
still user preferences, they are just scoped per-cluster. The other fields of
the `UserPreferences` object are "global" in that they are always fetched from
the root server.

```go
	// Fetches the user's cluster preferences.
	h.GET("/webapi/user/preferences/:site", h.WithClusterAuth(h.getUserClusterPreferences))

	// Updates the user's cluster preferences.
	h.PUT("/webapi/user/preferences/:site", h.WithClusterAuth(h.updateUserClusterPreferences))
```

#### Filtering Pinned Resources

You can think of Pinned Resources as just another filter. However, rather than
ascending the tree and populating a list until a limit is met based on
RBAC/filters, we individually get each resource by the id passed in `ids`. This
means that the returned list is **NOT** sorted (because we are fetching in the
order the resource was pinned) and must be sorted manually after.

```go
// GetUnifiedResourcesByIDs will take a list of ids and return any items found in the unifiedResourceCache tree by id and that return true from matchFn
func (c *UnifiedResourceCache) GetUnifiedResourcesByIDs(ctx context.Context, ids []string, matchFn func(types.ResourceWithLabels) bool) ([]types.ResourceWithLabels, error) {
	var resources []types.ResourceWithLabels

	err := c.read(ctx, func(cache *UnifiedResourceCache) error {
		for _, id := range ids {
			key := backend.Key(prefix, id)
			res, found := cache.nameTree.Get(&item{Key: key})
			if !found || res == nil {
				continue
			}
			resource := cache.resources[res.Value]
			if matched := matchFn(resource); matched {
				resources = append(resources, resource.CloneResource())
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resources by id")
	}

	return resources, nil
}
```

### Space limitations for UserPreferences storage

> [!NOTE]
> The maximum item size in DynamoDB is **400 KB**, which includes both attribute
> name binary length (UTF-8 length) and attribute value lengths (again binary
> length). The attribute name counts towards the size limit.

If we assume an average resource ID is something like
`db-name-1aaa8584-0e54-4c89-bec9-34f957512078`, then we can store well above
10,000 pinned resources per user. This is a very unlikely scenario as any amount
of pinned resources over 20, lets just say for conversation sake, defeats the
purpose of pinning a resource in the first place. We don't expect anyone to pin
more than a "page" worth. We can still limit the resources in the backend to a
total (per cluster) of something like 500. These are knobs we can easily turn if
necessary but it seems unlikely to "deliberately" go over this cap.

### "What happens if a resource I have pinned becomes unavailable?"

In phase 1 (released with Teleport 14.1) if the user preferences contains pinned resource ID that is
no longer available, then that resource will not be included in the API response and will not
be rendered on-screen. If this resource comes back online, it will start to show up again automatically.

This is acceptable for now, but poses a long-term problem. The reference to the
pinned resource ID cannot be deleted since the card is never rendered in the UI and the user
doesn't see an "unpin" button.

Phase 2 (approved but not yet implemented) will address this issue. In phase 2,
we will return an additional field of 'not found' resource IDs. This will allow the client to
render the resource in a "disconnected" state, which will allow the user to decide whether to: 
- keep the pin (because they expect the resource will come back online) or 
- remove the pin (because the resource has been decommissioned).

![Untitled-2022-09-11-1530](https://github.com/gravitational/teleport/assets/5201977/e52c4286-bf57-49cc-bfb5-d541146f6896)

### Security Concerns

Pinned resources go through the same RBAC as unified resources so no additional security concerns matter in the listing.

### Backward compatibility
If the user tries to access a cluster that doesn't have access to pinned resources, we can hide the feature and show
the normal unified resource view without pinning capability (the same as v14.0 view). This mechanism will be similar
to the one used to check if unified resources is enabled by making a fetch to see if the endpoint exists. This can be
done per cluster as well since we will be fetching each time the cluster is changed.

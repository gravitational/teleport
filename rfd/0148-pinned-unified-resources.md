
---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
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

Pinned resources will be stored in the `ClusterPreferences` object as `PinnedResources`, an array of resource IDs (that match the id used in our unified resources caches, ex: `theservername/node`, `grafana/app`, etc). 


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
User preferences are generally fetched from the root server, but requests for `ClusterPreferences` return the user preferences from the server it's being requested for. This means that leaf clusters will return their own unique `ClusterPreferences`. There are two new endpoints to retrieve the cluster preferences. We decided to use the same backend store as User Preferences with Cluster Preferences as a subfield because it is still a user preference, just differs per cluster. The other fields of the `UserPreferences` tab are "global" in that they are always fetched from the root server.

```go
	// Fetches the user's cluster preferences.
	h.GET("/webapi/user/preferences/:site", h.WithClusterAuth(h.getUserClusterPreferences))

	// Updates the user's cluster preferences.
	h.PUT("/webapi/user/preferences/:site", h.WithClusterAuth(h.updateUserClusterPreferences))
```

#### Filtering Pinned Resources

You can think of Pinned Resources as just another filter. However, rather than ascending the tree and populating a list until a limit is met based on RBAC/filters, we individually get each resource by the id passed in `ids`. This means that the returned list is **NOT** sorted (because we are fetching in the order the resource was pinned) and must be sorted manually after.

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
> The maximum item size in DynamoDB is **400 KB**, which includes both attribute name binary length (UTF-8 length) and attribute value lengths (again binary length). The attribute name counts towards the size limit.

If we assume an average resource ID is something like `db-name-1aaa8584-0e54-4c89-bec9-34f957512078`, then we can
store well above 10,000 pinned resources per user. This is a very unlikely scenario as any amount of pinned resources over 20, lets just say for conversation sake, defeats the purpose of pinning a resource in the first place. We don't expect anyone to pin more than a "page" worth. We can still limit the resources in the backend to a total (per cluster) of something like 500. 
These are knobs we can easily turn if necessary but it seems unlikely to "deliberately" go over this cap.

### "What happens if a resource I have pinned becomes unavailable?"
If a resource isn't found for whatever reason when fetching, we can display it's name (name/type or hostname/type) in a "disconnected" state. This will allow the user to make the decision themselves to unpin something. Without the resource information the displayable card would only have it's name/type but that should be sufficient enough to know "what" is disconnected. An example of that would be like so

![Untitled-2022-09-11-1530](https://github.com/gravitational/teleport/assets/5201977/e52c4286-bf57-49cc-bfb5-d541146f6896)

### Security Concerns
Pinned resources go through the same RBAC as unified resources so no additional security concerns matter in the listing. 

### Backward compatibility
If the user tries to access a cluster that doesn't have access to pinned resources, we can hide the feature and show 
the normal unified resource view without pinning capability (the same as v14.0 view). This mechanism will be similar
to the one used to check if unified resources is enabled by making a fetch to see if the endpoint exists. This can be
done per cluster as well since we will be fetching each time the cluster is changed.

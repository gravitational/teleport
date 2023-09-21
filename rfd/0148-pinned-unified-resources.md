---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 0148 - Pinned Unified Resources in the web UI

## Required approvers

TODO

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

Pinned resources will be stored on the `UserPrefernces` object as an array of resource IDs (that match the id used in our unified resources caches, ex: `theservername/node`, `grafana/app`, etc). 


```diff
type UserPreferencesResponse struct {
	Assist AssistUserPreferencesResponse `json:"assist"`
	Theme userpreferencesv1.Theme `json:"theme"`
	Onboard OnboardUserPreferencesResponse `json:"onboard"`
+   PinnedResources []string `json:"pinnedResources"`
}
```

Defined in protobuf as below:
```protobuf
// PinnedResourcesUserPreferences is a collection of resource IDs that will be
// displayed in the user's pinned resources tab in the Web UI
message PinnedResourcesUserPreferences {
	// resource_ids is a list of resource ids
	repeated string resource_ids = 1;
}
```
Currently, user preferences are only access via the root auth server. This makes sense for things like theme where
it is expected that the user would want the same theme across all clusters. However, with pinned resources,
we would want a separate list per cluster. Instead of creating a new mechanism to store pinned resources, we can
reuse the current user preferences method but update/fetch pinned resources per cluster instead. This would require two 
new endpoints in the apiserver `pinnedResourcesUpsert` and `pinnedResourcesGet`. 

#### Filtering Pinned Resources

The current implementation of unified resources will pull the entire set of unified resources from the unified resource cache (we will call this the "to-be-filtered" list) and then filter down based on the provider params. You can think of Pinned Resources as just another filter, but instead of pulling everything into the "to-be-filtered" list, we only populate the "to-be-filtered" list with resources that match the provided resource IDs.

```go
func (c *UnifiedResourceCache) GetUnifiedResourcesByIDs(ctx context.Context, ids []string) ([]types.ResourceWithLabels, error) {
	var resources []types.ResourceWithLabels

	err := c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		for _, id := range ids {
			res, found := tree.Get(&item{Key: backend.Key(prefix, id)})
			if found {
				resources = append(resources, res.Value.CloneResource())
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
After this, any existing filters in the request will be applied the same (including RBAC).  

### Space limitations for UserPreferences storage
> The maximum item size in DynamoDB is **400 KB**, which includes both attribute name binary length (UTF-8 length) and attribute value lengths (again binary length). The attribute name counts towards the size limit.

If we assume an average resource ID is something like `db-name-1aaa8584-0e54-4c89-bec9-34f957512078`, then we can
store well above 10,000 pinned resources per user. This is a very unlikely scenario as any amount of pinned resources over 20, lets just say for conversation sake, defeats the purpose of pinning a resource in the first place. We don't expect anyone to pin more than a "page" worth. We can still limit the resources in the backend to a total (for all clusters) of something like 500. This would give someone 100 pins over 5 clusters, or 25 pins over 20 clusters. These are knobs we can easily turn if necessary but it seems unlikely to "deliberately" go over this cap.

### Automatic cleanup
The case in which this cap _could_ be reached is when we allow unavailable/unauthorized/unreachable resources to exist and fester in a user's preferences. (lets discuss)

Scenarios that would make a pinned resource ID stale
1. User loses access to the resource. In that case, I'm not sure we'd want to remove the resource from their pinned. I could see 
pinning a specific resource after assuming a role, and only caring about that resource's pin when you've assumed the role again.
2. Resource loses connectivity. We probably wouldn't want to remove a resource's pin due to connection issues unless x amount of time
has passed. 
3. Resource changes it's name. Idk how frequent this happens.
4. ...?

### "What happens if a resource I have pinned becomes unavailable?"
Similarly to the normal resource view, if a resource becomes unavailable (due to RBAC or being removed) it just won't be visible in the pinned view either. 


### Security Concerns
Pinned resources go through the same RBAC as unified resources so no additional security concerns matter in the listing. 
The only thing to consider is that leaf cluster resource IDs will exist in user preferences stored in the root cluster.

### Backward compatibility
If the user tries to access a cluster that doesn't have access to pinned resources, we can hide the feature and show 
the normal unified resource view without pinning capability (the same as v14.0 view). This mechanism will be similar
to the one used to check if unified resources is enabled by making a fetch to see if the endpoint exists. This can be
done per cluster as well since we will be fetching each time the cluster is changed.

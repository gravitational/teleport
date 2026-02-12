/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

func (id *ResourceID) CheckAndSetDefaults() error {
	if len(id.ClusterName) == 0 {
		return trace.BadParameter("ResourceID must include ClusterName")
	}
	if len(id.Kind) == 0 {
		return trace.BadParameter("ResourceID must include Kind")
	}
	if len(id.Name) == 0 {
		return trace.BadParameter("ResourceID must include Name")
	}

	// TODO(@creack): DELETE IN v20.0.0. Here to maintain backwards compatibility with older clients.
	if id.Kind != KindKubeNamespace && slices.Contains(KubernetesResourcesKinds, id.Kind) {
		apiGroup := KubernetesResourcesV7KindGroups[id.Kind]
		if slices.Contains(KubernetesClusterWideResourceKinds, id.Kind) {
			id.Kind = AccessRequestPrefixKindKubeClusterWide + KubernetesResourcesKindsPlurals[id.Kind]
		} else {
			id.Kind = AccessRequestPrefixKindKubeNamespaced + KubernetesResourcesKindsPlurals[id.Kind]
		}
		if apiGroup != "" {
			id.Kind += "." + apiGroup
		}
	}

	if id.Kind != KindKubeNamespace && !slices.Contains(RequestableResourceKinds, id.Kind) && !strings.HasPrefix(id.Kind, AccessRequestPrefixKindKube) {
		return trace.BadParameter("Resource kind %q is invalid or unsupported", id.Kind)
	}

	switch {
	case id.Kind == KindKubeNamespace || strings.HasPrefix(id.Kind, AccessRequestPrefixKindKube):
		return trace.Wrap(id.validateK8sSubResource())
	case id.SubResourceName != "":
		return trace.BadParameter("resource kind %q doesn't allow sub resources", id.Kind)
	}
	return nil
}

func (id *ResourceID) validateK8sSubResource() error {
	if id.SubResourceName == "" {
		return trace.BadParameter("resource of kind %q must include a subresource name", id.Kind)
	}
	isResourceClusterwide := id.Kind == KindKubeNamespace || slices.Contains(KubernetesClusterWideResourceKinds, id.Kind) || strings.HasPrefix(id.Kind, AccessRequestPrefixKindKubeClusterWide)
	switch split := strings.Split(id.SubResourceName, "/"); {
	case isResourceClusterwide && len(split) != 1:
		return trace.BadParameter("subresource %q must follow the following format: <name>", id.SubResourceName)
	case isResourceClusterwide && split[0] == "":
		return trace.BadParameter("subresource %q must include a non-empty name: <name>", id.SubResourceName)
	case !isResourceClusterwide && len(split) != 2:
		return trace.BadParameter("subresource %q must follow the following format: <namespace>/<name>", id.SubResourceName)
	case !isResourceClusterwide && split[0] == "":
		return trace.BadParameter("subresource %q must include a non-empty namespace: <namespace>/<name>", id.SubResourceName)
	case !isResourceClusterwide && split[1] == "":
		return trace.BadParameter("subresource %q must include a non-empty name: <namespace>/<name>", id.SubResourceName)
	}

	return nil
}

func (idl *ResourceAccessIDList) CheckAndSetDefaults() error {
	for _, r := range idl.Resources {
		rid, rc := r.GetResourceID(), r.GetConstraints()
		if err := rid.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		if rc == nil {
			continue
		} else if err := rc.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetResourceID returns the wrapped [ResourceID] from the [ResourceAccessID]
func (r *ResourceAccessID) GetResourceID() ResourceID {
	return r.Id
}

// GetConstraints returns any [ResourceConstraints] present on the [ResourceAccessID]
func (r *ResourceAccessID) GetConstraints() *ResourceConstraints {
	return r.Constraints
}

// ResourceIDsToResourceAccessIDs wraps a slice of [ResourceID]s into
// [ResourceAccessID] instances with no additional fields populated.
func ResourceIDsToResourceAccessIDs(ids []ResourceID) []ResourceAccessID {
	if ids == nil {
		return nil
	}
	wrapped := make([]ResourceAccessID, 0, len(ids))
	for _, id := range ids {
		wrapped = append(wrapped, ResourceAccessID{Id: id})
	}
	return wrapped
}

// CombineAsResourceAccessIDs converts plain [ResourceID]s to [ResourceAccessID]s
// and combines them with existing [ResourceAccessID]s into a single slice.
func CombineAsResourceAccessIDs(ids []ResourceID, accessIDs []ResourceAccessID) []ResourceAccessID {
	if ids == nil && accessIDs == nil {
		return nil
	}
	totalLen := len(ids) + len(accessIDs)
	zipped := make([]ResourceAccessID, 0, totalLen)
	zipped = append(zipped, ResourceIDsToResourceAccessIDs(ids)...)
	zipped = append(zipped, accessIDs...)
	return zipped
}

// UnwrapResourceAccessIDs separates a slice of [ResourceAccessID]s back into plain
// [ResourceID]s, preserving only [ResourceAccessID]s that carry additional
// information such as Constraints.
func UnwrapResourceAccessIDs(ids []ResourceAccessID) ([]ResourceID, []ResourceAccessID) {
	var plainIDs []ResourceID
	var accessIDs []ResourceAccessID
	for _, w := range ids {
		if w.GetConstraints() != nil {
			accessIDs = append(accessIDs, w)
			continue
		} else {
			plainIDs = append(plainIDs, w.GetResourceID())
		}
	}
	return plainIDs, accessIDs
}

// RiskyExtractResourceIDs extracts the underlying [ResourceID] from each
// [ResourceAccessID], discarding any additional information such as Constraints.
//
// This should *only* be used either when we're sure there is no additional
// information carried, or where we only care about the bare ResourceIDs
// and the ResourceAccessIDs are not being used for any access-control decision.
func RiskyExtractResourceIDs(accessIDs []ResourceAccessID) []ResourceID {
	ids := make([]ResourceID, 0, len(accessIDs))
	for _, w := range accessIDs {
		ids = append(ids, w.GetResourceID())
	}
	return ids
}

// ResourceIDToString marshals a ResourceID to a string.
func ResourceIDToString(id ResourceID) string {
	if id.SubResourceName == "" {
		return fmt.Sprintf("/%s/%s/%s", id.ClusterName, id.Kind, id.Name)
	}
	return fmt.Sprintf("/%s/%s/%s/%s", id.ClusterName, id.Kind, id.Name, id.SubResourceName)
}

// ResourceIDFromString parses a ResourceID from a string. The string should
// have been obtained from ResourceIDToString.
func ResourceIDFromString(raw string) (ResourceID, error) {
	if len(raw) < 1 || raw[0] != '/' {
		return ResourceID{}, trace.BadParameter("%s is not a valid ResourceID string", raw)
	}
	raw = raw[1:]
	// Should be safe for any Name as long as the ClusterName and Kind don't
	// contain slashes, which should never happen.
	parts := strings.SplitN(raw, "/", 3)
	if len(parts) != 3 {
		return ResourceID{}, trace.BadParameter("/%s is not a valid ResourceID string", raw)
	}
	resourceID := ResourceID{
		ClusterName: parts[0],
		Kind:        parts[1],
		Name:        parts[2],
	}

	switch {
	case slices.Contains(KubernetesResourcesKinds, resourceID.Kind) || strings.HasPrefix(resourceID.Kind, AccessRequestPrefixKindKube) || resourceID.Kind == KindKubeNamespace:
		isResourceClusterWide := resourceID.Kind == KindKubeNamespace || slices.Contains(KubernetesClusterWideResourceKinds, resourceID.Kind) || strings.HasPrefix(resourceID.Kind, AccessRequestPrefixKindKubeClusterWide)
		// Kubernetes forbids slashes "/" in Namespaces and Pod names, so it's safe to
		// explode the resourceID.Name and extract the last two entries as namespace
		// and name.
		// Teleport allows the resource names to contain slashes, so we need to join
		// splits[:len(splits)-2] to reconstruct the resource name that contains slashes.
		// If splits slice does not have the correct size, resourceID.CheckAndSetDefaults()
		// will fail because, for kind=pod, it's mandatory to present a non-empty
		// namespace and name.
		splits := strings.Split(resourceID.Name, "/")
		if !isResourceClusterWide && len(splits) >= 3 {
			resourceID.Name = strings.Join(splits[:len(splits)-2], "/")
			resourceID.SubResourceName = strings.Join(splits[len(splits)-2:], "/")
		} else if isResourceClusterWide && len(splits) >= 2 {
			resourceID.Name = strings.Join(splits[:len(splits)-1], "/")
			resourceID.SubResourceName = strings.Join(splits[len(splits)-1:], "/")
		}
	}

	return resourceID, trace.Wrap(resourceID.CheckAndSetDefaults())
}

// ResourceAccessIDsToString serializes a list of ResourceAccessIDs
// to a JSON string.
func ResourceAccessIDsToString(ids []ResourceAccessID) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}

	bytes, err := json.Marshal(&ResourceAccessIDList{Resources: ids})
	if err != nil {
		return "", trace.BadParameter("failed to marshal ResourceAccessIDs to JSON: %v", err)
	}

	return string(bytes), nil
}

// ResourceAccessIDsFromString deserializes a list of ResourceAccessIDs
// from a JSON string.
func ResourceAccessIDsFromString(raw string) ([]ResourceAccessID, error) {
	var resourceAccessIDList ResourceAccessIDList

	if raw == "" {
		return resourceAccessIDList.Resources, nil
	}
	if err := json.Unmarshal([]byte(raw), &resourceAccessIDList); err != nil {
		return nil, trace.BadParameter("failed to unmarshal ResourceAccessIDs from JSON: %v", err)
	}

	if err := resourceAccessIDList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return resourceAccessIDList.Resources, nil
}

// ResourceIDsFromStrings parses a list of ResourceIDs from a list of strings.
// Each string should have been obtained from ResourceIDToString.
func ResourceIDsFromStrings(resourceIDStrs []string) ([]ResourceID, error) {
	resourceIDs := make([]ResourceID, len(resourceIDStrs))
	var err error
	for i, resourceIDStr := range resourceIDStrs {
		resourceIDs[i], err = ResourceIDFromString(resourceIDStr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return resourceIDs, nil
}

// ResourceIDsToString marshals a list of ResourceIDs to a string.
func ResourceIDsToString(ids []ResourceID) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	// Marshal each ID to a string using the custom helper.
	var idStrings []string
	for _, id := range ids {
		idStrings = append(idStrings, ResourceIDToString(id))
	}
	// Marshal the entire list of strings as JSON (should properly handle any
	// IDs containing commas or quotes).
	bytes, err := json.Marshal(idStrings)
	if err != nil {
		return "", trace.BadParameter("failed to marshal resource IDs to JSON: %v", err)
	}
	return string(bytes), nil
}

// ResourceIDsFromString parses a list of resource IDs from a single string.
// The string should have been obtained from ResourceIDsToString.
func ResourceIDsFromString(raw string) ([]ResourceID, error) {
	if raw == "" {
		return nil, nil
	}
	// Parse the full list of strings.
	var idStrings []string
	if err := json.Unmarshal([]byte(raw), &idStrings); err != nil {
		return nil, trace.BadParameter("failed to parse resource IDs from JSON: %v", err)
	}
	// Parse each ID using the custom helper.
	resourceIDs := make([]ResourceID, 0, len(idStrings))
	for _, idString := range idStrings {
		id, err := ResourceIDFromString(idString)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resourceIDs = append(resourceIDs, id)
	}
	return resourceIDs, nil
}

const (
	ResourceIDSentinelValue = "__SENTINEL__"
)

// CreateSentinelResourceID returns a [ResourceID] that does not refer to any
// real resource.
//
// In mixed-version clusters, some authorization paths (e.g., older Auth) may not parse
// AllowedResourceAccessIDs from identities. In those paths, an empty
// AllowedResourceIDs slice is interpreted by AccessChecker as "no resource-specific restrictions".
//
// When an identity is resource-scoped exclusively via AllowedResourceAccessIDs, AllowedResourceIDs
// would otherwise be empty. To prevent those authorization paths from interpreting this as
// unconstrained, this sentinel value is injected into AllowedResourceIDs so authorization fails closed.
//
// Any code that understands AllowedResourceAccessIDs must remove or ignore this
// sentinel before evaluating resource restrictions or returning ResourceIDs to clients.
//
// TODO(kiosion): DELETE in 21.0.0
func CreateSentinelResourceID() ResourceID {
	return ResourceID{
		ClusterName: ResourceIDSentinelValue,
		Kind:        KindNode,
		Name:        ResourceIDSentinelValue,
	}
}

// IsSentinelResourceID reports whether the given [ResourceD] is a sentinel produced by
// [CreateSentinelResourceID]. See [CreateSentinelResourceID] for the rationale and required
// handling.
//
// TODO(kiosion): DELETE in 21.0.0
func IsSentinelResourceID(id ResourceID) bool {
	return id.ClusterName == ResourceIDSentinelValue &&
		id.Kind == KindNode &&
		id.Name == ResourceIDSentinelValue
}

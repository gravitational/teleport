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

	// TODO(@creack): Remove in v20. Here to maintain backwards compatibility with older clients.
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

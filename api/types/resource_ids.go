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
	"encoding/ascii85"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gogo/protobuf/proto"
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

func (r *ResourceAccessID) GetResourceID() ResourceID {
	return r.Id
}

func (r *ResourceAccessID) GetConstraints() *ResourceConstraints {
	return r.Constraints
}

// ResourceIDsToResourceAccessIDs wraps a slice of [ResourceID]s into
// [ResourceAccessID] instances with no additional fields populated.
func ResourceIDsToResourceAccessIDs(ids []ResourceID) []ResourceAccessID {
	wrapped := make([]ResourceAccessID, 0, len(ids))
	for _, id := range ids {
		wrapped = append(wrapped, ResourceAccessID{Id: id})
	}
	return wrapped
}

// CombineAsResourceAccessIDs converts plain [ResourceID]s to [ResourceAccessID]s
// and combines them with existing [ResourceAccessID]s into a single slice.
func CombineAsResourceAccessIDs(ids []ResourceID, accessIDs []ResourceAccessID) []ResourceAccessID {
	totalLen := len(ids) + len(accessIDs)
	zipped := make([]ResourceAccessID, 0, totalLen)
	for _, id := range ids {
		zipped = append(zipped, ResourceAccessID{Id: id})
	}
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

// ExtractResourceIDs extracts the underlying [ResourceID] from each
// [ResourceAccessID], discarding any additional information such as Constraints.
func ExtractResourceIDs(accessIDs []ResourceAccessID) []ResourceID {
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
	if len(raw) < 1 || (raw[0] != '/') {
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
// to an ascii85-encoded string of [ResourceAccessIDList] protobuf bytes.
func ResourceAccessIDsToString(ids []ResourceAccessID) (string, error) {
	b, err := proto.Marshal(&ResourceAccessIDList{Resources: ids})
	if err != nil {
		return "", trace.Wrap(err)
	}
	enc := make([]byte, ascii85.MaxEncodedLen(len(b)))
	n := ascii85.Encode(enc, b)
	return string(enc[:n]), nil
}

// ResourceAccessIDsFromString deserializes a list of ResourceAccessIDs
// from an ascii85-encoded string of [ResourceAccessIDList] protobuf bytes.
func ResourceAccessIDsFromString(raw string) ([]ResourceAccessID, error) {
	dec := make([]byte, len(raw))
	n, _, err := ascii85.Decode(dec, []byte(raw), true)
	if err != nil {
		return nil, trace.Wrap(err, "decoding ResourceAccessIDList from string")
	}
	var IDList ResourceAccessIDList
	if err := proto.Unmarshal(dec[:n], &IDList); err != nil {
		return nil, trace.Wrap(err, "unmarshalling ResourceAccessIDList from string")
	}
	return IDList.Resources, trace.Wrap(IDList.CheckAndSetDefaults())
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

// CreateSentinelResourceID creates a [ResourceID] that acts as a sentinel value
// or placeholder, indicating the absence of a real resource ID.
//
// This is necessary when handling requests/certs containing only [ResourceAccessID] s,
// as an empty list of [types.AccessRequest.AllowedResourceIDs] is interpreted as
// "no resource-specific restrictions".
func CreateSentinelResourceID() ResourceID {
	return ResourceID{
		ClusterName: ResourceIDSentinelValue,
		Kind:        KindNode,
		Name:        ResourceIDSentinelValue,
	}
}

// IsSentinelResourceID checks whether the given [ResourceID] is a sentinel value
// created by [CreateSentinelResourceID].
func IsSentinelResourceID(id ResourceID) bool {
	return id.ClusterName == ResourceIDSentinelValue &&
		id.Kind == KindNode &&
		id.Name == ResourceIDSentinelValue
}

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
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
)

func (id *ResourceID) CheckAndSetDefaults() error {
	if len(id.ClusterName) == 0 {
		return trace.BadParameter("ResourceID must include ClusterName")
	}
	if len(id.Kind) == 0 {
		return trace.BadParameter("ResourceID must include Kind")
	}
	if !slices.Contains(RequestableResourceKinds, id.Kind) {
		return trace.BadParameter("Resource kind %q is invalid or unsupported", id.Kind)
	}
	if len(id.Name) == 0 {
		return trace.BadParameter("ResourceID must include Name")
	}
	return nil
}

// ResourceIDToString marshals a ResourceID to a string.
func ResourceIDToString(id ResourceID) string {
	return fmt.Sprintf("/%s/%s/%s", id.ClusterName, id.Kind, id.Name)
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
	return resourceID, trace.Wrap(resourceID.CheckAndSetDefaults())
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

// ResourceIDsFromString parses a list for resource IDs from a string. The string
// should have been obtained from ResourceIDsToString.
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

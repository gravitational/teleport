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

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/trace"
)

// ResourceIDsToString marshals a list of ResourceIDs to a string.
func ResourceIDsToString(ids []ResourceID) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	bytes, err := json.Marshal(ids)
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
	resourceIDs := []ResourceID{}
	if err := json.Unmarshal([]byte(raw), &resourceIDs); err != nil {
		return nil, trace.BadParameter("failed to parse resource IDs from JSON: %v", err)
	}
	return resourceIDs, nil
}

// EventResourceIDs converts a []ResourceID to a []events.ResourceID
func EventResourceIDs(resourceIDs []ResourceID) []events.ResourceID {
	if resourceIDs == nil {
		return nil
	}
	out := make([]events.ResourceID, len(resourceIDs))
	for i := range resourceIDs {
		out[i].ClusterName = resourceIDs[i].ClusterName
		out[i].Kind = resourceIDs[i].Kind
		out[i].Name = resourceIDs[i].Name
	}
	return out
}

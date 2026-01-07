/*
Copyright 2025 Gravitational, Inc.

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

package events

import "github.com/gravitational/teleport/api/types"

// MaxAuditRoleARNPreview controls how many role ARNs we include in audit log events for AWS Console
// ResourceConstraints to keep event size bounded.
const MaxAuditRoleARNPreview = 10

// EventResourceIDs converts a []ResourceID to a []events.ResourceID
func ResourceIDs(resourceIDs []types.ResourceID) []ResourceID {
	if resourceIDs == nil {
		return nil
	}
	out := make([]ResourceID, len(resourceIDs))
	for i := range resourceIDs {
		out[i].ClusterName = resourceIDs[i].ClusterName
		out[i].Kind = resourceIDs[i].Kind
		out[i].Name = resourceIDs[i].Name
		out[i].SubResourceName = resourceIDs[i].SubResourceName
	}
	return out
}

// ToEventResourceAccessIDs converts []types.ResourceAccessID to []ResourceAccessID for events.
func ToEventResourceAccessIDs(in []types.ResourceAccessID) []ResourceAccessID {
	out := make([]ResourceAccessID, 0, len(in))
	for _, r := range in {
		out = append(out, ToEventResourceAccessID(r))
	}
	return out
}

// ToEventResourceAccessID converts a types.ResourceAccessID to a ResourceAccessID for events.
func ToEventResourceAccessID(in types.ResourceAccessID) ResourceAccessID {
	out := ResourceAccessID{
		Id: ResourceID{
			ClusterName:     in.Id.ClusterName,
			Kind:            in.Id.Kind,
			Name:            in.Id.Name,
			SubResourceName: in.Id.SubResourceName,
		},
	}

	c := in.Constraints
	if c == nil {
		return out
	}

	details := c.GetDetails()
	if details == nil {
		return out
	}

	switch d := details.(type) {
	// AWS Console constraints variant
	case *types.ResourceConstraints_AwsConsole:
		if d.AwsConsole == nil {
			// If payload is missing treat as unknown/unsupported
			out.Constraints = &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}}
			break
		}

		roleARNs := d.AwsConsole.RoleArns
		count := len(roleARNs)
		preview := previewStrings(roleARNs, MaxAuditRoleARNPreview)

		out.Constraints = &ResourceAccessID_AwsConsole{
			AwsConsole: &AWSConsoleConstraints{
				RoleArnsCount:   uint32(count),
				RoleArnsPreview: preview,
			},
		}
	// Unknown/unsupported constraint variant
	default:
		out.Constraints = &ResourceAccessID_UnknownConstraints{UnknownConstraints: &UnknownConstraints{}}
	}

	return out
}

// previewStrings returns up to limit elements from in, and a truncation flag
func previewStrings(in []string, limit int) []string {
	if limit <= 0 || len(in) == 0 {
		return nil
	}
	if len(in) <= limit {
		return append([]string(nil), in...)
	}
	return append([]string(nil), in[:limit]...)
}

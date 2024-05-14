/*
Copyright 2023 Gravitational, Inc.

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
	"fmt"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/compare"
	"github.com/gravitational/teleport/api/utils"
)

var _ compare.IsEqual[UserGroup] = (*UserGroupV1)(nil)

// UserGroup specifies an externally sourced group.
type UserGroup interface {
	ResourceWithLabels

	// GetApplications will return a list of application IDs associated with the user group.
	GetApplications() []string
	// SetApplications will set the list of application IDs associated with the user group.
	SetApplications([]string)
}

var _ ResourceWithLabels = (*UserGroupV1)(nil)

// NewUserGroup returns a new UserGroup.
func NewUserGroup(metadata Metadata, spec UserGroupSpecV1) (UserGroup, error) {
	g := &UserGroupV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: spec,
	}
	if err := g.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return g, nil
}

// GetApplications will return a list of application IDs associated with the user group.
func (g *UserGroupV1) GetApplications() []string {
	return g.Spec.Applications
}

// SetApplications will set the list of application IDs associated with the user group.
func (g *UserGroupV1) SetApplications(applications []string) {
	g.Spec.Applications = applications
}

// String returns the user group string representation.
func (g *UserGroupV1) String() string {
	return fmt.Sprintf("UserGroupV1(Name=%v, Labels=%v)",
		g.GetName(), g.GetAllLabels())
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (g *UserGroupV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(g.GetAllLabels()), g.GetName(), g.GetMetadata().Description)
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (g *UserGroupV1) setStaticFields() {
	g.Kind = KindUserGroup
	g.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (g *UserGroupV1) CheckAndSetDefaults() error {
	g.setStaticFields()
	if err := g.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// IsEqual determines if two user group resources are equivalent to one another.
func (g *UserGroupV1) IsEqual(i UserGroup) bool {
	if other, ok := i.(*UserGroupV1); ok {
		return deriveTeleportEqualUserGroupV1(g, other)
	}
	return false
}

// UserGroups is a list of UserGroup resources.
type UserGroups []UserGroup

// AsResources returns these groups as resources with labels.
func (g UserGroups) AsResources() []ResourceWithLabels {
	resources := make([]ResourceWithLabels, len(g))
	for i, group := range g {
		resources[i] = group
	}
	return resources
}

// SortByCustom custom sorts by given sort criteria.
func (g UserGroups) SortByCustom(sortBy SortBy) error {
	if sortBy.Field == "" {
		return nil
	}

	isDesc := sortBy.IsDesc
	switch sortBy.Field {
	case ResourceMetadataName:
		sort.SliceStable(g, func(i, j int) bool {
			groupA := g[i]
			groupB := g[j]

			groupAName := FriendlyName(groupA)
			groupBName := FriendlyName(groupB)

			if groupAName == "" {
				groupAName = groupA.GetName()
			}
			if groupBName == "" {
				groupBName = groupB.GetName()
			}

			return stringCompare(strings.ToLower(groupAName), strings.ToLower(groupBName), isDesc)
		})
	case ResourceSpecDescription:
		sort.SliceStable(g, func(i, j int) bool {
			groupA := g[i]
			groupB := g[j]

			groupADescription := groupA.GetMetadata().Description
			groupBDescription := groupB.GetMetadata().Description

			if oktaDescription, ok := groupA.GetLabel(OktaGroupDescriptionLabel); ok {
				groupADescription = oktaDescription
			}
			if oktaDescription, ok := groupB.GetLabel(OktaGroupDescriptionLabel); ok {
				groupBDescription = oktaDescription
			}

			return stringCompare(strings.ToLower(groupADescription), strings.ToLower(groupBDescription), isDesc)
		})

	default:
		return trace.NotImplemented("sorting by field %q for resource %q is not supported", sortBy.Field, KindKubeServer)
	}

	return nil
}

// Len returns the slice length.
func (g UserGroups) Len() int { return len(g) }

// Less compares user groups by name.
func (g UserGroups) Less(i, j int) bool { return g[i].GetName() < g[j].GetName() }

// Swap swaps two user groups.
func (g UserGroups) Swap(i, j int) { g[i], g[j] = g[j], g[i] }

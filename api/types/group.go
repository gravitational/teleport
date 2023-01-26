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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// Group specifies an externally sourced group.
type Group interface {
	ResourceWithLabels
}

// NewGroup returns a new Group.
func NewGroup(metadata Metadata) (Group, error) {
	g := &GroupV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
	}
	if err := g.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return g, nil
}

// String returns the group string representation.
func (g *GroupV1) String() string {
	return fmt.Sprintf("GroupV1(Name=%v, Labels=%v)",
		g.GetName(), g.GetAllLabels())
}

// Origin returns the origin value of the resource.
func (g *GroupV1) Origin() string {
	return g.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (g *GroupV1) SetOrigin(origin string) {
	g.Metadata.SetOrigin(origin)
}

// GetStaticLabels returns the group static labels.
func (g *GroupV1) GetStaticLabels() map[string]string {
	return g.Metadata.Labels
}

// SetStaticLabels sets the group static labels.
func (g *GroupV1) SetStaticLabels(sl map[string]string) {
	g.Metadata.Labels = sl
}

// GetAllLabels returns all labels from the group.
func (g *GroupV1) GetAllLabels() map[string]string {
	return g.Metadata.Labels
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (g *GroupV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(g.GetAllLabels()), g.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (g *GroupV1) setStaticFields() {
	g.Kind = KindGroup
	g.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (g *GroupV1) CheckAndSetDefaults() error {
	g.setStaticFields()
	if err := g.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Groups is a list of Group resources.
type Groups []Group

// AsResources returns these groups as resources with labels.
func (g Groups) AsResources() (resources ResourcesWithLabels) {
	for _, group := range g {
		resources = append(resources, group)
	}
	return resources
}

// Len returns the slice length.
func (g Groups) Len() int { return len(g) }

// Less compares groups by name.
func (g Groups) Less(i, j int) bool { return g[i].GetName() < g[j].GetName() }

// Swap swaps two groups.
func (g Groups) Swap(i, j int) { g[i], g[j] = g[j], g[i] }

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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// DiscoveredServer represents a DiscoveredServer.
type DiscoveredServer interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels

	// GetNamespace returns the resource namespace.
	GetNamespace() string

	// GetResourceMatchers returns the resource matchers of the DiscoveredServer.
	GetResourceMatchers() []*DiscoveredServerResourceMatcher
}

// NewDiscoveredServerV1 creates a new DiscoveredServer instance.
func NewDiscoveredServerV1(meta Metadata, spec DiscoveredServerSpecV1) (*DiscoveredServerV1, error) {
	s := &DiscoveredServerV1{
		ResourceHeader: ResourceHeader{
			Metadata: meta,
		},
		Spec: spec,
	}

	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

func (s *DiscoveredServerV1) setStaticFields() {
	s.Kind = KindDiscoveredServer
	s.Version = V1
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *DiscoveredServerV1) CheckAndSetDefaults() error {
	s.setStaticFields()
	return trace.Wrap(s.ResourceHeader.CheckAndSetDefaults())
}

// GetResourceMatchers returns the resource matchers of the DiscoveredServer.
func (s *DiscoveredServerV1) GetResourceMatchers() []*DiscoveredServerResourceMatcher {
	return s.Spec.ResourceMatchers
}

// GetNamespace returns the resource namespace.
func (s *DiscoveredServerV1) GetNamespace() string {
	return s.Metadata.Namespace
}

// GetAllLabels returns combined static and dynamic labels.
func (s *DiscoveredServerV1) GetAllLabels() map[string]string {
	return s.Metadata.Labels
}

// GetStaticLabels returns the static labels.
func (s *DiscoveredServerV1) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the static labels.
func (s *DiscoveredServerV1) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *DiscoveredServerV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(s.GetAllLabels()), s.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// Origin returns the origin value of the resource.
func (s *DiscoveredServerV1) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *DiscoveredServerV1) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

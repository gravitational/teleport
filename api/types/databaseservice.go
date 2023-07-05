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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// DatabaseService represents a DatabaseService (agent).
type DatabaseService interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels

	// GetNamespace returns the resource namespace.
	GetNamespace() string

	// GetResourceMatchers returns the resource matchers of the DatabaseService.
	GetResourceMatchers() []*DatabaseResourceMatcher
}

// NewDatabaseServiceV1 creates a new DatabaseService instance.
func NewDatabaseServiceV1(meta Metadata, spec DatabaseServiceSpecV1) (*DatabaseServiceV1, error) {
	s := &DatabaseServiceV1{
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

func (s *DatabaseServiceV1) setStaticFields() {
	s.Kind = KindDatabaseService
	s.Version = V1
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *DatabaseServiceV1) CheckAndSetDefaults() error {
	s.setStaticFields()

	return trace.Wrap(s.ResourceHeader.CheckAndSetDefaults())
}

// GetResourceMatchers returns the resource matchers of the DatabaseService.
func (s *DatabaseServiceV1) GetResourceMatchers() []*DatabaseResourceMatcher {
	return s.Spec.ResourceMatchers
}

// GetNamespace returns the resource namespace.
func (s *DatabaseServiceV1) GetNamespace() string {
	return s.Metadata.Namespace
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (s *DatabaseServiceV1) GetLabel(key string) (val string, ok bool) {
	v, ok := s.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns combined static and dynamic labels.
func (s *DatabaseServiceV1) GetAllLabels() map[string]string {
	return s.Metadata.Labels
}

// GetStaticLabels returns the static labels.
func (s *DatabaseServiceV1) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the static labels.
func (s *DatabaseServiceV1) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *DatabaseServiceV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(s.GetAllLabels()), s.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// Origin returns the origin value of the resource.
func (s *DatabaseServiceV1) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *DatabaseServiceV1) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

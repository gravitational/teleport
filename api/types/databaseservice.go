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
	// Database services deployed by Teleport have known configurations where
	// we will only define a single resource matcher.
	GetResourceMatchers() []*DatabaseResourceMatcher

	// GetHostname returns the hostname where this Database Service is running.
	GetHostname() string
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

// GetHostname returns the hostname where this Database Service is running.
func (s *DatabaseServiceV1) GetHostname() string {
	return s.Spec.Hostname
}

// GetNamespace returns the resource namespace.
func (s *DatabaseServiceV1) GetNamespace() string {
	return s.Metadata.Namespace
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *DatabaseServiceV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(s.GetAllLabels()), s.GetName())
	return MatchSearch(fieldVals, values, nil)
}

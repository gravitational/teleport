/*
Copyright 2024 Gravitational, Inc.

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

package clusterconfig

import (
	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAccessGraphState creates a new AccessGraphState resource.
func NewAccessGraphState(spec *clusterconfigpb.AccessGraphStateSpec) (*clusterconfigpb.AccessGraphState, error) {
	state := &clusterconfigpb.AccessGraphState{
		Kind:    types.KindAccessGraphState,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAccessGraphState,
		},
		Spec: spec,
	}
	if err := ValidateAccessGraphState(state); err != nil {
		return nil, trace.Wrap(err)
	}

	return state, nil
}

// ValidateAccessGraphState checks that required parameters are set
func ValidateAccessGraphState(s *clusterconfigpb.AccessGraphState) error {
	if s == nil {
		return trace.BadParameter("AccessGraphState is nil")
	}
	if s.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if s.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if s.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}

	if s.Metadata.Name != types.MetaNameAccessGraphState {
		return trace.BadParameter("Name is not %s", types.MetaNameAccessGraphState)
	}

	if s.Kind != types.KindAccessGraphState {
		return trace.BadParameter("Kind is not AccessGraphState")
	}
	if s.Version != types.V1 {
		return trace.BadParameter("Version is not V1")
	}

	return nil
}

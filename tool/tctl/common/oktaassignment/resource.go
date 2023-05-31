// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oktaassignment

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// Resource is a type to represent on Okta assignment that implements types.Resource
// and custom YAML marshaling. This is entirely to present a version of the Okta assignment
// with human readable statuses.
type Resource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the login rule specification
	Spec spec `json:"spec"`
}

// spec holds the Okta assignment spec.
type spec struct {
	CleanupTime    time.Time `json:"cleanup_time"`
	Finalized      bool      `json:"finalized"`
	LastTransition time.Time `json:"last_transition"`
	User           string    `json:"user"`
	Status         string    `json:"status"`
	Targets        []target  `json:"targets"`
}

// target holds an Okta assignment target.
type target struct {
	ID         string `json:"id"`
	TargetType string `json:"type"`
}

// ToResource converts an OktaAssignment into a *Resource which
// implements types.Resource and can be marshaled to YAML or JSON in a
// human-friendly format.
func ToResource(assignment types.OktaAssignment) *Resource {
	sourceTargets := assignment.GetTargets()
	resourceTargets := make([]target, len(sourceTargets))

	for i, sourceTarget := range sourceTargets {
		resourceTargets[i] = target{
			ID:         sourceTarget.GetID(),
			TargetType: sourceTarget.GetTargetType(),
		}
	}

	return &Resource{
		ResourceHeader: types.ResourceHeader{
			Kind:     assignment.GetKind(),
			Version:  assignment.GetVersion(),
			Metadata: assignment.GetMetadata(),
		},
		Spec: spec{
			CleanupTime:    assignment.GetCleanupTime(),
			Finalized:      assignment.IsFinalized(),
			LastTransition: assignment.GetLastTransition(),
			User:           assignment.GetUser(),
			Status:         assignment.GetStatus(),
			Targets:        resourceTargets,
		},
	}
}

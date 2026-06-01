/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

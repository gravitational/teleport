/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package userprovisioning

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// StaticHostUser is a resource that represents host users that should be
// created on matching nodes.
type StaticHostUser struct {
	headerv1.ResourceHeader
	// Spec is the static host user spec.
	Spec Spec
}

// Spec is the static host user spec.
type Spec struct {
	// Login is the login to create on the node.
	Login string `json:"login"`
	// Groups is a list of additional groups to add the user to.
	Groups []string `json:"groups"`
	// Sudoers is a list of sudoer entries to add.
	Sudoers []string `json:"sudoers"`
	// Uid is the new user's uid.
	Uid string `json:"uid"`
	// Gid is the new user's gid.
	Gid string `json:"gid"`
	// NodeLabels is a map of node labels that will create a user from this
	// resource.
	NodeLabels types.Labels `json:"node_labels"`
	// NodeLabelsExpression is a predicate expression to create a user from
	// this resource.
	NodeLabelsExpression string `json:"node_labels_expression"`
}

// NewStaticHostUser creates a new host user to be applied to matching SSH nodes.
func NewStaticHostUser(metadata *headerv1.Metadata, spec Spec) *StaticHostUser {
	return &StaticHostUser{
		ResourceHeader: headerv1.ResourceHeader{
			Kind:     types.KindStaticHostUser,
			Version:  types.V1,
			Metadata: metadata,
		},
		Spec: spec,
	}
}

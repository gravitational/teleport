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

type StaticHostUser struct {
	headerv1.ResourceHeader

	Spec Spec
}

type Spec struct {
	Login                string       `json:"login"`
	Groups               []string     `json:"groups"`
	Sudoers              []string     `json:"sudoers"`
	Uid                  string       `json:"uid"`
	Gid                  string       `json:"gid"`
	NodeLabels           types.Labels `json:"node_labels"`
	NodeLabelsExpression string       `json:"node_labels_expression"`
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

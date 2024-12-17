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
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
)

// NewStaticHostUser creates a new host user to be applied to matching SSH nodes.
func NewStaticHostUser(name string, spec *userprovisioningpb.StaticHostUserSpec) *userprovisioningpb.StaticHostUser {
	return &userprovisioningpb.StaticHostUser{
		Kind:    types.KindStaticHostUser,
		Version: types.V2,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
}

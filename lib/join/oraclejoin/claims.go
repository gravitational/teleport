// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oraclejoin

import (
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// Claims are the claims extracted from the instance certificate.
type Claims struct {
	// TenancyID is the ID of the instance's tenant.
	TenancyID string `json:"tenant_id"`
	// CompartmentID is the ID of the instance's compartment.
	CompartmentID string `json:"compartment_id"`
	// InstanceID is the instance's ID.
	InstanceID string `json:"instance_id"`
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c Claims) JoinAttrs() *workloadidentityv1pb.JoinAttrsOracle {
	return &workloadidentityv1pb.JoinAttrsOracle{
		TenancyId:     c.TenancyID,
		CompartmentId: c.CompartmentID,
		InstanceId:    c.InstanceID,
	}
}

// Region extracts the region from an instance's claims.
func (c Claims) Region() string {
	region, err := ParseRegionFromOCID(c.InstanceID)
	if err != nil {
		return ""
	}
	return region
}

// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package services

import (
	"context"

	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// WorkloadIdentities is an interface over the WorkloadIdentities service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type WorkloadIdentities interface {
	// GetWorkloadIdentity gets a SPIFFE Federation by name.
	GetWorkloadIdentity(
		ctx context.Context, name string,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// ListWorkloadIdentities lists all WorkloadIdentities using Google style
	// pagination.
	ListWorkloadIdentities(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*workloadidentityv1pb.WorkloadIdentity, string, error)
	// CreateWorkloadIdentity creates a new WorkloadIdentity.
	CreateWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
	// DeleteWorkloadIdentity deletes a SPIFFE Federation by name.
	DeleteWorkloadIdentity(ctx context.Context, name string) error
	// UpdateWorkloadIdentity updates a WorkloadIdentity. It will not act if the
	// resource is not found or where the revision does not match.
	UpdateWorkloadIdentity(
		ctx context.Context, workloadIdentity *workloadidentityv1pb.WorkloadIdentity,
	) (*workloadidentityv1pb.WorkloadIdentity, error)
}

// MarshalWorkloadIdentity marshals the WorkloadIdentity object into a JSON byte
// array.
func MarshalWorkloadIdentity(
	object *workloadidentityv1pb.WorkloadIdentity, opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalWorkloadIdentity unmarshals the WorkloadIdentity object from a
// JSON byte array.
func UnmarshalWorkloadIdentity(
	data []byte, opts ...MarshalOption,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	return UnmarshalProtoResource[*workloadidentityv1pb.WorkloadIdentity](data, opts...)
}

// ValidateWorkloadIdentity validates the WorkloadIdentity object. This is
// performed prior to writing to the backend.
func ValidateWorkloadIdentity(s *workloadidentityv1pb.WorkloadIdentity) error {
	switch {
	case s == nil:
		return trace.BadParameter("object cannot be nil")
	case s.Version != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.Kind != types.KindWorkloadIdentity:
		return trace.BadParameter("kind: must be %q", types.KindWorkloadIdentity)
	case s.Metadata == nil:
		return trace.BadParameter("metadata: is required")
	case s.Metadata.Name == "":
		return trace.BadParameter("metadata.name: is required")
	case s.Spec == nil:
		return trace.BadParameter("spec: is required")
	}

	// TODO: More validation here!!
	return nil
}

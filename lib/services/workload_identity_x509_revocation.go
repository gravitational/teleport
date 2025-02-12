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

package services

import (
	"context"
	"regexp"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// WorkloadIdentityX509Revocations is an interface over the
// WorkloadIdentityX509Revocations service. This  interface may also be
// implemented by a client to allow remote and local consumers to access the
// resource in a similar way.
type WorkloadIdentityX509Revocations interface {
	// GetWorkloadIdentityX509Revocation gets a WorkloadIdentityX509Revocation
	// by name.
	GetWorkloadIdentityX509Revocation(
		ctx context.Context, name string,
	) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	// ListWorkloadIdentityX509Revocations lists all
	// WorkloadIdentityX509Revocation using Google style pagination.
	ListWorkloadIdentityX509Revocations(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, string, error)
	// CreateWorkloadIdentityX509Revocation creates a new
	// WorkloadIdentityX509Revocation.
	CreateWorkloadIdentityX509Revocation(
		ctx context.Context,
		workloadIdentityX509Revocation *workloadidentityv1pb.WorkloadIdentityX509Revocation,
	) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	// DeleteWorkloadIdentityX509Revocation deletes a
	// WorkloadIdentityX509Revocation by name.
	DeleteWorkloadIdentityX509Revocation(ctx context.Context, name string) error
	// UpdateWorkloadIdentityX509Revocation updates a specific
	// WorkloadIdentityX509Revocation. The resource must already exist, and,
	// conditional update semantics are used - e.g the submitted resource must
	// have a revision matching the revision of the resource in the backend.
	UpdateWorkloadIdentityX509Revocation(
		ctx context.Context,
		workloadIdentityX509Revocation *workloadidentityv1pb.WorkloadIdentityX509Revocation,
	) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
	// UpsertWorkloadIdentityX509Revocation creates or updates a
	// WorkloadIdentityX509Revocation.
	UpsertWorkloadIdentityX509Revocation(
		ctx context.Context,
		workloadIdentityX509Revocation *workloadidentityv1pb.WorkloadIdentityX509Revocation,
	) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error)
}

// MarshalWorkloadIdentityX509Revocation marshals the
// WorkloadIdentityX509Revocation object into a JSON byte array.
func MarshalWorkloadIdentityX509Revocation(
	object *workloadidentityv1pb.WorkloadIdentityX509Revocation, opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalWorkloadIdentityX509Revocation unmarshals the
// WorkloadIdentityX509Revocation object from a JSON byte array.
func UnmarshalWorkloadIdentityX509Revocation(
	data []byte, opts ...MarshalOption,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	return UnmarshalProtoResource[*workloadidentityv1pb.WorkloadIdentityX509Revocation](data, opts...)
}

var validSerialRe = regexp.MustCompile("^(?:[0-9a-f]{2})+$")

// ValidateWorkloadIdentityX509Revocation validates the
// WorkloadIdentityX509Revocation object.
// It returns a nil if the object is valid, otherwise an error.
func ValidateWorkloadIdentityX509Revocation(s *workloadidentityv1pb.WorkloadIdentityX509Revocation) error {
	switch {
	case s == nil:
		return trace.BadParameter("object cannot be nil")
	case s.Version != types.V1:
		return trace.BadParameter("version: only %q is supported", types.V1)
	case s.Kind != types.KindWorkloadIdentityX509Revocation:
		return trace.BadParameter("kind: must be %q", types.KindWorkloadIdentityX509Revocation)
	case s.Metadata == nil:
		return trace.BadParameter("metadata: is required")
	case s.Metadata.Name == "":
		return trace.BadParameter("metadata.name: is required")
	case s.Metadata.Expires == nil:
		return trace.BadParameter("metadata.expires: is required")
	case s.Metadata.Expires.IsValid() == false:
		return trace.BadParameter("metadata.expires: must be valid")
	case s.Metadata.Expires.AsTime().IsZero():
		return trace.BadParameter("metadata.expires: must be non-zero")
	case s.Spec == nil:
		return trace.BadParameter("spec: is required")
	case s.Spec.Reason == "":
		return trace.BadParameter("spec.reason: is required")
	case s.Spec.RevokedAt == nil:
		return trace.BadParameter("spec.revoked_at: is required")
	case s.Spec.RevokedAt.IsValid() == false:
		return trace.BadParameter("spec.revoked_at: must be valid")
	case s.Spec.RevokedAt.AsTime().IsZero():
		return trace.BadParameter("spec.revoked_at: must be non-zero")
	}

	// Name must be a integer encoded as hex - this is the serial number of the
	// X509 cert. Whilst typically presented using a colon separated hex string,
	// here we will remove the colons. We will also ensure it is encoded in
	// lowercase, to ensure consistency.
	if !validSerialRe.MatchString(s.Metadata.Name) {
		return trace.BadParameter("metadata.name: must be a lower-case hex encoded integer without colons")
	}

	return nil
}

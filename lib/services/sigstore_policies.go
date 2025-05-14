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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// SigstorePolicies is an interface over the SigstorePolicy service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type SigstorePolicies interface {
	// GetSigstorePolicy gets a SigstorePolicy by name.
	GetSigstorePolicy(
		ctx context.Context, name string,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// ListSigtorePolicies lists all SigstorePolicy resources using Google style
	// pagination.
	ListSigstorePolicies(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*workloadidentityv1pb.SigstorePolicy, string, error)

	// CreateSigstorePolicy creates a new SigstorePolicy.
	CreateSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// DeleteSigstorePolicy deletes a SigstorePolicy by name.
	DeleteSigstorePolicy(ctx context.Context, name string) error

	// UpdateSigstorePolicy updates a specific SigstorePolicy. The resource must
	// already exist, and, conditional update semantics are used - e.g the
	// submitted resource must have a revision matching the revision of the
	// resource in the backend.
	UpdateSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)

	// UpsertSigstorePolicy creates or updates a SigstorePolicy.
	UpsertSigstorePolicy(
		ctx context.Context,
		sigstorePolicy *workloadidentityv1pb.SigstorePolicy,
	) (*workloadidentityv1pb.SigstorePolicy, error)
}

// MarshalSigstorePolicy marshals the SigstorePolicy object into a JSON byte
// slice.
func MarshalSigstorePolicy(
	object *workloadidentityv1pb.SigstorePolicy, opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalSigstorePolicy unmarshals the SigstorePolicy object from a JSON byte
// slice.
func UnmarshalSigstorePolicy(
	data []byte, opts ...MarshalOption,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	return UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](data, opts...)
}

// ValidateSigstorePolicy validates the SigstorePolicy object.
func ValidateSigstorePolicy(s *workloadidentityv1pb.SigstorePolicy) error {
	// TODO(boxofrad): Implement this in a follow-up PR.
	return nil
}

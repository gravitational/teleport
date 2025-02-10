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

package local

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	workloadIdentityX509RevocationPrefix = "workload_identity_x509_revocation"
)

// WorkloadIdentityX509RevocationService exposes backend functionality for
// storing WorkloadIdentityX509Revocation resources
type WorkloadIdentityX509RevocationService struct {
	service *generic.ServiceWrapper[*workloadidentityv1pb.WorkloadIdentityX509Revocation]
}

// NewWorkloadIdentityX509RevocationService creates a new
// WorkloadIdentityX509RevocationService
func NewWorkloadIdentityX509RevocationService(
	b backend.Backend,
) (*WorkloadIdentityX509RevocationService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*workloadidentityv1pb.WorkloadIdentityX509Revocation]{
			Backend:       b,
			ResourceKind:  types.KindWorkloadIdentityX509Revocation,
			BackendPrefix: backend.NewKey(workloadIdentityX509RevocationPrefix),
			MarshalFunc:   services.MarshalWorkloadIdentityX509Revocation,
			UnmarshalFunc: services.UnmarshalWorkloadIdentityX509Revocation,
			ValidateFunc:  services.ValidateWorkloadIdentityX509Revocation,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &WorkloadIdentityX509RevocationService{
		service: service,
	}, nil
}

// CreateWorkloadIdentityX509Revocation inserts a new
// WorkloadIdentityX509Revocation into the backend.
func (b *WorkloadIdentityX509RevocationService) CreateWorkloadIdentityX509Revocation(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	created, err := b.service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// GetWorkloadIdentityX509Revocation retrieves a specific
// WorkloadIdentityX509Revocation given a name
func (b *WorkloadIdentityX509RevocationService) GetWorkloadIdentityX509Revocation(
	ctx context.Context, name string,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	resource, err := b.service.GetResource(ctx, name)
	return resource, trace.Wrap(err)
}

// ListWorkloadIdentityX509Revocations lists all WorkloadIdentityX509Revocation
// using a given page size and last key.
func (b *WorkloadIdentityX509RevocationService) ListWorkloadIdentityX509Revocations(
	ctx context.Context, pageSize int, currentToken string,
) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, string, error) {
	r, nextToken, err := b.service.ListResources(ctx, pageSize, currentToken)
	return r, nextToken, trace.Wrap(err)
}

// DeleteWorkloadIdentityX509Revocation deletes a specific
// WorkloadIdentityX509Revocation.
func (b *WorkloadIdentityX509RevocationService) DeleteWorkloadIdentityX509Revocation(
	ctx context.Context, name string,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, name))
}

// DeleteAllWorkloadIdentityX509Revocations deletes all
// WorkloadIdentityX509Revocation resources, this is typically only meant to be
// used by the cache.
func (b *WorkloadIdentityX509RevocationService) DeleteAllWorkloadIdentityX509Revocations(
	ctx context.Context,
) error {
	return trace.Wrap(b.service.DeleteAllResources(ctx))
}

// UpsertWorkloadIdentityX509Revocation upserts a WorkloadIdentityX509Revocation.
// Prefer using CreateWorkloadIdentity. This is only designed for usage by the
// cache.
func (b *WorkloadIdentityX509RevocationService) UpsertWorkloadIdentityX509Revocation(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	upserted, err := b.service.UpsertResource(ctx, resource)
	return upserted, trace.Wrap(err)
}

// UpdateWorkloadIdentityX509Revocation updates a specific
// WorkloadIdentityX509Revocation. The resource must already exist, and,
// conditional update semantics are used - e.g the submitted resource must have
// a revision matching the revision of the resource in the backend.
func (b *WorkloadIdentityX509RevocationService) UpdateWorkloadIdentityX509Revocation(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentityX509Revocation,
) (*workloadidentityv1pb.WorkloadIdentityX509Revocation, error) {
	updated, err := b.service.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

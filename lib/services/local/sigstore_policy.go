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

const sigstorePolicyPrefix = "sigstore_policy"

// SigstorePolicyService exposes backend functionality for storing
// SigstorePolicy resources
type SigstorePolicyService struct {
	service *generic.ServiceWrapper[*workloadidentityv1pb.SigstorePolicy]
}

// NewSigstorePolicyService creates a new SigstorePolicyService.
func NewSigstorePolicyService(
	b backend.Backend,
) (*SigstorePolicyService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*workloadidentityv1pb.SigstorePolicy]{
			Backend:       b,
			ResourceKind:  types.KindSigstorePolicy,
			BackendPrefix: backend.NewKey(sigstorePolicyPrefix),
			MarshalFunc:   services.MarshalSigstorePolicy,
			UnmarshalFunc: services.UnmarshalSigstorePolicy,
			ValidateFunc:  services.ValidateSigstorePolicy,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SigstorePolicyService{
		service: service,
	}, nil
}

// CreateSigstorePolicy inserts a new SigstorePolicy into the backend.
func (b *SigstorePolicyService) CreateSigstorePolicy(
	ctx context.Context, resource *workloadidentityv1pb.SigstorePolicy,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	created, err := b.service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// GetSigstorePolicy retrieves a specific SigstorePolicy given a name.
func (b *SigstorePolicyService) GetSigstorePolicy(
	ctx context.Context, name string,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	resource, err := b.service.GetResource(ctx, name)
	return resource, trace.Wrap(err)
}

// ListSigstorePolicies lists all SigstorePolicy resources using a given page
// size and last key.
func (b *SigstorePolicyService) ListSigstorePolicies(
	ctx context.Context, pageSize int, currentToken string,
) ([]*workloadidentityv1pb.SigstorePolicy, string, error) {
	r, nextToken, err := b.service.ListResources(ctx, pageSize, currentToken)
	return r, nextToken, trace.Wrap(err)
}

// DeleteSigstorePolicy deletes a specific SigstorePolicy.
func (b *SigstorePolicyService) DeleteSigstorePolicy(
	ctx context.Context, name string,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, name))
}

// DeleteAllSigstorePolicies deletes all SigstorePolicy resources, this is
// typically only meant to be used by the cache.
func (b *SigstorePolicyService) DeleteAllSigstorePolicies(
	ctx context.Context,
) error {
	return trace.Wrap(b.service.DeleteAllResources(ctx))
}

// UpsertSigstorePolicy upserts a SigstorePolicy.
//
// Prefer using CreateSigstorePolicy. This is only designed for use by the cache.
func (b *SigstorePolicyService) UpsertSigstorePolicy(
	ctx context.Context, resource *workloadidentityv1pb.SigstorePolicy,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	upserted, err := b.service.UpsertResource(ctx, resource)
	return upserted, trace.Wrap(err)
}

// UpdateSigstorePolicy updates a specific SigstorePolicy. The resource must
// already exist, and, conditional update semantics are used - e.g the submitted
// resource must have a revision matching the revision of the resource in the backend.
func (b *SigstorePolicyService) UpdateSigstorePolicy(
	ctx context.Context, resource *workloadidentityv1pb.SigstorePolicy,
) (*workloadidentityv1pb.SigstorePolicy, error) {
	updated, err := b.service.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

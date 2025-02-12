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

package local

import (
	"context"
	"strings"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	workloadIdentityPrefix = "workload_identity"
)

// WorkloadIdentityService exposes backend functionality for storing
// WorkloadIdentity resources
type WorkloadIdentityService struct {
	service *generic.ServiceWrapper[*workloadidentityv1pb.WorkloadIdentity]
}

// NewWorkloadIdentityService creates a new WorkloadIdentityService
func NewWorkloadIdentityService(b backend.Backend) (*WorkloadIdentityService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*workloadidentityv1pb.WorkloadIdentity]{
			Backend:       b,
			ResourceKind:  types.KindWorkloadIdentity,
			BackendPrefix: workloadIdentityPrefix,
			MarshalFunc:   services.MarshalWorkloadIdentity,
			UnmarshalFunc: services.UnmarshalWorkloadIdentity,
			ValidateFunc:  services.ValidateWorkloadIdentity,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &WorkloadIdentityService{
		service: service,
	}, nil
}

// CreateWorkloadIdentity inserts a new WorkloadIdentity into the backend.
func (b *WorkloadIdentityService) CreateWorkloadIdentity(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentity,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	created, err := b.service.CreateResource(ctx, resource)
	return created, trace.Wrap(err)
}

// GetWorkloadIdentity retrieves a specific WorkloadIdentity given a name
func (b *WorkloadIdentityService) GetWorkloadIdentity(
	ctx context.Context, name string,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	resource, err := b.service.GetResource(ctx, name)
	return resource, trace.Wrap(err)
}

// ListWorkloadIdentities lists all WorkloadIdentities using a given page size
// and last key.
func (b *WorkloadIdentityService) ListWorkloadIdentities(
	ctx context.Context, pageSize int, currentToken string,
) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
	r, nextToken, err := b.service.ListResources(ctx, pageSize, currentToken)
	return r, nextToken, trace.Wrap(err)
}

// DeleteWorkloadIdentity deletes a specific WorkloadIdentity.
func (b *WorkloadIdentityService) DeleteWorkloadIdentity(
	ctx context.Context, name string,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, name))
}

// DeleteAllWorkloadIdentities deletes all SPIFFE resources, this is typically
// only meant to be used by the cache.
func (b *WorkloadIdentityService) DeleteAllWorkloadIdentities(
	ctx context.Context,
) error {
	return trace.Wrap(b.service.DeleteAllResources(ctx))
}

// UpsertWorkloadIdentity upserts a WorkloadIdentitys. Prefer using
// CreateWorkloadIdentity. This is only designed for usage by the cache.
func (b *WorkloadIdentityService) UpsertWorkloadIdentity(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentity,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	upserted, err := b.service.UpsertResource(ctx, resource)
	return upserted, trace.Wrap(err)
}

// UpdateWorkloadIdentity updates a specific WorkloadIdentity. The resource must
// already exist, and, condition update semantics are used - e.g the submitted
// resource must have a revision matching the revision of the resource in the
// backend.
func (b *WorkloadIdentityService) UpdateWorkloadIdentity(
	ctx context.Context, resource *workloadidentityv1pb.WorkloadIdentity,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	updated, err := b.service.ConditionalUpdateResource(ctx, resource)
	return updated, trace.Wrap(err)
}

func newWorkloadIdentityParser() *workloadIdentityParser {
	return &workloadIdentityParser{
		baseParser: newBaseParser(backend.NewKey(workloadIdentityPrefix)),
	}
}

type workloadIdentityParser struct {
	baseParser
}

func (p *workloadIdentityParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(workloadIdentityPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		return &types.ResourceHeader{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalWorkloadIdentity(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

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
	"iter"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const workloadIdentityPrefix = "workload_identity"

// WorkloadIdentityService exposes backend functionality for storing
// WorkloadIdentity resources
type WorkloadIdentityService struct {
	service *generic.ScopeAwareServiceWrapper[*workloadidentityv1pb.WorkloadIdentity]
}

// NewWorkloadIdentityService creates a new WorkloadIdentityService
func NewWorkloadIdentityService(b backend.Backend) (*WorkloadIdentityService, error) {
	service, err := generic.NewScopeAwareServiceWrapper(
		generic.ScopeAwareServiceWrapperConfig[*workloadidentityv1pb.WorkloadIdentity]{
			Backend:      b,
			ResourceKind: types.KindWorkloadIdentity,
			// Unscoped resources keep their historical key range so existing
			// WorkloadIdentities are unaffected.
			UnscopedBackendPrefix: backend.NewKey(workloadIdentityPrefix),
			ScopedBackendPrefix:   backend.NewKey(scopedPrefix, workloadIdentityPrefix),
			MarshalFunc:           services.MarshalWorkloadIdentity,
			UnmarshalFunc:         services.UnmarshalWorkloadIdentity,
			ValidateFunc:          services.ValidateWorkloadIdentity,
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

// GetWorkloadIdentity retrieves a WorkloadIdentity by the name and scope in the
// request.
func (b *WorkloadIdentityService) GetWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityRequest,
) (*workloadidentityv1pb.WorkloadIdentity, error) {
	resource, err := b.service.GetResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
	return resource, trace.Wrap(err)
}

// RangeWorkloadIdentities returns WorkloadIdentity resources within the range
// [start, end). The backend only supports ordering by name in ascending order;
// any other sort field or a descending order returns an error.
func (b *WorkloadIdentityService) RangeWorkloadIdentities(
	ctx context.Context, start, end string, sortField services.WorkloadIdentitySortField, sortDesc bool,
) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error] {
	if sortField != "" && sortField != services.WorkloadIdentitySortFieldName {
		return stream.Fail[*workloadidentityv1pb.WorkloadIdentity](
			trace.BadParameter("unsupported sort, only name field is supported, but got %q", sortField),
		)
	}
	if sortDesc {
		return stream.Fail[*workloadidentityv1pb.WorkloadIdentity](
			trace.BadParameter("unsupported sort, only ascending order is supported"),
		)
	}
	return b.service.Resources(ctx, start, end)
}

// DeleteWorkloadIdentity deletes a specific WorkloadIdentity given the name and
// scope in the request.
func (b *WorkloadIdentityService) DeleteWorkloadIdentity(
	ctx context.Context, req *workloadidentityv1pb.DeleteWorkloadIdentityRequest,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}))
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

// AppendPutWorkloadIdentityActions adds conditional actions to an atomic write
// to create or update a WorkloadIdentity.
func (b *WorkloadIdentityService) AppendPutWorkloadIdentityActions(
	actions []backend.ConditionalAction,
	resource *workloadidentityv1pb.WorkloadIdentity,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	if err := services.ValidateWorkloadIdentity(resource); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := b.service.MakeBackendItem(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(actions, backend.ConditionalAction{
		Key:       item.Key,
		Condition: condition,
		Action:    backend.Put(item),
	}), nil
}

// AppendDeleteWorkloadIdentityActions adds conditional actions to an atomic
// write to delete a WorkloadIdentity given its scope-qualified name.
func (b *WorkloadIdentityService) AppendDeleteWorkloadIdentityActions(
	actions []backend.ConditionalAction,
	name scopes.QualifiedName,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	key, err := b.service.BackendKey(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(actions, backend.ConditionalAction{
		Key:       key,
		Condition: condition,
		Action:    backend.Delete(),
	}), nil
}

func workloadIdentityUnscopedWatchPrefix() backend.Key {
	return backend.ExactKey(workloadIdentityPrefix)
}

func workloadIdentityScopedWatchPrefix() backend.Key {
	return backend.ExactKey(scopedPrefix, workloadIdentityPrefix)
}

// workloadIdentityNameFromKey recovers the scope-qualified name of a deleted
// WorkloadIdentity from its backend key. It handles both the unscoped key
// range (<workload_identity>/<name>) and the scope-namespaced range
// (<scoped>/<workload_identity>/<encoded-scope>/<name>).
func workloadIdentityNameFromKey(key backend.Key) (scopes.QualifiedName, error) {
	switch {
	case key.HasPrefix(workloadIdentityScopedWatchPrefix()):
		components := key.TrimPrefix(workloadIdentityScopedWatchPrefix()).Components()
		if len(components) != 2 {
			return scopes.QualifiedName{}, trace.NotFound(
				"expected 2 components, got %d parsing backend key %v",
				len(components),
				key.String(),
			)
		}
		encodedScope, name := components[0], components[1]
		scope, err := scopes.DecodeFromKey(encodedScope)
		if err != nil {
			return scopes.QualifiedName{}, trace.Wrap(err)
		}
		return scopes.QualifiedName{
			Scope: scope,
			Name:  name,
		}, nil
	case key.HasPrefix(workloadIdentityUnscopedWatchPrefix()):
		components := key.TrimPrefix(workloadIdentityUnscopedWatchPrefix()).Components()
		if len(components) != 1 {
			return scopes.QualifiedName{}, trace.NotFound(
				"expected 1 component, got %d parsing backend key %v",
				len(components),
				key.String(),
			)
		}
		return scopes.QualifiedName{
			Name: components[0],
		}, nil
	default:
		return scopes.QualifiedName{}, trace.NotFound(
			"unexpected prefix parsing backend key %v", key.String(),
		)
	}
}

func newWorkloadIdentityParser() *workloadIdentityParser {
	return &workloadIdentityParser{
		baseParser: newBaseParser(
			workloadIdentityUnscopedWatchPrefix(),
			workloadIdentityScopedWatchPrefix(),
		),
	}
}

type workloadIdentityParser struct {
	baseParser
}

func (p *workloadIdentityParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name, err := workloadIdentityNameFromKey(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if name.Scope != "" {
			// For scoped wids, we need to leverage a "skeleton" rather than the
			// ResourceHeader which has no place for the scope.
			// At some later date, we'll migrate to using the skeleton rather
			// than ResourceHeader for all WorkloadIdentity Delete events.
			return types.Resource153ToLegacy(workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: name.Name,
				}.Build(),
				Scope: name.Scope,
			}.Build()), nil
		}

		// Unscoped deletes keep their historical ResourceHeader representation
		// so existing consumers of these events are unaffected.
		return &types.ResourceHeader{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name.Name,
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
		// Downstream consumers (e.g. cache index keys) assume resource scopes
		// have passed weak validation, so drop events that violate that here.
		// A failure drops just this event: the watcher logs it and stays
		// healthy.
		if scope := resource.GetScope(); scope != "" {
			if err := scopes.WeakValidate(scope); err != nil {
				return nil, trace.Wrap(err, "validating scope of workload identity %q from event", resource.GetMetadata().GetName())
			}
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package local

import (
	"context"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	beamPrefix      = "beams"
	beamAliasPrefix = "beams_alias"
)

// BeamService manages Beam resources in the backend.
type BeamService struct {
	backend backend.Backend
	svc     *generic.ServiceWrapper[*beamsv1.Beam]
}

// NewBeamService creates a new BeamService.
func NewBeamService(b backend.Backend) (*BeamService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*beamsv1.Beam]{
			Backend:       b,
			ResourceKind:  types.KindBeam,
			BackendPrefix: backend.NewKey(beamPrefix),
			MarshalFunc:   services.MarshalProtoResource[*beamsv1.Beam],
			UnmarshalFunc: services.UnmarshalProtoResource[*beamsv1.Beam],
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &BeamService{
		backend: b,
		svc:     service,
	}, nil
}

// GetBeam returns the specified Beam resource.
func (s *BeamService) GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error) {
	item, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// GetBeamByAlias returns the specified Beam resource by alias.
func (s *BeamService) GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error) {
	if alias == "" {
		return nil, trace.BadParameter("alias: must be non-empty")
	}

	item, err := s.backend.Get(ctx, beamAliasKey(alias))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("beam %+q doesn't exist", alias)
		}
		return nil, trace.Wrap(err)
	}

	return s.GetBeam(ctx, string(item.Value))
}

// ListBeams returns a paginated list of Beam resources.
func (s *BeamService) ListBeams(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
	items, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextKey, nil
}

// AppendPutBeamActions adds conditional actions to an atomic write to create
// or update a Beam resource.
func (s *BeamService) AppendPutBeamActions(
	actions []backend.ConditionalAction,
	beam *beamsv1.Beam,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	if err := services.ValidateBeam(beam); err != nil {
		return nil, err
	}

	item, err := s.svc.MakeBackendItem(beam)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions = append(actions, backend.ConditionalAction{
		Key:       item.Key,
		Condition: condition,
		Action:    backend.Put(item),
	})

	// If this is a new beam, create a "lease" record for it to ensure the alias
	// is unique.
	if item.Revision == "" {
		aliasItem := backend.Item{
			Key:     beamAliasKey(beam.GetStatus().GetAlias()),
			Value:   []byte(beam.GetMetadata().GetName()),
			Expires: beam.GetSpec().GetExpires().AsTime(),
		}
		actions = append(actions, backend.ConditionalAction{
			Key:       aliasItem.Key,
			Condition: backend.NotExists(),
			Action:    backend.Put(aliasItem),
		})
	}

	return actions, nil
}

// AppendDeleteBeamActions adds conditional actions to an atomic write to delete
// a Beam resource.
func (s *BeamService) AppendDeleteBeamActions(
	actions []backend.ConditionalAction,
	beam *beamsv1.Beam,
	condition backend.Condition,
) ([]backend.ConditionalAction, error) {
	return append(actions,
		backend.ConditionalAction{
			Key:       s.svc.BackendKey(beam.GetMetadata().GetName()),
			Condition: condition,
			Action:    backend.Delete(),
		},
		backend.ConditionalAction{
			Key:       beamAliasKey(beam.GetStatus().GetAlias()),
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
	), nil
}

func beamAliasKey(alias string) backend.Key {
	return backend.NewKey(beamAliasPrefix, alias)
}

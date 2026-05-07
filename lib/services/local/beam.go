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
	"iter"
	"strings"

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
func (s *BeamService) ListBeams(ctx context.Context, pageSize int, pageToken string, options *services.ListBeamsRequestOptions) ([]*beamsv1.Beam, string, error) {
	if err := validateListOptions(options); err != nil {
		return nil, "", trace.Wrap(err)
	}

	items, nextKey, err := s.svc.ListResourcesWithFilter(ctx, pageSize, pageToken, services.MakeBeamFilterFunc(options))
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, nextKey, nil
}

// IterateBeams returns a sequence of beams starting from the given pageToken.
func (s *BeamService) IterateBeams(ctx context.Context, pageToken string) iter.Seq2[*beamsv1.Beam, error] {
	return s.IterateBeamsV2(ctx, pageToken, nil)
}

// IterateBeamsV2 returns a sequence of beams starting from the given pageToken
// with sorting and filtering.
func (s *BeamService) IterateBeamsV2(ctx context.Context, pageToken string, options *services.ListBeamsRequestOptions) iter.Seq2[*beamsv1.Beam, error] {
	if err := validateListOptions(options); err != nil {
		return func(yield func(*beamsv1.Beam, error) bool) {
			yield(nil, trace.Wrap(err))
		}
	}

	filterFn := services.MakeBeamFilterFunc(options)
	seq := s.svc.Resources(ctx, pageToken, "")

	return func(yield func(*beamsv1.Beam, error) bool) {
		for beam, err := range seq {
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			if !filterFn(beam) {
				continue
			}
			if !yield(beam, nil) {
				break
			}
		}
	}
}

func validateListOptions(options *services.ListBeamsRequestOptions) error {
	if options.GetSortField() != beamsv1.BeamSortField_BEAM_SORT_FIELD_UNSPECIFIED && options.GetSortField() != beamsv1.BeamSortField_BEAM_SORT_FIELD_NAME {
		return trace.CompareFailed("unsupported sort, only name field is supported")
	}
	if options.GetSortOrder() != beamsv1.BeamSortOrder_BEAM_SORT_ORDER_UNSPECIFIED && options.GetSortOrder() != beamsv1.BeamSortOrder_BEAM_SORT_ORDER_ASCENDING {
		return trace.CompareFailed("unsupported sort, only ascending order is supported")
	}
	return nil
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
			Key:   beamAliasKey(beam.GetStatus().GetAlias()),
			Value: []byte(beam.GetMetadata().GetName()),
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

func newBeamParser() *beamParser {
	return &beamParser{
		baseParser: newBaseParser(backend.NewKey(beamPrefix).ExactKey()),
	}
}

type beamParser struct {
	baseParser
}

func (p *beamParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(beamPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		return &types.ResourceHeader{
			Kind:    types.KindBeam,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: strings.TrimPrefix(name, backend.SeparatorString),
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*beamsv1.Beam](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

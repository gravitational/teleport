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
	"time"

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
			ValidateFunc:  services.ValidateBeam,
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

// CreateBeam creates a new Beam resource.
func (s *BeamService) CreateBeam(ctx context.Context, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
	created, err := s.svc.CreateResource(ctx, beam)
	return created, trace.Wrap(err)
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
			return nil, trace.NotFound("beam %q doesn't exist", alias)
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

// UpdateBeam updates an existing Beam resource.
func (s *BeamService) UpdateBeam(ctx context.Context, beam *beamsv1.Beam) (*beamsv1.Beam, error) {
	updated, err := s.svc.ConditionalUpdateResource(ctx, beam)
	return updated, trace.Wrap(err)
}

// DeleteBeam removes the specified Beam resource.
func (s *BeamService) DeleteBeam(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// CreateBeamAliasLease creates a new alias lease for a beam.
func (s *BeamService) CreateBeamAliasLease(ctx context.Context, alias, beamID string, expiry time.Time) error {
	if alias == "" {
		return trace.BadParameter("alias: must be non-empty")
	}
	if beamID == "" {
		return trace.BadParameter("beamID: must be non-empty")
	}

	_, err := s.backend.Create(ctx, backend.Item{
		Key:     beamAliasKey(alias),
		Value:   []byte(beamID),
		Expires: expiry,
	})
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("beam alias %q already exists", alias)
	}
	return trace.Wrap(err)
}

// DeleteBeamAliasLease deletes an alias lease for a Beam.
func (s *BeamService) DeleteBeamAliasLease(ctx context.Context, alias string) error {
	if alias == "" {
		return trace.BadParameter("alias: must be non-empty")
	}

	return trace.Wrap(s.backend.Delete(ctx, beamAliasKey(alias)))
}

// DeleteAllBeams removes all Beam resources.
func (s *BeamService) DeleteAllBeams(ctx context.Context) error {
	if err := s.svc.DeleteAllResources(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Delete all alias leases too.
	startKey := backend.NewKey(beamAliasPrefix).ExactKey()
	endKey := backend.RangeEnd(startKey)
	return trace.Wrap(s.backend.DeleteRange(ctx, startKey, endKey))
}

func beamAliasKey(alias string) backend.Key {
	return backend.NewKey(beamAliasPrefix, alias)
}

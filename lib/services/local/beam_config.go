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
	"strings"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const clusterBeamConfigPrefix = "cluster_beam_config"

// ClusterBeamConfigService manages [beamsv1.ClusterBeamConfig] resources in the backend.
type ClusterBeamConfigService struct {
	svc *generic.ServiceWrapper[*beamsv1.ClusterBeamConfig]
}

// NewClusterBeamConfigService creates a new ClusterBeamConfigService.
func NewClusterBeamConfigService(b backend.Backend) (*ClusterBeamConfigService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*beamsv1.ClusterBeamConfig]{
			Backend:       b,
			ResourceKind:  types.KindClusterBeamConfig,
			BackendPrefix: backend.NewKey(clusterBeamConfigPrefix),
			MarshalFunc:   services.MarshalProtoResource[*beamsv1.ClusterBeamConfig],
			UnmarshalFunc: services.UnmarshalProtoResource[*beamsv1.ClusterBeamConfig],
			ValidateFunc:  services.ValidateClusterBeamConfig,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClusterBeamConfigService{svc: service}, nil
}

// GetClusterBeamConfig returns the cluster-wide beam configuration.
func (s *ClusterBeamConfigService) GetClusterBeamConfig(ctx context.Context) (*beamsv1.ClusterBeamConfig, error) {
	cfg, err := s.svc.GetResource(ctx, types.MetaNameClusterBeamConfig)
	return cfg, trace.Wrap(err)
}

// CreateClusterBeamConfig creates the cluster-wide beam configuration.
func (s *ClusterBeamConfigService) CreateClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error) {
	created, err := s.svc.CreateResource(ctx, cfg)
	return created, trace.Wrap(err)
}

// UpdateClusterBeamConfig updates the cluster-wide beam configuration.
func (s *ClusterBeamConfigService) UpdateClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error) {
	updated, err := s.svc.ConditionalUpdateResource(ctx, cfg)
	return updated, trace.Wrap(err)
}

// UpsertClusterBeamConfig creates or updates the cluster-wide beam configuration.
func (s *ClusterBeamConfigService) UpsertClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error) {
	upserted, err := s.svc.UpsertResource(ctx, cfg)
	return upserted, trace.Wrap(err)
}

// DeleteClusterBeamConfig removes the cluster-wide beam configuration.
func (s *ClusterBeamConfigService) DeleteClusterBeamConfig(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, types.MetaNameClusterBeamConfig))
}

func newClusterBeamConfigParser() resourceParser {
	return &clusterBeamConfigParser{
		baseParser: newBaseParser(backend.NewKey(clusterBeamConfigPrefix)),
	}
}

type clusterBeamConfigParser struct {
	baseParser
}

// parse implements resourceParser.
func (p *clusterBeamConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		cfg, err := services.UnmarshalProtoResource[*beamsv1.ClusterBeamConfig](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(cfg), nil
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(clusterBeamConfigPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key)
		}
		return &types.ResourceHeader{
			Kind:    types.KindClusterBeamConfig,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: strings.TrimPrefix(name, backend.SeparatorString),
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

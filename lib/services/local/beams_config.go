// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// BeamsConfigService implements the storage layer for the BeamsConfig resource.
type BeamsConfigService struct {
	svc *generic.ServiceWrapper[*beamsv1.BeamsConfig]
}

// NewBeamsConfigService returns a new BeamsConfig storage service.
func NewBeamsConfigService(b backend.Backend) (*BeamsConfigService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*beamsv1.BeamsConfig]{
			Backend:       b,
			ResourceKind:  types.KindBeamsConfig,
			BackendPrefix: backend.NewKey(beamsConfigPrefix),
			MarshalFunc:   services.MarshalProtoResource[*beamsv1.BeamsConfig],
			UnmarshalFunc: services.UnmarshalProtoResource[*beamsv1.BeamsConfig],
			ValidateFunc:  validateBeamsConfig,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &BeamsConfigService{
		svc: svc,
	}, nil
}

// GetBeamsConfig returns the singleton BeamsConfig resource.
func (s *BeamsConfigService) GetBeamsConfig(ctx context.Context) (*beamsv1.BeamsConfig, error) {
	config, err := s.svc.GetResource(ctx, types.MetaNameBeamsConfig)
	return config, trace.Wrap(err)
}

// CreateBeamsConfig creates a new BeamsConfig resource.
func (s *BeamsConfigService) CreateBeamsConfig(ctx context.Context, config *beamsv1.BeamsConfig) (*beamsv1.BeamsConfig, error) {
	config, err := s.svc.CreateResource(ctx, config)
	return config, trace.Wrap(err)
}

// UpdateBeamsConfig updates an existing BeamsConfig resource using conditional update.
func (s *BeamsConfigService) UpdateBeamsConfig(ctx context.Context, config *beamsv1.BeamsConfig) (*beamsv1.BeamsConfig, error) {
	config, err := s.svc.ConditionalUpdateResource(ctx, config)
	return config, trace.Wrap(err)
}

// DeleteBeamsConfig deletes the singleton BeamsConfig resource.
func (s *BeamsConfigService) DeleteBeamsConfig(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, types.MetaNameBeamsConfig))
}

func validateBeamsConfig(config *beamsv1.BeamsConfig) error {
	if config.GetKind() != types.KindBeamsConfig {
		return trace.BadParameter("kind must be %q", types.KindBeamsConfig)
	}
	if config.GetVersion() != types.V1 {
		return trace.BadParameter("version must be %q", types.V1)
	}
	if config.GetMetadata().GetName() != types.MetaNameBeamsConfig {
		return trace.BadParameter("name must be %q", types.MetaNameBeamsConfig)
	}
	if config.GetMetadata().GetExpires() != nil {
		return trace.BadParameter("beams_config must not have an expiry")
	}
	return nil
}

const beamsConfigPrefix = types.KindBeamsConfig

func newBeamsConfigParser() resourceParser {
	return &beamsConfigParser{
		baseParser: newBaseParser(backend.NewKey(beamsConfigPrefix)),
	}
}

type beamsConfigParser struct {
	baseParser
}

func (p *beamsConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		config, err := services.UnmarshalProtoResource[*beamsv1.BeamsConfig](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.ProtoResource153ToLegacy(config), nil
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(beamsConfigPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key)
		}
		return &types.ResourceHeader{
			Kind:    types.KindBeamsConfig,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: strings.TrimPrefix(name, backend.SeparatorString),
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

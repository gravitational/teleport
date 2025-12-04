/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const appAuthConfigPrefix = "app_auth_config"

// AppAuthConfigService manages [appauthconfigv1.AppAuthConfig] resources in
// the backend.
type AppAuthConfigService struct {
	svc *generic.ServiceWrapper[*appauthconfigv1.AppAuthConfig]
}

// NewAppAuthConfigService creates a new AppAuthConfigService.
func NewAppAuthConfigService(b backend.Backend) (*AppAuthConfigService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*appauthconfigv1.AppAuthConfig]{
			Backend:       b,
			ResourceKind:  types.KindAppAuthConfig,
			BackendPrefix: backend.NewKey(appAuthConfigPrefix),
			MarshalFunc:   services.MarshalProtoResource[*appauthconfigv1.AppAuthConfig],
			UnmarshalFunc: services.UnmarshalProtoResource[*appauthconfigv1.AppAuthConfig],
			ValidateFunc:  services.ValidateAppAuthConfig,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AppAuthConfigService{
		svc: service,
	}, nil
}

// CreateAppAuthConfig creates a new AppAuthConfig resource.
func (s *AppAuthConfigService) CreateAppAuthConfig(ctx context.Context, config *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error) {
	created, err := s.svc.CreateResource(ctx, config)
	return created, trace.Wrap(err)
}

// GetAppAuthConfig returns the specified AppAuthConfig resource.
func (s *AppAuthConfigService) GetAppAuthConfig(ctx context.Context, name string) (*appauthconfigv1.AppAuthConfig, error) {
	item, err := s.svc.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// ListAppAuthConfigs returns a paginated list of AppAuthConfig resources.
func (s *AppAuthConfigService) ListAppAuthConfigs(ctx context.Context, pageSize int, pageToken string) ([]*appauthconfigv1.AppAuthConfig, string, error) {
	items, nextKey, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return items, nextKey, nil
}

// UpdateAppAuthConfig updates an existing AppAuthConfig resource.
func (s *AppAuthConfigService) UpdateAppAuthConfig(ctx context.Context, config *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error) {
	updated, err := s.svc.ConditionalUpdateResource(ctx, config)
	return updated, trace.Wrap(err)
}

// UpsertAppAuthConfig upserts an existing AppAuthConfig resource.
func (s *AppAuthConfigService) UpsertAppAuthConfig(ctx context.Context, config *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error) {
	upserted, err := s.svc.UpsertResource(ctx, config)
	return upserted, trace.Wrap(err)
}

// DeleteAppAuthConfig removes the specified AppAuthConfig resource.
func (s *AppAuthConfigService) DeleteAppAuthConfig(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllAppAuthConfigs removes all AppAuthConfig resources.
func (s *AppAuthConfigService) DeleteAllAppAuthConfigs(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

func newAppAuthConfigParser() resourceParser {
	return &appAuthConfigParser{
		baseParser: newBaseParser(backend.NewKey(appAuthConfigPrefix)),
	}
}

type appAuthConfigParser struct {
	baseParser
}

// parse implements resourceParser.
func (p *appAuthConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		config, err := services.UnmarshalProtoResource[*appauthconfigv1.AppAuthConfig](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(config), nil
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(appAuthConfigPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key)
		}
		return &types.ResourceHeader{
			Kind:    types.KindAppAuthConfig,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: strings.TrimPrefix(name, backend.SeparatorString),
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

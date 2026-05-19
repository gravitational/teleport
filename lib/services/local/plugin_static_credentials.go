/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	pluginStaticCredentialsPrefix = "plugin_static_credentials"
)

// PluginStaticCredentialsService manages plugin static credentials in the Backend.
type PluginStaticCredentialsService struct {
	svc generic.Service[types.PluginStaticCredentials]
}

// NewPluginStaticCredentialsService creates a new PluginStaticCredentialsService.
func NewPluginStaticCredentialsService(b backend.Backend) (*PluginStaticCredentialsService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.PluginStaticCredentials]{
		Backend:       b,
		ResourceKind:  types.KindPluginStaticCredentials,
		BackendPrefix: backend.NewKey(pluginStaticCredentialsPrefix),
		MarshalFunc:   services.MarshalPluginStaticCredentials,
		UnmarshalFunc: services.UnmarshalPluginStaticCredentials,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PluginStaticCredentialsService{
		svc: *svc,
	}, nil
}

// CreatePluginStaticCredentials will create a new plugin static credentials resource.
func (p *PluginStaticCredentialsService) CreatePluginStaticCredentials(ctx context.Context, pluginStaticCredentials types.PluginStaticCredentials) error {
	_, err := p.svc.CreateResource(ctx, pluginStaticCredentials)
	return trace.Wrap(err)
}

// GetPluginStaticCredentials will get a plugin static credentials resource by name.
func (p *PluginStaticCredentialsService) GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error) {
	creds, err := p.svc.GetResource(ctx, name)
	return creds, trace.Wrap(err)
}

func (p *PluginStaticCredentialsService) UpdatePluginStaticCredentials(ctx context.Context, item types.PluginStaticCredentials) (types.PluginStaticCredentials, error) {
	creds, err := p.svc.ConditionalUpdateResource(ctx, item)
	return creds, trace.Wrap(err)
}

// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
func (p *PluginStaticCredentialsService) GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	creds, err := p.svc.GetResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var foundCredentials []types.PluginStaticCredentials
	for _, cred := range creds {
		if types.MatchLabels(cred, labels) {
			foundCredentials = append(foundCredentials, cred)
		}
	}
	return foundCredentials, nil
}

// DeletePluginStaticCredentials will delete a plugin static credentials resource.
func (p *PluginStaticCredentialsService) DeletePluginStaticCredentials(ctx context.Context, name string) error {
	return trace.Wrap(p.svc.DeleteResource(ctx, name))
}

// GetAllPluginStaticCredentials will get all plugin static credentials. Cache
// use only.
func (p *PluginStaticCredentialsService) GetAllPluginStaticCredentials(ctx context.Context) ([]types.PluginStaticCredentials, error) {
	creds, err := p.svc.GetResources(ctx)
	return creds, trace.Wrap(err)
}

// DeleteAllPluginStaticCredentials will remove all plugin static credentials.
// Cache use only.
func (p *PluginStaticCredentialsService) DeleteAllPluginStaticCredentials(ctx context.Context) error {
	return trace.Wrap(p.svc.DeleteAllResources(ctx))
}

// UpsertPluginStaticCredentials will upsert a plugin static credentials. Cache
// use only.
func (p *PluginStaticCredentialsService) UpsertPluginStaticCredentials(ctx context.Context, item types.PluginStaticCredentials) (types.PluginStaticCredentials, error) {
	cred, err := p.svc.UpsertResource(ctx, item)
	return cred, trace.Wrap(err)
}

var _ services.PluginStaticCredentials = (*PluginStaticCredentialsService)(nil)

type pluginStaticCredentialsParser struct {
	baseParser
}

func newPluginStaticCredentialsParser() *pluginStaticCredentialsParser {
	return &pluginStaticCredentialsParser{
		baseParser: newBaseParser(backend.NewKey(pluginStaticCredentialsPrefix)),
	}
}

func (p *pluginStaticCredentialsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		parts := event.Item.Key.Components()
		if len(parts) != 2 {
			return nil, trace.BadParameter("malformed key for %s event: %s", types.KindPluginStaticCredentials, event.Item.Key)
		}

		return &types.ResourceHeader{
			Kind:    types.KindPluginStaticCredentials,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:        parts[1],
				Namespace:   apidefaults.Namespace,
				Description: parts[0],
			},
		}, nil

	case types.OpPut:
		return services.UnmarshalPluginStaticCredentials(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

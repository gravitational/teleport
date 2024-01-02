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
func NewPluginStaticCredentialsService(backend backend.Backend) (*PluginStaticCredentialsService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.PluginStaticCredentials]{
		Backend:       backend,
		ResourceKind:  types.KindPluginStaticCredentials,
		BackendPrefix: pluginStaticCredentialsPrefix,
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

var _ services.PluginStaticCredentials = (*PluginStaticCredentialsService)(nil)

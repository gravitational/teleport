/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	return p.svc.CreateResource(ctx, pluginStaticCredentials)
}

// GetPluginStaticCredentials will get a plugin static credentials resource by name.
func (p *PluginStaticCredentialsService) GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error) {
	return p.svc.GetResource(ctx, name)
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
	return p.svc.DeleteResource(ctx, name)
}

var _ services.PluginStaticCredentials = (*PluginStaticCredentialsService)(nil)

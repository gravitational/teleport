/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// AppService manages application resources in the backend.
type AppService struct {
	backend.Backend
}

// NewAppService creates a new AppService.
func NewAppService(backend backend.Backend) *AppService {
	return &AppService{Backend: backend}
}

// GetApps returns all application resources.
func (s *AppService) GetApps(ctx context.Context) ([]types.Application, error) {
	startKey := backend.Key(appPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps := make([]types.Application, len(result.Items))
	for i, item := range result.Items {
		app, err := services.UnmarshalApp(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		apps[i] = app
	}
	return apps, nil
}

// GetApp returns the specified application resource.
func (s *AppService) GetApp(ctx context.Context, name string) (types.Application, error) {
	item, err := s.Get(ctx, backend.Key(appPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("application %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	app, err := services.UnmarshalApp(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// CreateApp creates a new application resource.
func (s *AppService) CreateApp(ctx context.Context, app types.Application) error {
	if err := app.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalApp(app)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(appPrefix, app.GetName()),
		Value:   value,
		Expires: app.Expiry(),
		ID:      app.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateApp updates an existing application resource.
func (s *AppService) UpdateApp(ctx context.Context, app types.Application) error {
	if err := app.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalApp(app)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(appPrefix, app.GetName()),
		Value:   value,
		Expires: app.Expiry(),
		ID:      app.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteApp removes the specified application resource.
func (s *AppService) DeleteApp(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(appPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("application %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllApps removes all application resources.
func (s *AppService) DeleteAllApps(ctx context.Context) error {
	startKey := backend.Key(appPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	appPrefix = "applications"
)

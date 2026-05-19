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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// AppService manages application resources in the backend.
type AppService struct {
	backend.Backend
}

// NewAppService creates a new AppService.
func NewAppService(backend backend.Backend) *AppService {
	return &AppService{Backend: backend}
}

// Apps returns application resources within the range [start, end).
func (s *AppService) Apps(ctx context.Context, start, end string) iter.Seq2[types.Application, error] {
	return func(yield func(types.Application, error) bool) {
		appKey := backend.NewKey(appPrefix)
		endKey := backend.RangeEnd(appKey)
		if end != "" {
			endKey = appKey.AppendKey(backend.KeyFromString(end)).ExactKey()
		}
		for item, err := range s.Backend.Items(ctx, backend.ItemsParams{
			StartKey: appKey.AppendKey(backend.KeyFromString(start)),
			EndKey:   endKey,
		}) {
			if err != nil {
				yield(nil, err)
				return
			}

			app, err := services.UnmarshalApp(item.Value,
				services.WithExpires(item.Expires), services.WithRevision(item.Revision))
			if err != nil {
				continue
			}

			// The range is not inclusive of the end key, so return early
			// if the end has been reached.
			if end != "" && app.GetName() >= end {
				return
			}

			if !yield(app, nil) {
				return
			}
		}
	}
}

// ListApps returns a page of application resources.
func (s *AppService) ListApps(ctx context.Context, limit int, startKey string) ([]types.Application, string, error) {
	// Adjust page size, so it can't be too large.
	if limit <= 0 || limit > defaults.DefaultChunkSize {
		limit = defaults.DefaultChunkSize
	}

	appKey := backend.NewKey(appPrefix)
	var out []types.Application
	for item, err := range s.Backend.Items(ctx, backend.ItemsParams{
		StartKey: appKey.AppendKey(backend.KeyFromString(startKey)),
		EndKey:   backend.RangeEnd(appKey),
		Limit:    limit + 1,
	}) {
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		app, err := services.UnmarshalApp(item.Value,
			services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			continue
		}

		if len(out) == limit {
			return out, app.GetName(), nil
		}

		out = append(out, app)
	}

	return out, "", nil
}

// GetApps returns all application resources.
func (s *AppService) GetApps(ctx context.Context) ([]types.Application, error) {
	startKey := backend.ExactKey(appPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps := make([]types.Application, len(result.Items))
	for i, item := range result.Items {
		app, err := services.UnmarshalApp(item.Value,
			services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		apps[i] = app
	}
	return apps, nil
}

// GetApp returns the specified application resource.
func (s *AppService) GetApp(ctx context.Context, name string) (types.Application, error) {
	item, err := s.Get(ctx, backend.NewKey(appPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("application %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	app, err := services.UnmarshalApp(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// CreateApp creates a new application resource.
func (s *AppService) CreateApp(ctx context.Context, app types.Application) error {
	if err := services.CheckAndSetDefaults(app); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalApp(app)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(appPrefix, app.GetName()),
		Value:   value,
		Expires: app.Expiry(),
	}
	_, err = s.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("app %q already exists", app.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateApp updates an existing application resource.
func (s *AppService) UpdateApp(ctx context.Context, app types.Application) error {
	if err := services.CheckAndSetDefaults(app); err != nil {
		return trace.Wrap(err)
	}
	rev := app.GetRevision()
	value, err := services.MarshalApp(app)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(appPrefix, app.GetName()),
		Value:    value,
		Expires:  app.Expiry(),
		Revision: rev,
	}
	_, err = s.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("app %q doesn't exist", app.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteApp removes the specified application resource.
func (s *AppService) DeleteApp(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.NewKey(appPrefix, name))
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
	startKey := backend.ExactKey(appPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	appPrefix = "applications"
)

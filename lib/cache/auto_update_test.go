// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// TestAutoUpdateConfig tests that CRUD operations on AutoUpdateConfig resources are
// replicated from the backend to the cache.
func TestAutoUpdateConfig(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdatev1.AutoUpdateConfig]{
		newResource: func(name string) (*autoupdatev1.AutoUpdateConfig, error) {
			return newAutoUpdateConfig(t), nil
		},
		create: func(ctx context.Context, item *autoupdatev1.AutoUpdateConfig) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateConfig(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateConfig, error) {
			item, err := p.autoUpdateService.GetAutoUpdateConfig(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateConfig{}, nil
			}
			return []*autoupdatev1.AutoUpdateConfig{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateConfig, error) {
			item, err := p.cache.GetAutoUpdateConfig(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateConfig{}, nil
			}
			return []*autoupdatev1.AutoUpdateConfig{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateConfig(ctx))
		},
	})
}

// TestAutoUpdateVersion tests that CRUD operations on AutoUpdateVersion resource are
// replicated from the backend to the cache.
func TestAutoUpdateVersion(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdatev1.AutoUpdateVersion]{
		newResource: func(name string) (*autoupdatev1.AutoUpdateVersion, error) {
			return newAutoUpdateVersion(t), nil
		},
		create: func(ctx context.Context, item *autoupdatev1.AutoUpdateVersion) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateVersion(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateVersion, error) {
			item, err := p.autoUpdateService.GetAutoUpdateVersion(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateVersion{}, nil
			}
			return []*autoupdatev1.AutoUpdateVersion{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateVersion, error) {
			item, err := p.cache.GetAutoUpdateVersion(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateVersion{}, nil
			}
			return []*autoupdatev1.AutoUpdateVersion{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateVersion(ctx))
		},
	})
}

// TestAutoUpdateAgentRollout tests that CRUD operations on AutoUpdateAgentRollout resource are
// replicated from the backend to the cache.
func TestAutoUpdateAgentRollout(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdatev1.AutoUpdateAgentRollout]{
		newResource: func(name string) (*autoupdatev1.AutoUpdateAgentRollout, error) {
			return newAutoUpdateAgentRollout(t), nil
		},
		create: func(ctx context.Context, item *autoupdatev1.AutoUpdateAgentRollout) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateAgentRollout(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateAgentRollout, error) {
			item, err := p.autoUpdateService.GetAutoUpdateAgentRollout(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateAgentRollout{}, nil
			}
			return []*autoupdatev1.AutoUpdateAgentRollout{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdatev1.AutoUpdateAgentRollout, error) {
			item, err := p.cache.GetAutoUpdateAgentRollout(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdatev1.AutoUpdateAgentRollout{}, nil
			}
			return []*autoupdatev1.AutoUpdateAgentRollout{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateAgentRollout(ctx))
		},
	})
}

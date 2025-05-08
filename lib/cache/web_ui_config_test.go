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

	"github.com/gravitational/teleport/api/types"
)

func TestWebUIConfig(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.UIConfig]{
		newResource: func(name string) (types.UIConfig, error) {
			return &types.UIConfigV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: types.MetaNameUIConfig,
					},
				},
			}, nil
		},
		create: p.clusterConfigS.SetUIConfig,
		list: func(ctx context.Context) ([]types.UIConfig, error) {
			cfg, err := p.clusterConfigS.GetUIConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.UIConfig{cfg}, nil
		},
		cacheList: func(ctx context.Context) ([]types.UIConfig, error) {
			cfg, err := p.cache.GetUIConfig(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					return nil, nil
				}
				return nil, trace.Wrap(err)
			}

			return []types.UIConfig{cfg}, nil
		},
		deleteAll: p.clusterConfigS.DeleteUIConfig,
	})
}

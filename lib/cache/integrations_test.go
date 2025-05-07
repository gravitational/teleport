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

	"github.com/gravitational/teleport/api/types"
)

// TestIntegrations tests that CRUD operations on integrations resources are
// replicated from the backend to the cache.
func TestIntegrations(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Integration]{
		newResource: func(name string) (types.Integration, error) {
			return types.NewIntegrationAWSOIDC(
				types.Metadata{Name: name},
				&types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
				},
			)
		},
		create: func(ctx context.Context, i types.Integration) error {
			_, err := p.integrations.CreateIntegration(ctx, i)
			return err
		},
		list: func(ctx context.Context) ([]types.Integration, error) {
			results, _, err := p.integrations.ListIntegrations(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetIntegration,
		cacheList: func(ctx context.Context) ([]types.Integration, error) {
			results, _, err := p.cache.ListIntegrations(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, i types.Integration) error {
			_, err := p.integrations.UpdateIntegration(ctx, i)
			return err
		},
		deleteAll: p.integrations.DeleteAllIntegrations,
	})
}

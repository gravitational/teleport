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
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestDatabaseServices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewDatabaseServicesService(bk)

	// Upsert some DatabaseServices.
	ds1, err := types.NewDatabaseServiceV1(
		types.Metadata{Name: "ds1"},
		types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.DatabaseResourceMatcher{
				{
					Labels: &types.Labels{
						"env": []string{"ds1"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	_, err = service.UpsertDatabaseService(ctx, ds1)
	require.NoError(t, err)

	ds2, err := types.NewDatabaseServiceV1(
		types.Metadata{Name: "ds2"},
		types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.DatabaseResourceMatcher{
				{
					Labels: &types.Labels{
						"env": []string{"ds2"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	_, err = service.UpsertDatabaseService(ctx, ds2)
	require.NoError(t, err)

	// Replace the DS1
	initialResourceMatchers := ds1.GetResourceMatchers()
	initialResourceMatchers[0] = &types.DatabaseResourceMatcher{
		Labels: &types.Labels{"env": []string{"ds1", "ds2"}},
	}
	ds1.Spec.ResourceMatchers = initialResourceMatchers
	_, err = service.UpsertDatabaseService(ctx, ds1)
	require.NoError(t, err)

	// Remove one of the DatabaseServices
	err = service.DeleteDatabaseService(ctx, ds2.GetName())
	require.NoError(t, err)

	// Remove all of the DatabaseServices
	err = service.DeleteAllDatabaseServices(ctx)
	require.NoError(t, err)
}

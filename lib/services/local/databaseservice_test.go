/*
Copyright 2022 Gravitational, Inc.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

	// Initially we expect no DatabaseServices.
	out, err := service.GetAllDatabaseServices(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Upsert some DatabaseServices.
	ds1, err := types.NewDatabaseServiceV1(
		types.Metadata{Name: "ds1"},
		types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.ResourceMatcher{
				{
					Labels: &types.Labels{
						"env": []string{"ds1"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, service.UpsertDatabaseService(ctx, ds1))

	ds2, err := types.NewDatabaseServiceV1(
		types.Metadata{Name: "ds2"},
		types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.ResourceMatcher{
				{
					Labels: &types.Labels{
						"env": []string{"ds2"},
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, service.UpsertDatabaseService(ctx, ds2))

	// Test fetch all.
	out, err = service.GetAllDatabaseServices(ctx)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Empty(t, cmp.Diff([]types.DatabaseService{ds1, ds2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Replace the DS1
	initialResourceMatchers := ds1.GetResourceMatchers()
	initialResourceMatchers[0] = &types.ResourceMatcher{
		Labels: &types.Labels{"env": []string{"ds1", "ds2"}},
	}
	ds1.Spec.ResourceMatchers = initialResourceMatchers
	require.NoError(t, service.UpsertDatabaseService(ctx, ds1))

	// Remove one of the DatabaseServices
	err = service.DeleteDatabaseService(ctx, ds2.GetName())
	require.NoError(t, err)

	// Test fetch all again
	out, err = service.GetAllDatabaseServices(ctx)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff([]types.DatabaseService{ds1}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))
}

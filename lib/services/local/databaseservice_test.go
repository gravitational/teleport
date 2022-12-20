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
	presence := NewPresenceService(bk)

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
	_, err = presence.UpsertDatabaseService(ctx, ds1)
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
	_, err = presence.UpsertDatabaseService(ctx, ds2)
	require.NoError(t, err)

	// Replace the DS1
	initialResourceMatchers := ds1.GetResourceMatchers()
	initialResourceMatchers[0] = &types.DatabaseResourceMatcher{
		Labels: &types.Labels{"env": []string{"ds1", "ds2"}},
	}
	ds1.Spec.ResourceMatchers = initialResourceMatchers
	_, err = presence.UpsertDatabaseService(ctx, ds1)
	require.NoError(t, err)

	// Remove one of the DatabaseServices
	err = service.DeleteDatabaseService(ctx, ds2.GetName())
	require.NoError(t, err)

	// Remove all of the DatabaseServices
	err = service.DeleteAllDatabaseServices(ctx)
	require.NoError(t, err)
}

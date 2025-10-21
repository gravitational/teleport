/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestHandlers(t *testing.T) {
	t.Parallel()

	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(logtest.NewLogger()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

	updateResourceWithLabels := func(t *testing.T, r types.Resource) types.Resource {
		t.Helper()
		rWithLabels, ok := r.(types.ResourceWithLabels)
		require.True(t, ok)
		rWithLabels.SetStaticLabels(map[string]string{"updated": "true"})
		return r
	}

	tests := []struct {
		kind             string
		makeResource     func(*testing.T, string) types.Resource
		updateResource   func(*testing.T, types.Resource) types.Resource
		checkMFARequired require.BoolAssertionFunc
	}{
		{
			kind: types.KindDatabase,
			makeResource: func(t *testing.T, name string) types.Resource {
				t.Helper()
				app, err := types.NewDatabaseV3(
					types.Metadata{
						Name: name,
					},
					types.DatabaseSpecV3{
						URI:      "localhost:12345",
						Protocol: "mysql",
					},
				)
				require.NoError(t, err)
				return app
			},
			updateResource:   updateResourceWithLabels,
			checkMFARequired: require.False,
		},
		{
			kind: types.KindApp,
			makeResource: func(t *testing.T, name string) types.Resource {
				t.Helper()
				app, err := types.NewAppV3(
					types.Metadata{Name: name},
					types.AppSpecV3{URI: "http://localhost:12345"},
				)
				require.NoError(t, err)
				return app
			},
			updateResource:   updateResourceWithLabels,
			checkMFARequired: require.False,
		},
		{
			kind: types.KindAppServer,
			makeResource: func(t *testing.T, name string) types.Resource {
				t.Helper()
				app, err := types.NewAppV3(
					types.Metadata{
						Name: name,
					},
					types.AppSpecV3{
						URI:         "http://localhost:12345",
						Integration: "test-integration",
					},
				)
				require.NoError(t, err)
				appServer, err := types.NewAppServerV3FromApp(app, "hostname", "hostid")
				require.NoError(t, err)
				return appServer
			},
			updateResource:   updateResourceWithLabels,
			checkMFARequired: require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			handler := Handlers()[tt.kind]
			require.NotNil(t, handler)

			// TODO(greedy52): update logic for singleton
			resources := []types.Resource{
				tt.makeResource(t, fmt.Sprintf("%s-1", tt.kind)),
				tt.makeResource(t, fmt.Sprintf("%s-2", tt.kind)),
				tt.makeResource(t, fmt.Sprintf("%s-3", tt.kind)),
			}

			t.Run("MFARequired", func(t *testing.T) {
				tt.checkMFARequired(t, handler.MFARequired())
			})

			t.Run("Create", func(t *testing.T) {
				for _, r := range resources {
					raw := mustMakeUnknownResource(t, r)
					require.NoError(t, handler.Create(t.Context(), clt, raw, CreateOpts{}))
				}
			})

			// Test getting all resources of the same kind. Getting a single
			// resource by its name is tested in other test cases.
			t.Run("Get", func(t *testing.T) {
				collection, err := handler.Get(t.Context(), clt, services.Ref{}, GetOpts{})
				require.NoError(t, err)
				require.ElementsMatch(t,
					sliceutils.Map(resources, types.GetName),
					sliceutils.Map(collection.Resources(), types.GetName),
				)
			})

			t.Run("Update", func(t *testing.T) {
				r := tt.updateResource(t, resources[0])
				raw := mustMakeUnknownResource(t, r)
				require.NoError(t, handler.Update(t.Context(), clt, raw, CreateOpts{}))

				collection, err := handler.Get(t.Context(), clt, services.Ref{Name: r.GetName()}, GetOpts{})
				require.NoError(t, err)
				require.Len(t, collection.Resources(), 1)
				// Double-check revision is changed.
				require.NotEqual(t, r.GetRevision(), collection.Resources()[0].GetRevision())
			})

			t.Run("Delete", func(t *testing.T) {
				r := resources[0]
				require.NoError(t, handler.Delete(t.Context(), clt, services.Ref{Name: r.GetName()}))

				_, err := handler.Get(t.Context(), clt, services.Ref{Name: r.GetName()}, GetOpts{})
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			})
		})
	}
}

func mustMakeUnknownResource(t *testing.T, r types.Resource) services.UnknownResource {
	t.Helper()
	resourceJSON, err := json.Marshal(r)
	require.NoError(t, err)
	var unknown services.UnknownResource
	require.NoError(t, json.Unmarshal(resourceJSON, &unknown))
	return unknown
}

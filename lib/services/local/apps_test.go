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
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestAppsCRUD tests backend operations with application resources.
func TestAppsCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewAppService(backend)

	// Create a couple applications.
	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{
		Name: "app2",
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	// Initially we expect no apps.
	out, err := service.GetApps(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both apps.
	err = service.CreateApp(ctx, app1)
	require.NoError(t, err)
	err = service.CreateApp(ctx, app2)
	require.NoError(t, err)

	// Fetch all apps.
	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app1, app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific application.
	app, err := service.GetApp(ctx, app2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app2, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch an application that doesn't exist.
	_, err = service.GetApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same application.
	err = service.CreateApp(ctx, app1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update an application.
	app1.Metadata.Description = "description"
	err = service.UpdateApp(ctx, app1)
	require.NoError(t, err)
	app, err = service.GetApp(ctx, app1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app1, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete an application.
	err = service.DeleteApp(ctx, app1.GetName())
	require.NoError(t, err)
	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete an application that doesn't exist.
	err = service.DeleteApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all applications.
	err = service.DeleteAllApps(ctx)
	require.NoError(t, err)
	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Len(t, out, 0)
}

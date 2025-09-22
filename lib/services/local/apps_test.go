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
	"cmp"
	"context"
	"slices"
	"strconv"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAppsCRUD tests backend operations with application resources.
func TestAppsCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
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
	require.Empty(t, out)

	out, next, err := service.ListApps(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, next)

	var iterOut []types.Application
	for app, err := range service.Apps(ctx, "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	require.Empty(t, iterOut)

	// Create both apps.
	err = service.CreateApp(ctx, app1)
	require.NoError(t, err)
	err = service.CreateApp(ctx, app2)
	require.NoError(t, err)

	// Fetch all apps.
	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff([]types.Application{app1, app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, next, err = service.ListApps(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff([]types.Application{app1, app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Empty(t, next)

	iterOut = nil
	for app, err := range service.Apps(ctx, "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	require.Empty(t, gocmp.Diff([]types.Application{app1, app2}, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific application.
	app, err := service.GetApp(ctx, app2.GetName())
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(app2, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
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
	require.Empty(t, gocmp.Diff(app1, app,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete an application.
	err = service.DeleteApp(ctx, app1.GetName())
	require.NoError(t, err)

	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff([]types.Application{app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, next, err = service.ListApps(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff([]types.Application{app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Empty(t, next)

	iterOut = nil
	for app, err := range service.Apps(ctx, "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	require.Empty(t, gocmp.Diff([]types.Application{app2}, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete an application that doesn't exist.
	err = service.DeleteApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all applications.
	err = service.DeleteAllApps(ctx)
	require.NoError(t, err)

	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	out, next, err = service.ListApps(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, next)

	iterOut = nil
	for app, err := range service.Apps(ctx, "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	require.Empty(t, iterOut)

	// Test pagination
	var expected []types.Application
	for i := range 1324 {
		app, err := types.NewAppV3(types.Metadata{
			Name: "app" + strconv.Itoa(i+1),
		}, types.AppSpecV3{
			URI: "localhost",
		})
		require.NoError(t, err)

		require.NoError(t, service.CreateApp(ctx, app))
		expected = append(expected, app)
	}
	slices.SortFunc(expected, func(a, b types.Application) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})

	out, err = service.GetApps(ctx)
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, gocmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	page1, page2Start, err := service.ListApps(ctx, 0, "")
	require.NoError(t, err)
	assert.Len(t, page1, 1000)
	require.NotEmpty(t, page2Start)

	page2, next, err := service.ListApps(ctx, 1000, page2Start)
	require.NoError(t, err)
	assert.Len(t, page2, len(expected)-1000)
	require.Empty(t, next)

	listed := append(page1, page2...)
	assert.Empty(t, gocmp.Diff(expected, listed,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	iterOut = nil
	for app, err := range service.Apps(ctx, "", page2Start) {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	assert.Len(t, iterOut, len(page1))
	assert.Empty(t, gocmp.Diff(page1, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	iterOut = nil
	for app, err := range service.Apps(ctx, "", "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}
	assert.Len(t, iterOut, len(expected))
	assert.Empty(t, gocmp.Diff(expected, iterOut,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	iterOut = nil
	for app, err := range service.Apps(ctx, page2Start, "") {
		require.NoError(t, err)
		iterOut = append(iterOut, app)
	}

	assert.Len(t, iterOut, len(expected)-1000)
	assert.Empty(t, gocmp.Diff(expected, append(page1, iterOut...),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
}

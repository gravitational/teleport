/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func newDynamicDesktop(t *testing.T, name string) types.DynamicWindowsDesktop {
	desktop, err := types.NewDynamicWindowsDesktopV1(name, nil, types.DynamicWindowsDesktopSpecV1{
		Addr: "xyz",
	})
	require.NoError(t, err)
	return desktop
}

func setupDynamicDesktopTest(t *testing.T) (context.Context, *DynamicWindowsDesktopService) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewDynamicWindowsDesktopService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return ctx, service
}

func TestDynamicWindowsService_CreateDynamicDesktop(t *testing.T) {
	t.Parallel()
	ctx, service := setupDynamicDesktopTest(t)
	t.Run("ok", func(t *testing.T) {
		want := newDynamicDesktop(t, "example")
		got, err := service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		want.SetRevision(got.GetRevision())
		require.NoError(t, err)
		require.NotEmpty(t, got.GetRevision())
		require.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
		))
	})
	t.Run("no upsert", func(t *testing.T) {
		want := newDynamicDesktop(t, "upsert")
		_, err := service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		require.NoError(t, err)
		_, err = service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestDynamicWindowsService_UpsertDynamicDesktop(t *testing.T) {
	ctx, service := setupDynamicDesktopTest(t)
	want := newDynamicDesktop(t, "example")
	got, err := service.UpsertDynamicWindowsDesktop(ctx, want.Copy())
	want.SetRevision(got.GetRevision())
	require.NoError(t, err)
	require.NotEmpty(t, got.GetRevision())
	require.Empty(t, cmp.Diff(
		want,
		got,
		protocmp.Transform(),
	))
	_, err = service.UpsertDynamicWindowsDesktop(ctx, want.Copy())
	require.NoError(t, err)
}

func TestDynamicWindowsService_GetDynamicDesktop(t *testing.T) {
	t.Parallel()
	ctx, service := setupDynamicDesktopTest(t)
	t.Run("not found", func(t *testing.T) {
		_, err := service.GetDynamicWindowsDesktop(ctx, "notfound")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("ok", func(t *testing.T) {
		want := newDynamicDesktop(t, "example")
		created, err := service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		require.NoError(t, err)
		got, err := service.GetDynamicWindowsDesktop(ctx, "example")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(
			created,
			got,
			protocmp.Transform(),
		))
	})
}

func TestDynamicWindowsService_ListDynamicDesktop(t *testing.T) {
	t.Parallel()
	t.Run("none", func(t *testing.T) {
		ctx, service := setupDynamicDesktopTest(t)
		desktops, _, err := service.ListDynamicWindowsDesktops(ctx, 5, "")
		require.NoError(t, err)
		require.Empty(t, desktops)
	})
	t.Run("list all", func(t *testing.T) {
		ctx, service := setupDynamicDesktopTest(t)
		d1, err := service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d1"))
		require.NoError(t, err)
		d2, err := service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d2"))
		require.NoError(t, err)
		desktops, next, err := service.ListDynamicWindowsDesktops(ctx, 5, "")
		require.NoError(t, err)
		require.Len(t, desktops, 2)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(
			d1,
			desktops[0],
			protocmp.Transform(),
		))
		require.Empty(t, cmp.Diff(
			d2,
			desktops[1],
			protocmp.Transform(),
		))
	})
	t.Run("list paged", func(t *testing.T) {
		ctx, service := setupDynamicDesktopTest(t)
		d1, err := service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d1"))
		require.NoError(t, err)
		d2, err := service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d2"))
		require.NoError(t, err)
		desktops, next, err := service.ListDynamicWindowsDesktops(ctx, 1, "")
		require.NoError(t, err)
		require.Len(t, desktops, 1)
		require.NotEmpty(t, next)
		require.Empty(t, cmp.Diff(
			d1,
			desktops[0],
			protocmp.Transform(),
		))
		desktops, next, err = service.ListDynamicWindowsDesktops(ctx, 1, next)
		require.NoError(t, err)
		require.Len(t, desktops, 1)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(
			d2,
			desktops[0],
			protocmp.Transform(),
		))
	})
}

func TestDynamicWindowsService_UpdateDynamicDesktop(t *testing.T) {
	t.Parallel()
	ctx, service := setupDynamicDesktopTest(t)
	t.Run("not found", func(t *testing.T) {
		want := newDynamicDesktop(t, "example")
		_, err := service.UpdateDynamicWindowsDesktop(ctx, want.Copy())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("revision doesn't match", func(t *testing.T) {
		want := newDynamicDesktop(t, "example1")
		_, err := service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		require.NoError(t, err)
		_, err = service.UpdateDynamicWindowsDesktop(ctx, want)
		require.Error(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		want := newDynamicDesktop(t, "example2")
		created, err := service.CreateDynamicWindowsDesktop(ctx, want.Copy())
		require.NoError(t, err)
		updated, err := service.UpdateDynamicWindowsDesktop(ctx, created.Copy())
		require.NoError(t, err)
		require.NotEqual(t, created.GetRevision(), updated.GetRevision())
	})
}

func TestDynamicWindowsService_DeleteDynamicDesktop(t *testing.T) {
	t.Parallel()
	ctx, service := setupDynamicDesktopTest(t)
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteDynamicWindowsDesktop(ctx, "notfound")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "example"))
		require.NoError(t, err)
		_, err = service.GetDynamicWindowsDesktop(ctx, "example")
		require.NoError(t, err)
		err = service.DeleteDynamicWindowsDesktop(ctx, "example")
		require.NoError(t, err)
		_, err = service.GetDynamicWindowsDesktop(ctx, "example")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestDynamicWindowsService_DeleteAllDynamicDesktop(t *testing.T) {
	ctx, service := setupDynamicDesktopTest(t)
	err := service.DeleteAllDynamicWindowsDesktops(ctx)
	require.NoError(t, err)
	_, err = service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d1"))
	require.NoError(t, err)
	_, err = service.CreateDynamicWindowsDesktop(ctx, newDynamicDesktop(t, "d2"))
	require.NoError(t, err)
	desktops, _, err := service.ListDynamicWindowsDesktops(ctx, 5, "")
	require.NoError(t, err)
	require.Len(t, desktops, 2)
	err = service.DeleteAllDynamicWindowsDesktops(ctx)
	require.NoError(t, err)
	desktops, _, err = service.ListDynamicWindowsDesktops(ctx, 5, "")
	require.NoError(t, err)
	require.Empty(t, desktops)
}

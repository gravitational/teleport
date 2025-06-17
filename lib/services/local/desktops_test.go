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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestGetWindowsDesktops(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewWindowsDesktopService(bk)

	d1, err := types.NewWindowsDesktopV3("apple", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id-1"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d1))

	d2, err := types.NewWindowsDesktopV3("apple", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id-2"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d2))

	d3, err := types.NewWindowsDesktopV3("carrot", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id-2"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d3))

	// search by name and host ID
	result, err := service.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: "apple", HostID: "test-host-id-2"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, d1.GetName(), result[0].GetName())

	// search by host ID
	result, err = service.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{HostID: "test-host-id-2"})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, d2.GetName(), result[0].GetName())
	require.Equal(t, d3.GetName(), result[1].GetName())

	// no filter
	result, err = service.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	require.NoError(t, err)
	require.Len(t, result, 3)

	// non-matching filter
	result, err = service.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: "foo", HostID: "bar"})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Empty(t, result)
}

func TestListWindowsDesktops(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewWindowsDesktopService(bk)

	// Initially we expect no desktops.
	out, err := service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 10,
	})
	require.NoError(t, err)
	require.Empty(t, out.Desktops)
	require.Empty(t, out.NextKey)

	// Upsert some windows desktops.

	// With label.
	testLabel := map[string]string{"env": "test"}
	d1, err := types.NewWindowsDesktopV3("apple", testLabel, types.WindowsDesktopSpecV3{Addr: "_", HostID: "hostA"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d1))

	d2, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_", HostID: "hostA"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d2))

	// Without labels.
	d3, err := types.NewWindowsDesktopV3("carrot", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d3))

	// Test fetch by host ID
	out, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 10,
		WindowsDesktopFilter: types.WindowsDesktopFilter{
			HostID: "test-host-id",
		},
	})
	require.NoError(t, err)
	require.Len(t, out.Desktops, 1)
	require.Equal(t, "carrot", out.Desktops[0].GetName())
	require.Equal(t, "test-host-id", out.Desktops[0].GetHostID())

	// Test fetch all.
	out, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 10,
	})
	require.NoError(t, err)
	require.Empty(t, out.NextKey)
	require.Empty(t, cmp.Diff([]types.WindowsDesktop{d1, d2, d3}, out.Desktops,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Test pagination.

	// First fetch.
	resp, err := service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[0], resp.Desktops[0])
	require.Equal(t, backend.GetPaginationKey(out.Desktops[1]), resp.NextKey)

	// Middle fetch.
	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit:    1,
		StartKey: resp.NextKey,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[1], resp.Desktops[0])
	require.Equal(t, backend.GetPaginationKey(out.Desktops[2]), resp.NextKey)

	// Last fetch.
	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit:    1,
		StartKey: resp.NextKey,
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, out.Desktops[2], resp.Desktops[0])
	require.Empty(t, resp.NextKey)

	// Test paginating while filtering by Host ID

	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit: 1,
		WindowsDesktopFilter: types.WindowsDesktopFilter{
			HostID: "hostA",
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, "apple", resp.Desktops[0].GetName())

	resp, err = service.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		Limit:    1,
		StartKey: resp.NextKey,
		WindowsDesktopFilter: types.WindowsDesktopFilter{
			HostID: "hostA",
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Desktops, 1)
	require.Equal(t, "banana", resp.Desktops[0].GetName())
	require.Empty(t, resp.NextKey)
}

func TestListWindowsDesktops_Filters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewWindowsDesktopService(bk)

	// Upsert some windows desktops.

	// With label.
	testLabel := map[string]string{"env": "test"}
	d1, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d1))

	d2, err := types.NewWindowsDesktopV3("banana", testLabel, types.WindowsDesktopSpecV3{Addr: "_"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d2))

	// Without labels.
	d3, err := types.NewWindowsDesktopV3("carrot", nil, types.WindowsDesktopSpecV3{Addr: "_", HostID: "test-host-id"})
	require.NoError(t, err)
	require.NoError(t, service.CreateWindowsDesktop(ctx, d3))

	tests := []struct {
		name    string
		filter  types.ListWindowsDesktopsRequest
		assert  require.ErrorAssertionFunc
		wantLen int
	}{
		{
			name: "NOK non-matching host id and name",
			filter: types.ListWindowsDesktopsRequest{
				Limit: 10,
				WindowsDesktopFilter: types.WindowsDesktopFilter{
					HostID: "no-match",
					Name:   "no-match",
				},
			},
			assert:  require.NoError,
			wantLen: 0,
		},
		{
			name:    "NOK invalid limit",
			filter:  types.ListWindowsDesktopsRequest{},
			assert:  require.Error,
			wantLen: 0,
		},
		{
			name: "matching host id",
			filter: types.ListWindowsDesktopsRequest{
				Limit:                5,
				WindowsDesktopFilter: types.WindowsDesktopFilter{HostID: "test-host-id"},
			},
			assert:  require.NoError,
			wantLen: 2,
		},
		{
			name: "matching host id, mismatching labels",
			filter: types.ListWindowsDesktopsRequest{
				Limit:                5,
				Labels:               map[string]string{"doesnot": "exist"},
				WindowsDesktopFilter: types.WindowsDesktopFilter{HostID: "test-host-id"},
			},
			assert:  require.NoError,
			wantLen: 0,
		},
		{
			name: "matching name",
			filter: types.ListWindowsDesktopsRequest{
				Limit:                5,
				WindowsDesktopFilter: types.WindowsDesktopFilter{Name: "banana"},
			},
			assert:  require.NoError,
			wantLen: 2,
		},
		{
			name: "with search",
			filter: types.ListWindowsDesktopsRequest{
				Limit:          5,
				SearchKeywords: []string{"env", "test"},
			},
			assert:  require.NoError,
			wantLen: 2,
		},
		{
			name: "with labels",
			filter: types.ListWindowsDesktopsRequest{
				Limit:  5,
				Labels: testLabel,
			},
			assert:  require.NoError,
			wantLen: 2,
		},
		{
			name: "with predicate",
			filter: types.ListWindowsDesktopsRequest{
				Limit:               5,
				PredicateExpression: `labels.env == "test"`,
			},
			assert:  require.NoError,
			wantLen: 2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := service.ListWindowsDesktops(ctx, tc.filter)
			tc.assert(t, err)

			if resp != nil {
				require.Len(t, resp.Desktops, tc.wantLen)
			}
		})
	}
}

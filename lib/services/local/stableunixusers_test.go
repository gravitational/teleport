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

package local

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestStableUNIXUsersBasic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Component: "TestStableUNIXUsersBasic",
	})
	require.NoError(t, err)
	defer bk.Close()

	svc := &StableUNIXUsersService{
		Backend: bk,
	}

	_, err = svc.GetUIDForUsername(ctx, "notfound")
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	const baseUID int32 = 2000

	acts, err := svc.AppendCreateStableUNIXUser(nil, "found", baseUID)
	require.NoError(t, err)
	_, err = bk.AtomicWrite(ctx, acts)
	require.NoError(t, err)

	acts, err = svc.AppendCreateStableUNIXUser(nil, "found", baseUID)
	require.NoError(t, err)
	_, err = bk.AtomicWrite(ctx, acts)
	require.ErrorIs(t, err, backend.ErrConditionFailed)

	uid, err := svc.GetUIDForUsername(ctx, "found")
	require.NoError(t, err)
	require.Equal(t, baseUID, uid)

	uid, ok, err := svc.SearchFreeUID(ctx, baseUID, baseUID+100)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, baseUID+1, uid)

	for i := range 2 * defaults.DefaultChunkSize {
		acts, err := svc.AppendCreateStableUNIXUser(nil, fmt.Sprintf("user%05d", i), baseUID+1+int32(i))
		require.NoError(t, err)
		_, err = bk.AtomicWrite(ctx, acts)
		require.NoError(t, err)
	}

	_, ok, err = svc.SearchFreeUID(ctx, baseUID, baseUID+defaults.DefaultChunkSize)
	require.NoError(t, err)
	require.False(t, ok)

	uid, ok, err = svc.SearchFreeUID(ctx, baseUID, baseUID+3*defaults.DefaultChunkSize)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, baseUID+1+2*defaults.DefaultChunkSize, uid)

	// TODO(espadolini): remove or adjust this if SearchFreeUID ends up
	// searching more than the final part of the range
	uid, ok, err = svc.SearchFreeUID(ctx, baseUID-100, baseUID+3*defaults.DefaultChunkSize)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, baseUID+1+2*defaults.DefaultChunkSize, uid)

	resp, nextPageToken, err := svc.ListStableUNIXUsers(ctx, 0, "")
	require.NoError(t, err)
	// "found" followed by "user00000", "user00001", ..., "user00999"
	require.Equal(t, fmt.Sprintf("user%05d", defaults.DefaultChunkSize-1), nextPageToken)
	require.Len(t, resp, defaults.DefaultChunkSize)

	resp, _, err = svc.ListStableUNIXUsers(ctx, 0, nextPageToken)
	require.NoError(t, err)
	require.Len(t, resp, defaults.DefaultChunkSize)
	require.Equal(t, fmt.Sprintf("user%05d", defaults.DefaultChunkSize-1), resp[0].Username)
	require.Equal(t, baseUID+defaults.DefaultChunkSize, resp[0].UID)
}

func TestStableUNIXUsersUIDOrder(t *testing.T) {
	t.Parallel()

	nums := make([]int32, 0, 1000)
	for range 1000 {
		nums = append(nums, int32(rand.Uint32()))
	}
	slices.Sort(nums)

	keys := make([]string, 0, len(nums))
	for _, n := range slices.Backward(nums) {
		keys = append(keys, (*StableUNIXUsersService).uidToKey(nil, n).String())
	}

	require.True(t, slices.IsSorted(keys), "uidToKey didn't satisfy key order")
}

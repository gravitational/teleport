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

package stableunixusers

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	stableunixusersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/stableunixusers/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
)

func TestStableUNIXUsers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)
	defer bk.Close()

	clusterConfiguration := &local.ClusterConfigurationService{
		Backend: bk,
	}

	readOnlyCache, err := readonly.NewCache(readonly.CacheConfig{
		Upstream:    clusterConfiguration,
		ReloadOnErr: true,
	})
	require.NoError(t, err)

	svc := &server{
		authorizer: nil,

		backend:       bk,
		readOnlyCache: readOnlyCache,

		stableUNIXUsers: &local.StableUNIXUsersService{
			Backend: bk,
		},
		clusterConfiguration: clusterConfiguration,

		writerSem: make(chan struct{}, 1),
	}

	const firstUID int32 = 90000
	const lastUID int32 = 90003

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		StableUnixUserConfig: &types.StableUNIXUserConfig{
			Enabled:  true,
			FirstUid: firstUID,
			LastUid:  lastUID,
		},
	})
	require.NoError(t, err)
	_, err = clusterConfiguration.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	uid1, err := svc.obtainUIDForUsernameUncached(ctx, "user1")
	require.NoError(t, err)
	require.Equal(t, firstUID, uid1)

	uid1, err = svc.obtainUIDForUsernameUncached(ctx, "user1")
	require.NoError(t, err)
	require.Equal(t, firstUID, uid1)

	uid2, err := svc.obtainUIDForUsernameUncached(ctx, "user2")
	require.NoError(t, err)
	require.Equal(t, firstUID+1, uid2)

	uid1, err = svc.obtainUIDForUsernameUncached(ctx, "user1")
	require.NoError(t, err)
	require.Equal(t, firstUID, uid1)

	uid2, err = svc.obtainUIDForUsernameUncached(ctx, "user2")
	require.NoError(t, err)
	require.Equal(t, firstUID+1, uid2)

	uid3, err := svc.obtainUIDForUsernameUncached(ctx, "user3")
	require.NoError(t, err)
	require.Equal(t, firstUID+2, uid3)

	uid4, err := svc.obtainUIDForUsernameUncached(ctx, "user4")
	require.NoError(t, err)
	require.Equal(t, firstUID+3, uid4)

	_, err = svc.obtainUIDForUsernameUncached(ctx, "user5")
	require.ErrorAs(t, err, new(*trace.LimitExceededError))

	resp, err := svc.listStableUNIXUsers(ctx, &stableunixusersv1.ListStableUNIXUsersRequest{
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "user3", resp.GetNextPageToken())
	require.Len(t, resp.GetStableUnixUsers(), 2)

	require.Equal(t, "user1", resp.GetStableUnixUsers()[0].GetUsername())
	require.Equal(t, firstUID, resp.GetStableUnixUsers()[0].GetUid())
	require.Equal(t, "user2", resp.GetStableUnixUsers()[1].GetUsername())
	require.Equal(t, firstUID+1, resp.GetStableUnixUsers()[1].GetUid())

	resp, err = svc.listStableUNIXUsers(ctx, &stableunixusersv1.ListStableUNIXUsersRequest{
		PageSize:  2,
		PageToken: "user3",
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetNextPageToken())
	require.Len(t, resp.GetStableUnixUsers(), 2)

	require.Equal(t, "user3", resp.GetStableUnixUsers()[0].GetUsername())
	require.Equal(t, firstUID+2, resp.GetStableUnixUsers()[0].GetUid())
	require.Equal(t, "user4", resp.GetStableUnixUsers()[1].GetUsername())
	require.Equal(t, firstUID+3, resp.GetStableUnixUsers()[1].GetUid())

	authPref, err = types.NewAuthPreference(types.AuthPreferenceSpecV2{
		StableUnixUserConfig: &types.StableUNIXUserConfig{
			Enabled:  true,
			FirstUid: firstUID,
			LastUid:  firstUID + 2000,
		},
	})
	require.NoError(t, err)
	_, err = clusterConfiguration.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	readOnlyCache, err = readonly.NewCache(readonly.CacheConfig{
		Upstream:    clusterConfiguration,
		ReloadOnErr: true,
	})
	require.NoError(t, err)
	svc.readOnlyCache = readOnlyCache

	eg, ctx := errgroup.WithContext(ctx)
	for i := range 1000 {
		eg.Go(func() error {
			_, err := svc.obtainUIDForUsernameUncached(ctx, fmt.Sprintf("parallel%05d", i))
			return err
		})
	}
	require.NoError(t, eg.Wait())
}

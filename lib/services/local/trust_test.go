/*
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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
)

func TestRemoteClusterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)
	clock := clockwork.NewFakeClockAt(time.Now())

	originalLabels := map[string]string{
		"a": "b",
		"c": "d",
	}

	rc, err := types.NewRemoteCluster("foo")
	require.NoError(t, err)
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	rc.SetLastHeartbeat(clock.Now())
	rc.SetMetadata(types.Metadata{
		Name:   "foo",
		Labels: originalLabels,
	})

	src, err := types.NewRemoteCluster("bar")
	require.NoError(t, err)
	src.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	src.SetLastHeartbeat(clock.Now().Add(-time.Hour))

	// create remote clusters
	err = trustService.CreateRemoteCluster(rc)
	require.NoError(t, err)
	err = trustService.CreateRemoteCluster(src)
	require.NoError(t, err)

	// get remote cluster make sure it's correct
	gotRC, err := trustService.GetRemoteCluster("foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, originalLabels, gotRC.GetMetadata().Labels)

	updatedLabels := map[string]string{
		"e": "f",
		"g": "h",
	}

	// update remote clusters
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	rc.SetLastHeartbeat(clock.Now().Add(time.Hour))
	rc.SetMetadata(types.Metadata{
		Name:   "foo",
		Labels: updatedLabels,
	})
	err = trustService.UpdateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	src.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	src.SetLastHeartbeat(clock.Now())
	err = trustService.UpdateRemoteCluster(ctx, src)
	require.NoError(t, err)

	// get remote cluster make sure it's correct
	gotRC, err = trustService.GetRemoteCluster("foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOnline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Add(time.Hour).Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, updatedLabels, gotRC.GetMetadata().Labels)

	gotRC, err = trustService.GetRemoteCluster("bar")
	require.NoError(t, err)
	require.Equal(t, "bar", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())

	// get all clusters
	allRC, err := trustService.GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, allRC, 2)

	// delete cluster
	err = trustService.DeleteRemoteCluster(ctx, "foo")
	require.NoError(t, err)

	// make sure it's really gone
	err = trustService.DeleteRemoteCluster(ctx, "foo")
	require.Error(t, err)
	require.ErrorIs(t, err, trace.NotFound("key /remoteClusters/foo is not found"))
}

func TestTrustedClusterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	tc, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	require.NoError(t, err)

	// we just insert this one for get all
	stc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	require.NoError(t, err)

	// create trusted clusters
	_, err = trustService.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)
	_, err = trustService.UpsertTrustedCluster(ctx, stc)
	require.NoError(t, err)

	// get trusted cluster make sure it's correct
	gotTC, err := trustService.GetTrustedCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotTC.GetName())
	require.True(t, gotTC.GetEnabled())
	require.EqualValues(t, []string{"bar", "baz"}, gotTC.GetRoles())
	require.Equal(t, "qux", gotTC.GetToken())
	require.Equal(t, "quux", gotTC.GetProxyAddress())
	require.Equal(t, "quuz", gotTC.GetReverseTunnelAddress())

	// get all clusters
	allTC, err := trustService.GetTrustedClusters(ctx)
	require.NoError(t, err)
	require.Len(t, allTC, 2)

	// delete cluster
	err = trustService.DeleteTrustedCluster(ctx, "foo")
	require.NoError(t, err)

	// make sure it's really gone
	_, err = trustService.GetTrustedCluster(ctx, "foo")
	require.Error(t, err)
	require.ErrorIs(t, err, trace.NotFound("key /trustedclusters/foo is not found"))
}

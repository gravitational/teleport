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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestRemoteClusterCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
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
	gotRC, err := trustService.CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	gotSRC, err := trustService.CreateRemoteCluster(ctx, src)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(src, gotSRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// get remote cluster make sure it's correct
	gotRC, err = trustService.GetRemoteCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, originalLabels, gotRC.GetMetadata().Labels)

	rc = gotRC
	updatedLabels := map[string]string{
		"e": "f",
		"g": "h",
	}

	// update remote clusters
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
	rc.SetLastHeartbeat(clock.Now().Add(time.Hour))
	meta := rc.GetMetadata()
	meta.Labels = updatedLabels
	rc.SetMetadata(meta)
	gotRC, err = trustService.UpdateRemoteCluster(ctx, rc)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rc, gotRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	src = gotSRC
	src.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	src.SetLastHeartbeat(clock.Now())
	gotSRC, err = trustService.UpdateRemoteCluster(ctx, src)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(src, gotSRC, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// get remote cluster make sure it's correct
	gotRC, err = trustService.GetRemoteCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOnline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Add(time.Hour).Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())
	require.Equal(t, updatedLabels, gotRC.GetMetadata().Labels)

	gotRC, err = trustService.GetRemoteCluster(ctx, "bar")
	require.NoError(t, err)
	require.Equal(t, "bar", gotRC.GetName())
	require.Equal(t, teleport.RemoteClusterStatusOffline, gotRC.GetConnectionStatus())
	require.Equal(t, clock.Now().Nanosecond(), gotRC.GetLastHeartbeat().Nanosecond())

	// get all clusters
	allRC, err := trustService.GetRemoteClusters(ctx)
	require.NoError(t, err)
	require.Len(t, allRC, 2)

	// delete cluster
	err = trustService.DeleteRemoteCluster(ctx, "foo")
	require.NoError(t, err)

	// make sure it's really gone
	err = trustService.DeleteRemoteCluster(ctx, "foo")
	require.Error(t, err)
	require.ErrorIs(t, err, trace.NotFound("key \"/remoteClusters/foo\" is not found"))
}

func TestPresenceService_PatchRemoteCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	rc, err := types.NewRemoteCluster("bar")
	require.NoError(t, err)
	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
	_, err = trustService.CreateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	updatedRC, err := trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			require.Equal(t, teleport.RemoteClusterStatusOffline, rc.GetConnectionStatus())
			rc.SetConnectionStatus(teleport.RemoteClusterStatusOnline)
			return rc, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, teleport.RemoteClusterStatusOnline, updatedRC.GetConnectionStatus())

	// Ensure this was persisted.
	fetchedRC, err := trustService.GetRemoteCluster(ctx, rc.GetName())
	require.NoError(t, err)
	require.Equal(t, teleport.RemoteClusterStatusOnline, fetchedRC.GetConnectionStatus())
	// Ensure other fields unchanged
	require.Empty(t,
		cmp.Diff(
			rc,
			fetchedRC,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
			cmpopts.IgnoreFields(types.RemoteClusterStatusV3{}, "Connection"),
		),
	)

	// Ensure that name cannot be updated
	_, err = trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			rc.SetName("baz")
			return rc, nil
		},
	)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "metadata.name: cannot be patched")

	// Ensure that revision cannot be updated
	_, err = trustService.PatchRemoteCluster(
		ctx,
		rc.GetName(),
		func(rc types.RemoteCluster) (types.RemoteCluster, error) {
			rc.SetRevision("baz")
			return rc, nil
		},
	)
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.Contains(t, err.Error(), "metadata.revision: cannot be patched")
}

func TestPresenceService_ListRemoteClusters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	trustService := NewCAService(bk)

	// With no resources, we should not get an error but we should get an empty
	// token and an empty slice.
	rcs, pageToken, err := trustService.ListRemoteClusters(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Empty(t, rcs)

	// Create a few remote clusters
	for i := 0; i < 10; i++ {
		rc, err := types.NewRemoteCluster(fmt.Sprintf("rc-%d", i))
		require.NoError(t, err)
		_, err = trustService.CreateRemoteCluster(ctx, rc)
		require.NoError(t, err)
	}

	// Check limit behaves
	rcs, pageToken, err = trustService.ListRemoteClusters(ctx, 1, "")
	require.NoError(t, err)
	require.NotEmpty(t, pageToken)
	require.Len(t, rcs, 1)

	// Iterate through all pages with a low limit to ensure that pageToken
	// behaves correctly.
	rcs = []types.RemoteCluster{}
	pageToken = ""
	for i := 0; i < 10; i++ {
		var got []types.RemoteCluster
		got, pageToken, err = trustService.ListRemoteClusters(ctx, 1, pageToken)
		require.NoError(t, err)
		if i == 9 {
			// For the final page, we should not get a page token
			require.Empty(t, pageToken)
		} else {
			require.NotEmpty(t, pageToken)
		}
		require.Len(t, got, 1)
		rcs = append(rcs, got...)
	}
	require.Len(t, rcs, 10)

	// Check that with a higher limit, we get all resources
	rcs, pageToken, err = trustService.ListRemoteClusters(ctx, 20, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Len(t, rcs, 10)
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

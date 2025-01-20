/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func mustMakeGitHubServer(t *testing.T, org string) types.Server {
	t.Helper()
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Organization: org,
		Integration:  org,
	})
	require.NoError(t, err)
	return server
}

func compareGitServers(t *testing.T, listA, listB []types.Server) {
	t.Helper()
	sortedListA := services.SortedServers(listA)
	sortedListB := services.SortedServers(listB)
	sort.Sort(sortedListA)
	sort.Sort(sortedListB)

	require.Empty(t, cmp.Diff(sortedListA, sortedListB,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
}

func TestGitServerCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	server1 := mustMakeGitHubServer(t, "org1")
	server2 := mustMakeGitHubServer(t, "org2")
	service, err := NewGitServerService(backend)
	require.NoError(t, err)

	t.Run("nothing yet", func(t *testing.T) {
		out, _, err := service.ListGitServers(ctx, 10, "")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("invalid server", func(t *testing.T) {
		node, err := types.NewServerWithLabels("node1", types.KindNode, types.ServerSpecV2{}, nil)
		require.NoError(t, err)
		_, err = service.CreateGitServer(ctx, node)
		require.Error(t, err)
	})

	t.Run("create", func(t *testing.T) {
		_, err = service.CreateGitServer(ctx, server1)
		require.NoError(t, err)
		_, err = service.CreateGitServer(ctx, server2)
		require.NoError(t, err)
	})

	t.Run("list", func(t *testing.T) {
		out, _, err := service.ListGitServers(ctx, 10, "")
		require.NoError(t, err)
		compareGitServers(t, []types.Server{server1, server2}, out)
	})

	t.Run("list with token", func(t *testing.T) {
		out, token, err := service.ListGitServers(ctx, 1, "")
		require.NoError(t, err)
		require.NotEmpty(t, token)
		require.Len(t, out, 1)
		out2, token, err := service.ListGitServers(ctx, 1, token)
		require.NoError(t, err)
		require.Empty(t, token)
		require.Len(t, out2, 1)
		compareGitServers(t, []types.Server{server1, server2}, append(out, out2...))
	})

	t.Run("get and update", func(t *testing.T) {
		out, err := service.GetGitServer(ctx, server1.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(server1, out,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))

		out.GetGitHub().Integration = "updated"
		out, err = service.UpdateGitServer(ctx, out)
		require.NoError(t, err)
		require.Equal(t, "updated", out.GetGitHub().Integration)
	})

	t.Run("delete not found", func(t *testing.T) {
		err := service.DeleteGitServer(ctx, "doesnotexist")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, service.DeleteGitServer(ctx, server1.GetName()))
	})

	t.Run("upsert", func(t *testing.T) {
		server3 := mustMakeGitHubServer(t, "org3")
		_, err := service.UpsertGitServer(ctx, server3)
		require.NoError(t, err)

		out, _, err := service.ListGitServers(ctx, 10, "")
		require.NoError(t, err)
		compareGitServers(t, []types.Server{server2, server3}, out)
	})

	t.Run("delete all", func(t *testing.T) {
		require.NoError(t, service.DeleteAllGitServers(ctx))
		out, _, err := service.ListGitServers(ctx, 10, "")
		require.NoError(t, err)
		require.Empty(t, out)
	})
}

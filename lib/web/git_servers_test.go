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

package web

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	libui "github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestGitServers(t *testing.T) {
	t.Parallel()
	wPack := newWebPack(t, 1 /* proxies */)
	proxy := wPack.proxies[0]
	authPack := proxy.authPack(t, "user", []types.Role{services.NewPresetEditorRole()})
	ctx := context.Background()

	orgName := "my-org"
	integrationName := "github-" + orgName
	gitServerName := integrationName

	t.Run("create", func(t *testing.T) {
		endpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "gitservers")

		t.Run("missing github spec", func(t *testing.T) {
			_, err := authPack.clt.PutJSON(ctx, endpoint, ui.CreateGitServerRequest{
				Name:    gitServerName,
				SubKind: types.SubKindGitHub,
			})
			require.Error(t, err)
		})
		t.Run("success", func(t *testing.T) {
			createResp, err := authPack.clt.PutJSON(ctx, endpoint, ui.CreateGitServerRequest{
				Name:    gitServerName,
				SubKind: types.SubKindGitHub,
				GitHub: &ui.GitHubServerMetadata{
					Integration:  integrationName,
					Organization: "old-org-before-overwrite",
				},
			})
			require.NoError(t, err)
			require.Equal(t, 200, createResp.Code())
		})
		t.Run("already exists", func(t *testing.T) {
			_, err := authPack.clt.PutJSON(ctx, endpoint, ui.CreateGitServerRequest{
				Name:    gitServerName,
				SubKind: types.SubKindGitHub,
				GitHub: &ui.GitHubServerMetadata{
					Integration:  integrationName,
					Organization: orgName,
				},
			})
			require.Error(t, err)
			require.True(t, trace.IsAlreadyExists(err))
		})
		t.Run("overwrite", func(t *testing.T) {
			createResp, err := authPack.clt.PutJSON(ctx, endpoint, ui.CreateGitServerRequest{
				Name:    gitServerName,
				SubKind: types.SubKindGitHub,
				GitHub: &ui.GitHubServerMetadata{
					Integration:  integrationName,
					Organization: orgName,
				},
				Overwrite: true,
			})
			require.NoError(t, err)
			require.Equal(t, 200, createResp.Code())
		})
	})

	t.Run("get", func(t *testing.T) {
		endpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "gitservers", gitServerName)

		t.Run("no access", func(t *testing.T) {
			// Default editor role does not have permissions to any GitHub
			// organizations.
			_, err := authPack.clt.Get(ctx, endpoint, nil)
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})

		t.Run("success", func(t *testing.T) {
			gitServerAccessRole, err := types.NewRole("git-server-access", types.RoleSpecV6{
				Allow: types.RoleConditions{
					GitHubPermissions: []types.GitHubPermission{{
						Organizations: []string{orgName},
					}},
				},
			})
			require.NoError(t, err)
			authPack := proxy.authPack(t, "user-access", []types.Role{gitServerAccessRole})

			getResp, err := authPack.clt.Get(ctx, endpoint, nil)
			require.NoError(t, err)
			require.Equal(t, 200, getResp.Code())

			var resp ui.GitServer
			require.NoError(t, json.Unmarshal(getResp.Bytes(), &resp))
			require.Equal(t, ui.GitServer{
				Kind:        types.KindGitServer,
				SubKind:     types.SubKindGitHub,
				Name:        gitServerName,
				ClusterName: "localhost",
				Hostname:    "my-org.teleport-github-org",
				Addr:        "github.com:22",
				GitHub: &ui.GitHubServerMetadata{
					Integration:  integrationName,
					Organization: orgName,
				},
				Labels: []libui.Label{},
			}, resp)
		})
	})

	t.Run("delete", func(t *testing.T) {
		endpoint := authPack.clt.Endpoint("webapi", "sites", wPack.server.ClusterName(), "gitservers", gitServerName)
		t.Run("success", func(t *testing.T) {
			_, err := authPack.clt.Delete(ctx, endpoint)
			require.NoError(t, err)

			_, err = authPack.clt.Get(ctx, endpoint, nil)
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})

		t.Run("not found", func(t *testing.T) {
			_, err := authPack.clt.Delete(ctx, endpoint)
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})
	})
}

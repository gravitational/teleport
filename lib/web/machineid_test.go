// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBotsPagination(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	// create bots to test pagination against
	n := 1
	for n < 25 {
		n += 1
		_, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
			BotName: "test-bot-" + strconv.Itoa(n),
			Roles:   []string{""},
		})
		require.NoError(t, err)
	}

	tt := []struct {
		name                string
		pageSize            string
		expectedSize        int
		pageTwoExpectedSize int
	}{
		{
			name:                "defaults to 20",
			pageSize:            "0",
			expectedSize:        20,
			pageTwoExpectedSize: 5, // there are only 25 bots created
		},
		{
			name:                "set size",
			pageSize:            "5",
			expectedSize:        5,
			pageTwoExpectedSize: 5,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			page1, err := pack.clt.Get(ctx, endpoint, url.Values{
				"page_token": []string{""}, // default to the start
				"page_size":  []string{tc.pageSize},
			})
			require.NoError(t, err)

			var page1Bots ListBotsResponse
			require.NoError(t, json.Unmarshal(page1.Bytes(), &page1Bots), "invalid response received")
			assert.Equal(t, http.StatusOK, page1.Code(), "unexpected status code getting connectors")

			// todo (mberg) re-evaluate assertions once we agree on a pagination approach
			assert.True(t, len(page1Bots.Items) > 0)
			//assert.Equal(t, tc.expectedSize, len(page1Bots.Items))
			//assert.Equal(t, "test-bot-0", page1Bots.Items[0].Status.UserName)
			//assert.Equal(t, "test-bot-"+tc.pageSize, page1Bots.StartKey)
		})
	}
}

func TestListBots_UnauthenticatedError(t *testing.T) {
	ctx := context.Background()
	s := newWebSuite(t)
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	publicClt := s.client(t)
	_, err := publicClt.Get(ctx, endpoint, url.Values{
		"page_token": []string{""},
		"page_size":  []string{""},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestCreateBot(t *testing.T) {
	s := newWebSuite(t)
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})

	clusterName := env.server.ClusterName()

	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	ctx := context.Background()

	resp, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: "test-bot",
		Roles:   []string{"bot-role-0", "bot-role-1"},
	})
	require.NoError(t, err)

	var ret struct {
		Message string `json:"message"`
	}
	err = json.Unmarshal(resp.Bytes(), &ret)
	require.NoError(t, err)
	require.Equal(t, "ok", ret.Message)

	// fetch users and assert that the bot we created exists
	getUsersResp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "users"), nil)
	require.NoError(t, err)
	var users []ui.UserListEntry
	json.Unmarshal(getUsersResp.Bytes(), &users)

	found := slices.ContainsFunc(users, func(user ui.UserListEntry) bool {
		// bots users have a `bot-` prefix
		return user.Name == "bot-test-bot"
	})
	require.True(t, found)

	// Make sure an unauthenticated client can't create bots
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: "bot-name",
		Roles:   []string{"bot-role-0", "bot-role-1"},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

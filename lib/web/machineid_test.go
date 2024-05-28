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
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestListBots(t *testing.T) {
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

	created := 5
	n := 0
	for n < created {
		n += 1
		_, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
			BotName: "test-bot-" + strconv.Itoa(n),
			Roles:   []string{"test-role"},
		})
		require.NoError(t, err)
	}

	response, err := pack.clt.Get(ctx, endpoint, url.Values{
		"page_token": []string{""},  // default to the start
		"page_size":  []string{"2"}, // is ignored
	})
	require.NoError(t, err)

	var bots ListBotsResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &bots), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code getting connectors")

	assert.Len(t, bots.Items, created)
	assert.Equal(t, []string{"test-role"}, bots.Items[0].Spec.Roles)
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

	i := slices.IndexFunc(users, func(user ui.UserListEntry) bool {
		// bot name is prefixed with `bot` in UserList
		return user.Name == "bot-test-bot"
	})
	require.NotEqual(t, -1, i)
	// the user resource returned from ListUsers should only contain the roles created for the bot (not create/edit request roles)
	require.Equal(t, []string{"bot-test-bot"}, users[i].Roles)

	// fetch bots and assert that the bot we created exists
	getBotsResp, err := pack.clt.Get(ctx, endpoint, url.Values{
		"page_token": []string{""},  // default to the start
		"page_size":  []string{"2"}, // is ignored
	})
	require.NoError(t, err)

	var bots ListBotsResponse
	require.NoError(t, json.Unmarshal(getBotsResp.Bytes(), &bots), "invalid response received")

	i = slices.IndexFunc(bots.Items, func(bot *machineidv1.Bot) bool {
		// bot name is not prefixed in BotList
		return bot.Metadata.Name == "test-bot"
	})
	require.NotEqual(t, -1, i)
	// the bot resource returned from ListBots should only contain the roles attached to the bot via the create/edit request (not created roles)
	require.Equal(t, []string{"bot-role-0", "bot-role-1"}, bots.Items[i].Spec.GetRoles())

	// Make sure an unauthenticated client can't create bots
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: "bot-name",
		Roles:   []string{"bot-role-0", "bot-role-1"},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestCreateBotJoinToken(t *testing.T) {
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
		"token",
	)

	// add github join token
	integrationName := "my-app-deploy"
	validReq := CreateBotJoinTokenRequest{
		IntegrationName: integrationName,
		JoinMethod:      types.JoinMethodGitHub,
		GitHub: &types.ProvisionTokenSpecV2GitHub{
			Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
				{
					Repository: "gravitational/teleport",
					Actor:      "actor",
				},
			},
		},
		WebFlowLabel: webUIFlowBotGitHubActionsSSH,
	}
	resp, err := pack.clt.PostJSON(ctx, endpoint, validReq)
	require.NoError(t, err)

	var result nodeJoinToken
	json.Unmarshal(resp.Bytes(), &result)
	require.Equal(t, integrationName, result.ID)
	require.Equal(t, types.JoinMethod("github"), result.Method)

	// invalid join method
	invalidJoinMethodReq := validReq
	// use a different integration name so it doesn't error for being duplicated
	invalidJoinMethodReq.IntegrationName = "invalid-join-method-test"
	invalidJoinMethodReq.JoinMethod = "invalid-join-method"
	_, err = pack.clt.PostJSON(ctx, endpoint, invalidJoinMethodReq)
	require.Error(t, err)

	// no integration name
	invalidIntegrationNameReq := validReq
	invalidIntegrationNameReq.IntegrationName = ""
	_, err = pack.clt.PostJSON(ctx, endpoint, invalidIntegrationNameReq)
	require.Error(t, err)
}

func TestDeleteBot_UnauthenticatedError(t *testing.T) {
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
		"testname",
	)

	publicClt := s.client(t)
	_, err := publicClt.Delete(ctx, endpoint)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestDeleteBot(t *testing.T) {
	botName := "bot-bravo"

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

	// create bot to delete
	_, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
	})
	require.NoError(t, err)

	endpoint = pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
		botName,
	)

	resp, err := pack.clt.Delete(ctx, endpoint)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Code(), "unexpected status code getting connectors")
}

func TestGetBotByName(t *testing.T) {
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

	// create a bot nammed `test-bot-1`
	botName := "test-bot-1"
	_, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
	})
	require.NoError(t, err)

	response, err := pack.clt.Get(ctx, fmt.Sprintf("%s/%s", endpoint, botName), nil)
	require.NoError(t, err)

	var bot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &bot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code getting connectors")
	assert.Equal(t, botName, bot.Metadata.Name)

	// query an unexisting bot
	_, err = pack.clt.Get(ctx, fmt.Sprintf("%s/%s", endpoint, "invalid-bot"), nil)
	require.Error(t, err)
}

func TestEditBot(t *testing.T) {
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

	// create a bot named `test-bot-edit`
	botName := "test-bot-edit"
	_, err := pack.clt.PostJSON(ctx, endpoint, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
	})
	require.NoError(t, err)

	response, err := pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpoint, botName), updateBotRequest{
		Roles: []string{"new-new-role"},
	})
	require.NoError(t, err)

	var bot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &bot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code getting connectors")
	assert.Equal(t, botName, bot.Metadata.Name)
	assert.Equal(t, []string{"new-new-role"}, bot.Spec.Roles)
}

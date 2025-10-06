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
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestListBots(t *testing.T) {
	ctx := t.Context()
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
	ctx := t.Context()
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

	ctx := t.Context()

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
	ctx := t.Context()
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
	ctx := t.Context()
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

	ctx := t.Context()
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
	ctx := t.Context()
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
	ctx := t.Context()
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

	response, err := pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpoint, botName), updateBotRequestV1{
		Roles: []string{"new-new-role"},
	})
	require.NoError(t, err)

	var bot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &bot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code getting connectors")
	assert.Equal(t, botName, bot.Metadata.Name)
	assert.Equal(t, []string{"new-new-role"}, bot.Spec.Roles)
}

func TestEditBotRoles(t *testing.T) {
	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpointV1 := pack.clt.Endpoint(
		"v1",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)
	endpointV2 := pack.clt.Endpoint(
		"v2",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	// create a bot named `test-bot-edit`
	botName := "test-bot-edit"
	_, err := pack.clt.PostJSON(ctx, endpointV1, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
		Traits: []*machineidv1.Trait{
			{
				Name:   "test-trait-1",
				Values: []string{"value-1"},
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpointV2, botName), updateBotRequestV2{
		Roles: []string{"new-new-role"},
	})
	require.NoError(t, err)

	var bot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &bot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code updating bot")
	assert.Equal(t, botName, bot.GetMetadata().GetName())
	assert.Equal(t, []string{"new-new-role"}, bot.GetSpec().GetRoles())
	assert.Equal(t, []*machineidv1.Trait{
		{
			Name:   "test-trait-1",
			Values: []string{"value-1"},
		},
	}, bot.GetSpec().Traits)
	assert.Equal(t, int64((12*time.Hour)/time.Second), bot.GetSpec().GetMaxSessionTtl().GetSeconds())
}

func TestEditBotTraits(t *testing.T) {
	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpointV1 := pack.clt.Endpoint(
		"v1",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)
	endpointV2 := pack.clt.Endpoint(
		"v2",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	// create a bot named `test-bot-edit`
	botName := "test-bot-edit"
	_, err := pack.clt.PostJSON(ctx, endpointV1, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
		Traits: []*machineidv1.Trait{
			{
				Name:   "test-trait-1",
				Values: []string{"value-1", "value-2"},
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpointV2, botName), updateBotRequestV2{
		Traits: []updateBotRequestTrait{
			{
				Name:   "test-trait-1",
				Values: []string{"value-1", "value-2", "value-3"},
			},
			{
				Name:   "test-trait-2",
				Values: []string{"value-1"},
			},
		},
	})
	require.NoError(t, err)

	var bot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &bot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code updating bot")
	assert.Equal(t, botName, bot.GetMetadata().GetName())
	assert.Equal(t, []string{"test-role"}, bot.GetSpec().GetRoles())
	assert.Len(t, bot.GetSpec().Traits, 2)
	for _, trait := range bot.GetSpec().Traits {
		// Avoid ordering race - traits are stored in a map
		switch trait.GetName() {
		case "test-trait-1":
			assert.Equal(t, []string{"value-1", "value-2", "value-3"}, trait.GetValues())
		case "test-trait-2":
			assert.Equal(t, []string{"value-1"}, trait.GetValues())
		}
	}
	assert.Equal(t, int64((12*time.Hour)/time.Second), bot.GetSpec().GetMaxSessionTtl().GetSeconds())
}

func TestEditBotMaxSessionTTL(t *testing.T) {
	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpointV1 := pack.clt.Endpoint(
		"v1",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)
	endpointV2 := pack.clt.Endpoint(
		"v2",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	// create a bot named `test-bot-edit`
	botName := "test-bot-edit"
	_, err := pack.clt.PostJSON(ctx, endpointV1, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
		Traits: []*machineidv1.Trait{
			{
				Name:   "test-trait-1",
				Values: []string{"value-1"},
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.Get(ctx, fmt.Sprintf("%s/%s", endpointV1, botName), nil)
	require.NoError(t, err)

	var createdBot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &createdBot), "invalid response received")
	assert.Equal(t, int64(43200), createdBot.GetSpec().GetMaxSessionTtl().GetSeconds())

	response, err = pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpointV2, botName), updateBotRequestV2{
		MaxSessionTtl: "1h2m3s",
	})
	require.NoError(t, err)

	var updatedBot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &updatedBot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code updating bot")
	assert.Equal(t, botName, updatedBot.GetMetadata().GetName())
	assert.Equal(t, []string{"test-role"}, updatedBot.GetSpec().GetRoles())
	assert.Equal(t, []*machineidv1.Trait{
		{
			Name:   "test-trait-1",
			Values: []string{"value-1"},
		},
	}, updatedBot.GetSpec().Traits)
	assert.Equal(t, int64((1*time.Hour+2*time.Minute+3*time.Second)/time.Second), updatedBot.GetSpec().GetMaxSessionTtl().GetSeconds())
}

func TestEditBotDescription(t *testing.T) {
	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpointV1 := pack.clt.Endpoint(
		"v1",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)
	endpointV3 := pack.clt.Endpoint(
		"v3",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
	)

	// create a bot named `test-bot-edit`
	botName := "test-bot-edit"
	_, err := pack.clt.PostJSON(ctx, endpointV1, CreateBotRequest{
		BotName: botName,
		Roles:   []string{"test-role"},
		Traits: []*machineidv1.Trait{
			{
				Name:   "test-trait-1",
				Values: []string{"value-1"},
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.Get(ctx, fmt.Sprintf("%s/%s", endpointV1, botName), nil)
	require.NoError(t, err)

	var createdBot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &createdBot), "invalid response received")
	assert.Equal(t, int64(43200), createdBot.GetSpec().GetMaxSessionTtl().GetSeconds())

	description := "This is the bot's description."
	response, err = pack.clt.PutJSON(ctx, fmt.Sprintf("%s/%s", endpointV3, botName), updateBotRequestV3{
		Description: &description,
	})
	require.NoError(t, err)

	var updatedBot machineidv1.Bot
	require.NoError(t, json.Unmarshal(response.Bytes(), &updatedBot), "invalid response received")
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code updating bot")
	assert.Equal(t, botName, updatedBot.GetMetadata().GetName())
	assert.Equal(t, []string{"test-role"}, updatedBot.GetSpec().GetRoles())
	assert.Equal(t, []*machineidv1.Trait{
		{
			Name:   "test-trait-1",
			Values: []string{"value-1"},
		},
	}, updatedBot.GetSpec().Traits)
	assert.Equal(t, int64((12*time.Hour)/time.Second), updatedBot.GetSpec().GetMaxSessionTtl().GetSeconds())
	assert.Equal(t, description, updatedBot.GetMetadata().GetDescription())
}

func TestListBotInstances(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoints := []string{
		pack.clt.Endpoint(
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
		pack.clt.Endpoint(
			"v2",
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
	}
	instanceID := uuid.New().String()

	_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    "test-bot",
			InstanceId: instanceID,
		},
		Status: &machineidv1.BotInstanceStatus{
			LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
				{
					RecordedAt: &timestamppb.Timestamp{
						Seconds: 2,
						Nanos:   0,
					},
				},
				{
					RecordedAt: &timestamppb.Timestamp{
						Seconds: 1,
						Nanos:   0,
					},
				},
				{
					RecordedAt: &timestamppb.Timestamp{
						Seconds: 3,
						Nanos:   0,
					},
					Version:    "1.0.0",
					Hostname:   "test-hostname",
					JoinMethod: "test-join-method",
					Os:         "linux",
				},
			},
			LatestAuthentications: []*machineidv1.BotInstanceStatusAuthentication{
				{
					AuthenticatedAt: &timestamppb.Timestamp{
						Seconds: 2,
						Nanos:   0,
					},
				},
				{
					AuthenticatedAt: &timestamppb.Timestamp{
						Seconds: 1,
						Nanos:   0,
					},
				},
				{
					AuthenticatedAt: &timestamppb.Timestamp{
						Seconds: 2,
						Nanos:   0,
					},
					JoinMethod: "test-join-method",
				},
			},
		},
	})
	require.NoError(t, err)

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			response, err := pack.clt.Get(ctx, endpoint, url.Values{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

			var instances ListBotInstancesResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

			assert.Len(t, instances.BotInstances, 1)
			require.Empty(t, cmp.Diff(instances, ListBotInstancesResponse{
				BotInstances: []BotInstance{
					{
						InstanceId:       instanceID,
						BotName:          "test-bot",
						JoinMethodLatest: "test-join-method",
						HostNameLatest:   "test-hostname",
						VersionLatest:    "1.0.0",
						ActiveAtLatest:   "1970-01-01T00:00:03Z",
						OSLatest:         "linux",
					},
				},
			}))
		})
	}
}

func TestListBotInstancesWithInitialHeartbeat(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoints := []string{
		pack.clt.Endpoint(
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
		pack.clt.Endpoint(
			"v2",
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
	}

	instanceID := uuid.New().String()

	_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    "test-bot",
			InstanceId: instanceID,
		},
		Status: &machineidv1.BotInstanceStatus{
			InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				RecordedAt: &timestamppb.Timestamp{
					Seconds: 3,
					Nanos:   0,
				},
				Version:    "1.0.0",
				Hostname:   "test-hostname",
				JoinMethod: "test-join-method",
			},
			LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{},
			InitialAuthentication: &machineidv1.BotInstanceStatusAuthentication{
				JoinMethod: "test-join-method",
			},
		},
	})
	require.NoError(t, err)

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			response, err := pack.clt.Get(ctx, endpoint, url.Values{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

			var instances ListBotInstancesResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

			assert.Len(t, instances.BotInstances, 1)
			require.Empty(t, cmp.Diff(instances, ListBotInstancesResponse{
				BotInstances: []BotInstance{
					{
						InstanceId:       instanceID,
						BotName:          "test-bot",
						JoinMethodLatest: "test-join-method",
						HostNameLatest:   "test-hostname",
						VersionLatest:    "1.0.0",
						ActiveAtLatest:   "1970-01-01T00:00:03Z",
					},
				},
			}))
		})
	}
}

func TestListBotInstancesPaging(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()

	endpoints := []string{
		pack.clt.Endpoint(
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
		pack.clt.Endpoint(
			"v2",
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
	}

	tcs := []struct {
		name         string
		numInstances int
		pageSize     int
	}{
		{
			name:         "zero results",
			numInstances: 0,
			pageSize:     1,
		},
		{
			name:         "smaller page size",
			numInstances: 5,
			pageSize:     2,
		},
		{
			name:         "larger page size",
			numInstances: 2,
			pageSize:     5,
		},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			for _, tc := range tcs {
				t.Run(tc.name, func(t *testing.T) {
					for n := range tc.numInstances {
						_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
							Kind:    types.KindBotInstance,
							Version: types.V1,
							Spec: &machineidv1.BotInstanceSpec{
								BotName:    "bot-1",
								InstanceId: "instance-" + strconv.Itoa(n),
							},
							Status: &machineidv1.BotInstanceStatus{},
						})
						require.NoError(t, err)
					}

					response, err := pack.clt.Get(ctx, endpoint, url.Values{
						"page_token": []string{""}, // default to the start
						"page_size":  []string{strconv.Itoa(tc.pageSize)},
					})
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

					var resp ListBotInstancesResponse
					require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

					assert.Len(t, resp.BotInstances, int(math.Min(float64(tc.numInstances), float64(tc.pageSize))))

					// remove instances before next test
					for n := range tc.numInstances {
						err = env.server.Auth().DeleteBotInstance(ctx, "bot-1", "instance-"+strconv.Itoa(n))
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestListBotInstancesSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1, withWebPackAuthCacheEnabled(true))
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot-instance",
	)

	for i := range 10 {
		now := time.Now()
		_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:    types.KindBotInstance,
			Version: types.V1,
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: uuid.New().String(),
			},
			Status: &machineidv1.BotInstanceStatus{
				InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
					RecordedAt: &timestamppb.Timestamp{
						Seconds: now.Unix() + int64(i),
					},
				},
			},
		})
		require.NoError(t, err, "failed to create BotInstance index:%d", i)
	}

	response, err := pack.clt.Get(ctx, endpoint, url.Values{
		"page_token": []string{""}, // default to the start
		"page_size":  []string{"0"},
		"sort":       []string{"active_at_latest:desc"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

	var resp ListBotInstancesResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

	prevValue := "~"
	for _, r := range resp.BotInstances {
		assert.Less(t, r.ActiveAtLatest, prevValue)
		prevValue = r.ActiveAtLatest
	}
}

func TestListBotInstancesWithBotFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoints := []string{
		pack.clt.Endpoint(
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
		pack.clt.Endpoint(
			"v2",
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
	}

	n := 0
	for n < 5 {
		n += 1
		botName := "bot-" + strconv.Itoa(n%2)
		_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:    types.KindBotInstance,
			Version: types.V1,
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    botName,
				InstanceId: uuid.New().String(),
			},
			Status: &machineidv1.BotInstanceStatus{},
		})
		require.NoError(t, err)
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			response, err := pack.clt.Get(ctx, endpoint, url.Values{
				"bot_name": []string{"bot-1"},
			})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

			var instances ListBotInstancesResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

			assert.Len(t, instances.BotInstances, 3)
		})
	}
}

func TestListBotInstancesWithSearchTermFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoints := []string{
		pack.clt.Endpoint(
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
		pack.clt.Endpoint(
			"v2",
			"webapi",
			"sites",
			clusterName,
			"machine-id",
			"bot-instance",
		),
	}

	tcs := []struct {
		name       string
		searchTerm string
		spec       *machineidv1.BotInstanceSpec
		heartbeat  *machineidv1.BotInstanceStatusHeartbeat
	}{
		{
			name:       "match on bot name",
			searchTerm: "nick",
			spec: &machineidv1.BotInstanceSpec{
				BotName:    "this-is-nicks-test-bot",
				InstanceId: "00000000-0000-0000-0000-000000000000",
			},
		},
		{
			name:       "match on instance id",
			searchTerm: "0000000",
		},
		{
			name:       "match on join method",
			searchTerm: "uber",
			heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				JoinMethod: "kubernetes",
			},
		},
		{
			name:       "match on version",
			searchTerm: "1.0.0",
			heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Version: "1.0.0-dev-a2g3hd",
			},
		},
		{
			name:       "match on version (with v)",
			searchTerm: "v1.0.0",
			heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Version: "1.0.0-dev-a2g3hd",
			},
		},
		{
			name:       "match on hostname",
			searchTerm: "tel-123",
			heartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Hostname: "svr-eu-tel-123-a",
			},
		},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			for _, tc := range tcs {
				t.Run(tc.name, func(t *testing.T) {
					spec := tc.spec
					if spec == nil {
						spec = &machineidv1.BotInstanceSpec{
							BotName:    "test-bot",
							InstanceId: "00000000-0000-0000-0000-000000000000",
						}
					}

					_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
						Kind:    types.KindBotInstance,
						Version: types.V1,
						Spec:    spec,
						Status: &machineidv1.BotInstanceStatus{
							InitialHeartbeat: tc.heartbeat,
						},
					})
					require.NoError(t, err)

					_, err = env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
						Kind:    types.KindBotInstance,
						Version: types.V1,
						Spec: &machineidv1.BotInstanceSpec{
							BotName:    "bot-gone",
							InstanceId: uuid.New().String(),
						},
						Status: &machineidv1.BotInstanceStatus{
							InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
								Version:    "1.1.1-prod",
								Hostname:   "test-hostname",
								JoinMethod: "test-join-method",
							},
						},
					})
					require.NoError(t, err)

					response, err := pack.clt.Get(ctx, endpoint, url.Values{
						"search": []string{tc.searchTerm},
					})
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

					var instances ListBotInstancesResponse
					require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

					assert.Len(t, instances.BotInstances, 1)
					assert.Equal(t, "00000000-0000-0000-0000-000000000000", instances.BotInstances[0].InstanceId)

					// remove before next test
					err = env.server.Auth().DeleteBotInstance(ctx, spec.BotName, spec.InstanceId)
					require.NoError(t, err)
				})
			}
		})
	}
}

func TestListBotInstancesWithQueryFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint(
		"v2",
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot-instance",
	)

	_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    "test-bot-1",
			InstanceId: "00000000-0000-0000-0000-000000000000",
		},
		Status: &machineidv1.BotInstanceStatus{
			InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Hostname: "svr-eu-tel-123-a",
			},
		},
	})
	require.NoError(t, err)

	_, err = env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    "test-bot-2",
			InstanceId: uuid.New().String(),
		},
		Status: &machineidv1.BotInstanceStatus{
			InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				Hostname: "test-hostname",
			},
		},
	})
	require.NoError(t, err)

	response, err := pack.clt.Get(ctx, endpoint, url.Values{
		"query": []string{`status.latest_heartbeat.hostname == "svr-eu-tel-123-a"`},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

	var instances ListBotInstancesResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &instances), "invalid response received")

	assert.Len(t, instances.BotInstances, 1)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", instances.BotInstances[0].InstanceId)
}

func TestGetBotInstance(t *testing.T) {
	ctx := t.Context()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})
	clusterName := env.server.ClusterName()

	botName := "test-bot"
	instanceID := uuid.New().String()

	_, err := env.server.Auth().CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    botName,
			InstanceId: instanceID,
		},
		Status: &machineidv1.BotInstanceStatus{
			InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				RecordedAt: &timestamppb.Timestamp{
					Seconds: 1,
					Nanos:   0,
				},
			},
		},
	})
	require.NoError(t, err)

	endpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machine-id",
		"bot",
		botName,
		"bot-instance",
		instanceID,
	)
	response, err := pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code(), "unexpected status code")

	var resp GetBotInstanceResponse
	require.NoError(t, json.Unmarshal(response.Bytes(), &resp), "invalid response received")

	require.Empty(t, cmp.Diff(resp.BotInstance, machineidv1.BotInstance{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    botName,
			InstanceId: instanceID,
		},
		Status: &machineidv1.BotInstanceStatus{
			InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
				RecordedAt: &timestamppb.Timestamp{
					Seconds: 1,
					Nanos:   0,
				},
			},
		},
	}, protocmp.Transform(), protocmp.IgnoreFields(&machineidv1.BotInstance{}, "metadata")))
	assert.YAMLEq(t, fmt.Sprintf("kind: bot_instance\nmetadata:\n  name: %[1]s\n  revision: %[2]s\nspec:\n  bot_name: test-bot\n  instance_id: %[1]s\nstatus:\n  initial_heartbeat:\n    recorded_at: \"1970-01-01T00:00:01Z\"\nversion: v1\n", instanceID, resp.BotInstance.Metadata.Revision), resp.YAML)
}

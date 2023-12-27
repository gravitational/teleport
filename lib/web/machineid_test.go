package web

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGitHubBotCreate(t *testing.T) {
	s := newWebSuite(t)
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "admin", []types.Role{services.NewPresetEditorRole()})

	clusterName := env.server.ClusterName()

	createGitHubBotEndpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"integrations",
		"machine-id",
		"github-actions",
	)

	ctx := context.Background()

	resp, err := pack.clt.PostJSON(ctx, createGitHubBotEndpoint, CreateGitHubBotRequest{
		BotName: "test-bot",
		Roles:   []string{"bot-role-0", "bot-role-1"},
	})
	require.NoError(t, err)

	var expected struct {
		Message string `json:"message"`
	}
	err = json.Unmarshal(resp.Bytes(), &expected)
	require.NoError(t, err)
	require.Equal(t, expected.Message, "ok")

	// fetch users and assert that the bot we created exists
	getUsersResp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "users"), nil)
	require.NoError(t, err)
	var users []ui.UserListEntry
	json.Unmarshal(getUsersResp.Bytes(), &users)

	var found bool
	for _, u := range users {
		// bots users have a `bot-` prefix
		if u.Name == "bot-test-bot" {
			found = true
			break
		}
	}
	require.True(t, found)

	// Make sure an unauthenticated client can't create bots
	publicClt := s.client(t)
	_, err = publicClt.PostJSON(ctx, createGitHubBotEndpoint, CreateGitHubBotRequest{
		BotName: "bot-name",
		Roles:   []string{"bot-role-0", "bot-role-1"},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

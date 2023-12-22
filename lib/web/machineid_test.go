package web

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubBotCreate(t *testing.T) {
	s := newWebSuite(t)
	pack := s.authPack(t, "foo")
	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	createGitHubBotEndpoint := pack.clt.Endpoint(
		"webapi",
		"sites",
		clusterName,
		"machineid",
		"bot",
		"github",
	)

	ctx := context.Background()

	resp, err := pack.clt.PostJSON(ctx, createGitHubBotEndpoint, CreateGitHubBotRequest{
		BotName:         "bot-name",
		BotRoles:        []string{"editor", "auditor"},
		Repository:      "repo",
		Subject:         "subject",
		RepositoryOwner: "repo-owner",
		Workflow:        "workflow",
		Environment:     "env",
		Actor:           "actor",
		Ref:             "ref",
		RefType:         "ref-type",
	})
	require.NoError(t, err)
	require.Nil(t, resp) // this endpoints returns nothing

	// assert that the expected bots and join tokens exist
}

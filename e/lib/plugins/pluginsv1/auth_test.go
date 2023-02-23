package pluginsv1

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestGetPlugin(t *testing.T) {
	t.Parallel()

	const pluginName = "slack-default"
	ctx := context.Background()
	suite := createSuite(t)
	suite.createPlugin(t, pluginName)

	tt := []struct {
		Name             string
		Rules            []types.Rule
		ErrAssertionRead require.ErrorAssertionFunc
	}{
		{
			Name:             "deny if user has no rules regarding plugins",
			Rules:            nil,
			ErrAssertionRead: assertAccessDenied,
		},
		{
			Name: "allow if user has Read",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead},
				},
			},
			ErrAssertionRead: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			suite.setRules(tc.Rules)

			got, err := suite.svc.GetPlugin(ctx, &pluginspb.GetPluginRequest{Name: pluginName})
			tc.ErrAssertionRead(t, err)

			if got != nil {
				// Short-lived credentials never returned through the gRPC layer
				require.Nil(t, got.Credentials)
			}
		})
	}
}

func TestSetPluginCredentials(t *testing.T) {
	t.Parallel()

	const pluginName = "slack-default"
	ctx := context.Background()

	suite := createSuite(t)
	suite.createPlugin(t, pluginName)

	tt := []struct {
		Name         string
		Rules        []types.Rule
		ErrAssertion require.ErrorAssertionFunc
	}{
		{
			Name:         "deny if user has no rules regarding plugins",
			Rules:        nil,
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "deny secrets if user can update, but can not read secrets",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbReadNoSecrets, types.VerbUpdate},
				},
			},
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "deny if user can read secrets, but not update",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead},
				},
			},
			ErrAssertion: assertAccessDenied,
		},
		{
			Name: "allow if user can read secrets, and update",
			Rules: []types.Rule{
				{
					Resources: []string{types.KindPlugin},
					Verbs:     []string{types.VerbRead, types.VerbUpdate},
				},
			},
			ErrAssertion: require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			suite.setRules(tc.Rules)

			_, err := suite.svc.SetPluginCredentials(ctx, &pluginspb.SetPluginCredentialsRequest{
				Name: pluginName,
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
						Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
							AccessToken:  "foo",
							RefreshToken: "bar",
							Expires:      time.Now().Add(1 * time.Minute),
						},
					},
				},
			})
			tc.ErrAssertion(t, err)
		})
	}
}

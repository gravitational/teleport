package plugins

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
)

func TestPluginUpdateCredentials(t *testing.T) {
	ctx := t.Context()

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "admin", "editor"),
		common.WithUser(t, "regular-user", "requester"),
	)

	auth := sut.Teleport.Process.GetAuthServer()

	adminUser := sut.GetClusterClientForUser(t, "admin")
	regularUser := sut.GetClusterClientForUser(t, "regular-user")

	t.Run("target by name", func(t *testing.T) {
		creds := mustCreatePluginStaticCredentials(t, auth,
			"cred-number-1",
			withAPIToken("this is not a real token"))

		newValue := &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
				BasicAuth: &types.PluginStaticCredentialsBasicAuth{
					Username: "some-username",
					Password: "secret squirrel",
				},
			},
		}

		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Name:       proto.String(creds.GetName()),
				Credential: newValue,
			}.Build(),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, creds.GetName(), response.GetCredential().GetName())
		require.Equal(t, response.GetCredential().Spec, newValue)

		// check that the backend token value is updated
		cred, err := auth.GetPluginStaticCredentials(context.Background(), creds.GetName())
		require.NoError(t, err)
		require.Equal(t, newValue, cred.(*types.PluginStaticCredentialsV1).Spec)
	})

	t.Run("target by name (not found)", func(t *testing.T) {
		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Name: proto.String("no-such-credential"),
				Credential: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "some token or other",
					},
				},
			}.Build())
		require.True(t, trace.IsNotFound(err), "expected Not Found error, got %T %q", err, err)
		require.Nil(t, response)
	})

	t.Run("target by label", func(t *testing.T) {
		mustCreatePluginStaticCredentials(t, auth,
			"cred-number-1",
			withLabel("plugin-id", "1"),
			withLabel("is-target", "false"),
			withAPIToken("this is not a real token"))

		targetCreds := mustCreatePluginStaticCredentials(t, auth,
			"cred-number-2",
			withLabel("plugin-id", "1"),
			withLabel("is-target", "true"),
			withBasicAuth("some-user", "some-password"))

		newValue := &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "updated-token-value",
			},
		}

		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Query: pluginsv1.CredentialQuery_builder{
					Labels: map[string]string{
						"plugin-id": "1",
						"is-target": "true",
					},
				}.Build(),
				Credential: newValue,
			}.Build(),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, targetCreds.GetName(), response.GetCredential().GetName())
		require.Equal(t, response.GetCredential().Spec, newValue)

		// check that the backend token value is updated
		cred, err := auth.GetPluginStaticCredentials(context.Background(), "cred-number-2")
		require.NoError(t, err)
		require.Equal(t, newValue, cred.(*types.PluginStaticCredentialsV1).Spec)
	})

	t.Run("target by label (not found)", func(t *testing.T) {
		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Query: pluginsv1.CredentialQuery_builder{
					Labels: map[string]string{"exists": "false"},
				}.Build(),
				Credential: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "some token or other",
					},
				},
			}.Build())
		require.True(t, trace.IsNotFound(err), "expected Not Found error, got %T %q", err, err)
		require.Nil(t, response)
	})

	t.Run("access denied", func(t *testing.T) {
		originalCred := mustCreatePluginStaticCredentials(t, auth,
			"token-number-1",
			withBasicAuth("uid", "pwd"))

		response, err := regularUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Name: proto.String("token-number-1"),
				Credential: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "some random value",
					},
				},
			}.Build(),
		)
		require.True(t, trace.IsAccessDenied(err), "Expected Access Denied error, gt %T %q", err, err)
		require.Nil(t, response)

		// check that the backend token value is unchanged
		cred, err := auth.GetPluginStaticCredentials(context.Background(), "token-number-1")
		require.NoError(t, err)
		require.Equal(t, originalCred, cred)
	})

	t.Run("empty query is an error", func(t *testing.T) {
		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Query: &pluginsv1.CredentialQuery{},
				Credential: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "some random value",
					},
				},
			}.Build())
		require.True(t, trace.IsBadParameter(err), "Expected Bad Parameter error, gt %T %q", err, err)
		require.Nil(t, response)
	})

	t.Run("multiple match is an error", func(t *testing.T) {
		mustCreatePluginStaticCredentials(t, auth,
			"token-number-1",
			withLabel("plugin-id", "one"),
			withLabel("matches", "true"),
			withAPIToken("this is not a real token #1"))

		mustCreatePluginStaticCredentials(t, auth,
			"token-number-2",
			withLabel("plugin-id", "two"),
			withLabel("matches", "false"),
			withAPIToken("this is not a real token #2"))

		mustCreatePluginStaticCredentials(t, auth,
			"token-number-3",
			withLabel("plugin-id", "three"),
			withLabel("matches", "true"),
			withAPIToken("this is not a real token #3"))

		response, err := adminUser.AuthClient.PluginsClient().UpdatePluginStaticCredentials(ctx,
			pluginsv1.UpdatePluginStaticCredentialsRequest_builder{
				Query: pluginsv1.CredentialQuery_builder{
					Labels: map[string]string{"matches": "true"},
				}.Build(),
				Credential: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "some random value",
					},
				},
			}.Build())
		require.True(t, trace.IsBadParameter(err), "Expected Bad Parameter error, gt %T %q", err, err)
		require.Nil(t, response)
	})
}

package oktatest

import (
	"context"
	"testing" //nolint:depguard // this a shared test package

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require" //nolint:depguard // this a shared test package

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type pluginStaticCredentialsOption func(*types.PluginStaticCredentialsSpecV1)

func WithOAuthClientId(clientId string) pluginStaticCredentialsOption {
	return func(spec *types.PluginStaticCredentialsSpecV1) {
		spec.Credentials = &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     clientId,
				ClientSecret: "",
			},
		}
	}
}

func WithApiToken(apiToken string) pluginStaticCredentialsOption {
	return func(spec *types.PluginStaticCredentialsSpecV1) {
		spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: apiToken,
		}
	}
}

func NewPluginStaticCredentials(t *testing.T, plugin *types.PluginV1, opts ...pluginStaticCredentialsOption) types.PluginStaticCredentials {
	t.Helper()

	require.NotNil(t, plugin.GetCredentials().GetStaticCredentialsRef(), "plugin credentials ref missing")
	require.NotEmpty(t, plugin.GetCredentials().GetStaticCredentialsRef().Labels, "plugin credentials ref labels missing")

	spec := types.PluginStaticCredentialsSpecV1{
		Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     "test-client-id-" + uuid.NewString(),
				ClientSecret: "",
			},
		},
	}

	for _, opt := range opts {
		opt(&spec)
	}

	staticCreds, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:   types.PluginTypeOkta,
			Labels: plugin.GetCredentials().GetStaticCredentialsRef().Labels,
		},
		spec,
	)
	require.NoError(t, err, "types.NewPluginStaticCredentials")
	return staticCreds
}

// UpsertPluginStaticCredentials upserts Okta plugin ignoring the revision.
func UpsertPluginStaticCredentials(t *testing.T, credsSvc services.PluginStaticCredentials, staticCreds types.PluginStaticCredentials) {
	t.Helper()
	ctx := context.Background()

	require.Equal(t, types.PluginTypeOkta, staticCreds.GetName())

	old, err := credsSvc.GetPluginStaticCredentials(ctx, staticCreds.GetName())
	if trace.IsNotFound(err) {
		err := credsSvc.CreatePluginStaticCredentials(ctx, staticCreds)
		require.NoError(t, err, "creating test Okta plugin static credentials")
		return
	} else {
		require.NoError(t, err, "getting test Okta plugin static credentials")
	}

	staticCreds.SetRevision(old.GetRevision())
	_, err = credsSvc.UpdatePluginStaticCredentials(ctx, staticCreds)
	require.NoError(t, err, "updating test Okta plugin static credentials")
}

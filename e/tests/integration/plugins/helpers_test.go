package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type credOption func(*types.PluginStaticCredentialsV1)

func withAPIToken(token string) credOption {
	return func(cred *types.PluginStaticCredentialsV1) {
		cred.Spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: token,
		}
	}
}

func withBasicAuth(username, password string) credOption {
	return func(cred *types.PluginStaticCredentialsV1) {
		cred.Spec.Credentials = &types.PluginStaticCredentialsSpecV1_BasicAuth{
			BasicAuth: &types.PluginStaticCredentialsBasicAuth{
				Username: username,
				Password: password,
			},
		}
	}
}

func withLabel(key, value string) credOption {
	return func(cred *types.PluginStaticCredentialsV1) {
		if cred.Metadata.Labels == nil {
			cred.Metadata.Labels = make(map[string]string)
		}
		cred.Metadata.Labels[key] = value
	}
}

type pluginStaticCredentialsCreator interface {
	CreatePluginStaticCredentials(ctx context.Context, pluginStaticCredentials types.PluginStaticCredentials) error
	DeletePluginStaticCredentials(ctx context.Context, name string) error
}

func mustCreatePluginStaticCredentials(t *testing.T, dst pluginStaticCredentialsCreator, name string, options ...credOption) *types.PluginStaticCredentialsV1 {
	creds := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindPluginStaticCredentials,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: name,
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{},
	}

	for _, applyOption := range options {
		applyOption(creds)
	}

	require.NoError(t, creds.CheckAndSetDefaults())
	require.NoError(t, dst.CreatePluginStaticCredentials(context.Background(), creds))

	t.Cleanup(func() {
		require.NoError(t, dst.DeletePluginStaticCredentials(context.Background(), name))
	})

	return creds
}

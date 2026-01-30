package plugins

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

type pluginsService interface {
	GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error)
	SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error
}

// pluginStore is an implementation of state.Store backed by a Plugin resource
type pluginStore struct {
	service     pluginsService
	name        string
	retryConfig retryutils.LinearConfig
	log         *slog.Logger
}

func newPluginStore(service pluginsService, name string, log *slog.Logger) *pluginStore {
	retryConfig := retryutils.LinearConfig{
		Step: 100 * time.Millisecond,
		Max:  1 * time.Second,
	}
	return &pluginStore{
		service:     service,
		name:        name,
		retryConfig: retryConfig,
		log:         log,
	}
}

func (p *pluginStore) GetCredentials(ctx context.Context) (*storage.Credentials, error) {
	plugin, err := p.service.GetPlugin(ctx, p.name, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds := plugin.GetCredentials()
	if creds == nil {
		return nil, trace.BadParameter("plugin has no credentials set")
	}

	oauthCreds := creds.GetOauth2AccessToken()
	if oauthCreds == nil {
		return nil, trace.BadParameter("plugin credentials are not of OAuth2 access token type")
	}
	return &storage.Credentials{
		AccessToken:  oauthCreds.AccessToken,
		RefreshToken: oauthCreds.RefreshToken,
		ExpiresAt:    oauthCreds.Expires,
	}, nil
}

func (p *pluginStore) PutCredentials(ctx context.Context, creds *storage.Credentials) error {
	v1 := &types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
				AccessToken:  creds.AccessToken,
				RefreshToken: creds.RefreshToken,
				Expires:      creds.ExpiresAt,
			},
		},
	}
	retry, err := retryutils.NewLinear(p.retryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	// This timeout caps the total duration of retry attempts.
	// It is chosen to be relatively long to allow for several attempts
	// (although serious write contention on this resource is not expected)
	// but short enough to not interfere with plugin's responsiveness.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var attempt int
	err = retry.For(ctx, func() error {
		attempt++
		return p.service.SetPluginCredentials(ctx, p.name, v1)
	})
	if err != nil {
		p.log.WarnContext(ctx, "Failed to set plugin credentials", "name", p.name, "attempt", attempt)
		return trace.Wrap(err, "setting credentials for plugin %q", p.name)
	}
	return nil
}

package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/integrations/access/slack"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func Slack(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	slackSpec := plugin.Spec.GetSlackAccessPlugin()
	if slackSpec == nil {
		return nil, trace.BadParameter("field Spec.SlackAccessPlugin must be present")
	}

	switch len(deps.StaticCredentials) {
	case 0:
		return nil, trace.BadParameter("missing static credentials")
	case 1:
	default:
		deps.Logger.WarnContext(ctx, "multiple static credentials found, this is ambiguous, using the first one")
	}

	var (
		tokenProvider   auth.AccessTokenProvider
		runInBackground []func(ctx context.Context)
	)

	staticCreds := deps.StaticCredentials[0]
	switch {
	case staticCreds.GetOAuthClientID() != "":
		deps.Logger.DebugContext(ctx, "Using OAuth")
		// The plugin has non-nil Oauth creds, it should do oauth
		id, secret := staticCreds.GetOAuthClientSecret()
		if id == "" || secret == "" {
			return nil, trace.BadParameter("invalid static credentials, oauth client id or secret empty")
		}
		authorizer := slack.NewAuthorizer(id, secret, deps.Logger)

		// The plugin should contain initial credentials
		initialCreds := plugin.GetCredentials()
		if initialCreds.GetOauth2AccessToken() == nil || initialCreds.GetOauth2AccessToken().RefreshToken == "" || initialCreds.GetOauth2AccessToken().AccessToken == "" {
			return nil, trace.BadParameter("invalid initial oauth credentials")
		}

		oauthCfg := slack.OauthTokenRefresherConfig{
			InitialCreds: storage.Credentials{
				AccessToken:  initialCreds.GetOauth2AccessToken().AccessToken,
				RefreshToken: initialCreds.GetOauth2AccessToken().RefreshToken,
				ExpiresAt:    initialCreds.GetOauth2AccessToken().Expires,
			},
			SaveCreds: func(ctx context.Context, credentials storage.Credentials) error {
				storedCreds := types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
						Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
							AccessToken:  credentials.AccessToken,
							RefreshToken: credentials.RefreshToken,
							Expires:      credentials.ExpiresAt,
						},
					},
				}
				return trace.Wrap(deps.PluginsService.SetPluginCredentials(ctx, plugin.GetName(), &storedCreds))
			},
			Authorizer: authorizer,
			Log:        deps.Logger,
		}
		refresher, err := slack.NewOauthTokenRefresher(ctx, oauthCfg)
		if err != nil {
			return nil, trace.Wrap(err, "failed to initialize slack oauth token refresher")
		}
		tokenProvider = refresher
		runInBackground = append(runInBackground, refresher.RefreshLoop)
	case staticCreds.GetAPIToken() != "":
		deps.Logger.DebugContext(ctx, "Using API Token")
		tokenProvider = auth.NewStaticAccessTokenProvider(staticCreds.GetAPIToken())
	default:
		return nil, trace.BadParameter("unrecognized credentials, they support neither oauth nor id token")
	}

	pc := &pluginConfiguration{
		client:        deps.Client,
		defaultRoutes: []string{slackSpec.FallbackChannel},
		pluginConfig: &slack.Config{
			AccessTokenProvider: tokenProvider,
			StatusSink:          deps.StatusSink,
		},
		pluginType: types.PluginTypeSlack,
	}

	// The Delegate function might get called several times, in case of leader election so we make sure to
	// create the app inside, as the plugin's own process supervisor does not seem to support being restarted.
	return func(ctx context.Context) error {
		app := common.NewApp(pc, plugin.GetName())
		ctx = logger.WithLogger(ctx, deps.Logger)
		for _, fn := range runInBackground {
			go fn(ctx)
		}
		return trace.Wrap(app.Run(ctx))
	}, nil
}

package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/slack"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func Slack(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	slackSpec := plugin.Spec.GetSlackAccessPlugin()
	if slackSpec == nil {
		return nil, trace.BadParameter("field Spec.SlackAccessPlugin must be present")
	}

	tokenProvider, err := auth.NewRotatedTokenProvider(ctx, auth.RotatedAccessTokenProviderConfig{
		Store:     deps.Store,
		Refresher: deps.Authorizer,
		Log:       deps.Logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
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
		go tokenProvider.RefreshLoop(ctx)
		return trace.Wrap(app.Run(ctx))
	}, nil
}

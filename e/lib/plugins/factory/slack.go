package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/slack"
)

func Slack(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	slackSpec := plugin.Spec.GetSlackAccessPlugin()
	if slackSpec == nil {
		return nil, trace.BadParameter("field Spec.SlackAccessPlugin must be present")
	}

	tokenProvider, err := auth.NewRotatedTokenProvider(ctx, auth.RotatedAccessTokenProviderConfig{
		Store:     deps.Store,
		Refresher: deps.Authorizer,
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

	app := common.NewApp(pc, plugin.GetName())
	return func(ctx context.Context) error {
		go tokenProvider.RefreshLoop(ctx)
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}

package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/slack"
)

func slackInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	slackSpec := plugin.Spec.GetSlackAccessPlugin()
	if slackSpec == nil {
		return nil, trace.BadParameter("field Spec.SlackAccessPlugin must be present")
	}

	tokenProvider, err := auth.NewRotatedTokenProvider(ctx, auth.RotatedAccessTokenProviderConfig{
		Store:     deps.store,
		Refresher: deps.authorizer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pc := &pluginConfiguration{
		client:       deps.client,
		defaultRoute: slackSpec.FallbackChannel,
		pluginConfig: &slack.Config{
			AccessTokenProvider: tokenProvider,
			StatusSink:          deps.statusSink,
		},
		pluginType: types.PluginTypeSlack,
	}

	app := common.NewApp(pc, plugin.GetName())
	return func() error {
		go tokenProvider.RefreshLoop(deps.lifetime)
		err := app.Run(deps.lifetime)
		return trace.Wrap(err)
	}, nil
}

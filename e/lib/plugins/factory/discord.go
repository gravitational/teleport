package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/discord"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func Discord(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	discordSpec := plugin.Spec.GetDiscord()
	if discordSpec == nil {
		return nil, trace.BadParameter("field Spec.Discord must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("missing plugin static credentials")
	}

	recipients := make(common.RawRecipientsMap)
	for role, channels := range discordSpec.RoleToRecipients {
		recipients[role] = channels.ChannelIds
	}

	cfg := &discord.Config{
		BaseConfig: common.BaseConfig{
			Recipients: recipients,
		},
		Discord: common.GenericAPIConfig{
			Token: deps.StaticCredentials[0].GetAPIToken(),
		},
		Client:     deps.Client,
		StatusSink: deps.StatusSink,
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "set discord defaults")
	}

	app := discord.NewApp(cfg)
	return func(ctx context.Context) error {
		appCtx := logger.WithLogger(ctx, deps.Logger)
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}

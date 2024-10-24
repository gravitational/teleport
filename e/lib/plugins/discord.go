package plugins

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/discord"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func discordInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	discordSpec := plugin.Spec.GetDiscord()
	if discordSpec == nil {
		return nil, trace.BadParameter("field Spec.Discord must be present")
	}

	if len(deps.staticCredentials) == 0 {
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
			Token: deps.staticCredentials[0].GetAPIToken(),
		},
		Client:     deps.client,
		StatusSink: deps.statusSink,
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "set discord defaults")
	}

	app := discord.NewApp(cfg)
	// TODO(tross): convert logger library to use slog
	appCtx := logger.WithLogger(deps.lifetime, logrus.New())
	return func() error {
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}

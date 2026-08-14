package factory

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/mattermost"
)

func Mattermost(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	mattermostSpec := plugin.Spec.GetMattermost()
	if mattermostSpec == nil {
		return nil, trace.BadParameter("field Spec.Mattermost must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("missing Mattermost plugin static credentials")
	}

	var recipients []string

	if len(mattermostSpec.Team) > 0 && len(mattermostSpec.Channel) > 0 {
		recipients = []string{fmt.Sprintf("%s/%s", mattermostSpec.Team, mattermostSpec.Channel)}
	}

	if mattermostSpec.ReportToEmail != "" {
		recipients = append(recipients, mattermostSpec.ReportToEmail)
	}

	pc := &pluginConfiguration{
		client: deps.Client,
		pluginConfig: &mattermost.Config{
			Mattermost: mattermost.MattermostConfig{
				URL:        mattermostSpec.ServerUrl,
				Token:      deps.StaticCredentials[0].GetAPIToken(),
				Recipients: recipients,
			},
			StatusSink: deps.StatusSink,
		},
		defaultRoutes: recipients,
		pluginType:    types.PluginTypeMattermost,
	}

	app := common.NewApp(pc, plugin.GetName())
	return func(ctx context.Context) error {
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}

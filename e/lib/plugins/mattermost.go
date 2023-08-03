package plugins

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/mattermost"
)

func mattermostInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	mattermostSpec := plugin.Spec.GetMattermost()
	if mattermostSpec == nil {
		return nil, trace.BadParameter("field Spec.Mattermost must be present")
	}

	if len(deps.staticCredentials) == 0 {
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
		client: deps.client,
		pluginConfig: &mattermost.Config{
			Mattermost: mattermost.MattermostConfig{
				URL:        mattermostSpec.ServerUrl,
				Token:      deps.staticCredentials[0].GetAPIToken(),
				Recipients: recipients,
			},
			StatusSink: deps.statusSink,
		},
		defaultRoutes: recipients,
		pluginType:    types.PluginTypeMattermost,
	}

	app := common.NewApp(pc, plugin.GetName())
	return func() error {
		err := app.Run(deps.lifetime)
		return trace.Wrap(err)
	}, nil
}

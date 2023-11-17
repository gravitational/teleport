package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/opsgenie"
)

func opsgenieInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	opsgenieSpec := plugin.Spec.GetOpsgenie()
	if opsgenieSpec == nil {
		return nil, trace.BadParameter("field Spec.Opsgenie must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	staticToken := deps.staticCredentials[0].GetAPIToken()
	if staticToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	opsgenieConfig := plugin.Spec.GetOpsgenie()
	pc := &pluginConfiguration{
		client: deps.client,
		pluginConfig: &opsgenie.Config{
			ClientConfig: opsgenie.ClientConfig{
				APIKey:           staticToken,
				APIEndpoint:      opsgenieConfig.ApiEndpoint,
				DefaultSchedules: opsgenieConfig.DefaultSchedules,
				Priority:         opsgenieConfig.Priority,
			},
			StatusSink: deps.statusSink,
		},
	}

	app := common.NewApp(pc, plugin.GetName())
	return func() error {
		err := app.Run(deps.lifetime)
		return trace.Wrap(err)
	}, nil
}

package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/opsgenie"
)

func OpsGenie(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	opsgenieSpec := plugin.Spec.GetOpsgenie()
	if opsgenieSpec == nil {
		return nil, trace.BadParameter("field Spec.Opsgenie must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	staticToken := deps.StaticCredentials[0].GetAPIToken()
	if staticToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	opsgenieConfig := plugin.Spec.GetOpsgenie()
	app, err := opsgenie.NewOpsgenieApp(ctx, &opsgenie.Config{
		ClientConfig: opsgenie.ClientConfig{
			APIKey:           staticToken,
			APIEndpoint:      opsgenieConfig.ApiEndpoint,
			DefaultSchedules: opsgenieConfig.DefaultSchedules,
			Priority:         opsgenieConfig.Priority,
			StatusSink:       deps.StatusSink,
		},
		BaseConfig: common.BaseConfig{
			PluginType: types.PluginTypeOpsgenie,
		},
		TeleportUserName: teleport.SystemAccessApproverUserName,
		Client:           deps.Client,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}

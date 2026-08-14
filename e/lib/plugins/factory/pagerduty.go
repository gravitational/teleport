package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/pagerduty"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func PagerDuty(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	pagerDutySpec := plugin.Spec.GetPagerDuty()
	if pagerDutySpec == nil {
		return nil, trace.BadParameter("field Spec.PagerDuty must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("missing PagerDuty plugin static credentials")
	}

	pdc := pagerduty.Config{
		Pagerduty: pagerduty.PagerdutyConfig{
			APIKey:      deps.StaticCredentials[0].GetAPIToken(),
			APIEndpoint: pagerDutySpec.ApiEndpoint,
			UserEmail:   pagerDutySpec.UserEmail,
		},
		Client:       deps.Client,
		StatusSink:   deps.StatusSink,
		TeleportUser: teleport.SystemAccessApproverUserName,
	}
	if err := pdc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := pagerduty.NewApp(pdc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		appCtx := logger.WithLogger(ctx, deps.Logger)
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}

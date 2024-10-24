package plugins

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/pagerduty"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func pagerDutyInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	pagerDutySpec := plugin.Spec.GetPagerDuty()
	if pagerDutySpec == nil {
		return nil, trace.BadParameter("field Spec.PagerDuty must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("missing PagerDuty plugin static credentials")
	}

	pdc := pagerduty.Config{
		Pagerduty: pagerduty.PagerdutyConfig{
			APIKey:      deps.staticCredentials[0].GetAPIToken(),
			APIEndpoint: pagerDutySpec.ApiEndpoint,
			UserEmail:   pagerDutySpec.UserEmail,
		},
		Client:       deps.client,
		StatusSink:   deps.statusSink,
		TeleportUser: teleport.SystemAccessApproverUserName,
	}
	if err := pdc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := pagerduty.NewApp(pdc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(tross): convert logger library to use slog
	appCtx := logger.WithLogger(deps.lifetime, logrus.New())
	return func() error {
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}

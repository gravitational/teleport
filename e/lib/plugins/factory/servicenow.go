package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/servicenow"
)

func ServiceNow(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	serviceNowSpec := plugin.Spec.GetServiceNow()

	if serviceNowSpec == nil {
		return nil, trace.BadParameter("field Spec.ServiceNowAccessPlugin must be present")
	}
	if serviceNowSpec.ApiEndpoint == "" {
		return nil, trace.BadParameter("field Spec.ServiceNowAccessPlugin.APIEndpoint must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	username, password := deps.StaticCredentials[0].GetBasicAuth()
	if username == "" || password == "" {
		return nil, trace.BadParameter("username or password empty")
	}

	if serviceNowSpec.CloseCode == "" {
		return nil, trace.BadParameter("ServiceNow close code must be set")
	}

	snc := &servicenow.Config{
		ClientConfig: servicenow.ClientConfig{
			APIEndpoint: serviceNowSpec.ApiEndpoint,
			Username:    username,
			APIToken:    password,
			CloseCode:   serviceNowSpec.CloseCode,
			StatusSink:  deps.StatusSink,
		},
		TeleportUser: teleport.SystemAccessApproverUserName,
		Client:       deps.Client,
	}

	app, err := servicenow.NewServiceNowApp(ctx, snc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}

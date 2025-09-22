package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/datadog"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func Datadog(_ context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	datadogSpec := plugin.Spec.GetDatadog()
	if datadogSpec == nil {
		return nil, trace.BadParameter("field Spec.Datadog must be present")
	}

	apiToken, err := selectCredentials(deps.StaticCredentials, types.DatadogCredentialLabel, types.DatadogCredentialAPIKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applicationToken, err := selectCredentials(deps.StaticCredentials, types.DatadogCredentialLabel, types.DatadogCredentialApplicationKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := &datadog.Config{
		BaseConfig: common.BaseConfig{
			PluginType: types.PluginTypeDatadog,
			Recipients: common.RawRecipientsMap{
				types.Wildcard: []string{datadogSpec.FallbackRecipient},
			},
			TeleportUser: teleport.SystemAccessApproverUserName,
		},
		Datadog: datadog.DatadogConfig{
			APIEndpoint:    datadogSpec.ApiEndpoint,
			APIKey:         apiToken.GetAPIToken(),
			ApplicationKey: applicationToken.GetAPIToken(),
		},
		StatusSink: deps.StatusSink,
		Client:     deps.Client,
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	app := datadog.NewDatadogApp(cfg)

	return func(ctx context.Context) error {
		appCtx := logger.WithLogger(ctx, deps.Logger)
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}

// selectCredentials returns the credentials with the matching credential label and type values.
func selectCredentials(staticCredentials []types.PluginStaticCredentials, credentialLabel, credentialType string) (types.PluginStaticCredentials, error) {
	for _, cred := range staticCredentials {
		value, present := cred.GetLabel(credentialLabel)
		if present && value == credentialType {
			return cred, nil
		}
	}
	return nil, trace.NotFound("%q credentials not found", credentialType)
}

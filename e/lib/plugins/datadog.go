package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/datadog"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func datadogInstanceFactory(_ context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	datadogSpec := plugin.Spec.GetDatadog()
	if datadogSpec == nil {
		return nil, trace.BadParameter("field Spec.Datadog must be present")
	}

	apiToken, err := selectCredentials(deps.staticCredentials, types.DatadogCredentialLabel, types.DatadogCredentialAPIKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applicationToken, err := selectCredentials(deps.staticCredentials, types.DatadogCredentialLabel, types.DatadogCredentialApplicationKey)
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
		StatusSink: deps.statusSink,
		Client:     deps.client,
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	app := datadog.NewDatadogApp(cfg)
	// TODO(tross): convert logger library to use slog
	appCtx := logger.WithLogger(deps.lifetime, nil)
	return func() error {
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

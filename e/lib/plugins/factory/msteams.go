package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/msteams"
	"github.com/gravitational/teleport/integrations/access/msteams/msapi"
)

func MSTeams(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	msTeamsSpec := plugin.Spec.GetMsteams()
	if msTeamsSpec == nil {
		return nil, trace.BadParameter("field Spec.MsTeams must be present")
	}

	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	staticToken := deps.StaticCredentials[0].GetAPIToken()
	if staticToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	app, err := msteams.NewApp(msteams.Config{
		MSAPI: msapi.Config{
			AppID:      msTeamsSpec.AppId,
			AppSecret:  staticToken,
			TenantID:   msTeamsSpec.TenantId,
			Region:     msTeamsSpec.Region,
			TeamsAppID: msTeamsSpec.TeamsAppId,
		},
		BaseConfig: common.BaseConfig{
			Recipients: common.RawRecipientsMap{
				"*": []string{msTeamsSpec.DefaultRecipient},
			},
		},
		StatusSink: deps.StatusSink,
		Client:     deps.Client,
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}

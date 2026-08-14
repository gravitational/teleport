package factory

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	githubservice "github.com/gravitational/teleport/e/lib/accessgraph/github"
	"github.com/gravitational/teleport/e/lib/services"
)

// GitHub creates a Github service based on the plugin specification.
func GitHub(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	privKey := deps.StaticCredentials[0].GetPrivateKey()
	if len(privKey) == 0 {
		return nil, trace.BadParameter("private_key is empty")
	}

	githubSettings := plugin.Spec.GetGithub()
	if githubSettings == nil {
		return nil, trace.BadParameter("field Spec.Github must be present")
	}

	cfg := githubservice.GithubConfig{
		Address:            githubSettings.ApiEndpoint,
		ClientID:           githubSettings.ClientId,
		PrivateKey:         privKey,
		Organization:       githubSettings.OrganizationName,
		BootstrapStartDate: githubSettings.StartDate,
	}

	return func(ctx context.Context) error {
		closeEvent, err := services.GithubPluginInit(ctx, deps.ParentProcess, deps.StatusSink, cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		// wait for the calling context to finish before doing anything else.
		<-ctx.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.ParentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.Logger.DebugContext(ctx, "Error waiting for Github event", "error", err)
			return trace.Wrap(err)
		}

		deps.Logger.InfoContext(ctx, "Github plugin has stopped")
		return nil
	}, nil
}

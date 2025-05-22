package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	githubservice "github.com/gravitational/teleport/e/lib/accessgraph/github"
	"github.com/gravitational/teleport/e/lib/services"
)

// githubInstanceFactory creates a Github service based on the plugin specification.
func githubInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	privKey := deps.staticCredentials[0].GetPrivateKey()
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

	return func() error {
		closeEvent, err := services.GithubPluginInit(deps.lifetime, deps.parentProcess, deps.statusSink, cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		// wait for the calling context to finish before doing anything else.
		<-deps.lifetime.Done()

		// Wait 5 seconds for the close event.
		eventCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = deps.parentProcess.WaitForEvent(eventCtx, closeEvent)
		if err != nil {
			deps.logger.DebugContext(ctx, "Error waiting for Github event", "error", err)
			return trace.Wrap(err)
		}

		deps.logger.InfoContext(ctx, "Github plugin has stopped")
		return nil
	}, nil
}

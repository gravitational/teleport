package plugins

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	gitlabservice "github.com/gravitational/teleport/e/lib/accessgraph/gitlab"
	"github.com/gravitational/teleport/e/lib/services"
)

// gitlabInstanceFactory creates a Gitlab service based on the plugin specification.
func gitlabInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	token := deps.staticCredentials[0].GetAPIToken()
	if token == "" {
		return nil, trace.BadParameter("token is empty")
	}

	gitlabSettings := plugin.Spec.GetGitlab()
	if gitlabSettings == nil {
		return nil, trace.BadParameter("field Spec.Gitlab must be present")
	}

	cfg := gitlabservice.GitlabOpts{
		Token:   token,
		Address: gitlabSettings.ApiEndpoint,
	}

	return func() error {
		closeEvent, err := services.GitlabPluginInit(deps.lifetime, deps.parentProcess, deps.statusSink, cfg)
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
			deps.logger.DebugContext(ctx, "Error waiting for GitlabStopped event", "error", err)
			return trace.Wrap(err)
		}

		deps.logger.InfoContext(ctx, "Gitlab plugin has stopped")
		return nil
	}, nil
}

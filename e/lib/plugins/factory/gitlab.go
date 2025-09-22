package factory

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	gitlabservice "github.com/gravitational/teleport/e/lib/accessgraph/gitlab"
	"github.com/gravitational/teleport/e/lib/services"
)

// GitLab creates a Gitlab service based on the plugin specification.
func GitLab(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error) {
	if len(deps.StaticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	token := deps.StaticCredentials[0].GetAPIToken()
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

	return func(ctx context.Context) error {
		closeEvent, err := services.GitlabPluginInit(ctx, deps.ParentProcess, deps.StatusSink, cfg)
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
			deps.Logger.DebugContext(ctx, "Error waiting for GitlabStopped event", "error", err)
			return trace.Wrap(err)
		}

		deps.Logger.InfoContext(ctx, "Gitlab plugin has stopped")
		return nil
	}, nil
}

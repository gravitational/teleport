package services

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	gitlabservice "github.com/gravitational/teleport/e/lib/accessgraph/gitlab"
	ent "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/service"
)

const (
	gitlabIdentityEvent = "GitlabIdentity"
	// GitlabReadyEvent is generated when the Gitlab service is started.
	GitlabReadyEvent = "GitlabReady"
	// GitlabStoppedEvent is generated when the Gitlab service is stopped.
	GitlabStoppedEvent = "GitlabStopped"
)

func startGitlabService(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, gitlabOpts gitlabservice.GitlabOpts) error {
	// Register our request for AccessGraphPlugin credentials.
	process.RegisterWithAuthServer(types.RoleAccessGraphPlugin, gitlabIdentityEvent)

	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(ent.ComponentGitlab, process.GetID()))

	// Wait for Gitlab credentials.
	conn, err := process.WaitForConnector(gitlabIdentityEvent, logger)
	if err != nil {
		return trace.Wrap(err)
	}

	if conn == nil {
		// Is the server shutting down? Report back.
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("failed to acquire AccessGraphPlugin credentials from Auth")
	}

	s, err := gitlabservice.New(ctx, gitlabservice.Opts{
		GitlabOpts:        gitlabOpts,
		Clock:             process.Clock,
		Logger:            logger,
		AccessGraphConfig: process.Config.AccessGraph,
		HostID:            process.Config.HostUUID,
		GetCreds:          conn.ClientGetCertificate,
		AccessPoint:       conn.Client,
		ClusterFeatures:   process.GetClusterFeatures,
		PluginStatusSink:  statusSink,
	})
	if err != nil {
		// Update plugin status if the service is running as a plugin.
		if statusSink != nil {
			code := types.PluginStatusCode_OTHER_ERROR
			if errors.Is(err, gitlabservice.ErrGitlabInvalidCredentials) {
				code = types.PluginStatusCode_UNAUTHORIZED
			}
			statusSink.Emit(
				ctx,
				&types.PluginStatusV1{
					Code:         code,
					LastSyncTime: process.Clock.Now(),
					ErrorMessage: gitlabservice.GitlabMessageOrError(err),
					Details: &types.PluginStatusV1_Gitlab{
						Gitlab: &types.PluginGitlabStatusV1{},
					},
				},
			)

		}

		return trace.Wrap(err)
	}

	// Update plugin status if the service is running as a plugin.
	if statusSink != nil {
		statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})
	}

	// Broadcast that we are ready and start.
	process.BroadcastEvent(service.Event{Name: GitlabReadyEvent})
	err = s.Run(ctx)

	process.BroadcastEvent(service.Event{Name: GitlabStoppedEvent})
	return trace.Wrap(err)
}

// GitlabPluginInit initializes hosted Gitab service (hosted plugin).
// Returns immediately.
func GitlabPluginInit(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, gitlabSpec gitlabservice.GitlabOpts) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Set the expected instance role for this identity event since it's unique to this plugin.
	process.SetExpectedInstanceRole(types.RoleAccessGraphPlugin, gitlabIdentityEvent)

	// We don't want auth process to exit due to faulty gitlab config.
	process.RegisterFunc("gitlab.init", func() error {
		return startGitlabService(ctx, process, statusSink, gitlabSpec)
	})

	return EventWithComponents(GitlabStoppedEvent), nil
}

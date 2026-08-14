package services

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	githubservice "github.com/gravitational/teleport/e/lib/accessgraph/github"
	ent "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/service"
)

const (
	githubIdentityEvent = "GithubIdentity"
	// GithubReadyEvent is generated when the Github service is started.
	GithubReadyEvent = "GithubReady"
	// GithubStoppedEvent is generated when the Github service is stopped.
	GithubStoppedEvent = "GithubStopped"
)

func startGithubService(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, githubOpts githubservice.GithubConfig) error {
	// Register our request for AccessGraphPlugin credentials.
	process.RegisterWithAuthServer(types.RoleAccessGraphPlugin, githubIdentityEvent)

	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(ent.ComponentGithub, process.GetID()))

	// Wait for Github credentials.
	conn, err := process.WaitForConnector(githubIdentityEvent, logger)
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

	s, err := githubservice.New(ctx, githubservice.Config{
		GithubConfig:      githubOpts,
		Clock:             process.Clock,
		Logger:            logger,
		AccessGraphConfig: process.Config.AccessGraph,
		HostID:            conn.HostUUID(),
		GetCreds:          conn.ClientGetCertificate,
		AccessPoint:       conn.Client,
		ClusterFeatures:   process.GetClusterFeatures,
		PluginStatusSink:  statusSink,
	})
	if err != nil {
		// Update plugin status if the service is running as a plugin.
		if statusSink != nil {
			code := types.PluginStatusCode_OTHER_ERROR
			if errors.Is(err, githubservice.ErrGithubInvalidCredentials) {
				code = types.PluginStatusCode_UNAUTHORIZED
			}
			statusSink.Emit(
				ctx,
				&types.PluginStatusV1{
					Code:         code,
					LastSyncTime: process.Clock.Now(),
					ErrorMessage: err.Error(),
					LastRawError: err.Error(),
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
	process.BroadcastEvent(service.Event{Name: GithubReadyEvent})
	err = s.Run(ctx)

	process.BroadcastEvent(service.Event{Name: GithubStoppedEvent})
	return trace.Wrap(err)
}

// GithubPluginInit initializes hosted Github service (hosted plugin).
// Returns immediately.
func GithubPluginInit(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, githubOpts githubservice.GithubConfig) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Set the expected instance role for this identity event since it's unique to this plugin.
	process.SetExpectedInstanceRole(types.RoleAccessGraphPlugin, githubIdentityEvent)

	// We don't want auth process to exit due to faulty github config.
	process.RegisterFunc("github.init", func() error {
		return startGithubService(ctx, process, statusSink, githubOpts)
	})

	return EventWithComponents(GithubStoppedEvent), nil
}

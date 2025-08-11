package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/intune"
	"github.com/gravitational/teleport/e/lib/intune/api"
	ent "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/service"
)

const (
	// IntuneReadyEvent is generated when the Intune service is started.
	IntuneReadyEvent = "IntuneReady"
	// IntuneStoppedEvent is generated when the Intune service is stopped.
	IntuneStoppedEvent = "IntuneStopped"
)

// IntunePluginInit starts the Intune plugin and orchestrates its lifecycle.
// It immediately returns the name of the event that's going to be emitted when the plugin stops.
func IntunePluginInit(ctx context.Context, process *service.TeleportProcess, httpClient *http.Client, statusSink common.StatusSink, apiConfig api.Config) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	process.SetExpectedHostedPluginRole(types.RoleMDM, mdmIdentityEvent)

	process.RegisterFunc("intune.init", func() error {
		return startIntuneService(ctx, process, httpClient, statusSink, apiConfig)
	})

	return EventWithComponents(IntuneStoppedEvent), nil
}

func startIntuneService(ctx context.Context, process *service.TeleportProcess, httpClient *http.Client, statusSink common.StatusSink, apiConfig api.Config) error {
	process.RegisterWithAuthServer(types.RoleMDM, mdmIdentityEvent)

	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(ent.ComponentIntune, process.GetID()))

	// Wait for MDM credentials.
	conn, err := process.WaitForConnector(mdmIdentityEvent, logger)
	if err != nil {
		return trace.Wrap(err)
	}
	if conn == nil {
		// Is the server shutting down? Report back.
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("failed to acquire MDM credentials from Auth")
	}

	s, err := intune.NewService(ctx, intune.Config{
		APIConfig:  apiConfig,
		Logger:     logger,
		HTTPClient: httpClient,
	})
	if err != nil {
		code := types.PluginStatusCode_OTHER_ERROR
		if errors.Is(err, api.ErrIntuneClientInvalidCredentials) ||
			errors.Is(err, api.ErrIntuneClientTenantNotFound) {
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

		return trace.Wrap(err)
	}

	statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})

	// Broadcast that we are ready and start.
	process.BroadcastEvent(service.Event{Name: IntuneReadyEvent})
	err = s.Run(ctx)

	process.BroadcastEvent(service.Event{Name: IntuneStoppedEvent})
	return trace.Wrap(err)
}

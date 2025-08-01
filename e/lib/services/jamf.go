package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	jamf "github.com/gravitational/teleport/e/lib/jamf"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	ent "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	// JamfReadyEvent is generated when the Jamf service is started.
	JamfReadyEvent = "JamfReady"
	// JamfStoppedEvent is generated when the Jamf service is stopped.
	JamfStoppedEvent = "JamfStopped"
)

// JamfRegister adds additional roles and ready events expected by the Jamf
// service to `cfg`.
func JamfRegister(cfg *servicecfg.Config) {
	if cfg == nil || !cfg.Jamf.Enabled() {
		return
	}

	cfg.AdditionalExpectedRoles = append(cfg.AdditionalExpectedRoles, servicecfg.RoleAndIdentityEvent{
		Role:          types.RoleMDM,
		IdentityEvent: mdmIdentityEvent,
	})
	cfg.AdditionalReadyEvents = append(cfg.AdditionalReadyEvents, JamfReadyEvent)
}

// JamfStandaloneInit initializes standalone Jamf service.
// Use [JamfPluginInit] to run the Jamf service as a hosted plugin.
// Returns immediately.
func JamfStandaloneInit(process *service.TeleportProcess, httpClient *http.Client) error {
	if process == nil {
		return trace.BadParameter("process required")
	}

	// When running as a standalone service, we want process to exit on faulty config.
	process.RegisterCriticalFunc("jamf.init", func() error {
		ctx, cancel := context.WithCancel(process.ExitContext())
		defer cancel()
		return startJamfService(ctx, process, httpClient, nil /* plugin statusSink */)
	})
	return nil
}

func startJamfService(ctx context.Context, process *service.TeleportProcess, httpClient *http.Client, statusSink common.StatusSink) error {
	// Register our request for MDM credentials.
	process.RegisterWithAuthServer(types.RoleMDM, mdmIdentityEvent)

	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(ent.ComponentJamf, process.GetID()))

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

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 1 * time.Minute,
		}
	}

	s, err := jamfservice.New(ctx, jamfservice.Opts{
		Clock:            process.Clock,
		Logger:           process.Config.Logger.With(teleport.ComponentKey, teleport.Component(ent.ComponentJamf, process.GetID())),
		Config:           &process.Config.Jamf,
		DevicesClient:    conn.Client.DevicesClient(),
		HTTPClient:       httpClient,
		PluginStatusSink: statusSink,
	})
	if err != nil {
		// Update plugin status if the service is running as a plugin.
		if statusSink != nil {
			switch {
			case errors.Is(err, jamf.ErrJamfClientInvalidCredential):
				statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_UNAUTHORIZED})
			default:
				statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
			}
		}

		return trace.Wrap(err)
	}

	// Update plugin status if the service is running as a plugin.
	if statusSink != nil {
		statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})
	}

	// Broadcast that we are ready and start.
	process.BroadcastEvent(service.Event{Name: JamfReadyEvent})
	// The Jamf service doesn't have heartbeats so we cannot use them to check health.
	// For now, we just mark ourselves ready all the time on startup.
	// If we don't, a process only running the Jamf service will never report ready.
	process.OnHeartbeat(ent.ComponentJamf)(nil /* err */)

	err = s.Run(ctx)
	// err returned below.

	// Trigger exit_on_sync mechanism?
	if process.Config.Jamf.ExitOnSync {
		go func() {
			logger.InfoContext(process.ExitContext(), "Signaling shutdown to Teleport process [exit_on_sync=true]")

			// Attempt a graceful shutdown first...
			ctx := context.Background()
			process.Shutdown(ctx)

			// ... and follow up with a hard shutdown.
			// The Close is necessary, the process won't stop without it.
			process.Close()
		}()
	}

	process.BroadcastEvent(service.Event{Name: JamfStoppedEvent})
	return trace.Wrap(err)
}

// JamfPluginInit initializes hosted Jamf service (hosted plugin).
// Use [JamfStandaloneInit] to run the Jamf service as a standalone service.
// Returns immediately.
func JamfPluginInit(ctx context.Context, process *service.TeleportProcess, httpClient *http.Client, statusSink common.StatusSink, jamfSpec *types.JamfSpecV1, credentials *servicecfg.JamfCredentials) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Add Jamf spec to process config.
	process.Config.Jamf = servicecfg.JamfConfig{
		Spec:        jamfSpec,
		Credentials: credentials,
	}

	process.SetExpectedHostedPluginRole(types.RoleMDM, mdmIdentityEvent)

	// We don't want auth process to exit due to faulty jamf config.
	process.RegisterFunc("jamf.init", func() error {
		return startJamfService(ctx, process, httpClient, statusSink)
	})

	return EventWithComponents(JamfStoppedEvent), nil
}

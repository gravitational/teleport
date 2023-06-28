package services

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	ent "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	jamfIdentityEvent = "JamfIdentity"
	jamfReadyEvent    = "JamfReady"
)

// JamfRegister adds additional roles and ready events expected by the Jamf
// service to `cfg`.
func JamfRegister(cfg *servicecfg.Config) {
	if cfg == nil || !cfg.Jamf.Enabled() {
		return
	}

	cfg.AdditionalExpectedRoles = append(cfg.AdditionalExpectedRoles, servicecfg.RoleAndIdentityEvent{
		Role:          types.RoleMDM,
		IdentityEvent: jamfIdentityEvent,
	})
	cfg.AdditionalReadyEvents = append(cfg.AdditionalReadyEvents, jamfReadyEvent)
}

// JamfInit registers the necessary critical functions for the Jamf service
// within the [service.TeleportProcess].
// Returns immediately.
func JamfInit(process *service.TeleportProcess, httpClient *http.Client) error {
	if process == nil {
		return trace.BadParameter("process required")
	}

	// Register our request for MDM credentials.
	process.RegisterWithAuthServer(types.RoleMDM, jamfIdentityEvent)

	// Register Jamf initialization.
	process.RegisterCriticalFunc("jamf.init", func() error {
		ctx, cancel := context.WithCancel(process.ExitContext())
		defer cancel()

		logger := process.Config.Log.WithField(
			trace.Component,
			teleport.Component(ent.ComponentJamf, process.GetID()),
		)

		// Wait for MDM credentials.
		conn, err := process.WaitForConnector(jamfIdentityEvent, logger)
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
				Timeout: 5 * time.Minute,
			}
		}
		s, err := jamfservice.New(ctx, jamfservice.Opts{
			Clock:         process.Clock,
			Logger:        logger,
			Config:        &process.Config.Jamf,
			DevicesClient: conn.Client.DevicesClient(),
			HTTPClient:    httpClient,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Broadcast that we are ready and start.
		process.BroadcastEvent(service.Event{Name: jamfReadyEvent, Payload: nil})
		err = s.Run(ctx)
		// err returned below.

		// Trigger exit_on_sync mechanism?
		if process.Config.Jamf.ExitOnSync {
			go func() {
				logger.Info("Signaling shutdown to Teleport process [exit_on_sync=true]")

				// Attempt a graceful shutdown first...
				ctx := context.Background()
				process.Shutdown(ctx)

				// ... and follow up with a hard shutdown.
				// The Close is necessary, the process won't stop without it.
				process.Close()
			}()
		}

		return trace.Wrap(err)
	})
	return nil
}

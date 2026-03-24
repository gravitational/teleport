package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	netiqservice "github.com/gravitational/teleport/e/lib/netiq/service"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/service"
)

const (
	netIQIDIdentityEvent = "NetIQIdentity"
	// NetIQReadyEvent is generated when the NetIQ service is started.
	NetIQReadyEvent = "NetIQReady"
	// NetIQStoppedEvent is generated when the NetIQ service is stopped.
	NetIQStoppedEvent = "NetIQStopped"
)

func startnetIQService(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginNetIQSettings, credentials []types.PluginStaticCredentials) error {
	logger := process.Config.Logger.With(teleport.ComponentKey, teleport.Component(eteleport.ComponentNetIQ, process.GetID()))
	features := process.Config.Modules.Features()
	if !features.GetEntitlement(entitlements.Policy).Enabled {
		logger.ErrorContext(ctx, "NetIQ service requires Teleport Identity Security.")
		return nil
	}

	// Register our request for AccessGraphPlugin credentials.
	process.RegisterWithAuthServer(types.RoleAccessGraphPlugin, netIQIDIdentityEvent)

	authServer := process.GetAuthServer()

	// Wait for NetIQ credentials.
	conn, err := process.WaitForConnector(netIQIDIdentityEvent, logger)
	if err != nil {
		// Update plugin status if the service is running as a plugin.
		if statusSink != nil {
			statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
		}
		return trace.Wrap(err)
	}

	if conn == nil {
		// Update plugin status if the service is running as a plugin.
		if statusSink != nil {
			statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_OTHER_ERROR})
		}
		// Is the server shutting down? Report back.
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("failed to acquire AccessGraphPlugin credentials from Auth")
	}

	oAuthCreds, identityVaultCreds, err := getNetIQCreds(credentials)
	if err != nil {
		return trace.Wrap(err, "failed to get NetIQ credentials")
	}

	svc, err := netiqservice.New(
		ctx,
		netiqservice.Config{
			Logger:           logger,
			PluginStatusSink: statusSink,
			SemaphoreSvc:     authServer,
			HostID:           conn.HostUUID(),
			Clock:            process.Clock,
			ClientConfig: netiqservice.ClientConfig{
				OAuthClientID:         oAuthCreds.user,
				OAuthClientSecret:     oAuthCreds.secret,
				OSPURL:                spec.OauthIssuerEndpoint,
				APIURL:                spec.ApiEndpoint,
				IdentityVaultUser:     identityVaultCreds.user,
				IdentityVaultPassword: identityVaultCreds.secret,
				InsecureSkipVerify:    spec.InsecureSkipVerify,
			},
			AccessGraphConfig: process.Config.AccessGraph,
			GetCreds:          conn.ClientGetCertificate,
			ClusterFeatures:   process.GetClusterFeatures,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	// Update plugin status if the service is running as a plugin.
	if statusSink != nil {
		statusSink.Emit(ctx, &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING})
	}

	// Broadcast that we are ready and start.
	process.BroadcastEvent(service.Event{Name: NetIQReadyEvent})
	err = svc.Run(ctx)

	process.BroadcastEvent(service.Event{Name: NetIQStoppedEvent})
	return trace.Wrap(err)
}

// NetIQPluginInit initializes hosted NetQI service (hosted plugin).
// Returns immediately.
func NetIQPluginInit(ctx context.Context, process *service.TeleportProcess, statusSink common.StatusSink, spec *types.PluginNetIQSettings, credentials []types.PluginStaticCredentials) (string, error) {
	if process == nil {
		return "", trace.BadParameter("process required")
	}

	// Set the expected instance role for this identity event since it's unique to this plugin.
	process.SetExpectedInstanceRole(types.RoleAccessGraphPlugin, netIQIDIdentityEvent)

	process.RegisterFunc("netiq.init", func() error {
		return startnetIQService(ctx, process, statusSink, spec, credentials)
	})

	return EventWithComponents(NetIQStoppedEvent), nil
}

type netIQCredentials struct {
	user   string
	secret string
}

func getNetIQCreds(credentials []types.PluginStaticCredentials) (oauth, identityVault netIQCredentials, err error) {
	for _, cred := range credentials {
		if basicUser, basicSecret := cred.GetBasicAuth(); basicUser != "" && basicSecret != "" {
			identityVault.user = basicUser
			identityVault.secret = basicSecret
			continue
		} else if basicUser != "" || basicSecret != "" {
			return netIQCredentials{}, netIQCredentials{}, trace.BadParameter("both basic user and secret must be provided")
		}

		if clientID, clientSecret := cred.GetOAuthClientSecret(); clientID != "" && clientSecret != "" {
			oauth.user = clientID
			oauth.secret = clientSecret
			continue
		} else if clientID != "" || clientSecret != "" {
			return netIQCredentials{}, netIQCredentials{}, trace.BadParameter("both client ID and secret must be provided")
		}
	}

	if oauth.user == "" || oauth.secret == "" {
		return netIQCredentials{}, netIQCredentials{}, trace.BadParameter("OAuth client ID and secret must be provided")
	}

	if identityVault.user == "" || identityVault.secret == "" {
		return netIQCredentials{}, netIQCredentials{}, trace.BadParameter("Identity Vault user and password must be provided")
	}

	return oauth, identityVault, nil
}

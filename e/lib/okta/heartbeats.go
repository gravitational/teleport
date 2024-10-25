package okta

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// startHeartbeat starts the registration heartbeat to the auth server.
func (s *Service) startHeartbeat(ctx context.Context, app types.Application) error {
	appName := app.GetName()
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         ctx,
		Component:       eteleport.ComponentOkta,
		Mode:            srv.HeartbeatModeApp,
		Announcer:       s.accessPoint,
		GetServerInfo:   s.getServerInfoFunc(appName),
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     s.onHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := heartbeat.Run(); err != nil {
			s.logger.DebugContext(ctx, "Error running heartbeat", "error", err)
		}
	}()
	s.heartbeatsMu.Lock()
	defer s.heartbeatsMu.Unlock()
	s.heartbeats[appName] = heartbeat
	return nil
}

// stopHeartbeat stops the heartbeat for the specified app.
func (s *Service) stopHeartbeat(name string) error {
	s.heartbeatsMu.Lock()
	defer s.heartbeatsMu.Unlock()
	heartbeat, ok := s.heartbeats[name]
	if !ok {
		return nil
	}
	delete(s.heartbeats, name)
	return trace.Wrap(heartbeat.Close())
}

// getServerInfoFunc returns function that the heartbeater uses to report the
// provided app to the auth server.
func (s *Service) getServerInfoFunc(name string) func() (types.Resource, error) {
	return func() (types.Resource, error) {
		return s.getServerInfo(name)
	}
}

func (s *Service) getServerInfo(name string) (types.Resource, error) {
	// check for app in memory
	originalApp, ok := s.apps.Load(name)

	if !ok {
		return nil, trace.NotFound("unable to find app %s", name)
	}
	app := originalApp.Copy()

	expires := s.clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)
	appServer, err := types.NewAppServerV3(
		types.Metadata{
			Name:        app.GetName(),
			Description: app.GetDescription(),
			Labels:      app.GetStaticLabels(),
			Expires:     &expires,
		},
		types.AppServerSpecV3{
			Version:  teleport.Version,
			Hostname: s.hostname,
			HostID:   s.hostID,
			Rotation: s.getRotationState(),
			App:      app,
			ProxyIDs: s.proxyGetter.GetProxyIDs(),
		},
	)
	if err != nil {
		s.logger.ErrorContext(context.Background(), "Error getting server info", "error", err)
	}
	return appServer, trace.Wrap(err)
}

// getRotationState is a helper to return this server's CA rotation state.
func (s *Service) getRotationState() types.Rotation {
	rotation, err := s.rotationGetter(types.RoleOkta)
	if err != nil && !trace.IsNotFound(err) {
		s.logger.WarnContext(context.Background(), "Failed to get rotation state", "error", err)
	}
	if rotation != nil {
		return *rotation
	}
	return types.Rotation{}
}

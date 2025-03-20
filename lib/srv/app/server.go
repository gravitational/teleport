/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package app runs the application proxy process. It keeps dynamic labels
// updated, heart beats its presence, checks access controls, and forwards
// connections between the tunnel and the target host.
package app

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

type appServerContextKey string

const (
	connContextKey appServerContextKey = "teleport-connContextKey"
)

// Config is the configuration for an application server.
type Config struct {
	// Clock is used to control time.
	Clock clockwork.Clock

	// AuthClient is a client directly connected to the Auth server.
	AuthClient *authclient.Client

	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.AppsAccessPoint

	// Hostname is the hostname where this application agent is running.
	Hostname string

	// HostID is the id of the host where this application agent is running.
	HostID string

	// GetRotation returns the certificate rotation state.
	GetRotation services.RotationGetter

	// Apps is a list of statically registered apps this agent proxies.
	Apps types.Apps

	// CloudLabels is a service that imports labels from a cloud provider. The labels are shared
	// between all apps.
	CloudLabels labels.Importer

	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)

	// ResourceMatchers is a list of app resource matchers.
	ResourceMatchers []services.ResourceMatcher

	// OnReconcile is called after each database resource reconciliation.
	OnReconcile func(types.Apps)

	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter *reversetunnel.ConnectedProxyGetter

	// ConnectionsHandler handles the HTTP/TCP App proxy connections.
	ConnectionsHandler *ConnectionsHandler

	// InventoryHandle is used to send app server heartbeats via the inventory control stream.
	InventoryHandle inventory.DownstreamHandle
}

// CheckAndSetDefaults makes sure the configuration has the minimum required
// to function.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.AuthClient == nil {
		return trace.BadParameter("auth client log missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if c.Hostname == "" {
		return trace.BadParameter("hostname missing")
	}
	if c.HostID == "" {
		return trace.BadParameter("host id missing")
	}
	if c.GetRotation == nil {
		return trace.BadParameter("rotation getter missing")
	}
	if c.OnHeartbeat == nil {
		return trace.BadParameter("heartbeat missing")
	}
	if c.ConnectionsHandler == nil {
		return trace.BadParameter("connections handler missing")
	}
	if c.ConnectedProxyGetter == nil {
		c.ConnectedProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}
	return nil
}

// Server is an application server. It authenticates requests from the web
// proxy and forwards th to internal applications.
type Server struct {
	c         *Config
	legacyLog *logrus.Entry
	log       *slog.Logger

	closeContext context.Context
	closeFunc    context.CancelFunc

	mu            sync.RWMutex
	heartbeats    map[string]srv.HeartbeatI
	dynamicLabels map[string]*labels.Dynamic

	// apps are all apps this server currently proxies. Proxied apps are
	// reconciled against monitoredApps below.
	apps map[string]types.Application
	// monitoredApps contains all cluster apps the proxied apps are
	// reconciled against.
	monitoredApps monitoredApps
	// reconcileCh triggers reconciliation of proxied apps.
	reconcileCh chan struct{}

	// watcher monitors changes to application resources.
	watcher *services.GenericWatcher[types.Application, readonly.Application]
}

// monitoredApps is a collection of applications from different sources
// like configuration file and dynamic resources.
//
// It's updated by respective watchers and is used for reconciling with the
// currently proxied apps.
type monitoredApps struct {
	// static are apps from the agent's YAML configuration.
	static types.Apps
	// resources are apps created via CLI or API.
	resources types.Apps
	// mu protects access to the fields.
	mu sync.Mutex
}

func (m *monitoredApps) setResources(apps types.Apps) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = apps
}

func (m *monitoredApps) get() map[string]types.Application {
	m.mu.Lock()
	defer m.mu.Unlock()
	return utils.FromSlice(append(m.static, m.resources...), types.Application.GetName)
}

// New returns a new application server.
func New(ctx context.Context, c *Config) (*Server, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeFunc := context.WithCancel(ctx)
	// in case of errors cancel context to avoid context leak
	callClose := true
	defer func() {
		if callClose {
			closeFunc()
		}
	}()

	s := &Server{
		c: c,
		// TODO(greedy52) replace with slog from Config.Logger.
		legacyLog: logrus.WithFields(logrus.Fields{
			teleport.ComponentKey: teleport.ComponentApp,
		}),
		log:           slog.With(teleport.ComponentKey, teleport.ComponentApp),
		heartbeats:    make(map[string]srv.HeartbeatI),
		dynamicLabels: make(map[string]*labels.Dynamic),
		apps:          make(map[string]types.Application),
		monitoredApps: monitoredApps{
			static: c.Apps,
		},
		reconcileCh:  make(chan struct{}),
		closeFunc:    closeFunc,
		closeContext: closeContext,
	}

	s.c.ConnectionsHandler.SetApplicationsProvider(s.GetAppByPublicAddress)

	callClose = false
	return s, nil
}

// startApp registers the specified application.
func (s *Server) startApp(ctx context.Context, app types.Application) error {
	// Start a goroutine that will be updating apps's command labels (if any)
	// on the defined schedule.
	if err := s.startDynamicLabels(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	// Heartbeat will periodically report the presence of this proxied app
	// to the auth server.
	if err := s.startHeartbeat(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "App started.", "app", app)
	return nil
}

// stopApp uninitializes the app with the specified name.
func (s *Server) stopApp(ctx context.Context, name string) error {
	s.stopDynamicLabels(name)
	if err := s.stopHeartbeat(name); err != nil {
		return trace.Wrap(err)
	}
	s.log.DebugContext(ctx, "App stopped.", "app", name)
	return nil
}

// removeAppServer deletes app server for the specified app.
func (s *Server) removeAppServer(ctx context.Context, name string) error {
	return s.c.AuthClient.DeleteApplicationServer(ctx, apidefaults.Namespace,
		s.c.HostID, name)
}

// stopAndRemoveApp uninitializes and deletes the app with the specified name.
func (s *Server) stopAndRemoveApp(ctx context.Context, name string) error {
	if err := s.stopApp(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	// Heartbeat is stopped but if we don't remove this app server,
	// it can linger for up to ~10m until its TTL expires.
	if err := s.removeAppServer(ctx, name); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// startDynamicLabels starts dynamic labels for the app if it has them.
func (s *Server) startDynamicLabels(ctx context.Context, app types.Application) error {
	if len(app.GetDynamicLabels()) == 0 {
		return nil // Nothing to do.
	}
	dynamic, err := labels.NewDynamic(ctx, &labels.DynamicConfig{
		Labels: app.GetDynamicLabels(),
		Log:    s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	dynamic.Sync()
	dynamic.Start()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dynamicLabels[app.GetName()] = dynamic
	return nil
}

// stopDynamicLabels stops dynamic labels for the specified app.
func (s *Server) stopDynamicLabels(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dynamic, ok := s.dynamicLabels[name]
	if !ok {
		return
	}
	delete(s.dynamicLabels, name)
	dynamic.Close()
}

// startHeartbeat starts the registration heartbeat to the auth server.
func (s *Server) startHeartbeat(ctx context.Context, app types.Application) error {
	heartbeat, err := srv.NewAppServerHeartbeat(srv.HeartbeatV2Config[*types.AppServerV3]{
		InventoryHandle: s.c.InventoryHandle,
		GetResource:     s.getServerInfoFunc(app),
		OnHeartbeat:     s.c.OnHeartbeat,
		// Announcer is provided to allow falling back to non-ICS heartbeats if
		// the Auth server is older than the app service.
		// TODO(tross): DELETE IN 16.0.0
		Announcer: s.c.AccessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go heartbeat.Run()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeats[app.GetName()] = heartbeat
	return nil
}

// stopHeartbeat stops the heartbeat for the specified app.
func (s *Server) stopHeartbeat(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	heartbeat, ok := s.heartbeats[name]
	if !ok {
		return nil
	}
	delete(s.heartbeats, name)
	return heartbeat.Close()
}

// getServerInfoFunc returns function that the heartbeater uses to report the
// provided application to the auth server.
func (s *Server) getServerInfoFunc(app types.Application) func(context.Context) (*types.AppServerV3, error) {
	return func(context.Context) (*types.AppServerV3, error) {
		return s.getServerInfo(app)
	}
}

// getServerInfo returns up-to-date app resource.
func (s *Server) getServerInfo(app types.Application) (*types.AppServerV3, error) {
	// Make sure to return a new object, because it gets cached by
	// heartbeat and will always compare as equal otherwise.
	s.mu.RLock()
	copy := s.appWithUpdatedLabelsLocked(app)
	s.mu.RUnlock()
	expires := s.c.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL)
	server, err := types.NewAppServerV3(types.Metadata{
		Name:    copy.GetName(),
		Expires: &expires,
	}, types.AppServerSpecV3{
		Version:  teleport.Version,
		Hostname: s.c.Hostname,
		HostID:   s.c.HostID,
		Rotation: s.getRotationState(),
		App:      copy,
		ProxyIDs: s.c.ConnectedProxyGetter.GetProxyIDs(),
	})

	return server, trace.Wrap(err)
}

// getRotationState is a helper to return this server's CA rotation state.
func (s *Server) getRotationState() types.Rotation {
	rotation, err := s.c.GetRotation(types.RoleApp)
	if err != nil && !trace.IsNotFound(err) && !trace.IsConnectionProblem(err) {
		s.log.WarnContext(s.closeContext, "Failed to get rotation state.", "error", err)
	}
	if rotation != nil {
		return *rotation
	}
	return types.Rotation{}
}

// registerApp starts proxying the app.
func (s *Server) registerApp(ctx context.Context, app types.Application) error {
	if err := s.startApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apps[app.GetName()] = app
	return nil
}

// updateApp updates application that is already registered.
func (s *Server) updateApp(ctx context.Context, app types.Application) error {
	// Stop heartbeat and dynamic labels before starting new ones.
	if err := s.stopAndRemoveApp(ctx, app.GetName()); err != nil {
		return trace.Wrap(err)
	}
	if err := s.registerApp(ctx, app); err != nil {
		// If we failed to re-register, don't keep proxying the old app.
		if errUnregister := s.unregisterAndRemoveApp(ctx, app.GetName()); errUnregister != nil {
			return trace.NewAggregate(err, errUnregister)
		}
		return trace.Wrap(err)
	}
	return nil
}

// unregisterAndRemoveApp stops proxying the app and deltes it.
func (s *Server) unregisterAndRemoveApp(ctx context.Context, name string) error {
	if err := s.stopAndRemoveApp(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.apps, name)
	return nil
}

// getApps returns a list of all apps this server is proxying.
func (s *Server) getApps() (apps types.Apps) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, app := range s.apps {
		apps = append(apps, app)
	}
	return apps
}

// Start starts proxying all registered apps.
func (s *Server) Start(ctx context.Context) (err error) {
	// Register all apps from static configuration.
	for _, app := range s.c.Apps {
		if err := s.registerApp(ctx, app); err != nil {
			return trace.Wrap(err)
		}
	}

	// Start reconciler that will be reconciling proxied apps with
	// application resources.
	if err := s.startReconciler(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Initialize watcher that will be dynamically (un-)registering
	// proxied apps based on the application resources.
	if s.watcher, err = s.startResourceWatcher(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	return trace.Wrap(s.close(s.closeContext))
}

// Shutdown performs a graceful shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO wait active connections.
	return trace.Wrap(s.close(ctx))
}

func (s *Server) close(ctx context.Context) error {
	shouldDeleteApps := services.ShouldDeleteServerHeartbeatsOnShutdown(ctx)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(100)

	sender, ok := s.c.InventoryHandle.GetSender()
	if ok {
		// Manual deletion per app is only required if the auth server
		// doesn't support actively cleaning up app resources when the
		// inventory control stream is terminated during shutdown.
		if capabilities := sender.Hello().Capabilities; capabilities != nil {
			shouldDeleteApps = shouldDeleteApps && !capabilities.AppCleanup
		}
	}

	// Hold the READ lock while iterating the applications here to prevent
	// deadlocking in flight heartbeats. The heartbeat announce acquires
	// the lock to build the app resource to send. If the WRITE lock is
	// held during the shutdown procedure below, any in flight heartbeats
	// will block acquiring the mutex until shutdown completes, at which
	// point the heartbeat will be emitted and the removal of the app
	// server below would be undone.
	s.mu.RLock()
	for name := range s.apps {
		name := name
		heartbeat := s.heartbeats[name]

		if dynamic, ok := s.dynamicLabels[name]; ok {
			dynamic.Close()
		}

		if heartbeat != nil {
			log := s.log.With("app", name)
			log.DebugContext(ctx, "Stopping app")
			if err := heartbeat.Close(); err != nil {
				log.WarnContext(ctx, "Failed to stop app.", "error", err)
			} else {
				log.DebugContext(ctx, "Stopped app")
			}

			if shouldDeleteApps {
				g.Go(func() error {
					log.DebugContext(ctx, "Deleting app")
					if err := s.removeAppServer(gctx, name); err != nil {
						log.WarnContext(ctx, "Failed to delete app.", "error", err)
					} else {
						log.DebugContext(ctx, "Deleted app")
					}
					return nil
				})
			}
		}
	}
	s.mu.RUnlock()

	if err := g.Wait(); err != nil {
		s.log.WarnContext(ctx, "Deleting all apps failed", "error", err)
	}

	s.mu.Lock()
	clear(s.apps)
	clear(s.dynamicLabels)
	clear(s.heartbeats)
	s.mu.Unlock()

	errs := s.c.ConnectionsHandler.Close(ctx)

	// Signal to any blocking go routine that it should exit.
	s.closeFunc()

	// Stop the database resource watcher.
	if s.watcher != nil {
		s.watcher.Close()
	}

	return trace.NewAggregate(errs...)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	return s.closeContext.Err()
}

// HandleConnection takes a connection and wraps it in a listener, so it can
// be passed to http.Serve to process as a HTTP request.
func (s *Server) HandleConnection(conn net.Conn) {
	s.c.ConnectionsHandler.HandleConnection(conn)
}

// GetAppByPublicAddress returns an application matching the public address. If multiple
// matching applications exist, the first one is returned. Random selection
// (or round robin) does not need to occur here because they will all point
// to the same target address. Random selection (or round robin) occurs at the
// web proxy to load balance requests to the application service.
func (s *Server) GetAppByPublicAddress(ctx context.Context, publicAddr string) (types.Application, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// don't call s.getApps() as this will call RLock and potentially deadlock.
	for _, a := range s.apps {
		if publicAddr == a.GetPublicAddr() {
			return s.appWithUpdatedLabelsLocked(a), nil
		}
	}
	return nil, trace.NotFound("no application at %v found", publicAddr)
}

// appWithUpdatedLabelsLocked will inject updated dynamic and cloud labels into
// an application object.
// The caller must invoke an RLock on `s.mu` before calling this function.
func (s *Server) appWithUpdatedLabelsLocked(app types.Application) *types.AppV3 {
	// Create a copy of the application to modify
	copy := app.Copy()

	// Update dynamic labels if the app has them.
	labels := s.dynamicLabels[copy.GetName()]

	if labels != nil {
		copy.SetDynamicLabels(labels.Get())
	}

	// Add in the cloud labels if the app has them.
	if s.c.CloudLabels != nil {
		s.c.CloudLabels.Apply(copy)
	}

	return copy
}

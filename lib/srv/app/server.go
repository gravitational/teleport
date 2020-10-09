/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// app package runs the application proxy process. It keeps dynamic labels
// updated, heart beats it's presence, check access controls, and forwards
// connections between the tunnel and the target host.
package app

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

type RotationGetter func(role teleport.Role) (*services.Rotation, error)

// Config is the configuration for an application server.
type Config struct {
	// Clock used to control time.
	Clock clockwork.Clock

	// AuthClient is a client directly connected to the Auth server.
	AuthClient *auth.Client

	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint auth.AccessPoint

	// GetRotation returns the certificate rotation state.
	GetRotation RotationGetter

	// Server contains the list of applications that will be proxied.
	Server services.Server
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
	if c.GetRotation == nil {
		return trace.BadParameter("rotation getter missing")
	}
	if c.Server == nil {
		return trace.BadParameter("server missing")
	}

	return nil
}

// Server is an application server.
type Server struct {
	c   *Config
	log *logrus.Entry

	closeContext context.Context
	closeFunc    context.CancelFunc

	httpServer *http.Server

	heartbeat     *srv.Heartbeat
	dynamicLabels map[string]*labels.Dynamic

	clusterName string
	keepAlive   time.Duration

	tr http.RoundTripper

	mu    sync.Mutex
	cache *ttlmap.TTLMap

	activeConns int64
}

// New returns a new application server.
func New(ctx context.Context, c *Config) (*Server, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		c: c,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentApp,
		}),
	}

	s.closeContext, s.closeFunc = context.WithCancel(ctx)

	// Create HTTP server that will be forwarding requests to target application.
	s.httpServer = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: defaults.DefaultDialTimeout,
	}

	s.tr, err = newTransport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache of request forwarders.
	s.cache, err = ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create dynamic labels for all applications that are being proxied and
	// sync them right away so the first heartbeat has correct dynamic labels.
	s.dynamicLabels = make(map[string]*labels.Dynamic)
	for _, a := range c.Server.GetApps() {
		if len(a.DynamicLabels) == 0 {
			continue
		}
		dl, err := labels.NewDynamic(s.closeContext, &labels.DynamicConfig{
			Labels: services.V2ToLabels(a.DynamicLabels),
			Log:    s.log,
		})
		if err != nil {

			return nil, trace.Wrap(err)
		}
		dl.Sync()
		s.dynamicLabels[a.Name] = dl
	}

	// Create heartbeat loop so applications keep sending presence to backend.
	s.heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:            srv.HeartbeatModeApp,
		Context:         s.closeContext,
		Component:       teleport.ComponentApp,
		Announcer:       c.AccessPoint,
		GetServerInfo:   s.GetServerInfo,
		KeepAlivePeriod: defaults.ServerKeepAliveTTL,
		AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/2),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       defaults.ServerAnnounceTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pick up TCP keep-alive settings from the cluster level.
	clusterConfig, err := s.c.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.keepAlive = clusterConfig.GetKeepAliveInterval()

	cn, err := s.c.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.clusterName = cn.GetClusterName()

	return s, nil
}

// GetServerInfo returns a services.Server representing the application. Used
// in heartbeat code.
func (s *Server) GetServerInfo() (services.Server, error) {
	// Update dynamic labels on all apps.
	apps := s.c.Server.GetApps()
	for _, a := range apps {
		dl, ok := s.dynamicLabels[a.Name]
		if !ok {
			continue
		}
		a.DynamicLabels = services.LabelsToV2(dl.Get())
	}
	s.c.Server.SetApps(apps)

	// Update the TTL.
	s.c.Server.SetTTL(s.c.Clock, defaults.ServerAnnounceTTL)

	// Update rotation state.
	rotation, err := s.c.GetRotation(teleport.RoleApp)
	if err != nil {
		if !trace.IsNotFound(err) {
			s.log.Warningf("Failed to get rotation state: %v.", err)
		}
	} else {
		s.c.Server.SetRotation(*rotation)
	}

	return s.c.Server, nil
}

// Start starts heart beating the presence of service.Apps that this
// server is proxying along with any dynamic labels.
func (s *Server) Start() {
	for _, dynamicLabel := range s.dynamicLabels {
		go dynamicLabel.Start()
	}
	go s.heartbeat.Run()
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	if err := s.httpServer.Close(); err != nil {
		return trace.Wrap(err)
	}

	err := s.heartbeat.Close()
	for _, dynamicLabel := range s.dynamicLabels {
		dynamicLabel.Close()
	}
	s.closeFunc()

	return trace.Wrap(err)
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.closeContext.Done()
	return s.closeContext.Err()
}

// ForceHeartbeat is used in tests to force updating of services.Server.
func (s *Server) ForceHeartbeat() error {
	err := s.heartbeat.ForceSend(time.Second)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// HandleConnection takes a connection and wraps it in a listener so it can
// be passed to http.Serve to process as a HTTP request.
func (s *Server) HandleConnection(conn net.Conn) {
	if err := s.httpServer.Serve(newListener(s.closeContext, conn)); err != nil {
		s.log.Warnf("Failed to handle connection: %v.", err)
	}
}

// ServeHTTP will forward the *http.Request to the target application.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.serveHTTP(w, r); err != nil {
		s.log.Debugf("Failed to serve request: %v.", err)

		// Covert trace error type to HTTP and write response.
		code := trace.ErrorToCode(err)
		http.Error(w, http.StatusText(code), code)
	}
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	// Authenticate the request based off the "x-teleport-session-id" header.
	appSession, err := s.authenticate(s.closeContext, r)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch a cached request forwarder or create one if this is the first
	// request (or the process has been restarted).
	session, err := s.getSession(s.closeContext, appSession)
	if err != nil {
		return trace.Wrap(err)
	}

	// Forward request to the target application.
	session.fwd.ServeHTTP(w, r)
	return nil
}

// authenticate will check if request carries a session cookie matching a
// session in the backend.
func (s *Server) authenticate(ctx context.Context, r *http.Request) (services.AppSession, error) {
	sessionID := r.Header.Get(teleport.AppSessionIDHeader)
	if sessionID == "" {
		s.log.Warnf("Request missing session ID header.")
		return nil, trace.AccessDenied("invalid session")
	}

	// Always look for the session in the backend cache first. This allows the
	// session to be invalidated in the backend and be immediately reflected here.
	session, err := s.c.AccessPoint.GetAppSession(ctx, sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err != nil {
		s.log.Warnf("Failed to fetch application session: %v.", err)
		return nil, trace.AccessDenied("invalid session")
	}

	return session, nil
}

// activeConnections returns the number of active connections being proxied.
// Used in tests.
func (s *Server) activeConnections() int64 {
	return atomic.LoadInt64(&s.activeConns)
}

// newTransport returns a new http.RoundTripper with sensible defaults.
func newTransport() (http.RoundTripper, error) {
	// Clone the default transport to pick up sensible defaults.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", http.DefaultTransport)
	}
	tr := defaultTransport.Clone()

	// Increase the size of the transports connection pool. This substantially
	// improves the performance of Teleport under load as it reduces the number
	// of TLS handshakes performed.
	tr.MaxIdleConns = defaults.HTTPMaxIdleConns
	tr.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost

	// Set IdleConnTimeout on the transport, this defines the maximum amount of
	// time before idle connections are closed. Leaving this unset will lead to
	// connections open forever and will cause memory leaks in a long running
	// process.
	tr.IdleConnTimeout = defaults.HTTPIdleTimeout

	// If Teleport was started with the --insecure flag, then don't validate the
	// certificate if making TLS requests.
	tr.TLSClientConfig.InsecureSkipVerify = lib.IsInsecureDevMode()

	return tr, nil
}

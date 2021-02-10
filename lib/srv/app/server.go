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
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	auth "github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

type RotationGetter func(role teleport.Role) (*services.Rotation, error)

// Config is the configuration for an application server.
type Config struct {
	// Clock is used to control time.
	Clock clockwork.Clock

	// DataDir is the path to the data directory for the server.
	DataDir string

	// AuthClient is a client directly connected to the Auth server.
	AuthClient *client.Client

	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint libauth.ClientAccessPoint

	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config

	// CipherSuites is the list of TLS cipher suites that have been configured
	// for this process.
	CipherSuites []uint16

	// Authorizer is used to authorize requests.
	Authorizer auth.Authorizer

	// GetRotation returns the certificate rotation state.
	GetRotation RotationGetter

	// Server contains the list of applications that will be proxied.
	Server services.Server

	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)
}

// CheckAndSetDefaults makes sure the configuration has the minimum required
// to function.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.DataDir == "" {
		return trace.BadParameter("data dir missing")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("auth client log missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("tls config missing")
	}
	if len(c.CipherSuites) == 0 {
		return trace.BadParameter("cipersuites missing")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer missing")
	}
	if c.GetRotation == nil {
		return trace.BadParameter("rotation getter missing")
	}
	if c.Server == nil {
		return trace.BadParameter("server missing")
	}
	if c.OnHeartbeat == nil {
		return trace.BadParameter("heartbeat missing")
	}

	return nil
}

// Server is an application server. It authenticates requests from the web
// proxy and forwards them to internal applications.
type Server struct {
	c   *Config
	log *logrus.Entry

	closeContext context.Context
	closeFunc    context.CancelFunc

	mu     sync.RWMutex
	server services.Server

	httpServer *http.Server
	tlsConfig  *tls.Config

	heartbeat     *srv.Heartbeat
	dynamicLabels map[string]*labels.Dynamic

	keepAlive time.Duration
	proxyPort string

	cache *sessionCache
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
		server: c.Server,
	}

	s.closeContext, s.closeFunc = context.WithCancel(ctx)

	// Make copy of server's TLS configuration and update it with the specific
	// functionality this server needs, like requiring client certificates.
	s.tlsConfig = copyAndConfigureTLS(s.c.TLSConfig, s.getConfigForClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and configure HTTP server with authorizing middleware.
	s.httpServer = s.newHTTPServer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a new session cache, this holds sessions that can be used to
	// forward requests.
	s.cache, err = newSessionCache(s.closeContext, s.log)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create dynamic labels for all applications that are being proxied and
	// sync them right away so the first heartbeat has correct dynamic labels.
	s.dynamicLabels = make(map[string]*labels.Dynamic)
	for _, a := range s.server.GetApps() {
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
		OnHeartbeat:     c.OnHeartbeat,
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

	// Figure out the port the proxy is running on.
	s.proxyPort = s.getProxyPort()

	return s, nil
}

// GetServerInfo returns a services.Server representing the application. Used
// in heartbeat code.
func (s *Server) GetServerInfo() (services.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update dynamic labels on all apps.
	apps := s.server.GetApps()
	for _, a := range s.server.GetApps() {
		dl, ok := s.dynamicLabels[a.Name]
		if !ok {
			continue
		}
		a.DynamicLabels = services.LabelsToV2(dl.Get())
	}
	s.server.SetApps(apps)

	// Update the TTL.
	s.server.SetExpiry(s.c.Clock.Now().UTC().Add(defaults.ServerAnnounceTTL))

	// Update rotation state.
	rotation, err := s.c.GetRotation(teleport.RoleApp)
	if err != nil {
		if !trace.IsNotFound(err) {
			s.log.Warningf("Failed to get rotation state: %v.", err)
		}
	} else {
		s.server.SetRotation(*rotation)
	}

	return s.server, nil
}

// Start starts heartbeating the presence of service.Apps that this
// server is proxying along with any dynamic labels.
func (s *Server) Start() {
	for _, dynamicLabel := range s.dynamicLabels {
		go dynamicLabel.Start()
	}
	go s.heartbeat.Run()
}

// Close will shut the server down and unblock any resources.
func (s *Server) Close() error {
	var errs []error

	// Stop HTTP server.
	if err := s.httpServer.Close(); err != nil {
		errs = append(errs, err)
	}

	// Stop heartbeat to auth.
	if err := s.heartbeat.Close(); err != nil {
		errs = append(errs, err)
	}

	// Stop all dynamic labels from being updated.
	for _, dynamicLabel := range s.dynamicLabels {
		dynamicLabel.Close()
	}

	// Signal to any blocking go routine that it should exit.
	s.closeFunc()

	return trace.NewAggregate(errs...)
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
	// Wrap the listener in a TLS authorizing listener.
	listener := newListener(s.closeContext, conn)
	tlsListener := tls.NewListener(listener, s.tlsConfig)

	if err := s.httpServer.Serve(tlsListener); err != nil {
		s.log.Warnf("Failed to handle connection: %v.", err)
	}
}

// ServeHTTP will forward the *http.Request to the target application.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.serveHTTP(w, r); err != nil {
		s.log.Warnf("Failed to serve request: %v.", err)

		// Covert trace error type to HTTP and write response.
		code := trace.ErrorToCode(err)
		http.Error(w, http.StatusText(code), code)
	}
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	// Extract the identity and application being requested from the certificate
	// and check if the caller has access.
	identity, app, err := s.authorize(r.Context(), r)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch a cached request forwarder (or create one) that lives about 5
	// minutes. Used to stream session chunks to the Audit Log.
	session, err := s.getSession(r.Context(), identity, app)
	if err != nil {
		return trace.Wrap(err)
	}

	// Forward request to the target application.
	session.fwd.ServeHTTP(w, r)
	return nil
}

// authorize will check if request carries a session cookie matching a
// session in the backend.
func (s *Server) authorize(ctx context.Context, r *http.Request) (*tlsca.Identity, *services.App, error) {
	// Only allow local and remote identities to proxy to an application.
	userType := r.Context().Value(auth.ContextUser)
	switch userType.(type) {
	case auth.LocalUser, auth.RemoteUser:
	default:
		return nil, nil, trace.BadParameter("invalid identity: %T", userType)
	}

	// Extract authorizing context and identity of the user from the request.
	authContext, err := s.c.Authorizer.Authorize(r.Context())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()

	// Fetch the application and check if the identity has access.
	app, err := s.getApp(r.Context(), identity.RouteToApp.PublicAddr)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ap, err := s.c.AccessPoint.GetAuthPreference()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	mfaParams := libauth.AccessMFAParams{
		Verified:       identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}
	err = authContext.Checker.CheckAccessToApp(defaults.Namespace, app, mfaParams)
	if err != nil {
		return nil, nil, utils.OpaqueAccessDenied(err)
	}

	return &identity, app, nil
}

// getSession returns a request session used to proxy the request to the
// target application. Always checks if the session is valid first and if so,
// will return a cached session, otherwise will create one.
func (s *Server) getSession(ctx context.Context, identity *tlsca.Identity, app *services.App) (*session, error) {
	// If a cached forwarder exists, return it right away.
	session, err := s.cache.get(identity.RouteToApp.SessionID)
	if err == nil {
		return session, nil
	}

	// Create a new session with a recorder and forwarder in it.
	session, err = s.newSession(ctx, identity, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session in the cache so the next request can use it for 5 minutes
	// or the time until the certificate expires, whichever comes first.
	ttl := utils.MinTTL(identity.Expires.Sub(s.c.Clock.Now()), 5*time.Minute)
	err = s.cache.set(identity.RouteToApp.SessionID, session, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// getApp returns an application matching the public address. If multiple
// matching applications exist, the first one is returned. Random selection
// (or round robin) does not need to occur here because they will all point
// to the same target address. Random selection (or round robin) occurs at the
// web proxy to load balance requests to the application service.
func (s *Server) getApp(ctx context.Context, publicAddr string) (*services.App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.server.GetApps() {
		if publicAddr == a.PublicAddr {
			return a, nil
		}
	}
	return nil, trace.NotFound("no application at %v found", publicAddr)
}

// newHTTPServer creates an *http.Server that can authorize and forward
// requests to a target application.
func (s *Server) newHTTPServer() *http.Server {
	// Reuse the auth.Middleware to authorize requests but only accept
	// certificates that were specifically generated for applications.
	authMiddleware := &auth.Middleware{
		AccessPoint:   s.c.AccessPoint,
		AcceptedUsage: []string{teleport.UsageAppsOnly},
	}
	authMiddleware.Wrap(s)

	return &http.Server{
		Handler:           authMiddleware,
		ReadHeaderTimeout: defaults.DefaultDialTimeout,
		ErrorLog:          utils.NewStdlogger(s.log.Error, teleport.ComponentApp),
	}
}

// getProxyPort tries to figure out the address the proxy is running at.
func (s *Server) getProxyPort() string {
	servers, err := s.c.AccessPoint.GetProxies()
	if err != nil {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	if len(servers) == 0 {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	_, port, err := net.SplitHostPort(servers[0].GetPublicAddr())
	if err != nil {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	return port
}

// getConfigForClient returns the list of CAs that could have signed the
// client's certificate.
func (s *Server) getConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	var clusterName string
	var err error

	// Try and extract the name of the cluster that signed the client's certificate.
	if info.ServerName != "" {
		clusterName, err = libauth.DecodeClusterName(info.ServerName)
		if err != nil {
			if !trace.IsNotFound(err) {
				s.log.Debugf("Ignoring unsupported cluster name %q.", info.ServerName)
			}
		}
	}

	// Fetch list of CAs that could have signed this certificate. If clusterName
	// is empty, all CAs that this cluster knows about are returned.
	pool, err := auth.ClientCertPool(s.c.AccessPoint, clusterName)
	if err != nil {
		// If this request fails, return nil and fallback to the default ClientCAs.
		s.log.Debugf("Failed to retrieve client pool: %v.", trace.DebugReport(err))
		return nil, nil
	}

	// Don't modify the server's *tls.Config, create one per connection because
	// the requests could be coming from different clusters.
	tlsCopy := s.tlsConfig.Clone()
	tlsCopy.ClientCAs = pool
	return tlsCopy, nil
}

// copyAndConfigureTLS can be used to copy and modify an existing *tls.Config
// for Teleport application proxy servers.
func copyAndConfigureTLS(config *tls.Config, fn func(*tls.ClientHelloInfo) (*tls.Config, error)) *tls.Config {
	tlsConfig := config.Clone()

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = fn

	return tlsConfig
}

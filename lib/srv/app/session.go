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

package app

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	jwt_pkg "github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	session_pkg "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/pborman/uuid"
)

type session struct {
	uri *url.URL

	publicAddr string
	publicPort string

	fwd          *forward.Forwarder
	streamWriter events.StreamWriter
}

func (s *session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If path redirection needs to occur, redirect the caller and don't forward
	// the request.
	if path, ok := s.pathRedirect(r); ok {
		http.Redirect(w, r, path, http.StatusFound)
		return
	}

	s.fwd.ServeHTTP(w, r)
}

// getSession returns a request session used to proxy the request to the
// target application. Always checks if the session is valid first and if so,
// will return a cached session, otherwise will create one.
func (s *Server) getSession(ctx context.Context, identity *tlsca.Identity, app *services.App) (*session, error) {
	// If a cached forwarder exists, return it right away.
	session, err := s.cacheGet(identity.RouteToApp.SessionID)
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
	err = s.cacheSet(identity.RouteToApp.SessionID, session, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// newSession creates a new session which is used to cache the stream writer,
// JWT, and request forwarder.
func (s *Server) newSession(ctx context.Context, identity *tlsca.Identity, app *services.App) (*session, error) {
	uri, err := url.Parse(app.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the stream writer that will write this chunk to the audit log.
	streamWriter, err := s.newStreamWriter(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create JWT token that will be attached to all requests.
	jwt, err := s.c.AuthClient.GenerateJWT(ctx, jwt_pkg.SignParams{
		Username: identity.Username,
		Roles:    identity.Groups,
		URI:      app.URI,
		Expires:  identity.Expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the forwarder.
	fwder, err := s.newForwarder(s.closeContext,
		&forwarderConfig{
			w:                  streamWriter,
			uri:                app.URI,
			publicAddr:         app.PublicAddr,
			insecureSkipVerify: app.InsecureSkipVerify,
			jwt:                jwt,
			rewrite:            app.Rewrite,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(fwder),
		forward.Logger(s.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		uri:          uri,
		publicAddr:   app.PublicAddr,
		publicPort:   guessProxyPort(s.c.AccessPoint),
		streamWriter: streamWriter,
		fwd:          fwd,
	}, nil
}

// cacheGet will fetch the forwarder from the cache.
func (s *Server) cacheGet(key string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.cache.Get(key); ok {
		if fwd, fok := f.(*session); fok {
			return fwd, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", f)
	}
	return nil, trace.NotFound("session not found")
}

// cacheSet will add the forwarder to the cache.
func (s *Server) cacheSet(key string, value *session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(key, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cacheExpire will close the stream writer.
func (s *Server) cacheExpire(key string, el interface{}) {
	session, ok := el.(*session)
	if !ok {
		s.log.Debugf("Invalid type stored in cache: %T.", el)
		return
	}

	if err := session.streamWriter.Close(s.closeContext); err != nil {
		s.log.Debugf("Failed to close stream writer: %v.", err)
	}

	s.log.Debugf("Closing expired stream %v.", key)
}

func (s *Server) expireSessions() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.clearExpiredSessions()
		case <-s.closeContext.Done():
			return
		}
	}
}

func (s *Server) clearExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.RemoveExpired(10)
}

// newStreamWriter creates a streamer that will be used to stream the
// requests that occur within this session to the audit log.
func (s *Server) newStreamWriter(identity *tlsca.Identity) (events.StreamWriter, error) {
	clusterConfig, err := s.c.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Each chunk has it's own ID. Create a new UUID for this chunk which will be
	// emitted in a new event to the audit log that can be use to aggregate all
	// chunks for a particular session.
	chunkID := uuid.New()

	streamer, err := s.newStreamer(s.closeContext, chunkID, clusterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	streamWriter, err := events.NewAuditWriter(events.AuditWriterConfig{
		// Audit stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context:      s.closeContext,
		Streamer:     streamer,
		Clock:        s.c.Clock,
		SessionID:    session_pkg.ID(chunkID),
		Namespace:    defaults.Namespace,
		ServerID:     s.c.Server.GetName(),
		RecordOutput: clusterConfig.GetSessionRecording() != services.RecordOff,
		Component:    teleport.ComponentApp,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit an event to the Audit Log that a new session chunk has been created.
	appSessionCreateEvent := &events.AppSessionCreate{
		Metadata: events.Metadata{
			Type: events.AppSessionCreateEvent,
			Code: events.AppSessionCreateCode,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        s.c.Server.GetName(),
			ServerNamespace: defaults.Namespace,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: identity.RouteToApp.SessionID,
		},
		SessionChunkID: chunkID,
	}
	if err := s.c.AuthClient.EmitAuditEvent(s.closeContext, appSessionCreateEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	return streamWriter, nil
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (s *Server) newStreamer(ctx context.Context, sessionID string, clusterConfig services.ClusterConfig) (events.Streamer, error) {
	mode := clusterConfig.GetSessionRecording()
	if services.IsRecordSync(mode) {
		s.log.Debugf("Using sync streamer for session %v.", sessionID)
		return s.c.AuthClient, nil
	}

	s.log.Debugf("Using async streamer for session %v.", sessionID)
	uploadDir := filepath.Join(
		s.c.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingLogsDir, defaults.Namespace,
	)
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fileStreamer, nil
}

// forwarderConfig is the configuration for a forwarder.
type forwarderConfig struct {
	w                  events.StreamWriter
	uri                string
	publicAddr         string
	jwt                string
	insecureSkipVerify bool
	rewrite            *services.Rewrite
}

// Check will valid the configuration of a forwarder.
func (c *forwarderConfig) Check() error {
	if c.w == nil {
		return trace.BadParameter("stream writer missing")
	}
	if c.uri == "" {
		return trace.BadParameter("uri missing")
	}
	if c.publicAddr == "" {
		return trace.BadParameter("public addr missing")
	}
	if c.jwt == "" {
		return trace.BadParameter("jwt missing")
	}

	return nil
}

// forwarder will rewrite and forward the request to the target address.
type forwarder struct {
	closeContext context.Context

	c *forwarderConfig

	tr http.RoundTripper

	uri  *url.URL
	port string

	log *logrus.Entry
}

// newForwarder creates a new forwarder that can re-write and round trip a
// HTTP request.
func (s *Server) newForwarder(ctx context.Context, c *forwarderConfig) (*forwarder, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the target address once then inject it into all requests.
	uri, err := url.Parse(c.uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tr, err := newTransport(c.insecureSkipVerify)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &forwarder{
		closeContext: ctx,
		c:            c,
		uri:          uri,
		port:         guessProxyPort(s.c.AccessPoint),
		tr:           tr,
		log:          s.log,
	}, nil
}

// RoundTrip make the request and log the request/response pair in the audit log.
func (f *forwarder) RoundTrip(r *http.Request) (*http.Response, error) {
	// Update the target address of the request so it's forwarded correctly.
	r.URL.Scheme = f.uri.Scheme
	r.URL.Host = f.uri.Host

	r.Header.Add(teleport.AppJWTHeader, f.c.jwt)
	r.Header.Add(teleport.AppCFHeader, f.c.jwt)

	// Forward the request to the target application.
	resp, err := f.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit the result of each request to the audit log.
	appSessionRequestEvent := &events.AppSessionRequest{
		Metadata: events.Metadata{
			Type: events.AppSessionRequestEvent,
			Code: events.AppSessionRequestCode,
		},
		StatusCode: uint32(resp.StatusCode),
		Path:       r.URL.Path,
		RawQuery:   r.URL.RawQuery,
	}
	if err := f.c.w.EmitAuditEvent(f.closeContext, appSessionRequestEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform any response rewriting before writing the response.
	if err := f.rewriteResponse(resp); err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// TODO(russjones): Really what this function should do is if you are trying
// to access any address that does not that the prefix matching what the URI is, then redir.
// pathRedirect checks if the caller is asking for a specific internal URI,
// for example http://localhost:8080/appName and the request comes in simply
// for https://publicAddr. In that case, redirect the user to the specific
// path being requested.
func (s *session) pathRedirect(r *http.Request) (string, bool) {
	// If the URI does not contain a path, no redirection needs to occur.
	if s.uri.Path == "" || s.uri.Path == "/" {
		return "", false
	}
	// If the requested URL is a path, then path redirection does not occur.
	if r.URL.Path != "" && r.URL.Path != "/" {
		return "", false
	}

	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(s.publicAddr, s.publicPort),
		Path:   s.uri.Path,
	}
	return u.String(), true
}

func (f *forwarder) rewriteResponse(resp *http.Response) error {
	switch {
	case f.c.rewrite != nil && len(f.c.rewrite.Redirect) > 0:
		err := f.rewriteRedirect(resp)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
	}
	return nil
}

func (f *forwarder) rewriteRedirect(resp *http.Response) error {
	if isRedirect(resp.StatusCode) {
		// Parse the "Location" header.
		u, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			return trace.Wrap(err)
		}

		// If the redirect location is one of the hosts specified in the list of
		// redirects, rewrite the header.
		if utils.SliceContainsStr(f.c.rewrite.Redirect, host(u.Host)) {
			u.Scheme = "https"
			u.Host = net.JoinHostPort(f.c.publicAddr, f.port)
		}
		resp.Header.Set("Location", u.String())
	}
	return nil
}

func isRedirect(code int) bool {
	if code >= http.StatusMultipleChoices && code <= http.StatusPermanentRedirect {
		return true
	}
	return false
}

func host(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func guessProxyPort(accessPoint auth.AccessPoint) string {
	servers, err := accessPoint.GetProxies()
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

// newTransport returns a new http.RoundTripper with sensible defaults.
func newTransport(insecureSkipVerify bool) (http.RoundTripper, error) {
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

	// Don't verify the servers certificate if either Teleport was started with
	// the --insecure flag or insecure skip verify was specifically requested in
	// application config.
	tr.TLSClientConfig.InsecureSkipVerify = (lib.IsInsecureDevMode() || insecureSkipVerify)

	return tr, nil
}

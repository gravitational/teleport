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
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	session_pkg "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

type session struct {
	fwd          *forward.Forwarder
	streamWriter events.StreamWriter
}

// getSession returns a request session used to proxy the request to the
// target application. Always checks if the session is valid first and if so,
// will return a cached session, otherwise will create one.
func (s *Server) getSession(ctx context.Context, appSession services.AppSession) (*session, error) {
	// If a cached forwarder exists, return it right away.
	session, err := s.cacheGet(appSession.GetName())
	if err == nil {
		return session, nil
	}

	// Create a new session with a recorder and forwarder in it.
	session, err = s.newSession(ctx, appSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session in the cache so the next request can use it.
	// TODO(russjones): Make this smaller of now+5 mins or expiry time.
	//err = s.cacheSet(session.GetName(), session, session.Expiry().Sub(s.c.Clock.Now()))
	err = s.cacheSet(appSession.GetName(), session, 2*time.Second)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *Server) newSession(ctx context.Context, appSession services.AppSession) (*session, error) {
	// Locally lookup the application the caller is targeting.
	app, err := s.getApp(ctx, appSession.GetPublicAddr())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the stream writer that will write this chunk to the audit log.
	streamWriter, err := s.newStreamWriter(appSession)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the forwarder.
	fwder, err := newForwarder(s.closeContext,
		&forwarderConfig{
			w:          streamWriter,
			publicAddr: app.PublicAddr,
			uri:        app.URI,
			jwt:        appSession.GetJWT(),
			tr:         s.tr,
			log:        s.log,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(fwder),
		forward.Rewriter(fwder),
		forward.Logger(s.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		streamWriter: streamWriter,
		fwd:          fwd,
	}, nil
}

// getApp returns an application matching the public address. If multiple
// matching applications exist, the first one is returned. Random selection
// (or round robin) does not need to occur here because they will all point
// to the same target address. Random selection (or round robin) occurs at the
// proxy to load balance requests to the application service.
func (s *Server) getApp(ctx context.Context, publicAddr string) (*services.App, error) {
	for _, a := range s.c.Server.GetApps() {
		if publicAddr == a.PublicAddr {
			return a, nil
		}
	}

	return nil, trace.NotFound("no application at %v found", publicAddr)
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

	//if err := session.streamWriter.Close(s.closeContext); err != nil {
	if err := session.streamWriter.Close(context.Background()); err != nil {
		s.log.Debugf("Failed to close stream writer: %v.", err)
	}

	s.log.Debugf("Closing expired stream %v.", key)
}

func (s *Server) newStreamWriter(appSession services.AppSession) (events.StreamWriter, error) {
	clusterConfig, err := s.c.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionUUID := uuid.New()

	streamer, err := s.newStreamer(s.closeContext, sessionUUID, clusterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	streamWriter, err := events.NewAuditWriter(events.AuditWriterConfig{
		// Audit stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context:  s.closeContext,
		Streamer: streamer,
		Clock:    s.c.Clock,
		// Each session chunk has it's own unique ID. An event will be emitted to
		// the main log that links the services.AppSession to this chunk.
		SessionID:    session_pkg.ID(sessionUUID),
		Namespace:    defaults.Namespace,
		ServerID:     s.c.Server.GetName(),
		RecordOutput: clusterConfig.GetSessionRecording() != services.RecordOff,
		Component:    teleport.ComponentApp,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit the result of each request to the audit log.
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
			SessionID: appSession.GetName(),
		},
		SessionChunkID: sessionUUID,
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
	//// TeeStreamer sends non-print and non disk events
	//// to the audit log in async mode, while buffering all
	//// events on disk for further upload at the end of the session
	//return events.NewTeeStreamer(fileStreamer, s.c.AuthClient), nil
}

// forwarderConfig is the configuration for a forwarder.
type forwarderConfig struct {
	publicAddr string
	uri        string
	jwt        string
	tr         http.RoundTripper
	log        *logrus.Entry
	w          events.StreamWriter
}

// Check will valid the configuration of a forwarder.
func (c *forwarderConfig) Check() error {
	if c.jwt == "" {
		return trace.BadParameter("jwt missing")
	}
	if c.uri == "" {
		return trace.BadParameter("uri missing")
	}
	if c.publicAddr == "" {
		return trace.BadParameter("public addr missing")
	}
	if c.tr == nil {
		return trace.BadParameter("round tripper missing")
	}
	if c.log == nil {
		return trace.BadParameter("logger missing")
	}
	return nil
}

// forwarder will rewrite and forward the request to the target address.
type forwarder struct {
	closeContext context.Context

	c *forwarderConfig

	uri *url.URL
}

// newForwarder creates a new forwarder that can re-write and round trip a
// HTTP request.
func newForwarder(ctx context.Context, c *forwarderConfig) (*forwarder, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the target address once then inject it into all requests.
	uri, err := url.Parse(c.uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &forwarder{
		closeContext: ctx,
		c:            c,
		uri:          uri,
	}, nil
}

// RoundTrip make the request and log the request/response pair in the audit log.
func (f *forwarder) RoundTrip(r *http.Request) (*http.Response, error) {
	// Update the target address of the request so it's forwarded correctly.
	r.URL.Scheme = f.uri.Scheme
	r.URL.Host = f.uri.Host

	resp, err := f.c.tr.RoundTrip(r)
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
		fmt.Printf("--> failing to emit: %v.\n", err)
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// Rewrite request headers to add in JWT header and remove any Teleport
// related authentication headers.
func (f *forwarder) Rewrite(r *http.Request) {
	// Add in JWT headers.
	r.Header.Add(teleport.AppJWTHeader, f.c.jwt)
	r.Header.Add(teleport.AppCFHeader, f.c.jwt)

	// Remove the session ID header before forwarding the session to the
	// target application.
	r.Header.Del(teleport.AppSessionIDHeader)
}

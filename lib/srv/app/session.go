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
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/sirupsen/logrus"
)

// session holds a request forwarder and audit log for this chunk.
type session struct {
	// id is the session's uuid
	id string
	// fwd can rewrite and forward requests to the target application.
	fwd *forward.Forwarder
	// streamWriter can emit events to the audit log.
	streamWriter events.StreamWriter
}

// newSession creates a new session.
func (s *Server) newSession(ctx context.Context, identity *tlsca.Identity, app types.Application) (*session, error) {
	// Create the stream writer that will write this chunk to the audit log.
	streamWriter, err := s.newStreamWriter(identity, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Request a JWT token that will be attached to all requests.
	jwt, err := s.c.AuthClient.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username: identity.Username,
		Roles:    identity.Groups,
		URI:      app.GetURI(),
		Expires:  identity.Expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(s.closeContext,
		&transportConfig{
			w:            streamWriter,
			app:          app,
			publicPort:   s.proxyPort,
			cipherSuites: s.c.CipherSuites,
			jwt:          jwt,
			traits:       identity.Traits,
			log:          s.log,
			user:         identity.Username,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.FlushInterval(100*time.Millisecond),
		forward.RoundTripper(transport),
		forward.Logger(logrus.StandardLogger()),
		forward.WebsocketRewriter(transport.ws),
		forward.WebsocketDial(transport.ws.dialer),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess := &session{
		id:           identity.RouteToApp.SessionID,
		fwd:          fwd,
		streamWriter: streamWriter,
	}

	// Create a session tracker so that other services, such as
	// the session upload completer, can track the session's lifetime.
	if err := s.trackSession(sess, identity); err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session in the cache so the next request can use it for 5 minutes
	// or the time until the certificate expires, whichever comes first.
	ttl := utils.MinTTL(identity.Expires.Sub(s.c.Clock.Now()), 5*time.Minute)
	err = s.cache.set(sess, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (s *Server) closeSession(sess *session) {
	if err := sess.streamWriter.Close(s.closeContext); err != nil {
		s.log.Debugf("Failed to close stream writer: %v.", err)
	}

	if err := services.UpdateSessionTrackerState(s.closeContext, s.c.AuthClient, sess.id, types.SessionState_SessionStateTerminated); err != nil {
		s.log.WithError(err).Warningf("Failed to update session tracker state for session %v.", sess.id)
	}
}

// newStreamWriter creates a streamer that will be used to stream the
// requests that occur within this session to the audit log.
func (s *Server) newStreamWriter(identity *tlsca.Identity, app types.Application) (events.StreamWriter, error) {
	recConfig, err := s.c.AccessPoint.GetSessionRecordingConfig(s.closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.c.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a sync or async streamer depending on configuration of cluster.
	streamer, err := s.newStreamer(s.closeContext, identity.RouteToApp.SessionID, recConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	streamWriter, err := events.NewAuditWriter(events.AuditWriterConfig{
		// Audit stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context:      s.closeContext,
		Streamer:     streamer,
		Clock:        s.c.Clock,
		SessionID:    rsession.ID(identity.RouteToApp.SessionID),
		Namespace:    apidefaults.Namespace,
		ServerID:     s.c.HostID,
		RecordOutput: recConfig.GetMode() != types.RecordOff,
		Component:    teleport.ComponentApp,
		ClusterName:  clusterName.GetClusterName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit an event to the Audit Log that a new session chunk has been created.
	appSessionChunkEvent := &apievents.AppSessionChunk{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionChunkEvent,
			Code:        events.AppSessionChunkCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        s.c.HostID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: identity.RouteToApp.SessionID,
			WithMFA:   identity.MFAVerified,
		},
		UserMetadata: identity.GetUserMetadata(),
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppName:       app.GetName(),
		},
		SessionChunkID: identity.RouteToApp.SessionID,
	}
	if err := s.c.AuthClient.EmitAuditEvent(s.closeContext, appSessionChunkEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	return streamWriter, nil
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (s *Server) newStreamer(ctx context.Context, sessionID string, recConfig types.SessionRecordingConfig) (events.Streamer, error) {
	if services.IsRecordSync(recConfig.GetMode()) {
		s.log.Debugf("Using sync streamer for session %v.", sessionID)
		return s.c.AuthClient, nil
	}

	s.log.Debugf("Using async streamer for session %v.", sessionID)
	uploadDir := filepath.Join(
		s.c.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingLogsDir, apidefaults.Namespace,
	)
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fileStreamer, nil
}

// trackSession creates a new session tracker for the app session.
func (s *Server) trackSession(sess *session, identity *tlsca.Identity) error {
	s.log.Debug("Creating session tracker")
	initiator := &types.Participant{
		User: identity.Username,
	}

	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:    sess.id,
		Kind:         string(types.AppSessionKind),
		State:        types.SessionState_SessionStateRunning,
		Hostname:     s.c.HostID,
		AppName:      identity.RouteToApp.Name,
		ClusterName:  identity.RouteToApp.ClusterName,
		Login:        "root",
		Participants: []types.Participant{*initiator},
		HostUser:     initiator.User,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.c.AuthClient.UpsertSessionTracker(s.closeContext, tracker)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// sessionCache holds a cache of sessions that are used to forward requests.
type sessionCache struct {
	srv *Server

	mu    sync.Mutex
	cache *ttlmap.TTLMap
}

// newSessionCache creates a new session cache.
func (s *Server) newSessionCache() (*sessionCache, error) {
	sessionCache := &sessionCache{srv: s}

	// Cache of request forwarders. Set an expire function that can be used to
	// close and upload the stream of events to the Audit Log.
	var err error
	sessionCache.cache, err = ttlmap.New(defaults.ClientCacheSize, ttlmap.CallOnExpire(sessionCache.expire), ttlmap.Clock(s.c.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go sessionCache.expireSessions()

	return sessionCache, nil
}

// get will fetch the session from the cache.
func (s *sessionCache) get(key string) (*session, error) {
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

// set will add the session to the cache.
func (s *sessionCache) set(value *session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(value.id, value, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// expire will close the stream writer.
func (s *sessionCache) expire(key string, el interface{}) {
	// Closing the session stream writer may trigger a flush operation which could be
	// time-consuming. Launch in another goroutine since this occurs under a
	// lock and expire can get called during a "get" operation on the ttlmap.
	go s.closeSession(el)
	s.srv.log.Debugf("Closing expired stream %v.", key)
}

// expire will close the stream writer.
func (s *sessionCache) closeSession(el interface{}) {
	switch sess := el.(type) {
	case *session:
		s.srv.closeSession(sess)
	default:
		s.srv.log.Debugf("Invalid type stored in cache: %T.", el)
	}
}

// expireSessions ticks every second trying to close expired sessions.
func (s *sessionCache) expireSessions() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.expiredSessions()
		case <-s.srv.closeContext.Done():
			return
		}
	}
}

// expiredSession tries to expire sessions in the cache.
func (s *sessionCache) expiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.RemoveExpired(10)
}

// closeAllSessions will remove and close all sessions in the cache.
func (s *sessionCache) closeAllSessions() {
	_, el, cont := s.cache.Pop()
	for cont {
		s.closeSession(el)
		_, el, cont = s.cache.Pop()
	}
}

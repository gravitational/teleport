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
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// sessionChunkCloseTimeout is the default timeout used for sessionChunk.closeTimeout
const sessionChunkCloseTimeout = 1 * time.Hour

var errSessionChunkAlreadyClosed = errors.New("session chunk already closed")

// sessionChunk holds an open request forwarder and audit log for an app session.
//
// An app session is only bounded by the lifetime of the certificate in
// the caller's identity, so we create sessionChunks to track and record
// chunks of live app session activity.
//
// Each chunk will emit an "app.session.chunk" event with the chunk ID
// corresponding to the session chunk's uploaded recording. These emitted
// chunk IDs can be used to aggregate all session uploads tied to the
// overarching identity SessionID.
type sessionChunk struct {
	closeC chan struct{}
	// id is the session chunk's uuid, which is used as the id of its session upload.
	id string
	// fwd can rewrite and forward requests to the target application.
	fwd *forward.Forwarder
	// streamWriter can emit events to the audit log.
	streamWriter events.StreamWriter

	// inflightCond protects and signals change of inflight
	inflightCond *sync.Cond
	// inflight is the amount of in-flight requests
	// closing the chunk is only allowed when this is 0, or after closeTimeout elapses.
	// On session expiration, this will first be atomically decremented to -1,
	// preventing any new requests from using the closing/closed session.
	inflight int64
	// closeTimeout is the timeout after which close() will forcibly close the chunk,
	// even if there are still ongoing requests.
	// E.g. with 5 minute chunk TTL and 2 minute closeTimeout, the chunk will live
	// for ~7 minutes at most.
	closeTimeout time.Duration

	log *logrus.Entry
}

// newSessionChunk creates a new chunk session.
// The session chunk is created with inflight=1,
// and as such expects `release()` to eventually be called
// by the caller of this function.
func (s *Server) newSessionChunk(ctx context.Context, identity *tlsca.Identity, app types.Application) (*sessionChunk, error) {
	sess := &sessionChunk{
		id:           uuid.New().String(),
		closeC:       make(chan struct{}),
		inflightCond: sync.NewCond(&sync.Mutex{}),
		inflight:     1,
		closeTimeout: sessionChunkCloseTimeout,
		log:          s.log,
	}

	sess.log.Debugf("Created app session chunk %s", sess.id)

	// Create a session tracker so that other services, such as the
	// session upload completer, can track the session chunk's lifetime.
	if err := s.createTracker(sess, identity); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the stream writer that will write this chunk to the audit log.
	var err error
	sess.streamWriter, err = s.newStreamWriter(identity, app, sess.id)
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

	// Add JWT token to the traits so it can be used in headers templating.
	traits := identity.Traits
	if traits == nil {
		traits = make(wrappers.Traits)
	}
	traits[teleport.TraitJWT] = []string{jwt}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(s.closeContext,
		&transportConfig{
			w:            sess.streamWriter,
			app:          app,
			publicPort:   s.proxyPort,
			cipherSuites: s.c.CipherSuites,
			jwt:          jwt,
			traits:       traits,
			log:          s.log,
			user:         identity.Username,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess.fwd, err = forward.New(
		forward.FlushInterval(100*time.Millisecond),
		forward.RoundTripper(transport),
		forward.Logger(logrus.StandardLogger()),
		forward.WebsocketRewriter(transport.ws),
		forward.WebsocketDial(transport.ws.dialer),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session chunk in the cache so that upcoming requests can use it for
	// 5 minutes or the time until the certificate expires, whichever comes first.
	ttl := utils.MinTTL(identity.Expires.Sub(s.c.Clock.Now()), 5*time.Minute)
	err = s.cache.set(identity.RouteToApp.SessionID, sess, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// acquire() increments in-flight request count by 1.
// It is supposed to be paired with a `release()` call,
// after the chunk is done with for the individual request
func (s *sessionChunk) acquire() error {
	s.inflightCond.L.Lock()
	defer s.inflightCond.L.Unlock()

	if s.inflight == -1 {
		return trace.Wrap(errSessionChunkAlreadyClosed)
	}

	s.inflight++
	return nil
}

func (s *sessionChunk) release() {
	s.inflightCond.L.Lock()
	defer s.inflightCond.L.Unlock()
	if s.inflight == -1 {
		return
	}
	s.inflight--
	s.inflightCond.Signal()
}

func (s *sessionChunk) close(ctx context.Context) error {
	deadline := time.Now().Add(s.closeTimeout)
	t := time.AfterFunc(s.closeTimeout, func() { s.inflightCond.Signal() })
	defer t.Stop()

	// Wait until there are no requests in-flight,
	// then mark the session as not accepting new requests,
	// and close it.
	s.inflightCond.L.Lock()
	for {
		if s.inflight == 0 {
			break
		} else if time.Now().After(deadline) {
			s.log.Debugf("Timeout expired, forcibly closing session chunk %s, inflight requests: %d", s.id, s.inflight)
			break
		}
		s.log.Debugf("Inflight requests: %d, waiting to close session chunk %s", s.inflight, s.id)
		s.inflightCond.Wait()
	}
	s.inflight = -1
	s.inflightCond.L.Unlock()
	close(s.closeC)
	s.log.Debugf("Closed session chunk %s", s.id)
	err := s.streamWriter.Close(ctx)
	return trace.Wrap(err)
}

func (s *Server) closeSession(sess *sessionChunk) {
	if err := sess.close(s.closeContext); err != nil {
		s.log.WithError(err).Debugf("Error closing session %v", sess.id)
	}
}

// newStreamWriter creates a session stream that will be used to record
// requests that occur within this session chunk and upload the recording
// to the Auth server.
func (s *Server) newStreamWriter(identity *tlsca.Identity, app types.Application, chunkID string) (events.StreamWriter, error) {
	recConfig, err := s.c.AccessPoint.GetSessionRecordingConfig(s.closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.c.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a sync or async streamer depending on configuration of cluster.
	streamer, err := s.newStreamer(s.closeContext, chunkID, recConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	streamWriter, err := events.NewAuditWriter(events.AuditWriterConfig{
		// Audit stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context:      s.closeContext,
		Streamer:     streamer,
		Clock:        s.c.Clock,
		SessionID:    rsession.ID(chunkID),
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
		SessionChunkID: chunkID,
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
func (s *Server) newStreamer(ctx context.Context, chunkID string, recConfig types.SessionRecordingConfig) (events.Streamer, error) {
	if services.IsRecordSync(recConfig.GetMode()) {
		s.log.Debugf("Using sync streamer for session chunk %v.", chunkID)
		return s.c.AuthClient, nil
	}

	s.log.Debugf("Using async streamer for session chunk %v.", chunkID)
	uploadDir := filepath.Join(
		s.c.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingSessionsDir, apidefaults.Namespace,
	)
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fileStreamer, nil
}

// createTracker creates a new session tracker for the session chunk.
func (s *Server) createTracker(sess *sessionChunk, identity *tlsca.Identity) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:   sess.id,
		Kind:        string(types.AppSessionKind),
		State:       types.SessionState_SessionStateRunning,
		Hostname:    s.c.HostID,
		AppName:     identity.RouteToApp.Name,
		ClusterName: identity.RouteToApp.ClusterName,
		Login:       identity.GetUserMetadata().Login,
		Participants: []types.Participant{{
			User: identity.Username,
		}},
		HostUser:     identity.Username,
		Created:      s.c.Clock.Now(),
		AppSessionID: identity.RouteToApp.SessionID,
	}

	s.log.Debugf("Creating tracker for session chunk %v", sess.id)
	tracker, err := srv.NewSessionTracker(s.closeContext, trackerSpec, s.c.AuthClient)
	if err != nil {
		// Ignore access denied errors, which we will get if the auth
		// server is v9.2.1 or earlier, since only node, proxy, and
		// kube roles had permission to create session trackers.
		if trace.IsAccessDenied(err) {
			s.log.Debugf("Insufficient permissions to create session tracker, skipping session tracking for session chunk %v", sess.id)
			return nil
		}
		return trace.Wrap(err)
	}

	go func() {
		<-sess.closeC
		if err := tracker.Close(s.closeContext); err != nil {
			s.log.WithError(err).Debugf("Failed to close session tracker for session chunk %v", sess.id)
		}
	}()

	return nil
}

// sessionChunkCache holds a cache of session chunks.
type sessionChunkCache struct {
	srv *Server

	mu    sync.Mutex
	cache *ttlmap.TTLMap
}

// newSessionChunkCache creates a new session chunk cache.
func (s *Server) newSessionChunkCache() (*sessionChunkCache, error) {
	sessionCache := &sessionChunkCache{srv: s}

	// Cache of session chunks. Set an expire function that can be used
	// to close and upload the stream of events to the Audit Log.
	var err error
	sessionCache.cache, err = ttlmap.New(defaults.ClientCacheSize, ttlmap.CallOnExpire(sessionCache.expire), ttlmap.Clock(s.c.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go sessionCache.expireSessions()

	return sessionCache, nil
}

// get will fetch the session chunk from the cache.
func (s *sessionChunkCache) get(key string) (*sessionChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.cache.Get(key); ok {
		if fwd, fok := f.(*sessionChunk); fok {
			return fwd, nil
		}
		return nil, trace.BadParameter("invalid type stored in cache: %T", f)
	}
	return nil, trace.NotFound("session not found")
}

// set will add the session chunk to the cache.
func (s *sessionChunkCache) set(sessionID string, sess *sessionChunk, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.cache.Set(sessionID, sess, ttl); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// expire will close the stream writer.
func (s *sessionChunkCache) expire(key string, el interface{}) {
	// Closing the session stream writer may trigger a flush operation which could
	// be time-consuming. Launch in another goroutine since this occurs under a
	// lock and expire can get called during a "get" operation on the ttlmap.
	go s.closeSession(el)
	s.srv.log.Debugf("Closing expired stream %v.", key)
}

func (s *sessionChunkCache) closeSession(el interface{}) {
	switch sess := el.(type) {
	case *sessionChunk:
		s.srv.closeSession(sess)
	default:
		s.srv.log.Debugf("Invalid type stored in cache: %T.", el)
	}
}

// expireSessions ticks every second trying to close expired sessions.
func (s *sessionChunkCache) expireSessions() {
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
func (s *sessionChunkCache) expiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.RemoveExpired(10)
}

// closeAllSessions will remove and close all sessions in the cache.
func (s *sessionChunkCache) closeAllSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session, ok := s.cache.Pop(); ok; _, session, ok = s.cache.Pop() {
		s.closeSession(session)
	}
}

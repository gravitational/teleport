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

package app

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// sessionChunkCloseTimeout is the default timeout used for sessionChunk.closeTimeout
const sessionChunkCloseTimeout = 1 * time.Hour

var errSessionChunkAlreadyClosed = errors.New("session chunk already closed")

// sessionChunk holds an open request handler and stream closer for an app session.
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
	// streamCloser closes the session chunk stream.
	streamCloser utils.WriteContextCloser
	// audit is the session chunk audit logger.
	audit common.Audit
	// handler handles requests for this session chunk.
	handler http.Handler

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

// sessionOpt defines an option function for creating sessionChunk.
type sessionOpt func(context.Context, *sessionChunk, *tlsca.Identity, types.Application) error

// newSessionChunk creates a new chunk session.
// The session chunk is created with inflight=1,
// and as such expects `release()` to eventually be called
// by the caller of this function.
func (c *ConnectionsHandler) newSessionChunk(ctx context.Context, identity *tlsca.Identity, app types.Application, startTime time.Time, opts ...sessionOpt) (*sessionChunk, error) {
	sess := &sessionChunk{
		id:           uuid.New().String(),
		closeC:       make(chan struct{}),
		inflightCond: sync.NewCond(&sync.Mutex{}),
		closeTimeout: sessionChunkCloseTimeout,
		log:          c.legacyLogger,
	}

	sess.log.Debugf("Creating app session chunk %s", sess.id)

	// Create a session tracker so that other services, such as the
	// session upload completer, can track the session chunk's lifetime.
	if err := c.createTracker(sess, identity, app.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the stream writer that will write this chunk to the audit log.
	// Audit stream is using server context, not session context,
	// to make sure that session is uploaded even after it is closed.
	rec, err := c.newSessionRecorder(c.closeContext, startTime, sess.id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.streamCloser = rec

	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  c.cfg.Emitter,
		Recorder: rec,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.audit = audit

	for _, opt := range opts {
		if err = opt(ctx, sess, identity, app); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// only emit a session chunk if we didn't get an error making the new session chunk
	if err := sess.audit.OnSessionChunk(ctx, c.cfg.HostID, sess.id, identity, app); err != nil {
		return nil, trace.Wrap(err)
	}

	sess.log.Debugf("Created app session chunk %s", sess.id)
	return sess, nil
}

// withJWTTokenForwarder is a sessionOpt that creates a forwarder that attaches
// a generated JWT token to all requests.
func (c *ConnectionsHandler) withJWTTokenForwarder(ctx context.Context, sess *sessionChunk, identity *tlsca.Identity, app types.Application) error {
	rewrite := app.GetRewrite()
	traits := identity.Traits
	roles := identity.Groups
	if rewrite != nil {
		switch rewrite.JWTClaims {
		case types.JWTClaimsRewriteNone:
			traits = nil
			roles = nil
		case types.JWTClaimsRewriteRoles:
			traits = nil
		case types.JWTClaimsRewriteTraits:
			roles = nil
		case "", types.JWTClaimsRewriteRolesAndTraits:
		}
	}

	// Request a JWT token that will be attached to all requests.
	jwt, err := c.cfg.AuthClient.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username: identity.Username,
		Roles:    roles,
		Traits:   traits,
		URI:      app.GetURI(),
		Expires:  identity.Expires,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Add JWT token to the traits so it can be used in headers templating.
	if traits == nil {
		traits = make(wrappers.Traits)
	}
	traits[teleport.TraitJWT] = []string{jwt}

	// Create a rewriting transport that will be used to forward requests.
	transport, err := newTransport(c.closeContext,
		&transportConfig{
			app:          app,
			publicPort:   c.proxyPort,
			cipherSuites: c.cfg.CipherSuites,
			jwt:          jwt,
			traits:       traits,
			log:          c.legacyLogger,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	delegate := reverseproxy.NewHeaderRewriter()
	sess.handler, err = reverseproxy.New(
		reverseproxy.WithFlushInterval(100*time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(sess.log),
		reverseproxy.WithRewriter(common.NewHeaderRewriter(delegate)),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// withAWSSigner is a sessionOpt that uses an AWS signing service handler.
func (c *ConnectionsHandler) withAWSSigner(_ context.Context, sess *sessionChunk, _ *tlsca.Identity, _ types.Application) error {
	sess.handler = c.awsHandler
	return nil
}

func (c *ConnectionsHandler) withAzureHandler(ctx context.Context, sess *sessionChunk, identity *tlsca.Identity, app types.Application) error {
	sess.handler = c.azureHandler
	return nil
}

func (c *ConnectionsHandler) withGCPHandler(ctx context.Context, sess *sessionChunk, identity *tlsca.Identity, app types.Application) error {
	sess.handler = c.gcpHandler
	return nil
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
	return trace.Wrap(s.streamCloser.Close(ctx))
}

func (c *ConnectionsHandler) onSessionExpired(ctx context.Context, key, expired any) {
	sess, ok := expired.(*sessionChunk)
	if !ok {
		return
	}

	// Closing the session stream writer may trigger a flush operation which could
	// be time-consuming. Launch in another goroutine to prevent interfering with
	// cache operations.
	c.cacheCloseWg.Add(1)
	go func() {
		defer c.cacheCloseWg.Done()
		if err := sess.close(ctx); err != nil {
			c.log.DebugContext(ctx, "Error closing session", "session_id", sess.id, "error", err)
		}
	}()
}

// newSessionRecorder creates a session stream that will be used to record
// requests that occur within this session chunk and upload the recording
// to the Auth server.
func (c *ConnectionsHandler) newSessionRecorder(ctx context.Context, startTime time.Time, chunkID string) (events.SessionPreparerRecorder, error) {
	recConfig, err := c.cfg.AccessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := c.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rec, err := recorder.New(recorder.Config{
		SessionID:    rsession.ID(chunkID),
		ServerID:     c.cfg.HostID,
		Namespace:    apidefaults.Namespace,
		Clock:        c.cfg.Clock,
		ClusterName:  clusterName.GetClusterName(),
		RecordingCfg: recConfig,
		SyncStreamer: c.cfg.AuthClient,
		DataDir:      c.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentApp),
		Context:      ctx,
		StartTime:    startTime,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rec, nil
}

// createTracker creates a new session tracker for the session chunk.
func (c *ConnectionsHandler) createTracker(sess *sessionChunk, identity *tlsca.Identity, appName string) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:   sess.id,
		Kind:        string(types.AppSessionKind),
		State:       types.SessionState_SessionStateRunning,
		Hostname:    c.cfg.HostID,
		ClusterName: identity.RouteToApp.ClusterName,
		Login:       identity.GetUserMetadata().Login,
		Participants: []types.Participant{{
			User: identity.Username,
		}},
		HostUser:     identity.Username,
		Created:      c.cfg.Clock.Now(),
		AppName:      appName, // app name is only present in RouteToApp for CLI sessions
		AppSessionID: identity.RouteToApp.SessionID,
		HostID:       c.cfg.HostID,
	}

	c.log.DebugContext(c.closeContext, "Creating tracker for session chunk", "session", sess.id)
	tracker, err := srv.NewSessionTracker(c.closeContext, trackerSpec, c.cfg.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		<-sess.closeC
		if err := tracker.Close(c.closeContext); err != nil {
			c.log.DebugContext(c.closeContext, "Failed to close session tracker for session chunk", "session", sess.id, "error", err)
		}
	}()

	return nil
}

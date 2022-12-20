/*
Copyright 2015-2020 Gravitational, Inc.

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

package srv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const sessionRecorderID = "session-recorder"

const (
	PresenceVerifyInterval = time.Second * 15
	PresenceMaxDifference  = time.Minute
)

// SessionControlsInfoBroadcast is sent in tandem with session creation
// to inform any joining users about the session controls.
const SessionControlsInfoBroadcast = "Controls\r\n  - CTRL-C: Leave the session\r\n  - t: Forcefully terminate the session (moderators only)"

var serverSessions = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: teleport.MetricServerInteractiveSessions,
		Help: "Number of active sessions to this host",
	},
)

// SessionRegistry holds a map of all active sessions on a given
// SSH server
type SessionRegistry struct {
	SessionRegistryConfig

	// log holds the structured logger
	log *log.Entry

	// sessions holds a map between session ID and the session object. Used to
	// find active sessions as well as close all sessions when the registry
	// is closing.
	sessions    map[rsession.ID]*session
	sessionsMux sync.Mutex
}

type SessionRegistryConfig struct {
	// clock is the registry's internal clock. used in testing.
	clock clockwork.Clock

	// srv refers to the upon which this session registry is created.
	Srv Server

	// sessiontrackerService is used to share session activity to
	// other teleport components through the auth server.
	SessionTrackerService services.SessionTrackerService
}

func (sc *SessionRegistryConfig) CheckAndSetDefaults() error {
	if sc.SessionTrackerService == nil {
		return trace.BadParameter("session tracker service is required")
	}

	if sc.Srv == nil {
		return trace.BadParameter("server is required")
	}

	if sc.Srv.GetSessionServer() == nil {
		return trace.BadParameter("session server is required")
	}

	if sc.clock == nil {
		sc.clock = sc.Srv.GetClock()
	}

	return nil
}

func NewSessionRegistry(cfg SessionRegistryConfig) (*SessionRegistry, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	err := utils.RegisterPrometheusCollectors(serverSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SessionRegistry{
		SessionRegistryConfig: cfg,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, cfg.Srv.Component()),
		}),
		sessions: make(map[rsession.ID]*session),
	}, nil
}

func (s *SessionRegistry) addSession(sess *session) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	s.sessions[sess.id] = sess
}

func (s *SessionRegistry) removeSession(sess *session) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	delete(s.sessions, sess.id)
}

func (s *SessionRegistry) findSessionLocked(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func (s *SessionRegistry) findSession(id rsession.ID) (*session, bool) {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()
	return s.findSessionLocked(id)
}

func (s *SessionRegistry) Close() {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()

	// End all sessions and allow session cleanup
	// goroutines to complete.
	for _, se := range s.sessions {
		se.Stop()
	}

	s.log.Debug("Closing Session Registry.")
}

// OpenSession either joins an existing active session or starts a new session.
func (s *SessionRegistry) OpenSession(ctx context.Context, ch ssh.Channel, scx *ServerContext) error {
	session := scx.getSession()
	if session != nil && !session.isStopped() {
		scx.Infof("Joining existing session %v.", session.id)

		mode := types.SessionParticipantMode(scx.env[teleport.EnvSSHJoinMode])
		switch mode {
		case types.SessionModeratorMode, types.SessionObserverMode:
		default:
			if mode == types.SessionPeerMode || len(mode) == 0 {
				mode = types.SessionPeerMode
			} else {
				return trace.BadParameter("Unrecognized session participant mode: %v", mode)
			}
		}

		// Update the in-memory data structure that a party member has joined.
		_, err := session.join(ch, scx, mode)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	}
	// session not found? need to create one. start by getting/generating an ID for it
	sid, found := scx.GetEnv(sshutils.SessionEnvVar)
	if !found {
		sid = string(rsession.NewID())
		scx.SetEnv(sshutils.SessionEnvVar, sid)
	}
	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	sess, err := newSession(ctx, rsession.ID(sid), s, scx)
	if err != nil {
		return trace.Wrap(err)
	}
	scx.setSession(sess)
	s.addSession(sess)
	scx.Infof("Creating (interactive) session %v.", sid)

	// Start an interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	if err := sess.startInteractive(ctx, ch, scx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}
	return nil
}

// OpenExecSession opens an non-interactive exec session.
func (s *SessionRegistry) OpenExecSession(ctx context.Context, channel ssh.Channel, scx *ServerContext) error {
	// Create a new session ID. These sessions can not be joined so no point in
	// looking for an exisiting one.
	sessionID := rsession.NewID()

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition.
	sess, err := newSession(ctx, sessionID, s, scx)
	if err != nil {
		return trace.Wrap(err)
	}
	scx.Infof("Creating (exec) session %v.", sessionID)

	canStart, _, err := sess.checkIfStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if !canStart {
		return errCannotStartUnattendedSession
	}

	// Start a non-interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	scx.setSession(sess)
	if err := sess.startExec(ctx, channel, scx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}

	return nil
}

func (s *SessionRegistry) ForceTerminate(ctx *ServerContext) error {
	sess := ctx.getSession()
	if sess == nil {
		s.log.Debug("Unable to terminate session, no session found in context.")
		return nil
	}

	sess.BroadcastMessage("Forcefully terminating session...")

	// Stop session, it will be cleaned up in the background to ensure
	// the session recording is uploaded.
	sess.Stop()

	return nil
}

// NotifyWinChange is called to notify all members in the party that the PTY
// size has changed. The notification is sent as a global SSH request and it
// is the responsibility of the client to update it's window size upon receipt.
func (s *SessionRegistry) NotifyWinChange(ctx context.Context, params rsession.TerminalParams, scx *ServerContext) error {
	session := scx.getSession()
	if session == nil {
		s.log.Debug("Unable to update window size, no session found in context.")
		return nil
	}
	sid := session.id

	// Build the resize event.
	resizeEvent := &apievents.Resize{
		Metadata: apievents.Metadata{
			Type:        events.ResizeEvent,
			Code:        events.TerminalResizeCode,
			ClusterName: scx.ClusterName,
		},
		ServerMetadata: session.serverMeta,
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(sid),
		},
		UserMetadata: scx.Identity.GetUserMetadata(),
		TerminalSize: params.Serialize(),
	}

	// Report the updated window size to the event log (this is so the sessions
	// can be replayed correctly).
	if err := session.recorder.EmitAuditEvent(s.Srv.Context(), resizeEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit resize audit event.")
	}

	// Update the size of the server side PTY.
	err := session.term.SetWinSize(ctx, params)
	if err != nil {
		return trace.Wrap(err)
	}

	// If sessions are being recorded at the proxy, sessions can not be shared.
	// In that situation, PTY size information does not need to be propagated
	// back to all clients and we can return right away.
	if services.IsRecordAtProxy(scx.SessionRecordingConfig.GetMode()) {
		return nil
	}

	// Notify all members of the party (except originator) that the size of the
	// window has changed so the client can update it's own local PTY. Note that
	// OpenSSH clients will ignore this and not update their own local PTY.
	for _, p := range session.getParties() {
		// Don't send the window change notification back to the originator.
		if p.ctx.ID() == scx.ID() {
			continue
		}

		eventPayload, err := json.Marshal(resizeEvent)
		if err != nil {
			s.log.Warnf("Unable to marshal resize event for %v: %v.", p.sconn.RemoteAddr(), err)
			continue
		}

		// Send the message as a global request.
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			s.log.Warnf("Unable to resize event to %v: %v.", p.sconn.RemoteAddr(), err)
			continue
		}
		s.log.Debugf("Sent resize event %v to %v.", params, p.sconn.RemoteAddr())
	}

	return nil
}

func (s *SessionRegistry) broadcastResult(sid rsession.ID, r ExecResult) error {
	s.sessionsMux.Lock()
	defer s.sessionsMux.Unlock()

	sess, found := s.findSessionLocked(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	sess.broadcastResult(r)
	return nil
}

// SessionAccessEvaluator is the interface that defines criteria needed to be met
// in order to start and join sessions.
type SessionAccessEvaluator interface {
	IsModerated() bool
	FulfilledFor(participants []auth.SessionAccessContext) (bool, auth.PolicyOptions, error)
	PrettyRequirementsList() string
	CanJoin(user auth.SessionAccessContext) []types.SessionParticipantMode
}

// session struct describes an active (in progress) SSH session. These sessions
// are managed by 'SessionRegistry' containers which are attached to SSH servers.
type session struct {
	mu sync.RWMutex

	// log holds the structured logger
	log *log.Entry

	// session ID. unique GUID, this is what people use to "join" sessions
	id rsession.ID

	// parent session container
	registry *SessionRegistry

	// parties is the set of current connected clients/users. This map may grow
	// and shrink as members join and leave the session.
	parties map[rsession.ID]*party

	// participants is the set of users that have joined this session. Users are
	// never removed from this map as it's used to report the full list of
	// participants at the end of a session.
	participants map[rsession.ID]*party

	io       *TermManager
	inWriter io.Writer

	term Terminal

	// stopC channel is used to kill all goroutines owned
	// by the session
	stopC chan struct{}

	// startTime is the time when this session was created.
	startTime time.Time

	// login stores the login of the initial session creator
	login string

	recorder events.StreamWriter

	// hasEnhancedRecording returns true if this session has enhanced session
	// recording events associated.
	hasEnhancedRecording bool

	// serverCtx is used to control clean up of internal resources
	serverCtx context.Context

	access SessionAccessEvaluator

	tracker *SessionTracker

	initiator string

	scx *ServerContext

	presenceEnabled bool

	doneCh chan struct{}

	displayParticipantRequirements bool

	// endingContext is the server context which closed this session.
	endingContext *ServerContext

	// lingerAndDieCancel is a context cancel func which will cancel
	// an ongoing lingerAndDie goroutine. This is used by joining parties
	// to cancel the goroutine and prevent the session from closing prematurely.
	lingerAndDieCancel func()

	// serverMeta contains metadata about the target node of this session.
	serverMeta apievents.ServerMetadata
}

// newSession creates a new session with a given ID within a given context.
func newSession(ctx context.Context, id rsession.ID, r *SessionRegistry, scx *ServerContext) (*session, error) {
	serverSessions.Inc()
	startTime := time.Now().UTC()
	rsess := rsession.Session{
		ID: id,
		TerminalParams: rsession.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:          scx.Identity.Login,
		Created:        startTime,
		LastActive:     startTime,
		ServerID:       scx.srv.ID(),
		Namespace:      r.Srv.GetNamespace(),
		ServerHostname: scx.srv.GetInfo().GetHostname(),
		ServerAddr:     scx.ServerConn.LocalAddr().String(),
		ClusterName:    scx.ClusterName,
	}

	term := scx.GetTerm()
	if term != nil {
		winsize, err := term.GetWinSize()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rsess.TerminalParams.W = int(winsize.Width)
		rsess.TerminalParams.H = int(winsize.Height)
	}

	// get the session server where session information lives. if the recording
	// proxy is being used and this is a node, then a discard session server will
	// be returned here.
	sessionServer := r.Srv.GetSessionServer()

	err := sessionServer.CreateSession(ctx, rsess)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			// if session already exists, make sure they are compatible
			// Login matches existing login
			existing, err := sessionServer.GetSession(ctx, r.Srv.GetNamespace(), id)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if existing.Login != rsess.Login {
				return nil, trace.AccessDenied(
					"can't switch users from %v to %v for session %v",
					rsess.Login, existing.Login, id)
			}
		}
		// return nil, trace.Wrap(err)
		// No need to abort. Perhaps the auth server is down?
		// Log the error and continue:
		r.log.Errorf("Failed to create new session: %v.", err)
	}

	var policySets []*types.SessionTrackerPolicySet
	for _, role := range scx.Identity.RoleSet {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}

	access := auth.NewSessionAccessEvaluator(policySets, types.SSHSessionKind)
	sess := &session{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, r.Srv.Component()),
			"session_id":    id,
		}),
		id:                             id,
		registry:                       r,
		parties:                        make(map[rsession.ID]*party),
		participants:                   make(map[rsession.ID]*party),
		login:                          scx.Identity.Login,
		stopC:                          make(chan struct{}),
		startTime:                      startTime,
		serverCtx:                      scx.srv.Context(),
		access:                         &access,
		scx:                            scx,
		presenceEnabled:                scx.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified] != "",
		io:                             NewTermManager(),
		doneCh:                         make(chan struct{}),
		initiator:                      scx.Identity.TeleportUser,
		displayParticipantRequirements: utils.AsBool(scx.env[teleport.EnvSSHSessionDisplayParticipantRequirements]),
		serverMeta:                     scx.srv.TargetMetadata(),
	}

	sess.io.OnWriteError = func(idString string, err error) {
		if idString == sessionRecorderID {
			sess.log.Error("Failed to write to session recorder, stopping session.")
			// stop in goroutine to avoid deadlock
			go sess.Stop()
		}
	}

	go func() {
		if _, open := <-sess.io.TerminateNotifier(); open {
			err := sess.registry.ForceTerminate(sess.scx)
			if err != nil {
				sess.log.Errorf("Failed to terminate session: %v.", err)
			}
		}
	}()

	if err = sess.trackSession(ctx, scx.Identity.TeleportUser, policySets); err != nil {
		if trace.IsNotImplemented(err) {
			return nil, trace.NotImplemented("Attempted to use Moderated Sessions with an Auth Server below the minimum version of 9.0.0.")
		}
		return nil, trace.Wrap(err)
	}

	sess.recorder, err = newRecorder(sess, scx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// ID returns a string representation of the session ID.
func (s *session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id.String()
}

// PID returns the PID of the Teleport process under which the shell is running.
func (s *session) PID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.term.PID()
}

// Recorder returns a events.SessionRecorder which can be used to emit events
// to a session as well as the audit log.
func (s *session) Recorder() events.StreamWriter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recorder
}

// Stop ends the active session and forces all clients to disconnect.
// This will trigger background goroutines to complete session cleanup.
func (s *session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopC:
		return
	default:
		close(s.stopC)
	}

	s.BroadcastMessage("Stopping session...")
	s.log.Info("Stopping session")

	// close io copy loops
	s.io.Close()

	// Close and kill terminal
	if s.term != nil {
		if err := s.term.Close(); err != nil {
			s.log.WithError(err).Debug("Failed to close the shell")
		}
		if err := s.term.Kill(context.TODO()); err != nil {
			s.log.WithError(err).Debug("Failed to kill the shell")
		}
	}

	// Close session tracker and mark it as terminated
	if err := s.tracker.Close(s.serverCtx); err != nil {
		s.log.WithError(err).Debug("Failed to close session tracker")
	}
}

// Close ends the active session and frees all resources. This should only be called
// by the creator of the session, other closers should use Stop instead. Calling this
// prematurely can result in missing audit events, session recordings, and other
// unexpected errors.
func (s *session) Close() error {
	s.Stop()

	s.BroadcastMessage("Closing session...")
	s.log.Infof("Closing session")

	serverSessions.Dec()

	// Remove session parties and close client connections.
	for _, p := range s.getParties() {
		p.Close()
	}

	s.registry.removeSession(s)

	// Remove the session from the backend.
	if s.scx.srv.GetSessionServer() != nil {
		err := s.scx.srv.GetSessionServer().DeleteSession(s.serverCtx, s.scx.srv.GetNamespace(), s.id)
		if err != nil {
			s.log.Errorf("Failed to remove active session: %v: %v. "+
				"Access to backend may be degraded, check connectivity to backend.",
				s.id, err)
		}
	}

	// Complete the session recording
	if s.recorder != nil {
		if err := s.recorder.Complete(s.serverCtx); err != nil {
			s.log.WithError(err).Warn("Failed to close recorder.")
		}
	}

	return nil
}

func (s *session) BroadcastMessage(format string, args ...interface{}) {
	if s.access.IsModerated() && !services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode()) {
		s.io.BroadcastMessage(fmt.Sprintf(format, args...))
	}
}

// emitSessionStartEvent emits a session start event.
func (s *session) emitSessionStartEvent(ctx *ServerContext) {
	var initialCommand []string
	if execRequest, err := ctx.GetExecRequest(); err == nil {
		initialCommand = []string{execRequest.GetCommand()}
	}
	sessionStartEvent := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: ctx.ClusterName,
			ID:          uuid.New().String(),
		},
		ServerMetadata: s.serverMeta,
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(s.id),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
			Protocol:   events.EventProtocolSSH,
		},
		SessionRecording: ctx.SessionRecordingConfig.GetMode(),
		InitialCommand:   initialCommand,
	}

	if s.term != nil {
		params := s.term.GetTerminalParams()
		sessionStartEvent.TerminalSize = params.Serialize()
	}

	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionStartEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}

	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionStartEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session start event.")
	}
}

// emitSessionJoinEvent emits a session join event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
// Must be called under session Lock.
func (s *session) emitSessionJoinEvent(ctx *ServerContext) {
	sessionJoinEvent := &apievents.SessionJoin{
		Metadata: apievents.Metadata{
			Type:        events.SessionJoinEvent,
			Code:        events.SessionJoinCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata: s.serverMeta,
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(ctx.SessionID()),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		},
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionJoinEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}

	// Emit session join event to Audit Log.
	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionJoinEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session join event.")
	}

	// Notify all members of the party that a new member has joined over the
	// "x-teleport-event" channel.
	for _, p := range s.parties {
		eventPayload, err := json.Marshal(sessionJoinEvent)
		if err != nil {
			s.log.Warnf("Unable to marshal %v for %v: %v.", events.SessionJoinEvent, p.sconn.RemoteAddr(), err)
			continue
		}
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			s.log.Warnf("Unable to send %v to %v: %v.", events.SessionJoinEvent, p.sconn.RemoteAddr(), err)
			continue
		}
		s.log.Debugf("Sent %v to %v.", events.SessionJoinEvent, p.sconn.RemoteAddr())
	}
}

// emitSessionLeaveEvent emits a session leave event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
// Must be called under session Lock.
func (s *session) emitSessionLeaveEvent(ctx *ServerContext) {
	sessionLeaveEvent := &apievents.SessionLeave{
		Metadata: apievents.Metadata{
			Type:        events.SessionLeaveEvent,
			Code:        events.SessionLeaveCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata: s.serverMeta,
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(s.id),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
	}

	// Emit session leave event to Audit Log.
	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionLeaveEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session leave event.")
	}

	// Notify all members of the party that a new member has left over the
	// "x-teleport-event" channel.
	for _, p := range s.parties {
		eventPayload, err := utils.FastMarshal(sessionLeaveEvent)
		if err != nil {
			s.log.Warnf("Unable to marshal %v for %v: %v.", events.SessionLeaveEvent, p.sconn.RemoteAddr(), err)
			continue
		}
		_, _, err = p.sconn.SendRequest(teleport.SessionEvent, false, eventPayload)
		if err != nil {
			// The party's connection may already be closed, in which case we expect an EOF
			if !trace.IsEOF(err) {
				s.log.Warnf("Unable to send %v to %v: %v.", events.SessionLeaveEvent, p.sconn.RemoteAddr(), err)
			}
			continue
		}
		s.log.Debugf("Sent %v to %v.", events.SessionLeaveEvent, p.sconn.RemoteAddr())
	}
}

// emitSessionEndEvent emits a session end event.
func (s *session) emitSessionEndEvent() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := s.scx
	if s.endingContext != nil {
		ctx = s.endingContext
	}

	start, end := s.startTime, time.Now().UTC()
	sessionEndEvent := &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Type:        events.SessionEndEvent,
			Code:        events.SessionEndCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata: s.serverMeta,
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(s.id),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
			Protocol:   events.EventProtocolSSH,
		},
		EnhancedRecording: s.hasEnhancedRecording,
		Interactive:       s.term != nil,
		StartTime:         start,
		EndTime:           end,
		SessionRecording:  ctx.SessionRecordingConfig.GetMode(),
	}

	for _, p := range s.participants {
		sessionEndEvent.Participants = append(sessionEndEvent.Participants, p.user)
	}

	// If there are 0 participants, this is an exec session.
	// Use the user from the session context.
	if len(s.participants) == 0 {
		sessionEndEvent.Participants = []string{s.scx.Identity.TeleportUser}
	}

	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionEndEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session end event.")
	}
}

func (s *session) setEndingContext(ctx *ServerContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endingContext = ctx
}

func (s *session) setHasEnhancedRecording(val bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasEnhancedRecording = val
}

// launch launches the session.
// Must be called under session Lock.
func (s *session) launch(ctx *ServerContext) error {
	s.log.Debug("Launching session")
	s.BroadcastMessage("Connecting to %v over SSH", s.serverMeta.ServerHostname)

	s.io.On()

	if err := s.tracker.UpdateState(s.serverCtx, types.SessionState_SessionStateRunning); err != nil {
		s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStateRunning)
	}

	// If the identity is verified with an MFA device, we enabled MFA-based presence for the session.
	if s.presenceEnabled {
		go func() {
			ticker := time.NewTicker(PresenceVerifyInterval)
			defer ticker.Stop()
		outer:
			for {
				select {
				case <-ticker.C:
					err := s.checkPresence()
					if err != nil {
						s.log.WithError(err).Error("Failed to check presence, terminating session as a security measure")
						s.Stop()
					}
				case <-s.stopC:
					break outer
				}
			}
		}()
	}

	// copy everything from the pty to the writer. this lets us capture all input
	// and output of the session (because input is echoed to stdout in the pty).
	// the writer contains multiple writers: the session logger and a direct
	// connection to members of the "party" (other people in the session).
	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)

		// once everything has been copied, notify the goroutine below. if this code
		// is running in a teleport node, when the exec.Cmd is done it will close
		// the PTY, allowing io.Copy to return. if this is a teleport forwarding
		// node, when the remote side closes the channel (which is what s.term.PTY()
		// returns) io.Copy will return.
		defer close(s.doneCh)

		_, err := io.Copy(s.io, s.term.PTY())
		s.log.Debugf("Copying from PTY to writer completed with error %v.", err)
	}()

	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)

		_, err := io.Copy(s.term.PTY(), s.io)
		s.log.Debugf("Copying from reader to PTY completed with error %v.", err)
	}()

	return nil
}

// startInteractive starts a new interactive process (or a shell) in the
// current session.
func (s *session) startInteractive(ctx context.Context, ch ssh.Channel, scx *ServerContext) error {
	inReader, inWriter := io.Pipe()
	s.inWriter = inWriter
	s.io.AddReader("reader", inReader)
	s.io.AddWriter(sessionRecorderID, utils.WriteCloserWithContext(scx.srv.Context(), s.recorder))
	s.BroadcastMessage("Creating session with ID: %v...", s.id)
	s.BroadcastMessage(SessionControlsInfoBroadcast)

	if err := s.startTerminal(ctx, scx); err != nil {
		return trace.Wrap(err)
	}

	// Emit a session.start event for the interactive session.
	s.emitSessionStartEvent(scx)

	// create a new "party" (connected client) and launch/join the session.
	p := newParty(s, types.SessionPeerMode, ch, scx)
	if err := s.addParty(p, types.SessionPeerMode); err != nil {
		return trace.Wrap(err)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		Context:   scx.srv.Context(),
		PID:       s.term.PID(),
		Emitter:   s.recorder,
		Namespace: scx.srv.GetNamespace(),
		SessionID: s.id.String(),
		ServerID:  scx.srv.HostUUID(),
		Login:     scx.Identity.Login,
		User:      scx.Identity.TeleportUser,
		Events:    scx.Identity.RoleSet.EnhancedRecordingSet(),
	}

	if cgroupID, err := scx.srv.GetBPF().OpenSession(sessionContext); err != nil {
		s.log.WithError(err).Error("Failed to open enhanced recording (interactive) session")
		return trace.Wrap(err)
	} else if cgroupID > 0 {
		// If a cgroup ID was assigned then enhanced session recording was enabled.
		s.setHasEnhancedRecording(true)
		scx.srv.GetRestrictedSessionManager().OpenSession(sessionContext, cgroupID)
		go func() {
			// Close the BPF recording session once the session is closed
			<-s.stopC
			scx.srv.GetRestrictedSessionManager().CloseSession(sessionContext, cgroupID)
			err = scx.srv.GetBPF().CloseSession(sessionContext)
			if err != nil {
				s.log.WithError(err).Error("Failed to close enhanced recording (interactive) session")
			}
		}()
	}

	s.log.Debug("Waiting for continue signal")

	// Process has been placed in a cgroup, continue execution.
	s.term.Continue()

	s.log.Debug("Got continue signal")

	// Start a heartbeat that marks this session as active with current members
	// of party in the backend.
	go s.heartbeat(ctx, scx)

	// wait for exec.Cmd (or receipt of "exit-status" for a forwarding node),
	// once it is received wait for the io.Copy above to finish, then broadcast
	// the "exit-status" to the client.
	go func() {
		result, err := s.term.Wait()
		if err != nil {
			s.log.WithError(err).Error("Received error waiting for the interactive session to finish")
		}

		// wait for copying from the pty to be complete or a timeout before
		// broadcasting the result (which will close the pty) if it has not been
		// closed already.
		select {
		case <-time.After(defaults.WaitCopyTimeout):
			s.log.Error("Timed out waiting for PTY copy to finish, session data  may be missing.")
		case <-s.doneCh:
		}

		if execRequest, err := scx.GetExecRequest(); err == nil && execRequest.GetCommand() != "" {
			emitExecAuditEvent(scx, execRequest.GetCommand(), err)
		}

		if result != nil {
			if err := s.registry.broadcastResult(s.id, *result); err != nil {
				s.log.Warningf("Failed to broadcast session result: %v", err)
			}
		}

		s.emitSessionEndEvent()
		s.Close()
	}()

	return nil
}

func (s *session) startTerminal(ctx context.Context, scx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// allocate a terminal or take the one previously allocated via a
	// separate "allocate TTY" SSH request
	var err error
	if s.term = scx.GetTerm(); s.term != nil {
		scx.SetTerm(nil)
	} else if s.term, err = NewTerminal(scx); err != nil {
		s.log.Infof("Unable to allocate new terminal: %v", err)
		return trace.Wrap(err)
	}

	if err := s.term.Run(ctx); err != nil {
		s.log.Errorf("Unable to run shell command: %v.", err)
		return trace.ConvertSystemError(err)
	}

	return nil
}

// newRecorder creates a new events.StreamWriter to be used as the recorder
// of the passed in session.
func newRecorder(s *session, ctx *ServerContext) (events.StreamWriter, error) {
	// Nodes discard events in cases when proxies are already recording them.
	if s.registry.Srv.Component() == teleport.ComponentNode &&
		services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		return &events.DiscardStream{}, nil
	}

	streamer, err := s.newStreamer(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rec, err := events.NewAuditWriter(events.AuditWriterConfig{
		// Audit stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context:      ctx.srv.Context(),
		Streamer:     streamer,
		SessionID:    s.id,
		Clock:        s.registry.clock,
		Namespace:    s.serverMeta.ServerNamespace,
		ServerID:     s.serverMeta.ServerID,
		RecordOutput: ctx.SessionRecordingConfig.GetMode() != types.RecordOff,
		Component:    teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		ClusterName:  ctx.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rec, nil
}

func (s *session) startExec(ctx context.Context, channel ssh.Channel, scx *ServerContext) error {
	// Emit a session.start event for the exec session.
	s.emitSessionStartEvent(scx)

	execRequest, err := scx.GetExecRequest()
	if err != nil {
		return trace.Wrap(err)
	}

	// Start execution. If the program failed to start, send that result back.
	// Note this is a partial start. Teleport will have re-exec'ed itself and
	// wait until it's been placed in a cgroup and told to continue.
	result, err := execRequest.Start(ctx, channel)
	if err != nil {
		return trace.Wrap(err)
	}
	if result != nil {
		s.log.Debugf("Exec request (%v) result: %v.", execRequest, result)
		scx.SendExecResult(*result)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		Context:   scx.srv.Context(),
		PID:       execRequest.PID(),
		Emitter:   s.recorder,
		Namespace: scx.srv.GetNamespace(),
		SessionID: string(s.id),
		ServerID:  scx.srv.HostUUID(),
		Login:     scx.Identity.Login,
		User:      scx.Identity.TeleportUser,
		Events:    scx.Identity.RoleSet.EnhancedRecordingSet(),
	}
	cgroupID, err := scx.srv.GetBPF().OpenSession(sessionContext)
	if err != nil {
		s.log.WithError(err).Errorf("Failed to open enhanced recording (exec) session: %v", execRequest.GetCommand())
		return trace.Wrap(err)
	}

	// If a cgroup ID was assigned then enhanced session recording was enabled.
	if cgroupID > 0 {
		s.setHasEnhancedRecording(true)
		scx.srv.GetRestrictedSessionManager().OpenSession(sessionContext, cgroupID)
	}

	// Process has been placed in a cgroup, continue execution.
	execRequest.Continue()

	// Process is running, wait for it to stop.
	go func() {
		result = execRequest.Wait()
		if result != nil {
			scx.SendExecResult(*result)
		}

		// Wait a little bit to let all events filter through before closing the
		// BPF session so everything can be recorded.
		time.Sleep(2 * time.Second)

		scx.srv.GetRestrictedSessionManager().CloseSession(sessionContext, cgroupID)

		// Close the BPF recording session. If BPF was not configured, not available,
		// or running in a recording proxy, this is simply a NOP.
		err = scx.srv.GetBPF().CloseSession(sessionContext)
		if err != nil {
			s.log.WithError(err).Error("Failed to close enhanced recording (exec) session")
		}

		s.emitSessionEndEvent()
		s.Close()
	}()

	return nil
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (s *session) newStreamer(ctx *ServerContext) (events.Streamer, error) {
	mode := ctx.SessionRecordingConfig.GetMode()
	if services.IsRecordSync(mode) {
		s.log.Debug("Using sync streamer for session")
		return ctx.srv, nil
	}

	if ctx.IsTestStub {
		s.log.Debugf("Using discard streamer for test")
		return events.NewDiscardEmitter(), nil
	}

	s.log.Debug("Using async streamer for session")
	fileStreamer, err := filesessions.NewStreamer(sessionsStreamingUploadDir(ctx))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TeeStreamer sends non-print and non disk events
	// to the audit log in async mode, while buffering all
	// events on disk for further upload at the end of the session.
	return events.NewTeeStreamer(fileStreamer, ctx.srv), nil
}

func sessionsStreamingUploadDir(ctx *ServerContext) string {
	return filepath.Join(
		ctx.srv.GetDataDir(), teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingSessionsDir, ctx.srv.GetNamespace(),
	)
}

func (s *session) broadcastResult(r ExecResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.parties {
		p.ctx.SendExecResult(r)
	}
}

func (s *session) String() string {
	return fmt.Sprintf("session(id=%v, parties=%v)", s.id, len(s.parties))
}

// removePartyUnderLock removes the party from the in-memory map that holds all party members
// and closes their underlying ssh channels. This may also trigger the session to end
// if the party is the last in the session or has policies that dictate it to end.
// Must be called under session Lock.
func (s *session) removePartyUnderLock(p *party) error {
	s.log.Infof("Removing party %v from session", p)

	// Remove participant from in-memory map of party members.
	delete(s.parties, p.id)

	s.BroadcastMessage("User %v left the session.", p.user)

	// Update session tracker
	s.log.Debugf("No longer tracking participant: %v", p.id)
	if err := s.tracker.RemoveParticipant(s.serverCtx, p.id.String()); err != nil {
		return trace.Wrap(err)
	}

	// Remove party for the term writer
	s.io.DeleteWriter(string(p.id))

	// Emit session leave event to both the Audit Log as well as over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionLeaveEvent(p.ctx)

	canRun, policyOptions, err := s.checkIfStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if !canRun {
		if policyOptions.TerminateOnLeave {
			// Force termination in goroutine to avoid deadlock
			go s.registry.ForceTerminate(s.scx)
			return nil
		}

		// pause session and wait for another party to resume
		s.io.Off()
		s.BroadcastMessage("Session paused, Waiting for required participants...")
		if err := s.tracker.UpdateState(s.serverCtx, types.SessionState_SessionStatePending); err != nil {
			s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStatePending)
		}

		go func() {
			if state := s.tracker.WaitForStateUpdate(types.SessionState_SessionStatePending); state == types.SessionState_SessionStateRunning {
				s.BroadcastMessage("Resuming session...")
				s.io.On()
			}
		}()
	}

	// If the leaving party was the last one in the session, start the lingerAndDie
	// goroutine. Parties that join during the linger duration will cancel the
	// goroutine to prevent the session from ending with active parties.
	if len(s.parties) == 0 && !s.isStopped() {
		ctx, cancel := context.WithCancel(s.serverCtx)
		s.lingerAndDieCancel = cancel
		go s.lingerAndDie(ctx, p)
	}

	return nil
}

// isStopped does not need to be called under sessionLock
func (s *session) isStopped() bool {
	select {
	case <-s.stopC:
		return true
	default:
		return false
	}
}

// lingerAndDie will let the party-less session linger for a short
// duration, and then die if no parties have joined.
func (s *session) lingerAndDie(ctx context.Context, party *party) {
	s.log.Debug("Session has no active party members.")

	select {
	case <-s.registry.clock.After(defaults.SessionIdlePeriod):
		s.log.Info("Session will be garbage collected.")

		// set closing context to the leaving party to show who ended the session.
		s.setEndingContext(party.ctx)

		// Stop the session, and let the background processes
		// complete cleanup and close the session.
		s.Stop()
	case <-ctx.Done():
		s.log.Info("Session has become active again.")
		return
	case <-s.stopC:
		return
	}
}

// exportPartyMembers exports participants in the in-memory map of party
// members.
func (s *session) exportPartyMembers() []rsession.Party {
	s.mu.Lock()
	defer s.mu.Unlock()

	var partyList []rsession.Party
	for _, p := range s.parties {
		partyList = append(partyList, rsession.Party{
			ID:         p.id,
			User:       p.user,
			ServerID:   p.serverID,
			RemoteAddr: p.site,
			LastActive: p.getLastActive(),
		})
	}

	return partyList
}

// heartbeat will loop as long as the session is not closed and mark it as
// active and update the list of party members. If the session are recorded at
// the proxy, then this function does nothing as it's counterpart
// in the proxy will do this work.
func (s *session) heartbeat(ctx context.Context, scx *ServerContext) {
	// If sessions are being recorded at the proxy, an identical version of this
	// goroutine is running in the proxy, which means it does not need to run here.
	if services.IsRecordAtProxy(scx.SessionRecordingConfig.GetMode()) &&
		s.registry.Srv.Component() == teleport.ComponentNode {
		return
	}

	// If no session server (endpoint interface for active sessions) is passed in
	// (for example Teleconsole does this) then nothing to sync.
	sessionServer := s.registry.Srv.GetSessionServer()
	if sessionServer == nil {
		return
	}

	s.log.Debugf("Starting poll and sync of terminal size to all parties.")
	defer s.log.Debugf("Stopping poll and sync of terminal size to all parties.")

	tickerCh := time.NewTicker(defaults.SessionRefreshPeriod)
	defer tickerCh.Stop()

	// Loop as long as the session is active, updating the session in the backend.
	for {
		select {
		case <-tickerCh.C:
			partyList := s.exportPartyMembers()

			err := sessionServer.UpdateSession(ctx, rsession.UpdateRequest{
				Namespace: scx.srv.GetNamespace(),
				ID:        s.id,
				Parties:   &partyList,
			})

			switch {
			case trace.IsNotFound(err):
				s.log.Warnf("Aborting heartbeat for non-existent session %v ", s.id)
				return
			case err != nil:
				s.log.Warnf("Unable to update session %v as active: %v", s.id, err)
			}
		case <-ctx.Done():
			return
		case <-s.stopC:
			return
		}
	}
}

func (s *session) checkPresence() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, participant := range s.tracker.GetParticipants() {
		if participant.User == s.initiator {
			continue
		}

		if participant.Mode == string(types.SessionModeratorMode) && time.Now().UTC().After(participant.LastActive.Add(PresenceMaxDifference)) {
			s.log.Warnf("Participant %v is not active, kicking.", participant.ID)
			party := s.parties[rsession.ID(participant.ID)]
			if party != nil {
				party.closeUnderSessionLock()
			}
		}
	}

	return nil
}

func (s *session) checkIfStart() (bool, auth.PolicyOptions, error) {
	var participants []auth.SessionAccessContext

	for _, party := range s.parties {
		if party.ctx.Identity.TeleportUser == s.initiator {
			continue
		}

		participants = append(participants, auth.SessionAccessContext{
			Username: party.ctx.Identity.TeleportUser,
			Roles:    party.ctx.Identity.RoleSet,
			Mode:     party.mode,
		})
	}

	shouldStart, policyOptions, err := s.access.FulfilledFor(participants)
	if err != nil {
		return false, auth.PolicyOptions{}, trace.Wrap(err)
	}

	return shouldStart, policyOptions, nil
}

// addParty is called when a new party joins the session.
func (s *session) addParty(p *party, mode types.SessionParticipantMode) error {
	if s.login != p.login {
		return trace.AccessDenied(
			"can't switch users from %v to %v for session %v",
			s.login, p.login, s.id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.login != p.login {
		return trace.AccessDenied(
			"can't switch users from %v to %v for session %v",
			s.login, p.login, s.id)
	}

	if s.tracker.GetState() == types.SessionState_SessionStateTerminated {
		return trace.AccessDenied("The requested session is not active")
	}

	if len(s.parties) == 0 {
		canStart, _, err := s.checkIfStart()
		if err != nil {
			return trace.Wrap(err)
		}

		if !canStart && services.IsRecordAtProxy(p.ctx.SessionRecordingConfig.GetMode()) {
			go s.Stop()
			return trace.AccessDenied("session requires additional moderation but is in proxy-record mode")
		}
	}

	// Cancel lingerAndDie goroutine if one is running.
	if s.lingerAndDieCancel != nil {
		s.lingerAndDieCancel()
		s.lingerAndDieCancel = nil
	}

	// Adds participant to in-memory map of party members.
	s.parties[p.id] = p
	s.participants[p.id] = p
	p.ctx.AddCloser(p)

	s.log.Debugf("Tracking participant: %s", p.id)
	participant := &types.Participant{
		ID:         p.id.String(),
		User:       p.user,
		Mode:       string(p.mode),
		LastActive: time.Now().UTC(),
	}
	if err := s.tracker.AddParticipant(s.serverCtx, participant); err != nil {
		return trace.Wrap(err)
	}

	// Write last chunk (so the newly joined parties won't stare at a blank
	// screen).
	if _, err := p.Write(s.io.GetRecentHistory()); err != nil {
		return trace.Wrap(err)
	}

	// Register this party as one of the session writers (output will go to it).
	s.io.AddWriter(string(p.id), p)

	s.BroadcastMessage("User %v joined the session.", p.user)
	s.log.Infof("New party %v joined session", p.String())

	if mode == types.SessionPeerMode {
		s.term.AddParty(1)

		// This goroutine keeps pumping party's input into the session.
		go func() {
			defer s.term.AddParty(-1)
			_, err := io.Copy(s.inWriter, p)
			s.log.Debugf("Copying from Party %v to session writer completed with error %v.", p.id, err)
		}()
	}

	if s.tracker.GetState() == types.SessionState_SessionStatePending {
		canStart, _, err := s.checkIfStart()
		if err != nil {
			return trace.Wrap(err)
		}

		if canStart {
			if err := s.launch(s.scx); err != nil {
				s.log.WithError(err).Error("Failed to launch session")
			}
			return nil
		}

		base := "Waiting for required participants..."
		if s.displayParticipantRequirements {
			s.BroadcastMessage(base+"\r\n%v", s.access.PrettyRequirementsList())
		} else {
			s.BroadcastMessage(base)
		}
	}

	return nil
}

func (s *session) join(ch ssh.Channel, ctx *ServerContext, mode types.SessionParticipantMode) (*party, error) {
	if ctx.Identity.TeleportUser != s.initiator {
		roles := []types.Role(ctx.Identity.RoleSet)
		accessContext := auth.SessionAccessContext{
			Roles: roles,
		}

		modes := s.access.CanJoin(accessContext)
		if !auth.SliceContainsMode(modes, mode) {
			return nil, trace.AccessDenied("insufficient permissions to join session %v", s.id)
		}

		if s.presenceEnabled {
			_, err := ch.SendRequest(teleport.MFAPresenceRequest, false, nil)
			if err != nil {
				return nil, trace.WrapWithMessage(err, "failed to send MFA presence request")
			}
		}
	}

	p := newParty(s, mode, ch, ctx)
	if err := s.addParty(p, mode); err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit session join event to both the Audit Log as well as over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionJoinEvent(p.ctx)

	return p, nil
}

func (s *session) getParties() (parties []*party) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.parties {
		parties = append(parties, p)
	}
	return parties
}

type party struct {
	sync.Mutex

	log        *log.Entry
	login      string
	user       string
	serverID   string
	site       string
	id         rsession.ID
	s          *session
	sconn      *ssh.ServerConn
	ch         ssh.Channel
	ctx        *ServerContext
	lastActive time.Time
	mode       types.SessionParticipantMode
	closeOnce  sync.Once
}

func newParty(s *session, mode types.SessionParticipantMode, ch ssh.Channel, ctx *ServerContext) *party {
	return &party{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		}),
		user:     ctx.Identity.TeleportUser,
		login:    ctx.Identity.Login,
		serverID: s.registry.Srv.ID(),
		site:     ctx.ServerConn.RemoteAddr().String(),
		id:       rsession.NewID(),
		ch:       ch,
		ctx:      ctx,
		s:        s,
		sconn:    ctx.ServerConn,
		mode:     mode,
	}
}

func (p *party) updateActivity() {
	p.Lock()
	defer p.Unlock()
	p.lastActive = time.Now()
}

func (p *party) getLastActive() time.Time {
	p.Lock()
	defer p.Unlock()
	return p.lastActive
}

func (p *party) Read(bytes []byte) (int, error) {
	p.updateActivity()
	return p.ch.Read(bytes)
}

func (p *party) Write(bytes []byte) (int, error) {
	return p.ch.Write(bytes)
}

func (p *party) String() string {
	return fmt.Sprintf("%v party(id=%v)", p.ctx, p.id)
}

// Close is called when the party's session ctx is closed.
func (p *party) Close() error {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	p.closeUnderSessionLock()
	return nil
}

// closeUnderSessionLock closes the party, and removes it from it's session.
// Must be called under session Lock.
func (p *party) closeUnderSessionLock() {
	p.closeOnce.Do(func() {
		p.log.Infof("Closing party %v", p.id)
		// Remove party from its session
		if err := p.s.removePartyUnderLock(p); err != nil {
			p.ctx.Errorf("Failed to remove party %v: %v", p.id, err)
		}
		p.ch.Close()
	})
}

// trackSession creates a new session tracker for the ssh session.
// While ctx is open, the session tracker's expiration will be extended
// on an interval until the session tracker is closed.
func (s *session) trackSession(ctx context.Context, teleportUser string, policySet []*types.SessionTrackerPolicySet) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:   s.id.String(),
		Kind:        string(types.SSHSessionKind),
		State:       types.SessionState_SessionStatePending,
		Hostname:    s.serverMeta.ServerHostname,
		Address:     s.serverMeta.ServerAddr,
		ClusterName: s.scx.ClusterName,
		Login:       s.login,
		Participants: []types.Participant{{
			ID:         teleportUser,
			User:       teleportUser,
			LastActive: s.registry.clock.Now(),
		}},
		HostUser:     teleportUser,
		Reason:       s.scx.env[teleport.EnvSSHSessionReason],
		HostPolicies: policySet,
		Created:      s.registry.clock.Now(),
	}

	if s.scx.env[teleport.EnvSSHSessionInvited] != "" {
		if err := json.Unmarshal([]byte(s.scx.env[teleport.EnvSSHSessionInvited]), &trackerSpec.Invited); err != nil {
			return trace.Wrap(err)
		}
	}

	svc := s.registry.SessionTrackerService
	// only propagate the session tracker when the recording mode and component are in sync
	if (s.registry.Srv.Component() == teleport.ComponentNode && services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode())) ||
		(s.registry.Srv.Component() == teleport.ComponentProxy && !services.IsRecordAtProxy(s.scx.SessionRecordingConfig.GetMode())) {
		svc = nil
	}

	s.log.Debug("Attempting to create session tracker")
	tracker, err := NewSessionTracker(ctx, trackerSpec, svc)
	switch {
	// there was an error creating the tracker for a moderated session - terminate the session
	case err != nil && svc != nil && s.access.IsModerated():
		s.log.WithError(err).Warn("Failed to create session tracker, unable to proceed for moderated session")
		return trace.Wrap(err)
	// there was an error creating the tracker for a non-moderated session - permit the session with a local tracker
	case err != nil && svc != nil && !s.access.IsModerated():
		s.log.Warn("Failed to create session tracker, proceeding with local session tracker for non-moderated session")

		localTracker, err := NewSessionTracker(ctx, trackerSpec, nil)
		// this error means there are problems with the trackerSpec, we need to return it
		if err != nil {
			return trace.Wrap(err)
		}

		s.tracker = localTracker
	// there was an error even though the tracker wasn't being propagated - return it
	case err != nil && svc == nil:
		return trace.Wrap(err)
	// the tracker was created successfully
	case err == nil:
		s.tracker = tracker
	}

	go func() {
		ctx, span := tracing.DefaultProvider().Tracer("session").Start(
			s.serverCtx,
			"session/UpdateExpirationLoop",
			oteltrace.WithLinks(oteltrace.LinkFromContext(ctx)),
			oteltrace.WithAttributes(
				attribute.String("session_id", s.id.String()),
				attribute.String("kind", string(types.SSHSessionKind)),
			),
		)
		defer span.End()

		if err := s.tracker.UpdateExpirationLoop(ctx, s.registry.clock); err != nil {
			s.log.WithError(err).Warn("Failed to update session tracker expiration")
		}
	}()

	return nil
}

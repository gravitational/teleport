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

	"golang.org/x/crypto/ssh"

	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
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

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const PresenceVerifyInterval = time.Second * 15
const PresenceMaxDifference = time.Minute

var serverSessions = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: teleport.MetricServerInteractiveSessions,
		Help: "Number of active sessions to this host",
	},
)

// SessionRegistry holds a map of all active sessions on a given
// SSH server
type SessionRegistry struct {
	mu sync.Mutex

	// log holds the structured logger
	log *log.Entry

	// sessions holds a map between session ID and the session object. Used to
	// find active sessions as well as close all sessions when the registry
	// is closing.
	sessions map[rsession.ID]*session

	// srv refers to the upon which this session registry is created.
	srv Server

	auth auth.ClientI
}

func NewSessionRegistry(srv Server, auth auth.ClientI) (*SessionRegistry, error) {
	err := utils.RegisterPrometheusCollectors(serverSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if srv.GetSessionServer() == nil {
		return nil, trace.BadParameter("session server is required")
	}

	return &SessionRegistry{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, srv.Component()),
		}),
		srv:      srv,
		sessions: make(map[rsession.ID]*session),
		auth:     auth,
	}, nil
}

func (s *SessionRegistry) addSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.id] = sess
}

func (s *SessionRegistry) removeSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sess.id)
}

func (s *SessionRegistry) findSessionLocked(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func (s *SessionRegistry) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, se := range s.sessions {
		se.Close()
	}

	s.log.Debug("Closing Session Registry.")
}

// emitSessionJoinEvent emits a session join event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
func (s *SessionRegistry) emitSessionJoinEvent(ctx *ServerContext) {
	sessionJoinEvent := &apievents.SessionJoin{
		Metadata: apievents.Metadata{
			Type:        events.SessionJoinEvent,
			Code:        events.SessionJoinCode,
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.srv.HostUUID(),
			ServerLabels:    ctx.srv.GetInfo().GetAllLabels(),
			ServerNamespace: s.srv.GetNamespace(),
			ServerHostname:  s.srv.GetInfo().GetHostname(),
			ServerAddr:      ctx.ServerConn.LocalAddr().String(),
		},
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
	session := ctx.getSession()
	if err := session.recorder.EmitAuditEvent(ctx.srv.Context(), sessionJoinEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session join event.")
	}

	// Notify all members of the party that a new member has joined over the
	// "x-teleport-event" channel.
	for _, p := range session.getParties() {
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

// OpenSession either joins an existing session or starts a new session.
func (s *SessionRegistry) OpenSession(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	session := ctx.getSession()
	if session != nil {
		ctx.Infof("Joining existing session %v.", session.id)

		mode := types.SessionParticipantMode(ctx.env[teleport.EnvSSHJoinMode])
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
		_, err := session.join(ch, req, ctx, mode)
		if err != nil {
			return trace.Wrap(err)
		}

		// Emit session join event to both the Audit Log as well as over the
		// "x-teleport-event" channel in the SSH connection.
		s.emitSessionJoinEvent(ctx)

		return nil
	}
	// session not found? need to create one. start by getting/generating an ID for it
	sid, found := ctx.GetEnv(sshutils.SessionEnvVar)
	if !found {
		sid = string(rsession.NewID())
		ctx.SetEnv(sshutils.SessionEnvVar, sid)
	}
	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	sess, err := newSession(rsession.ID(sid), s, ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx.setSession(sess)
	s.addSession(sess)
	ctx.Infof("Creating (interactive) session %v.", sid)

	// Start an interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	if err := sess.startInteractive(ch, ctx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}
	return nil
}

// OpenExecSession opens an non-interactive exec session.
func (s *SessionRegistry) OpenExecSession(channel ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	// Create a new session ID. These sessions can not be joined so no point in
	// looking for an exisiting one.
	sessionID := rsession.NewID()

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition.
	sess, err := newSession(sessionID, s, ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx.Infof("Creating (exec) session %v.", sessionID)

	canStart, _, err := sess.checkIfStart()
	if err != nil {
		return trace.Wrap(err)
	}

	if !canStart {
		return trace.AccessDenied("lacking privileges to start unattended session")
	}

	// Start a non-interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
	ctx.setSession(sess)
	err = sess.startExec(channel, ctx)
	defer sess.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// emitSessionLeaveEvent emits a session leave event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
func (s *SessionRegistry) emitSessionLeaveEvent(party *party) {
	sessionLeaveEvent := &apievents.SessionLeave{
		Metadata: apievents.Metadata{
			Type:        events.SessionLeaveEvent,
			Code:        events.SessionLeaveCode,
			ClusterName: party.ctx.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        party.ctx.srv.HostUUID(),
			ServerLabels:    party.ctx.srv.GetInfo().GetAllLabels(),
			ServerNamespace: s.srv.GetNamespace(),
			ServerHostname:  s.srv.GetInfo().GetHostname(),
			ServerAddr:      party.ctx.ServerConn.LocalAddr().String(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: party.s.ID(),
		},
		UserMetadata: apievents.UserMetadata{
			User: party.user,
		},
	}

	// Emit session leave event to Audit Log.
	if err := party.s.recorder.EmitAuditEvent(s.srv.Context(), sessionLeaveEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session leave event.")
	}

	// Notify all members of the party that a new member has left over the
	// "x-teleport-event" channel.
	for _, p := range party.s.getParties() {
		eventPayload, err := utils.FastMarshal(sessionLeaveEvent)
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

func (s *SessionRegistry) ForceTerminate(ctx *ServerContext) error {
	sess := ctx.getSession()
	if sess == nil {
		s.log.Debug("Unable to update window size, no session found in context.")
		return nil
	}

	sess.terminated = true
	sess.io.BroadcastMessage("Forcefully terminating session...")
	sess.term.Kill()
	sess.doneCh <- struct{}{}

	if err := sess.Close(); err != nil {
		s.log.Errorf("Unable to close session %v: %v", sess.id, err)
	}

	s.removeSession(sess)

	// Remove the session from the backend.
	if s.srv.GetSessionServer() != nil {
		err := s.srv.GetSessionServer().DeleteSession(s.srv.GetNamespace(), sess.id)
		if err != nil {
			s.log.Errorf("Failed to remove active session: %v: %v. "+
				"Access to backend may be degraded, check connectivity to backend.",
				sess.id, err)
		}
	}

	return nil
}

// leaveSession removes the given party from this session.
func (s *SessionRegistry) leaveSession(party *party) error {
	sess := party.s
	s.mu.Lock()
	defer s.mu.Unlock()

	// Emit session leave event to both the Audit Log as well as over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionLeaveEvent(party)

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Remove member from in-members representation of party.
	if err := sess.removeParty(party); err != nil {
		return trace.Wrap(err)
	}

	// this goroutine runs for a short amount of time only after a session
	// becomes empty (no parties). It allows session to "linger" for a bit
	// allowing parties to reconnect if they lost connection momentarily
	lingerAndDie := func() {
		lingerTTL := sess.GetLingerTTL()
		if lingerTTL > 0 {
			time.Sleep(lingerTTL)
		}
		// not lingering anymore? someone reconnected? cool then... no need
		// to die...
		if !sess.isLingering() {
			s.log.Infof("Session %v has become active again.", sess.id)
			return
		}
		s.log.Infof("Session %v will be garbage collected.", sess.id)

		// no more people left? Need to end the session!
		s.removeSession(sess)

		start, end := sess.startTime, time.Now().UTC()

		// Emit a session.end event for this (interactive) session.
		sessionEndEvent := &apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type:        events.SessionEndEvent,
				Code:        events.SessionEndCode,
				ClusterName: party.ctx.ClusterName,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:        party.ctx.srv.HostUUID(),
				ServerLabels:    party.ctx.srv.GetInfo().GetAllLabels(),
				ServerNamespace: s.srv.GetNamespace(),
				ServerHostname:  s.srv.GetInfo().GetHostname(),
				ServerAddr:      party.ctx.ServerConn.LocalAddr().String(),
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: string(sess.id),
			},
			UserMetadata: apievents.UserMetadata{
				User: party.user,
			},
			EnhancedRecording: sess.hasEnhancedRecording,
			Participants:      sess.exportParticipants(),
			Interactive:       true,
			StartTime:         start,
			EndTime:           end,
			SessionRecording:  party.ctx.SessionRecordingConfig.GetMode(),
		}
		if err := sess.recorder.EmitAuditEvent(s.srv.Context(), sessionEndEvent); err != nil {
			s.log.WithError(err).Warn("Failed to emit session end event.")
		}

		// close recorder to free up associated resources and flush data
		if err := sess.recorder.Close(s.srv.Context()); err != nil {
			s.log.WithError(err).Warn("Failed to close recorder.")
		}

		if err := sess.Close(); err != nil {
			s.log.Errorf("Unable to close session %v: %v", sess.id, err)
		}

		// Remove the session from the backend.
		if s.srv.GetSessionServer() != nil {
			err := s.srv.GetSessionServer().DeleteSession(s.srv.GetNamespace(), sess.id)
			if err != nil {
				s.log.Errorf("Failed to remove active session: %v: %v. "+
					"Access to backend may be degraded, check connectivity to backend.",
					sess.id, err)
			}
		}
	}
	go lingerAndDie()
	return nil
}

// NotifyWinChange is called to notify all members in the party that the PTY
// size has changed. The notification is sent as a global SSH request and it
// is the responsibility of the client to update it's window size upon receipt.
func (s *SessionRegistry) NotifyWinChange(params rsession.TerminalParams, ctx *ServerContext) error {
	session := ctx.getSession()
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
			ClusterName: ctx.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.srv.HostUUID(),
			ServerLabels:    ctx.srv.GetInfo().GetAllLabels(),
			ServerNamespace: s.srv.GetNamespace(),
			ServerHostname:  s.srv.GetInfo().GetHostname(),
			ServerAddr:      ctx.ServerConn.LocalAddr().String(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(sid),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		TerminalSize: params.Serialize(),
	}

	// Report the updated window size to the event log (this is so the sessions
	// can be replayed correctly).
	if err := session.recorder.EmitAuditEvent(s.srv.Context(), resizeEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit resize audit event.")
	}

	// Update the size of the server side PTY.
	err := session.term.SetWinSize(params)
	if err != nil {
		return trace.Wrap(err)
	}

	// If sessions are being recorded at the proxy, sessions can not be shared.
	// In that situation, PTY size information does not need to be propagated
	// back to all clients and we can return right away.
	if services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		return nil
	}

	// Notify all members of the party (except originator) that the size of the
	// window has changed so the client can update it's own local PTY. Note that
	// OpenSSH clients will ignore this and not update their own local PTY.
	for _, p := range session.getParties() {
		// Don't send the window change notification back to the originator.
		if p.ctx.ID() == ctx.ID() {
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
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, found := s.findSessionLocked(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	sess.broadcastResult(r)
	return nil
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

	// closeC channel is used to kill all goroutines owned
	// by the session
	closeC chan bool

	// Linger TTL means "how long to keep session in memory after the last client
	// disconnected". It's useful to keep it alive for a bit in case the client
	// temporarily dropped the connection and will reconnect (or a browser-based
	// client hits "page refresh").
	lingerTTL time.Duration

	// startTime is the time when this session was created.
	startTime time.Time

	// login stores the login of the initial session creator
	login string

	closeOnce sync.Once

	recorder events.StreamWriter

	// hasEnhancedRecording returns true if this session has enhanced session
	// recording events associated.
	hasEnhancedRecording bool

	// serverCtx is used to control clean up of internal resources
	serverCtx context.Context

	state types.SessionState

	access auth.SessionAccessEvaluator

	stateUpdate *sync.Cond

	initiator string

	scx *ServerContext

	presenceEnabled bool

	terminated bool

	started bool

	doneCh chan struct{}

	bpfContext *bpf.SessionContext

	cgroupID uint64
}

// newSession creates a new session with a given ID within a given context.
func newSession(id rsession.ID, r *SessionRegistry, ctx *ServerContext) (*session, error) {
	serverSessions.Inc()
	startTime := time.Now().UTC()
	rsess := rsession.Session{
		ID: id,
		TerminalParams: rsession.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:          ctx.Identity.Login,
		Created:        startTime,
		LastActive:     startTime,
		ServerID:       ctx.srv.ID(),
		Namespace:      r.srv.GetNamespace(),
		ServerHostname: ctx.srv.GetInfo().GetHostname(),
		ServerAddr:     ctx.ServerConn.LocalAddr().String(),
		ClusterName:    ctx.ClusterName,
	}

	term := ctx.GetTerm()
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
	sessionServer := r.srv.GetSessionServer()

	err := sessionServer.CreateSession(rsess)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			// if session already exists, make sure they are compatible
			// Login matches existing login
			existing, err := sessionServer.GetSession(r.srv.GetNamespace(), id)
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
	for _, role := range ctx.Identity.RoleSet {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}

	sess := &session{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, r.srv.Component()),
		}),
		id:              id,
		registry:        r,
		parties:         make(map[rsession.ID]*party),
		participants:    make(map[rsession.ID]*party),
		login:           ctx.Identity.Login,
		closeC:          make(chan bool),
		lingerTTL:       defaults.SessionIdlePeriod,
		startTime:       startTime,
		serverCtx:       ctx.srv.Context(),
		state:           types.SessionState_SessionStatePending,
		access:          auth.NewSessionAccessEvaluator(policySets, types.SSHSessionKind),
		scx:             ctx,
		presenceEnabled: ctx.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified] != "",
		io:              NewTermManager(),
		stateUpdate:     sync.NewCond(&sync.Mutex{}),
		doneCh:          make(chan struct{}, 2),
		initiator:       ctx.Identity.TeleportUser,
	}

	err = sess.trackerCreate(ctx.Identity.TeleportUser, policySets)
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

// Close ends the active session forcing all clients to disconnect and freeing all resources
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		serverSessions.Dec()
		// closing needs to happen asynchronously because the last client
		// (session writer) will try to close this session, causing a deadlock
		// because of closeOnce
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()

			close(s.closeC)
			s.io.BroadcastMessage("Closing session...")
			s.log.Infof("Closing session %v.", s.id)
			if s.term != nil {
				s.term.Close()
			}

			for _, p := range s.parties {
				err := p.Close()
				if err != nil {
					s.log.WithError(err).Errorf("Failed to close party.")
				}
			}

			if s.recorder != nil {
				s.recorder.Close(s.serverCtx)
			}
			s.state = types.SessionState_SessionStateTerminated
			s.stateUpdate.Broadcast()
			err := s.trackerUpdateState(types.SessionState_SessionStateRunning)
			if err != nil {
				s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStateRunning)
			}
		}()
	})
	return nil
}

// isLingering returns true if every party has left this session. Occurs
// under a lock.
func (s *session) isLingering() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.parties) == 0 && !s.terminated
}

func (s *session) waitOnAccess() error {
	s.io.Off()
	err := s.io.BroadcastMessage("Session paused, Waiting for required participants...")
	if err != nil {
		log.WithError(err).Errorf("Failed to broadcast message.")
	}

	s.stateUpdate.L.Lock()
	defer s.stateUpdate.L.Unlock()
outer:
	for {
		switch s.state {
		case types.SessionState_SessionStatePending:
			continue
		case types.SessionState_SessionStateTerminated:
			return nil
		case types.SessionState_SessionStateRunning:
			break outer
		}

		s.stateUpdate.Wait()
	}

	s.io.BroadcastMessage("Resuming session...")
	s.io.On()
	return nil
}

func (s *session) launch(ctx *ServerContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateUpdate.L.Lock()
	defer s.stateUpdate.L.Unlock()

	s.log.Debugf("Launching session %v.", s.id)
	s.io.BroadcastMessage("Launching session...")
	s.state = types.SessionState_SessionStateRunning

	err := s.io.On()
	if err != nil {
		s.log.Warnf("Failed to turn enable IO: %v.", err)
	}

	s.stateUpdate.Broadcast()
	err = s.trackerUpdateState(types.SessionState_SessionStateRunning)
	if err != nil {
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
						s.log.WithError(err).Error("Failed to check presence, closing session as a security measure")
						err := s.Close()
						if err != nil {
							s.log.WithError(err).Error("Failed to close session")
						}
					}
				case <-s.closeC:
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

		_, err := io.Copy(s.io, s.term.PTY())
		s.log.Debugf("Copying from PTY to writer completed with error %v.", err)

		// once everything has been copied, notify the goroutine below. if this code
		// is running in a teleport node, when the exec.Cmd is done it will close
		// the PTY, allowing io.Copy to return. if this is a teleport forwarding
		// node, when the remote side closes the channel (which is what s.term.PTY()
		// returns) io.Copy will return.
		s.doneCh <- struct{}{}
	}()

	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)
		_, err := io.Copy(s.term.PTY(), s.io)
		s.log.Debugf("Copying from reader to PTY completed with error %v.", err)
	}()

	// wait for exec.Cmd (or receipt of "exit-status" for a forwarding node),
	// once it is received wait for the io.Copy above to finish, then broadcast
	// the "exit-status" to the client.
	go func() {
		result, err := s.term.Wait()
		if err != nil {
			ctx.Errorf("Received error waiting for the interactive session %v to finish: %v.", s.id, err)
		}

		// wait for copying from the pty to be complete or a timeout before
		// broadcasting the result (which will close the pty) if it has not been
		// closed already.
		select {
		case <-time.After(defaults.WaitCopyTimeout):
			s.log.Errorf("Timed out waiting for PTY copy to finish, session data for %v may be missing.", s.id)
		case <-s.doneCh:
		}

		ctx.srv.GetRestrictedSessionManager().CloseSession(s.bpfContext, s.cgroupID)

		// Close the BPF recording session. If BPF was not configured, not available,
		// or running in a recording proxy, this is simply a NOP.
		err = ctx.srv.GetBPF().CloseSession(s.bpfContext)
		if err != nil {
			ctx.Errorf("Failed to close enhanced recording (interactive) session: %v: %v.", s.id, err)
		}

		if ctx.ExecRequest.GetCommand() != "" {
			emitExecAuditEvent(ctx, ctx.ExecRequest.GetCommand(), err)
		}

		if result != nil {
			if err := s.registry.broadcastResult(s.id, *result); err != nil {
				s.log.Warningf("Failed to broadcast session result: %v", err)
			}
		}
		if err != nil {
			s.log.Infof("Shell exited with error: %v", err)
		} else {
			// no error? this means the command exited cleanly: no need
			// for this session to "linger" after this.
			s.SetLingerTTL(time.Duration(0))
		}
	}()

	// wait for the session to end before the shell, kill the shell
	go func() {
		<-s.closeC
		if err := s.term.Kill(); err != nil {
			s.log.Debugf("Failed killing the shell: %v", err)
		}
	}()

	return nil
}

// startInteractive starts a new interactive process (or a shell) in the
// current session.
func (s *session) startInteractive(ch ssh.Channel, ctx *ServerContext) error {
	// create a new "party" (connected client)
	p := newParty(s, types.SessionPeerMode, ch, ctx)

	// Nodes discard events in cases when proxies are already recording them.
	if s.registry.srv.Component() == teleport.ComponentNode &&
		services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		s.recorder = &events.DiscardStream{}
	} else {
		streamer, err := s.newStreamer(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		s.recorder, err = events.NewAuditWriter(events.AuditWriterConfig{
			// Audit stream is using server context, not session context,
			// to make sure that session is uploaded even after it is closed
			Context:      ctx.srv.Context(),
			Streamer:     streamer,
			Clock:        ctx.srv.GetClock(),
			SessionID:    s.id,
			Namespace:    ctx.srv.GetNamespace(),
			ServerID:     ctx.srv.HostUUID(),
			RecordOutput: ctx.SessionRecordingConfig.GetMode() != types.RecordOff,
			Component:    teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
			ClusterName:  ctx.ClusterName,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var err error

	// allocate a terminal or take the one previously allocated via a
	// seaprate "allocate TTY" SSH request
	if ctx.GetTerm() != nil {
		s.term = ctx.GetTerm()
		ctx.SetTerm(nil)
	} else {
		if s.term, err = NewTerminal(ctx); err != nil {
			ctx.Infof("Unable to allocate new terminal: %v", err)
			return trace.Wrap(err)
		}
	}

	params := s.term.GetTerminalParams()
	// Emit "new session created" event for the interactive session.
	sessionStartEvent := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: ctx.ClusterName,
			ID:          uuid.New().String(),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.srv.HostUUID(),
			ServerLabels:    ctx.srv.GetInfo().GetAllLabels(),
			ServerHostname:  ctx.srv.GetInfo().GetHostname(),
			ServerAddr:      ctx.ServerConn.LocalAddr().String(),
			ServerNamespace: ctx.srv.GetNamespace(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(s.id),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		},
		TerminalSize:     params.Serialize(),
		SessionRecording: ctx.SessionRecordingConfig.GetMode(),
	}

	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionStartEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}
	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionStartEvent); err != nil {
		s.log.WithError(err).Warn("Failed to emit session start event.")
	}

	inReader, inWriter := io.Pipe()
	s.inWriter = inWriter
	s.io.AddReader("reader", inReader)
	s.io.AddWriter("session-recorder", utils.WriteCloserWithContext(ctx.srv.Context(), s.recorder))
	err = s.io.BroadcastMessage(fmt.Sprintf("Creating session with ID: %v...", s.id))
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.term.Run(); err != nil {
		ctx.Errorf("Unable to run shell command: %v.", err)
		return trace.ConvertSystemError(err)
	}

	if err := s.addParty(p, types.SessionPeerMode); err != nil {
		return trace.Wrap(err)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	s.bpfContext = &bpf.SessionContext{
		Context:   ctx.srv.Context(),
		PID:       s.term.PID(),
		Emitter:   s.recorder,
		Namespace: ctx.srv.GetNamespace(),
		SessionID: s.id.String(),
		ServerID:  ctx.srv.HostUUID(),
		Login:     ctx.Identity.Login,
		User:      ctx.Identity.TeleportUser,
		Events:    ctx.Identity.RoleSet.EnhancedRecordingSet(),
	}
	s.cgroupID, err = ctx.srv.GetBPF().OpenSession(s.bpfContext)
	if err != nil {
		ctx.Errorf("Failed to open enhanced recording (interactive) session: %v: %v.", s.id, err)
		return trace.Wrap(err)
	}

	// If a cgroup ID was assigned then enhanced session recording was enabled.
	if s.cgroupID > 0 {
		s.hasEnhancedRecording = true
		ctx.srv.GetRestrictedSessionManager().OpenSession(s.bpfContext, s.cgroupID)
	}

	// Process has been placed in a cgroup, continue execution.
	s.term.Continue()

	// Start a heartbeat that marks this session as active with current members
	// of party in the backend.
	go s.heartbeat(ctx)
	return nil
}

func (s *session) startExec(channel ssh.Channel, ctx *ServerContext) error {
	var err error

	// Nodes discard events in cases when proxies are already recording them.
	if s.registry.srv.Component() == teleport.ComponentNode &&
		services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		s.recorder = &events.DiscardStream{}
	} else {
		streamer, err := s.newStreamer(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		rec, err := events.NewAuditWriter(events.AuditWriterConfig{
			// Audit stream is using server context, not session context,
			// to make sure that session is uploaded even after it is closed
			Context:      ctx.srv.Context(),
			Streamer:     streamer,
			SessionID:    s.id,
			Clock:        ctx.srv.GetClock(),
			Namespace:    ctx.srv.GetNamespace(),
			ServerID:     ctx.srv.HostUUID(),
			RecordOutput: ctx.SessionRecordingConfig.GetMode() != types.RecordOff,
			Component:    teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
			ClusterName:  ctx.ClusterName,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		s.recorder = rec
	}

	// Emit a session.start event for the exec session.
	sessionStartEvent := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: ctx.ClusterName,
			ID:          uuid.New().String(),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        ctx.srv.HostUUID(),
			ServerLabels:    ctx.srv.GetInfo().GetAllLabels(),
			ServerHostname:  ctx.srv.GetInfo().GetHostname(),
			ServerAddr:      ctx.ServerConn.LocalAddr().String(),
			ServerNamespace: ctx.srv.GetNamespace(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(s.id),
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		},
		SessionRecording: ctx.SessionRecordingConfig.GetMode(),
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionStartEvent.ConnectionMetadata.LocalAddr = ctx.ServerConn.LocalAddr().String()
	}
	if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionStartEvent); err != nil {
		ctx.WithError(err).Warn("Failed to emit session start event.")
	}

	// Start execution. If the program failed to start, send that result back.
	// Note this is a partial start. Teleport will have re-exec'ed itself and
	// wait until it's been placed in a cgroup and told to continue.
	result, err := ctx.ExecRequest.Start(channel)
	if err != nil {
		return trace.Wrap(err)
	}
	if result != nil {
		ctx.Debugf("Exec request (%v) result: %v.", ctx.ExecRequest, result)
		ctx.SendExecResult(*result)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		Context:   ctx.srv.Context(),
		PID:       ctx.ExecRequest.PID(),
		Emitter:   s.recorder,
		Namespace: ctx.srv.GetNamespace(),
		SessionID: string(s.id),
		ServerID:  ctx.srv.HostUUID(),
		Login:     ctx.Identity.Login,
		User:      ctx.Identity.TeleportUser,
		Events:    ctx.Identity.RoleSet.EnhancedRecordingSet(),
	}
	cgroupID, err := ctx.srv.GetBPF().OpenSession(sessionContext)
	if err != nil {
		ctx.Errorf("Failed to open enhanced recording (exec) session: %v: %v.", ctx.ExecRequest.GetCommand(), err)
		return trace.Wrap(err)
	}

	// If a cgroup ID was assigned then enhanced session recording was enabled.
	if cgroupID > 0 {
		s.hasEnhancedRecording = true
		ctx.srv.GetRestrictedSessionManager().OpenSession(sessionContext, cgroupID)
	}

	// Process has been placed in a cgroup, continue execution.
	ctx.ExecRequest.Continue()

	// Process is running, wait for it to stop.
	go func() {
		result = ctx.ExecRequest.Wait()
		if result != nil {
			ctx.SendExecResult(*result)
		}

		// Wait a little bit to let all events filter through before closing the
		// BPF session so everything can be recorded.
		time.Sleep(2 * time.Second)

		ctx.srv.GetRestrictedSessionManager().CloseSession(sessionContext, cgroupID)

		// Close the BPF recording session. If BPF was not configured, not available,
		// or running in a recording proxy, this is simply a NOP.
		err = ctx.srv.GetBPF().CloseSession(sessionContext)
		if err != nil {
			ctx.Errorf("Failed to close enhanced recording (exec) session: %v: %v.", s.id, err)
		}

		// Remove the session from the in-memory map.
		s.registry.removeSession(s)

		start, end := s.startTime, time.Now().UTC()

		// Emit a session.end event for this (exec) session.
		sessionEndEvent := &apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type:        events.SessionEndEvent,
				Code:        events.SessionEndCode,
				ClusterName: ctx.ClusterName,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:        ctx.srv.HostUUID(),
				ServerLabels:    ctx.srv.GetInfo().GetAllLabels(),
				ServerNamespace: ctx.srv.GetNamespace(),
				ServerHostname:  ctx.srv.GetInfo().GetHostname(),
				ServerAddr:      ctx.ServerConn.LocalAddr().String(),
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: string(s.id),
			},
			UserMetadata:      ctx.Identity.GetUserMetadata(),
			EnhancedRecording: s.hasEnhancedRecording,
			Interactive:       false,
			Participants: []string{
				ctx.Identity.TeleportUser,
			},
			StartTime:        start,
			EndTime:          end,
			SessionRecording: ctx.SessionRecordingConfig.GetMode(),
		}
		if err := s.recorder.EmitAuditEvent(ctx.srv.Context(), sessionEndEvent); err != nil {
			ctx.WithError(err).Warn("Failed to emit session end event.")
		}

		// Close recorder to free up associated resources and flush data.
		if err := s.recorder.Close(ctx.srv.Context()); err != nil {
			ctx.WithError(err).Warn("Failed to close recorder.")
		}

		// Close the session.
		err = s.Close()
		if err != nil {
			ctx.Errorf("Failed to close session %v: %v.", s.id, err)
		}

		// Remove the session from the backend.
		if ctx.srv.GetSessionServer() != nil {
			err := ctx.srv.GetSessionServer().DeleteSession(ctx.srv.GetNamespace(), s.id)
			if err != nil {
				ctx.Errorf("Failed to remove active session: %v: %v. "+
					"Access to backend may be degraded, check connectivity to backend.",
					s.id, err)
			}
		}
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
		s.log.Debugf("Using sync streamer for session %v.", s.id)
		return ctx.srv, nil
	}
	s.log.Debugf("Using async streamer for session %v.", s.id)
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
		events.StreamingLogsDir, ctx.srv.GetNamespace(),
	)
}

func (s *session) broadcastResult(r ExecResult) {
	for _, p := range s.parties {
		p.ctx.SendExecResult(r)
	}
}

func (s *session) String() string {
	return fmt.Sprintf("session(id=%v, parties=%v)", s.id, len(s.parties))
}

// removePartyMember removes participant from in-memory representation of
// party members. Occurs under a lock.
func (s *session) removePartyMember(party *party) {
	delete(s.parties, party.id)
}

// removeParty removes the party from the in-memory map that holds all party
// members.
func (s *session) removeParty(p *party) error {
	p.ctx.Infof("Removing party %v from session %v", p, s.id)

	// Removes participant from in-memory map of party members.
	s.removePartyMember(p)

	canRun, policyOptions, err := s.checkIfStart()
	if err != nil {
		return trace.Wrap(err)
	}

	s.stateUpdate.L.Lock()
	defer s.stateUpdate.L.Unlock()

	if !canRun && s.state == types.SessionState_SessionStateRunning {
		if policyOptions.TerminateOnLeave {
			s.registry.ForceTerminate(s.scx)
			return nil
		}

		s.state = types.SessionState_SessionStatePending
		s.stateUpdate.Broadcast()
		err := s.trackerUpdateState(types.SessionState_SessionStatePending)
		if err != nil {
			s.log.Warnf("Failed to set tracker state to %v", types.SessionState_SessionStatePending)
		}

		go s.waitOnAccess()
	}

	s.io.DeleteWriter(string(p.id))
	s.io.BroadcastMessage(fmt.Sprintf("User %v left the session.", p.user))
	return nil
}

func (s *session) GetLingerTTL() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lingerTTL
}

func (s *session) SetLingerTTL(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lingerTTL = ttl
}

func (s *session) getNamespace() string {
	return s.registry.srv.GetNamespace()
}

// exportPartyMembers exports participants in the in-memory map of party
// members. Occurs under a lock.
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

// exportParticipants returns a list of all members that joined the party.
func (s *session) exportParticipants() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var participants []string
	for _, p := range s.participants {
		participants = append(participants, p.user)
	}

	return participants
}

// heartbeat will loop as long as the session is not closed and mark it as
// active and update the list of party members. If the session are recorded at
// the proxy, then this function does nothing as it's counterpart
// in the proxy will do this work.
func (s *session) heartbeat(ctx *ServerContext) {
	// If sessions are being recorded at the proxy, an identical version of this
	// goroutine is running in the proxy, which means it does not need to run here.
	if services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) &&
		s.registry.srv.Component() == teleport.ComponentNode {
		return
	}

	// If no session server (endpoint interface for active sessions) is passed in
	// (for example Teleconsole does this) then nothing to sync.
	sessionServer := s.registry.srv.GetSessionServer()
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

			err := sessionServer.UpdateSession(rsession.UpdateRequest{
				Namespace: s.getNamespace(),
				ID:        s.id,
				Parties:   &partyList,
			})
			if err != nil {
				s.log.Warnf("Unable to update session %v as active: %v", s.id, err)
			}
		case <-s.closeC:
			return
		}
	}
}

func (s *session) checkPresence() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, err := s.trackerGet()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, participant := range sess.GetParticipants() {
		if participant.User == s.initiator {
			continue
		}

		if participant.Mode == string(types.SessionModeratorMode) && time.Now().UTC().After(participant.LastActive.Add(PresenceMaxDifference)) {
			s.log.Warnf("Participant %v is not active, kicking.", participant.ID)
			party := s.parties[rsession.ID(participant.ID)]
			if party != nil {
				err := party.Close()
				if err != nil {
					s.log.WithError(err).Warnf("Failed to kick participant %v for inactivity.", participant.ID)
				}
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

	// Adds participant to in-memory map of party members.
	s.parties[p.id] = p
	s.participants[p.id] = p

	err := s.trackerAddParticipant(p)
	if err != nil {
		return trace.Wrap(err)
	}

	// Write last chunk (so the newly joined parties won't stare at a blank
	// screen).
	if _, err := p.Write(s.io.GetRecentHistory()); err != nil {
		return trace.Wrap(err)
	}

	// Register this party as one of the session writers (output will go to it).
	s.io.AddWriter(string(p.id), p)
	p.ctx.AddCloser(p)
	s.term.AddParty(1)

	s.io.BroadcastMessage(fmt.Sprintf("User %v joined the session.", p.user))
	s.log.Infof("New party %v joined session: %v", p.String(), s.id)

	if mode == types.SessionPeerMode {
		// This goroutine keeps pumping party's input into the session.
		go func() {
			defer s.term.AddParty(-1)
			_, err := io.Copy(s.inWriter, p)
			if err != nil {
				s.log.Errorf("Party member %v left session %v due an error: %v", p.id, s.id, err)
			}
			s.log.Infof("Party member %v left session %v.", p.id, s.id)

			err = s.trackerRemoveParticipant(p.user)
			if err != nil {
				s.log.Errorf("Failed to remove participant %v from session tracker %v: %v", p.id, s.id, err)
			}
		}()
	}

	s.stateUpdate.L.Lock()
	defer s.stateUpdate.L.Unlock()
	if s.state == types.SessionState_SessionStatePending {
		canStart, _, err := s.checkIfStart()
		if err != nil {
			return trace.Wrap(err)
		}

		if canStart {
			if !s.started {
				s.started = true

				go func() {
					err := s.launch(s.scx)
					if err != nil {
						s.log.Errorf("Failed to launch session %v: %v", s.id, err)
					}
				}()
			} else {
				s.state = types.SessionState_SessionStateRunning
				s.stateUpdate.Broadcast()
			}
		}
	}

	return nil
}

func (s *session) join(ch ssh.Channel, req *ssh.Request, ctx *ServerContext, mode types.SessionParticipantMode) (*party, error) {
	if ctx.Identity.TeleportUser != s.initiator {
		roles := []types.Role(ctx.Identity.RoleSet)
		accessContext := auth.SessionAccessContext{
			Roles: roles,
		}

		modes, err := s.access.CanJoin(accessContext)
		if err != nil {
			return nil, trace.Wrap(err)
		}

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
	closeC     chan bool
	termSizeC  chan []byte
	lastActive time.Time
	closeOnce  sync.Once
	mode       types.SessionParticipantMode
}

func newParty(s *session, mode types.SessionParticipantMode, ch ssh.Channel, ctx *ServerContext) *party {
	return &party{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		}),
		user:      ctx.Identity.TeleportUser,
		login:     ctx.Identity.Login,
		serverID:  s.registry.srv.ID(),
		site:      ctx.ServerConn.RemoteAddr().String(),
		id:        rsession.NewID(),
		ch:        ch,
		ctx:       ctx,
		s:         s,
		sconn:     ctx.ServerConn,
		termSizeC: make(chan []byte, 5),
		closeC:    make(chan bool),
		mode:      mode,
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

func (p *party) Close() (err error) {
	p.closeOnce.Do(func() {
		p.log.Infof("Closing party %v", p.id)
		if err = p.s.registry.leaveSession(p); err != nil {
			p.ctx.Error(err)
		}
		close(p.closeC)
		close(p.termSizeC)
		err = p.ch.Close()
	})
	return err
}

func (s *session) trackerGet() (types.SessionTracker, error) {
	if s.registry.auth == nil {
		return nil, trace.BadParameter("cannot fetch session without auth service")
	}

	// get the session from the registry
	sess, err := s.registry.auth.GetSessionTracker(s.serverCtx, s.id.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (s *session) trackerCreate(teleportUser string, policySet []*types.SessionTrackerPolicySet) error {
	if s.registry.auth == nil {
		return nil
	}

	s.log.Debug("Creating tracker")
	initator := &types.Participant{
		ID:         teleportUser,
		User:       teleportUser,
		LastActive: time.Now().UTC(),
	}

	reason := s.scx.env[teleport.EnvSSHSessionReason]
	var invited []string
	err := json.Unmarshal([]byte(s.scx.env[teleport.EnvSSHSessionInvited]), &invited)
	if err != nil {
		return trace.Wrap(err)
	}

	req := &proto.CreateSessionTrackerRequest{
		ID:           s.id.String(),
		Namespace:    apidefaults.Namespace,
		Type:         string(types.KubernetesSessionKind),
		Hostname:     s.registry.srv.GetInfo().GetHostname(),
		Address:      s.scx.ServerConn.LocalAddr().String(),
		ClusterName:  s.scx.ClusterName,
		Login:        "root",
		Initiator:    initator,
		Expires:      time.Now().UTC().Add(time.Hour * 24),
		HostUser:     initator.User,
		Reason:       reason,
		Invited:      invited,
		HostPolicies: policySet,
	}

	_, err = s.registry.auth.CreateSessionTracker(s.serverCtx, req)
	return trace.Wrap(err)
}

func (s *session) trackerAddParticipant(participant *party) error {
	if s.registry.auth == nil {
		return nil
	}

	s.log.Debugf("Tracking participant: %v", participant.user)
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: &types.Participant{
					ID:         participant.user,
					User:       participant.user,
					Mode:       string(participant.mode),
					LastActive: time.Now().UTC(),
				},
			},
		},
	}

	err := s.registry.auth.UpdateSessionTracker(s.serverCtx, req)
	return trace.Wrap(err)
}

func (s *session) trackerRemoveParticipant(participantID string) error {
	if s.registry.auth == nil {
		return nil
	}

	s.log.Debugf("Not tracking participant: %v", participantID)
	req := &proto.UpdateSessionTrackerRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: participantID,
			},
		},
	}

	err := s.registry.auth.UpdateSessionTracker(s.serverCtx, req)
	return trace.Wrap(err)
}

func (s *session) trackerUpdateState(state types.SessionState) error {
	if s.registry.auth == nil {
		return nil
	}

	req := &proto.UpdateSessionTrackerRequest{
		SessionID: s.id.String(),
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{
			UpdateState: &proto.SessionTrackerUpdateState{
				State: state,
			},
		},
	}

	err := s.registry.auth.UpdateSessionTracker(s.serverCtx, req)
	return trace.Wrap(err)
}

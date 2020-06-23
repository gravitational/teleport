/*
Copyright 2015 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	// number of the most recent session writes (what's been written
	// in a terminal) to be instanly replayed to the newly joining
	// parties
	instantReplayLen = 20
)

var (
	serverSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "server_interactive_sessions_total",
			Help: "Number of active sessions",
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(serverSessions)
}

// SessionRegistry holds a map of all active sessions on a given
// SSH server
type SessionRegistry struct {
	sync.Mutex

	// log holds the structured logger
	log *logrus.Entry

	// sessions holds a map between session ID and the session object. Used to
	// find active sessions as well as close all sessions when the registry
	// is closing.
	sessions map[rsession.ID]*session

	// srv refers to the upon which this session registry is created.
	srv Server
}

func NewSessionRegistry(srv Server) (*SessionRegistry, error) {
	if srv.GetSessionServer() == nil {
		return nil, trace.BadParameter("session server is required")
	}

	return &SessionRegistry{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, srv.Component()),
		}),
		srv:      srv,
		sessions: make(map[rsession.ID]*session),
	}, nil
}

func (s *SessionRegistry) addSession(sess *session) {
	s.Lock()
	defer s.Unlock()
	s.sessions[sess.id] = sess
}

func (s *SessionRegistry) removeSession(sess *session) {
	s.Lock()
	defer s.Unlock()
	delete(s.sessions, sess.id)
}

func (s *SessionRegistry) findSession(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func (s *SessionRegistry) Close() {
	s.Lock()
	defer s.Unlock()

	for _, se := range s.sessions {
		se.Close()
	}

	s.log.Debugf("Closing Session Registry.")
}

// emitSessionJoinEvent emits a session join event to both the Audit Log as
// well as sending a "x-teleport-event" global request on the SSH connection.
func (s *SessionRegistry) emitSessionJoinEvent(ctx *ServerContext) {
	sessionJoinEvent := events.EventFields{
		events.EventType:       events.SessionJoinEvent,
		events.SessionEventID:  string(ctx.session.id),
		events.EventNamespace:  s.srv.GetNamespace(),
		events.EventLogin:      ctx.Identity.Login,
		events.EventUser:       ctx.Identity.TeleportUser,
		events.RemoteAddr:      ctx.ServerConn.RemoteAddr().String(),
		events.SessionServerID: ctx.srv.HostUUID(),
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		sessionJoinEvent[events.LocalAddr] = ctx.ServerConn.LocalAddr().String()
	}

	// Emit session join event to Audit Log.
	ctx.session.emitAuditEvent(events.SessionJoin, sessionJoinEvent)

	// Notify all members of the party that a new member has joined over the
	// "x-teleport-event" channel.
	for _, p := range s.getParties(ctx.session) {
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
	if ctx.session != nil {
		ctx.Infof("Joining existing session %v.", ctx.session.id)

		// Update the in-memory data structure that a party member has joined.
		_, err := ctx.session.join(ch, req, ctx)
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
	ctx.session = sess
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

	// Start a non-interactive session (TTY attached). Close the session if an error
	// occurs, otherwise it will be closed by the callee.
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
	sessionLeaveEvent := events.EventFields{
		events.EventType:       events.SessionLeaveEvent,
		events.SessionEventID:  party.id.String(),
		events.EventUser:       party.user,
		events.SessionServerID: party.ctx.srv.HostUUID(),
		events.EventNamespace:  s.srv.GetNamespace(),
	}

	// Emit session leave event to Audit Log.
	party.s.emitAuditEvent(events.SessionLeave, sessionLeaveEvent)

	// Notify all members of the party that a new member has left over the
	// "x-teleport-event" channel.
	for _, p := range s.getParties(party.s) {
		eventPayload, err := json.Marshal(sessionLeaveEvent)
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

// leaveSession removes the given party from this session.
func (s *SessionRegistry) leaveSession(party *party) error {
	sess := party.s
	s.Lock()
	defer s.Unlock()

	// Emit session leave event to both the Audit Log as well as over the
	// "x-teleport-event" channel in the SSH connection.
	s.emitSessionLeaveEvent(party)

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
		eventFields := events.EventFields{
			events.SessionEventID:           string(sess.id),
			events.SessionServerID:          party.ctx.srv.HostUUID(),
			events.EventUser:                party.user,
			events.EventNamespace:           s.srv.GetNamespace(),
			events.SessionInteractive:       true,
			events.SessionEnhancedRecording: sess.hasEnhancedRecording,
			events.SessionParticipants:      sess.exportParticipants(),
			events.SessionServerHostname:    s.srv.GetInfo().GetHostname(),
			events.SessionServerAddr:        s.srv.GetInfo().GetAddr(),
			events.SessionStartTime:         start,
			events.SessionEndTime:           end,
		}
		sess.emitAuditEvent(events.SessionEnd, eventFields)

		// close recorder to free up associated resources
		// and flush data
		sess.recorder.Close()

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

// getParties allows to safely return a list of parties connected to this
// session (as determined by ctx)
func (s *SessionRegistry) getParties(sess *session) []*party {
	var parties []*party

	if sess == nil {
		return parties
	}

	sess.Lock()
	defer sess.Unlock()

	for _, p := range sess.parties {
		parties = append(parties, p)
	}

	return parties
}

// NotifyWinChange is called to notify all members in the party that the PTY
// size has changed. The notification is sent as a global SSH request and it
// is the responsibility of the client to update it's window size upon receipt.
func (s *SessionRegistry) NotifyWinChange(params rsession.TerminalParams, ctx *ServerContext) error {
	if ctx.session == nil {
		s.log.Debugf("Unable to update window size, no session found in context.")
		return nil
	}
	sid := ctx.session.id

	// Build the resize event.
	resizeEvent := events.EventFields{
		events.EventType:       events.ResizeEvent,
		events.EventNamespace:  s.srv.GetNamespace(),
		events.SessionEventID:  sid,
		events.SessionServerID: ctx.srv.HostUUID(),
		events.EventLogin:      ctx.Identity.Login,
		events.EventUser:       ctx.Identity.TeleportUser,
		events.TerminalSize:    params.Serialize(),
	}

	// Report the updated window size to the event log (this is so the sessions
	// can be replayed correctly).
	ctx.session.emitAuditEvent(events.TerminalResize, resizeEvent)

	// Update the size of the server side PTY.
	err := ctx.session.term.SetWinSize(params)
	if err != nil {
		return trace.Wrap(err)
	}

	// If sessions are being recorded at the proxy, sessions can not be shared.
	// In that situation, PTY size information does not need to be propagated
	// back to all clients and we can return right away.
	if ctx.ClusterConfig.GetSessionRecording() == services.RecordAtProxy {
		return nil
	}

	// Notify all members of the party (except originator) that the size of the
	// window has changed so the client can update it's own local PTY. Note that
	// OpenSSH clients will ignore this and not update their own local PTY.
	for _, p := range s.getParties(ctx.session) {
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
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	sess.broadcastResult(r)
	return nil
}

// session struct describes an active (in progress) SSH session. These sessions
// are managed by 'SessionRegistry' containers which are attached to SSH servers.
type session struct {
	sync.Mutex

	// log holds the structured logger
	log *logrus.Entry

	// session ID. unique GUID, this is what people use to "join" sessions
	id rsession.ID

	// parent session container
	registry *SessionRegistry

	// this writer is used to broadcast terminal I/O to different clients
	writer *multiWriter

	// parties is the set of current connected clients/users. This map may grow
	// and shrink as members join and leave the session.
	parties map[rsession.ID]*party

	// participants is the set of users that have joined this session. Users are
	// never removed from this map as it's used to report the full list of
	// participants at the end of a session.
	participants map[rsession.ID]*party

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

	recorder events.SessionRecorder

	// hasEnhancedRecording returns true if this session has enhanced session
	// recording events associated.
	hasEnhancedRecording bool
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

	sess := &session{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, r.srv.Component()),
		}),
		id:           id,
		registry:     r,
		parties:      make(map[rsession.ID]*party),
		participants: make(map[rsession.ID]*party),
		writer:       newMultiWriter(),
		login:        ctx.Identity.Login,
		closeC:       make(chan bool),
		lingerTTL:    defaults.SessionIdlePeriod,
		startTime:    startTime,
	}
	return sess, nil
}

// ID returns a string representation of the session ID.
func (s *session) ID() string {
	return s.id.String()
}

// PID returns the PID of the Teleport process under which the shell is running.
func (s *session) PID() int {
	return s.term.PID()
}

// Close ends the active session forcing all clients to disconnect and freeing all resources
func (s *session) Close() error {
	serverSessions.Dec()
	s.closeOnce.Do(func() {
		// closing needs to happen asynchronously because the last client
		// (session writer) will try to close this session, causing a deadlock
		// because of closeOnce
		go func() {
			s.log.Infof("Closing session %v", s.id)
			if s.term != nil {
				s.term.Close()
			}
			close(s.closeC)

			// close all writers in our multi-writer
			s.writer.Lock()
			defer s.writer.Unlock()
			for writerName, writer := range s.writer.writers {
				s.log.Infof("Closing session writer: %v", writerName)
				closer, ok := io.Writer(writer).(io.WriteCloser)
				if ok {
					closer.Close()
				}
			}
		}()
	})
	return nil
}

// isLingering returns true if every party has left this session. Occurs
// under a lock.
func (s *session) isLingering() bool {
	s.Lock()
	defer s.Unlock()

	return len(s.parties) == 0
}

// startInteractive starts a new interactive process (or a shell) in the
// current session.
func (s *session) startInteractive(ch ssh.Channel, ctx *ServerContext) error {
	var err error

	// create a new "party" (connected client)
	p := newParty(s, ch, ctx)

	// Get the audit log from the server and create a session recorder. this will
	// be a discard audit log if the proxy is in recording mode and a teleport
	// node so we don't create double recordings.
	auditLog := s.registry.srv.GetAuditLog()
	if auditLog == nil || isDiscardAuditLog(auditLog) {
		s.recorder = &events.DiscardRecorder{}
	} else {
		s.recorder, err = events.NewForwardRecorder(events.ForwardRecorderConfig{
			DataDir:        filepath.Join(ctx.srv.GetDataDir(), teleport.LogsDir),
			SessionID:      s.id,
			Namespace:      ctx.srv.GetNamespace(),
			RecordSessions: ctx.ClusterConfig.GetSessionRecording() != services.RecordOff,
			Component:      teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
			ForwardTo:      auditLog,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	s.writer.addWriter("session-recorder", s.recorder, true)

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

	if err := s.term.Run(); err != nil {
		ctx.Errorf("Unable to run shell command: %v.", err)
		return trace.ConvertSystemError(err)
	}
	if err := s.addParty(p); err != nil {
		return trace.Wrap(err)
	}

	// Open a BPF recording session. If BPF was not configured, not available,
	// or running in a recording proxy, OpenSession is a NOP.
	sessionContext := &bpf.SessionContext{
		PID:       s.term.PID(),
		AuditLog:  s.recorder.GetAuditLog(),
		Namespace: ctx.srv.GetNamespace(),
		SessionID: s.id.String(),
		ServerID:  ctx.srv.HostUUID(),
		Login:     ctx.Identity.Login,
		User:      ctx.Identity.TeleportUser,
		Events:    ctx.Identity.RoleSet.EnhancedRecordingSet(),
	}
	cgroupID, err := ctx.srv.GetBPF().OpenSession(sessionContext)
	if err != nil {
		ctx.Errorf("Failed to open enhanced recording (interactive) session: %v: %v.", s.id, err)
		return trace.Wrap(err)
	}

	// If a cgroup ID was assigned then enhanced session recording was enabled.
	if cgroupID > 0 {
		s.hasEnhancedRecording = true
	}

	// Process has been placed in a cgroup, continue execution.
	s.term.Continue()

	params := s.term.GetTerminalParams()

	// Emit "new session created" event for the interactive session.
	eventFields := events.EventFields{
		events.EventNamespace:        ctx.srv.GetNamespace(),
		events.SessionEventID:        string(s.id),
		events.SessionServerID:       ctx.srv.HostUUID(),
		events.EventLogin:            ctx.Identity.Login,
		events.EventUser:             ctx.Identity.TeleportUser,
		events.RemoteAddr:            ctx.ServerConn.RemoteAddr().String(),
		events.TerminalSize:          params.Serialize(),
		events.SessionServerHostname: ctx.srv.GetInfo().GetHostname(),
		events.SessionServerLabels:   ctx.srv.GetInfo().GetAllLabels(),
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		eventFields[events.LocalAddr] = ctx.ServerConn.LocalAddr().String()
	}
	s.emitAuditEvent(events.SessionStart, eventFields)

	// Start a heartbeat that marks this session as active with current members
	// of party in the backend.
	go s.heartbeat(ctx)

	doneCh := make(chan bool, 1)

	// copy everything from the pty to the writer. this lets us capture all input
	// and output of the session (because input is echoed to stdout in the pty).
	// the writer contains multiple writers: the session logger and a direct
	// connection to members of the "party" (other people in the session).
	s.term.AddParty(1)
	go func() {
		defer s.term.AddParty(-1)

		_, err := io.Copy(s.writer, s.term.PTY())
		s.log.Debugf("Copying from PTY to writer completed with error %v.", err)

		// once everything has been copied, notify the goroutine below. if this code
		// is running in a teleport node, when the exec.Cmd is done it will close
		// the PTY, allowing io.Copy to return. if this is a teleport forwarding
		// node, when the remote side closes the channel (which is what s.term.PTY()
		// returns) io.Copy will return.
		doneCh <- true
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
		case <-doneCh:
		}

		// Close the BPF recording session. If BPF was not configured, not available,
		// or running in a recording proxy, this is simply a NOP.
		err = ctx.srv.GetBPF().CloseSession(sessionContext)
		if err != nil {
			ctx.Errorf("Failed to close enhanced recording (interactive) session: %v: %v.", s.id, err)
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

func (s *session) startExec(channel ssh.Channel, ctx *ServerContext) error {
	var err error

	// Get the audit log from the server and create a session recorder. this will
	// be a discard audit log if the proxy is in recording mode and a teleport
	// node so we don't create double recordings.
	auditLog := s.registry.srv.GetAuditLog()
	if auditLog == nil || isDiscardAuditLog(auditLog) {
		s.recorder = &events.DiscardRecorder{}
	} else {
		s.recorder, err = events.NewForwardRecorder(events.ForwardRecorderConfig{
			DataDir:        filepath.Join(ctx.srv.GetDataDir(), teleport.LogsDir),
			SessionID:      s.id,
			Namespace:      ctx.srv.GetNamespace(),
			RecordSessions: ctx.ClusterConfig.GetSessionRecording() != services.RecordOff,
			Component:      teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
			ForwardTo:      auditLog,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Emit a session.start event for the exec session.
	eventFields := events.EventFields{
		events.EventNamespace:        ctx.srv.GetNamespace(),
		events.SessionEventID:        string(s.id),
		events.SessionServerID:       ctx.srv.HostUUID(),
		events.EventLogin:            ctx.Identity.Login,
		events.EventUser:             ctx.Identity.TeleportUser,
		events.RemoteAddr:            ctx.ServerConn.RemoteAddr().String(),
		events.SessionServerHostname: ctx.srv.GetInfo().GetHostname(),
		events.SessionServerLabels:   ctx.srv.GetInfo().GetAllLabels(),
	}
	// Local address only makes sense for non-tunnel nodes.
	if !ctx.srv.UseTunnel() {
		eventFields[events.LocalAddr] = ctx.ServerConn.LocalAddr().String()
	}
	s.emitAuditEvent(events.SessionStart, eventFields)

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
		PID:       ctx.ExecRequest.PID(),
		AuditLog:  s.recorder.GetAuditLog(),
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
		eventFields := events.EventFields{
			events.SessionEventID:           string(s.id),
			events.SessionServerID:          ctx.srv.HostUUID(),
			events.EventNamespace:           ctx.srv.GetNamespace(),
			events.SessionInteractive:       false,
			events.SessionEnhancedRecording: s.hasEnhancedRecording,
			events.SessionParticipants: []string{
				ctx.Identity.TeleportUser,
			},
			events.SessionServerHostname: ctx.srv.GetInfo().GetHostname(),
			events.SessionServerAddr:     ctx.srv.GetInfo().GetAddr(),
			events.SessionStartTime:      start,
			events.SessionEndTime:        end,
		}
		s.emitAuditEvent(events.SessionEnd, eventFields)

		// Close recorder to free up associated resources and flush data.
		s.recorder.Close()

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
	s.Lock()
	defer s.Unlock()

	delete(s.parties, party.id)
}

// removeParty removes the party from the in-memory map that holds all party
// members.
func (s *session) removeParty(p *party) error {
	p.ctx.Infof("Removing party %v from session %v", p, s.id)

	// Removes participant from in-memory map of party members.
	s.removePartyMember(p)

	s.writer.deleteWriter(string(p.id))

	return nil
}

func (s *session) GetLingerTTL() time.Duration {
	s.Lock()
	defer s.Unlock()
	return s.lingerTTL
}

func (s *session) SetLingerTTL(ttl time.Duration) {
	s.Lock()
	defer s.Unlock()
	s.lingerTTL = ttl
}

func (s *session) getNamespace() string {
	return s.registry.srv.GetNamespace()
}

// exportPartyMembers exports participants in the in-memory map of party
// members. Occurs under a lock.
func (s *session) exportPartyMembers() []rsession.Party {
	s.Lock()
	defer s.Unlock()

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
	s.Lock()
	defer s.Unlock()

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
	if ctx.ClusterConfig.GetSessionRecording() == services.RecordAtProxy &&
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

// addPartyMember adds participant to in-memory map of party members. Occurs
// under a lock.
func (s *session) addPartyMember(p *party) {
	s.Lock()
	defer s.Unlock()

	s.parties[p.id] = p
	s.participants[p.id] = p
}

// addParty is called when a new party joins the session.
func (s *session) addParty(p *party) error {
	if s.login != p.login {
		return trace.AccessDenied(
			"can't switch users from %v to %v for session %v",
			s.login, p.login, s.id)
	}

	// Adds participant to in-memory map of party members.
	s.addPartyMember(p)

	// Write last chunk (so the newly joined parties won't stare at a blank
	// screen).
	getRecentWrite := func() []byte {
		s.writer.Lock()
		defer s.writer.Unlock()
		data := make([]byte, 0, 1024)
		for i := range s.writer.recentWrites {
			data = append(data, s.writer.recentWrites[i]...)
		}
		return data
	}
	if _, err := p.Write(getRecentWrite()); err != nil {
		return trace.Wrap(err)
	}

	// Register this party as one of the session writers (output will go to it).
	s.writer.addWriter(string(p.id), p, true)
	p.ctx.AddCloser(p)
	s.term.AddParty(1)

	s.log.Infof("New party %v joined session: %v", p.String(), s.id)

	// This goroutine keeps pumping party's input into the session.
	go func() {
		defer s.term.AddParty(-1)
		_, err := io.Copy(s.term.PTY(), p)
		if err != nil {
			s.log.Errorf("Party member %v left session %v due an error: %v", p.id, s.id, err)
		}
		s.log.Infof("Party member %v left session %v.", p.id, s.id)
	}()
	return nil
}

func (s *session) join(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) (*party, error) {
	p := newParty(s, ch, ctx)
	if err := s.addParty(p); err != nil {
		return nil, trace.Wrap(err)
	}
	return p, nil
}

func (s *session) emitAuditEvent(e events.Event, f events.EventFields) {
	if err := s.recorder.GetAuditLog().EmitAuditEvent(e, f); err != nil {
		s.log.Warningf("Failed to emit audit event: %v", err)
	}
}

func newMultiWriter() *multiWriter {
	return &multiWriter{writers: make(map[string]writerWrapper)}
}

type multiWriter struct {
	sync.RWMutex
	writers      map[string]writerWrapper
	recentWrites [][]byte
}

type writerWrapper struct {
	io.WriteCloser
	closeOnError bool
}

func (m *multiWriter) addWriter(id string, w io.WriteCloser, closeOnError bool) {
	m.Lock()
	defer m.Unlock()
	m.writers[id] = writerWrapper{WriteCloser: w, closeOnError: closeOnError}
}

func (m *multiWriter) deleteWriter(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.writers, id)
}

func (m *multiWriter) lockedAddRecentWrite(p []byte) {
	// make a copy of it (this slice is based on a shared buffer)
	clone := make([]byte, len(p))
	copy(clone, p)
	// add to the list of recent writes
	m.recentWrites = append(m.recentWrites, clone)
	for len(m.recentWrites) > instantReplayLen {
		m.recentWrites = m.recentWrites[1:]
	}
}

// Write multiplexes the input to multiple sub-writers. The entire point
// of multiWriter is to do this
func (m *multiWriter) Write(p []byte) (n int, err error) {
	// lock and make a local copy of available writers:
	getWriters := func() (writers []writerWrapper) {
		m.RLock()
		defer m.RUnlock()
		writers = make([]writerWrapper, 0, len(m.writers))
		for _, w := range m.writers {
			writers = append(writers, w)
		}

		// add the recent write chunk to the "instant replay" buffer
		// of the session, to be replayed to newly joining parties:
		m.lockedAddRecentWrite(p)
		return writers
	}

	// unlock and multiplex the write to all writers:
	for _, w := range getWriters() {
		n, err = w.Write(p)
		if err != nil {
			if w.closeOnError {
				return
			}
			continue
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

type party struct {
	sync.Mutex

	log        *logrus.Entry
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
}

func newParty(s *session, ch ssh.Channel, ctx *ServerContext) *party {
	return &party{
		log: logrus.WithFields(logrus.Fields{
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
	p.ctx.UpdateClientActivity()
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
	})
	return err
}

func isDiscardAuditLog(alog events.IAuditLog) bool {
	_, ok := alog.(*events.DiscardAuditLog)
	return ok
}

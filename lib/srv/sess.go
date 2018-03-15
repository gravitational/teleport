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
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/state"

	"github.com/gravitational/trace"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	// number of the most recent session writes (what's been written
	// in a terminal) to be instanly replayed to the newly joining
	// parties
	instantReplayLen = 20

	// maxTermSyncErrorCount defines how many subsequent erorrs
	// we should tolerate before giving up trying to sync the
	// term size
	maxTermSyncErrorCount = 5
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

	// sessions holds a map between session ID and the session object.
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

func (s *SessionRegistry) Close() {
	s.Lock()
	defer s.Unlock()

	for _, se := range s.sessions {
		se.Close()
	}

	s.log.Debugf("Closing Session Registry.")
}

// joinShell either joins an existing session or starts a new shell
func (s *SessionRegistry) OpenSession(ch ssh.Channel, req *ssh.Request, ctx *ServerContext) error {
	if ctx.session != nil {
		// emit "joined session" event:
		ctx.session.recorder.alog.EmitAuditEvent(events.SessionJoinEvent, events.EventFields{
			events.SessionEventID:  string(ctx.session.id),
			events.EventNamespace:  s.srv.GetNamespace(),
			events.EventLogin:      ctx.Identity.Login,
			events.EventUser:       ctx.Identity.TeleportUser,
			events.LocalAddr:       ctx.Conn.LocalAddr().String(),
			events.RemoteAddr:      ctx.Conn.RemoteAddr().String(),
			events.SessionServerID: ctx.srv.ID(),
		})
		ctx.Infof("Joining existing session %v.", ctx.session.id)
		_, err := ctx.session.join(ch, req, ctx)
		return trace.Wrap(err)
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
	ctx.Infof("Creating session %v.", sid)

	if err := sess.start(ch, ctx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}
	return nil
}

// leaveSession removes the given party from this session
func (s *SessionRegistry) leaveSession(party *party) error {
	sess := party.s
	s.Lock()
	defer s.Unlock()

	// remove from in-memory representation of the session:
	if err := sess.removeParty(party); err != nil {
		return trace.Wrap(err)
	}

	// emit "session leave" event (party left the session)
	sess.recorder.alog.EmitAuditEvent(events.SessionLeaveEvent, events.EventFields{
		events.SessionEventID:  string(sess.id),
		events.EventUser:       party.user,
		events.SessionServerID: party.serverID,
		events.EventNamespace:  s.srv.GetNamespace(),
	})

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
		s.Lock()
		delete(s.sessions, sess.id)
		s.Unlock()

		// send an event indicating that this session has ended
		sess.recorder.alog.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
			events.SessionEventID: string(sess.id),
			events.EventUser:      party.user,
			events.EventNamespace: s.srv.GetNamespace(),
		})

		// close recorder to free up associated resources
		// and flush data
		sess.recorder.Close()

		if err := sess.Close(); err != nil {
			s.log.Errorf("Unable to close session %v: %v", sess.id, err)
		}

		// mark it as inactive in the DB
		if s.srv.GetSessionServer() != nil {
			False := false
			s.srv.GetSessionServer().UpdateSession(rsession.UpdateRequest{
				ID:        sess.id,
				Active:    &False,
				Namespace: s.srv.GetNamespace(),
			})
		}
	}
	go lingerAndDie()
	return nil
}

// getParties allows to safely return a list of parties connected to this
// session (as determined by ctx)
func (s *SessionRegistry) getParties(ctx *ServerContext) (parties []*party) {
	sess := ctx.session
	if sess != nil {
		sess.Lock()
		defer sess.Unlock()

		parties = make([]*party, 0, len(sess.parties))
		for _, p := range sess.parties {
			parties = append(parties, p)
		}
	}
	return parties
}

// notifyWinChange is called when an SSH server receives a command notifying
// us that the terminal size has changed
func (s *SessionRegistry) NotifyWinChange(params rsession.TerminalParams, ctx *ServerContext) error {
	if ctx.session == nil {
		s.log.Debugf("Unable to update window size, no session found in context.")
		return nil
	}
	sid := ctx.session.id
	// report this to the event/audit log:
	ctx.session.recorder.alog.EmitAuditEvent(events.ResizeEvent, events.EventFields{
		events.EventNamespace: s.srv.GetNamespace(),
		events.SessionEventID: sid,
		events.EventLogin:     ctx.Identity.Login,
		events.EventUser:      ctx.Identity.TeleportUser,
		events.TerminalSize:   params.Serialize(),
	})
	err := ctx.session.term.SetWinSize(params)
	if err != nil {
		return trace.Wrap(err)
	}

	// notify all connected parties about the change in real time
	// (if they're capable)
	for _, p := range s.getParties(ctx) {
		p.onWindowChanged(&params)
	}

	go func() {
		err := s.srv.GetSessionServer().UpdateSession(
			rsession.UpdateRequest{ID: sid, TerminalParams: &params, Namespace: s.srv.GetNamespace()})
		if err != nil {
			s.log.Errorf("Unable to update session %v: %v", sid, err)
		}
	}()
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

func (s *SessionRegistry) findSession(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func (r *SessionRegistry) PushTermSizeToParty(sconn *ssh.ServerConn, ch ssh.Channel) error {
	// the party may not be immediately available for this connection,
	// keep asking for a full second:
	for i := 0; i < 10; i++ {
		party := r.partyForConnection(sconn)
		if party == nil {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		// this starts a loop which will keep updating the terminal
		// size for every SSH write back to this connection
		party.termSizePusher(ch)
		return nil
	}

	return trace.Errorf("unable to push term size to party")
}

// partyForConnection finds an existing party which owns the given connection
func (r *SessionRegistry) partyForConnection(sconn *ssh.ServerConn) *party {
	r.Lock()
	defer r.Unlock()

	for _, session := range r.sessions {
		session.Lock()
		defer session.Unlock()
		parties := session.parties
		for _, party := range parties {
			if party.sconn == sconn {
				return party
			}
		}
	}
	return nil
}

// sessionRecorder implements io.Writer to be plugged into the multi-writer
// associated with every session. It forwards session stream to the audit log
type sessionRecorder struct {
	// log holds the structured logger
	log *logrus.Entry

	// alog is the audit log to store session chunks
	alog events.IAuditLog

	// sid defines the session to record
	sid rsession.ID

	// namespace is session namespace
	namespace string
}

func newSessionRecorder(alog events.IAuditLog, ctx *ServerContext, sid rsession.ID) (*sessionRecorder, error) {
	var auditLog events.IAuditLog
	if alog == nil {
		auditLog = &events.DiscardAuditLog{}
	} else {
		clusterConfig, err := ctx.srv.GetAccessPoint().GetClusterConfig()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if clusterConfig.GetAuditConfig().ShouldUploadSessions() {
			// in case of sessions upload, write sessions to local
			// disk, and forward only audit events to the remote audit logger
			forwarder, err := events.NewForwarder(events.ForwarderConfig{
				SessionID:      sid,
				ServerID:       "upload",
				DataDir:        filepath.Join(ctx.srv.GetDataDir(), teleport.LogsDir),
				RecordSessions: clusterConfig.GetSessionRecording() != services.RecordOff,
				Namespace:      ctx.srv.GetNamespace(),
				ForwardTo:      alog,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			alog = forwarder
		}
		cacher, err := state.NewCachingAuditLog(state.CachingAuditLogConfig{
			Namespace: ctx.srv.GetNamespace(),
			SessionID: string(sid),
			Server:    alog,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		auditLog = cacher
	}
	sr := &sessionRecorder{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentSession, ctx.srv.Component()),
		}),
		alog:      auditLog,
		sid:       sid,
		namespace: ctx.srv.GetNamespace(),
	}
	return sr, nil
}

// Write takes a chunk and writes it into the audit log
func (r *sessionRecorder) Write(data []byte) (int, error) {
	// we are copying buffer to prevent data corruption:
	// io.Copy allocates single buffer and calls multiple writes in a loop
	// our PostSessionChunk is async and sends reader wrapping buffer
	// to the channel. This can lead to cases when the buffer is re-used
	// and data is corrupted unless we copy the data buffer in the first place
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	// post the chunk of bytes to the audit log:
	chunk := &events.SessionChunk{
		Data: dataCopy,
		Time: time.Now().UTC().UnixNano(),
	}
	if err := r.alog.PostSessionSlice(events.SessionSlice{
		Namespace: r.namespace,
		SessionID: string(r.sid),
		Chunks:    []*events.SessionChunk{chunk},
	}); err != nil {
		r.log.Error(trace.DebugReport(err))
	}
	return len(data), nil
}

// Close closes audit log caching forwarder.
func (r *sessionRecorder) Close() error {
	var errors []error
	err := r.alog.Close()
	errors = append(errors, err)

	// wait until all events from recorder get flushed, it is important
	// to do so before we send SessionEndEvent to advise the audit log
	// to release resources associated with this session.
	// not doing so will not result in memory leak, but could result
	// in missing playback events
	context, cancel := context.WithTimeout(context.TODO(), defaults.ReadHeadersTimeout)
	defer cancel() // releases resources if slowOperation completes before timeout elapses
	err = r.alog.WaitForDelivery(context)
	if err != nil {
		errors = append(errors, err)
		r.log.Warnf("Timeout waiting for session to flush events: %v", trace.DebugReport(err))
	}

	return trace.NewAggregate(errors...)
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

	// parties are connected lients/users
	parties map[rsession.ID]*party

	term Terminal

	// closeC channel is used to kill all goroutines owned
	// by the session
	closeC chan bool

	// Linger TTL means "how long to keep session in memory after the last client
	// disconnected". It's useful to keep it alive for a bit in case the client
	// temporarily dropped the connection and will reconnect (or a browser-based
	// client hits "page refresh").
	lingerTTL time.Duration

	// termSizeC is used to push terminal resize events from SSH "on-size-changed"
	// event handler into "push-to-web-client" loop.
	termSizeC chan []byte

	// login stores the login of the initial session creator
	login string

	closeOnce sync.Once

	recorder *sessionRecorder
}

// newSession creates a new session with a given ID within a given context.
func newSession(id rsession.ID, r *SessionRegistry, ctx *ServerContext) (*session, error) {
	serverSessions.Inc()
	rsess := rsession.Session{
		ID: id,
		TerminalParams: rsession.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:      ctx.Identity.Login,
		Created:    time.Now().UTC(),
		LastActive: time.Now().UTC(),
		ServerID:   ctx.srv.ID(),
		Namespace:  r.srv.GetNamespace(),
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
		id:        id,
		registry:  r,
		parties:   make(map[rsession.ID]*party),
		writer:    newMultiWriter(),
		login:     ctx.Identity.Login,
		closeC:    make(chan bool),
		lingerTTL: defaults.SessionIdlePeriod,
	}
	return sess, nil
}

// isLingering returns 'true' if every party has left this session
func (s *session) isLingering() bool {
	s.Lock()
	defer s.Unlock()
	return len(s.parties) == 0
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

// start starts a new interactive process (or a shell) in the current session
func (s *session) start(ch ssh.Channel, ctx *ServerContext) error {
	var err error

	// create a new "party" (connected client)
	p := newParty(s, ch, ctx)

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
		ctx.Errorf("Unable to run shell command (%v): %v", ctx.ExecRequest.GetCommand(), err)
		return trace.ConvertSystemError(err)
	}
	if err := s.addParty(p); err != nil {
		return trace.Wrap(err)
	}

	params := s.term.GetTerminalParams()

	// get the audit log from the server and create a session recorder. this will
	// be a discard audit log if the proxy is in recording mode and a teleport
	// node so we don't create double recordings.
	auditLog := s.registry.srv.GetAuditLog()
	s.recorder, err = newSessionRecorder(auditLog, ctx, s.id)
	if err != nil {
		return trace.Wrap(err)
	}
	s.writer.addWriter("session-recorder", s.recorder, true)

	// emit "new session created" event:
	s.recorder.alog.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.EventNamespace:  ctx.srv.GetNamespace(),
		events.SessionEventID:  string(s.id),
		events.SessionServerID: ctx.srv.ID(),
		events.EventLogin:      ctx.Identity.Login,
		events.EventUser:       ctx.Identity.TeleportUser,
		events.LocalAddr:       ctx.Conn.LocalAddr().String(),
		events.RemoteAddr:      ctx.Conn.RemoteAddr().String(),
		events.TerminalSize:    params.Serialize(),
	})

	// start asynchronous loop of synchronizing session state with
	// the session server (terminal size and activity)
	go s.pollAndSync()

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

		// wait for copying from the pty to be complete or a timeout before
		// broadcasting the result (which will close the pty) if it has not been
		// closed already.
		select {
		case <-time.After(defaults.WaitCopyTimeout):
			s.log.Errorf("Timed out waiting for PTY copy to finish, session data for %v may be missing.", s.id)
		case <-doneCh:
		}

		if result != nil {
			s.registry.broadcastResult(s.id, *result)
		}
		if err != nil {
			s.log.Errorf("Shell exited with error: %v", err)
		} else {
			// no error? this means the command exited cleanly: no need
			// for this session to "linger" after this.
			s.SetLingerTTL(time.Duration(0))
		}
	}()

	// wait for the session to end before the shell, kill the shell
	go func() {
		<-s.closeC
		s.term.Kill()
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

// removeParty removes the party from two places:
//   1. from in-memory dictionary inside of this session
//   2. from sessin server's storage
func (s *session) removeParty(p *party) error {
	p.ctx.Infof("Removing party %v from session %v", p, s.id)

	ns := s.getNamespace()

	// in-memory locked remove:
	lockedRemove := func() {
		s.Lock()
		defer s.Unlock()
		delete(s.parties, p.id)
		s.writer.deleteWriter(string(p.id))
	}
	lockedRemove()

	// remove from the session server (asynchronously)
	storageRemove := func(db rsession.Service) {
		dbSession, err := db.GetSession(ns, s.id)
		if err != nil {
			s.log.Error("Unable to get session %v: %v", s.id, err)
			return
		}
		if dbSession != nil && dbSession.RemoveParty(p.id) {
			db.UpdateSession(rsession.UpdateRequest{
				ID:        dbSession.ID,
				Parties:   &dbSession.Parties,
				Namespace: ns,
			})
		}
	}
	if s.registry.srv.GetSessionServer() != nil {
		go storageRemove(s.registry.srv.GetSessionServer())
	}
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

// pollAndSync is a loops forever trying to sync terminal size to what's in
// the session (so all connected parties have the same terminal size) and
// update the "active" field of the session. If the session are recorded at
// the proxy, then this function does nothing as it's counterpart in the proxy
// will do this work.
func (s *session) pollAndSync() {
	// If sessions are being recorded at the proxy, an identical version of this
	// goroutine is running in the proxy, which means it does not need to run here.
	clusterConfig, err := s.registry.srv.GetAccessPoint().GetClusterConfig()
	if err != nil {
		s.log.Errorf("Unable to sync terminal size: %v.", err)
		return
	}
	if clusterConfig.GetSessionRecording() == services.RecordAtProxy &&
		s.registry.srv.Component() == teleport.ComponentNode {
		return
	}

	s.log.Debugf("Starting poll and sync of terminal size to all parties.")
	defer s.log.Debugf("Stopping poll and sync of terminal size to all parties.")

	ns := s.getNamespace()

	sessionServer := s.registry.srv.GetSessionServer()
	if sessionServer == nil {
		return
	}
	errCount := 0
	sync := func() error {
		sess, err := sessionServer.GetSession(ns, s.id)
		if err != nil || sess == nil {
			return trace.Wrap(err)
		}
		var active = true
		sessionServer.UpdateSession(rsession.UpdateRequest{
			Namespace: ns,
			ID:        sess.ID,
			Active:    &active,
			Parties:   nil,
		})
		winSize, err := s.term.GetWinSize()
		if err != nil {
			return err
		}
		termSizeChanged := (int(winSize.Width) != sess.TerminalParams.W ||
			int(winSize.Height) != sess.TerminalParams.H)
		if termSizeChanged {
			s.log.Debugf("Terminal changed from: %v to %v", sess.TerminalParams, winSize)
			err = s.term.SetWinSize(sess.TerminalParams)
		}
		return err
	}

	tick := time.NewTicker(defaults.TerminalSizeRefreshPeriod)
	defer tick.Stop()
	for {
		if err := sync(); err != nil {
			s.log.Infof("Unable to sync terminal: %v", err)
			errCount++
			// if the error count keeps going up, this means we're stuck in
			// a bad state: end this goroutine to avoid leaks
			if errCount > maxTermSyncErrorCount {
				return
			}
		} else {
			errCount = 0
		}
		select {
		case <-s.closeC:
			return
		case <-tick.C:
		}
	}
}

// addParty is called when a new party joins the session.
func (s *session) addParty(p *party) error {
	if s.login != p.login {
		return trace.AccessDenied(
			"can't switch users from %v to %v for session %v",
			s.login, p.login, s.id)
	}

	s.parties[p.id] = p
	// write last chunk (so the newly joined parties won't stare
	// at a blank screen)
	getRecentWrite := func() []byte {
		s.writer.Lock()
		defer s.writer.Unlock()
		data := make([]byte, 0, 1024)
		for i := range s.writer.recentWrites {
			data = append(data, s.writer.recentWrites[i]...)
		}
		return data
	}
	p.Write(getRecentWrite())

	// register this party as one of the session writers
	// (output will go to it)
	s.writer.addWriter(string(p.id), p, true)
	p.ctx.AddCloser(p)
	s.term.AddParty(1)

	// update session on the session server
	storageUpdate := func(db rsession.Service) {
		dbSession, err := db.GetSession(s.getNamespace(), s.id)
		if err != nil {
			s.log.Errorf("Unable to get session %v: %v", s.id, err)
			return
		}
		dbSession.Parties = append(dbSession.Parties, rsession.Party{
			ID:         p.id,
			User:       p.user,
			ServerID:   p.serverID,
			RemoteAddr: p.site,
			LastActive: p.getLastActive(),
		})
		db.UpdateSession(rsession.UpdateRequest{
			ID:        dbSession.ID,
			Parties:   &dbSession.Parties,
			Namespace: s.getNamespace(),
		})
	}
	if s.registry.srv.GetSessionServer() != nil {
		go storageUpdate(s.registry.srv.GetSessionServer())
	}

	s.log.Infof("New party %v joined session: %v", p.String(), s.id)

	// this goroutine keeps pumping party's input into the session
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
		site:      ctx.Conn.RemoteAddr().String(),
		id:        rsession.NewID(),
		ch:        ch,
		ctx:       ctx,
		s:         s,
		sconn:     ctx.Conn,
		termSizeC: make(chan []byte, 5),
		closeC:    make(chan bool),
	}
}

func (p *party) onWindowChanged(params *rsession.TerminalParams) {
	p.log.Debugf("Window size changed to %v in party: %v", params.Serialize(), p.id)

	p.Lock()
	defer p.Unlock()

	// this prefix will be appended to the end of every socker write going
	// to this party:
	prefix := []byte("\x00" + params.Serialize())
	if p.termSizeC != nil && len(p.termSizeC) == 0 {
		p.termSizeC <- prefix
	}
}

// This goroutine pushes terminal resize events directly into a connected web client
func (p *party) termSizePusher(ch ssh.Channel) {
	var (
		err error
		n   int
	)
	defer func() {
		if err != nil {
			p.log.Error(err)
		}
	}()

	for err == nil {
		select {
		case newSize := <-p.termSizeC:
			n, err = ch.Write(newSize)
			if err == io.EOF {
				continue
			}
			if err != nil || n == 0 {
				return
			}
		case <-p.closeC:
			return
		}
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
	})
	return err
}

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
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// sessionRegistry holds a map of all active sessions on a given
// SSH server
type sessionRegistry struct {
	sync.Mutex
	sessions map[rsession.ID]*session
	srv      *Server
}

func (s *sessionRegistry) addSession(sess *session) {
	s.Lock()
	defer s.Unlock()
	s.sessions[sess.id] = sess
}

func (r *sessionRegistry) Close() {
	r.Lock()
	defer r.Unlock()
	for _, s := range r.sessions {
		s.Close()
	}
	log.Infof("sessionRegistry.Close()")
}

// EmitAuditEvent logs a given event to the audit log attached to the
// server who owns these sessions
func (s *sessionRegistry) EmitAuditEvent(eventType string, fields events.EventFields) {
	alog := s.srv.alog
	if alog != nil {
		if err := alog.EmitAuditEvent(eventType, fields); err != nil {
			log.Error(err)
		}
	} else {
		log.Warn("SSH server has no audit log")
	}
}

// joinShell either joins an existing session or starts a new shell
func (s *sessionRegistry) joinShell(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	if ctx.session != nil {
		// emit "joined session" event:
		s.EmitAuditEvent(events.SessionJoinEvent, events.EventFields{
			events.SessionEventID:    string(ctx.session.id),
			events.SessionEventLogin: ctx.login,
		})
		ctx.Infof("joining session: %v", ctx.session.id)
		_, err := ctx.session.join(ch, req, ctx)
		return trace.Wrap(err)
	}
	// session not found? need to create one. start by getting/generating an ID for it
	sid, found := ctx.getEnv(sshutils.SessionEnvVar)
	if !found {
		sid = string(rsession.NewID())
		ctx.setEnv(sshutils.SessionEnvVar, sid)
	}
	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	sess, err := newSession(rsession.ID(sid), s, ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx.session = sess
	s.addSession(sess)
	ctx.Infof("ssh.joinShell created new session %v", sid)

	// emit "new session created" event:
	s.EmitAuditEvent(events.SessionJoinEvent, events.EventFields{
		events.SessionEventID:    string(sid),
		events.SessionEventLogin: ctx.login,
		events.SessionLocalAddr:  ctx.conn.LocalAddr().String(),
		events.SessionRemoveAddr: ctx.conn.RemoteAddr().String(),
	})

	if err := sess.startShell(ch, ctx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}

	return nil
}

// leaveShell remvoes a given party from this session
func (s *sessionRegistry) leaveShell(party *party) error {
	s.Lock()
	defer s.Unlock()
	sess := party.s

	// remove from in-memory representation of the session:
	if err := sess.removeParty(party); err != nil {
		return trace.Wrap(err)
	}

	// emit an audit event
	s.EmitAuditEvent(events.SessionLeaveEvent, events.EventFields{
		events.SessionEventID:    string(sess.id),
		events.SessionEventLogin: party.user,
	})

	if len(sess.parties) != 0 {
		return nil
	}

	// no more people left? Need to end the session!
	log.Infof("last party left %v, removing from server", sess)
	delete(s.sessions, sess.id)

	if err := sess.Close(); err != nil {
		log.Errorf("failed to close: %v", err)
		return err
	}
	return nil
}

// notifyWinChange is called when an SSH server receives a command notifying
// us that the terminal size has changed
func (s *sessionRegistry) notifyWinChange(params rsession.TerminalParams, ctx *ctx) error {
	if ctx.session == nil {
		err := trace.Errorf("SSH context has no associated session")
		ctx.Error(err)
		return nil
	}
	log.Infof("notifyWinChange(%v)", ctx.session.id)

	err := ctx.session.term.setWinsize(params)
	if err != nil {
		return trace.Wrap(err)
	}
	if s.srv.sessionServer == nil {
		return nil
	}
	go func() {
		sid := ctx.session.id
		err := s.srv.sessionServer.UpdateSession(
			rsession.UpdateRequest{ID: sid, TerminalParams: &params})
		if err != nil {
			log.Error(err)
		}
		// report this to the event/audit log:
		s.EmitAuditEvent(events.ResizeEvent, events.EventFields{
			events.SessionEventID:    string(sid),
			events.SessionEventLogin: ctx.login,
			events.ResizeSize:        fmt.Sprintf("%d:%d", params.W, params.H),
		})

	}()
	return nil
}

func (s *sessionRegistry) broadcastResult(sid rsession.ID, r execResult) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	sess.broadcastResult(r)
	return nil
}

func (s *sessionRegistry) findSession(id rsession.ID) (*session, bool) {
	sess, found := s.sessions[id]
	return sess, found
}

func newSessionRegistry(srv *Server) *sessionRegistry {
	if srv.sessionServer == nil {
		panic("need a session server")
	}
	return &sessionRegistry{
		srv:      srv,
		sessions: make(map[rsession.ID]*session),
	}
}

// session struct describes an active (in progress) SSH session. These sessions
// are managed by 'sessionRegistry' containers which are attached to SSH servers.
type session struct {
	sync.Mutex
	// session ID. unique GUID, this is what people use to "join" sessions

	id rsession.ID

	// parent session container
	registry *sessionRegistry

	// this writer is used to broadcast terminal I/O to different clients
	writer *multiWriter

	// parties are connected lients/users
	parties map[rsession.ID]*party
	term    *terminal

	// closeC channel is used to kill all goroutines owned
	// by the session
	closeC chan bool

	// login stores the login of the initial session creator
	login string

	closeOnce sync.Once
}

// newSession creates a new session with a given ID within a given context.
func newSession(id rsession.ID, r *sessionRegistry, context *ctx) (*session, error) {
	rsess := rsession.Session{
		ID:             id,
		TerminalParams: rsession.TerminalParams{W: 80, H: 25},
		Login:          context.login,
		Created:        time.Now().UTC(),
		LastActive:     time.Now().UTC(),
	}
	term := context.getTerm()
	if term != nil {
		winsize, err := term.getWinsize()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rsess.TerminalParams.W = int(winsize.Width - 1)
		rsess.TerminalParams.H = int(winsize.Height)
	}
	err := r.srv.sessionServer.CreateSession(rsess)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
		// if session already exists, make sure they are compatible
		// Login matches existing login
		existing, err := r.srv.sessionServer.GetSession(id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if existing.Login != rsess.Login {
			return nil, trace.AccessDenied(
				"can't switch users from %v to %v for session %v",
				rsess.Login, existing.Login, id)
		}
	}
	sess := &session{
		id:       id,
		registry: r,
		parties:  make(map[rsession.ID]*party),
		writer:   newMultiWriter(),
		login:    context.login,
		closeC:   make(chan bool),
	}
	return sess, nil
}

// Close ends the active session forcing all clients to disconnect and freeing all resources
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeC)
	})
	var err error
	if s.term != nil {
		err = s.term.Close()
	}
	return trace.Wrap(err)
}

// upsertSessionParty updates the persistence layer (session object stored somewhere on disk)
// with a new connected client.
func (s *session) upsertSessionParty(sid rsession.ID, p *party) error {
	if s.registry.srv.sessionServer == nil {
		return nil
	}
	// session registry has a "session server" (which is actually a "session serializer")
	// and we ask it to update the on-disk copy of this session with a new party
	return s.registry.srv.sessionServer.UpsertParty(sid, rsession.Party{
		ID:         p.id,
		User:       p.user,
		ServerID:   p.serverID,
		RemoteAddr: p.site,
		LastActive: p.getLastActive(),
	}, defaults.ActivePartyTTL)
}

// startShell starts a new shell process in the current session
func (s *session) startShell(ch ssh.Channel, ctx *ctx) error {
	// create a new "party" (connected client)
	p := newParty(s, ch, ctx)

	// allocate or borrow a terminal:
	if ctx.getTerm() != nil {
		s.term = ctx.getTerm()
		ctx.setTerm(nil)
	} else {
		var err error
		if s.term, err = newTerminal(); err != nil {
			ctx.Infof("handleShell failed to create term: %v", err)
			return trace.Wrap(err)
		}
	}
	// prepare environment & Launch shell:
	cmd, err := prepareOSCommand(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.term.run(cmd); err != nil {
		ctx.Errorf("shell command failed: %v", err)
		return trace.ConvertSystemError(err)
	}
	s.addParty(p)

	// emit "new session created" event:
	s.registry.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID:    string(s.id),
		events.SessionEventLogin: ctx.login,
		events.SessionLocalAddr:  ctx.conn.LocalAddr().String(),
		events.SessionRemoveAddr: ctx.conn.RemoteAddr().String(),
	})

	// start terminal size syncing loop:
	go s.pollAndSyncTerm()

	// Pipe session to shell and visa-versa capturing input and output
	s.term.Add(1)
	go func() {
		// notify terminal about a copy process going on
		defer s.term.Add(-1)
		io.Copy(s.writer, s.term.pty)
		log.Infof("session.io.copy() stopped")
	}()

	// wait for the shell to complete:
	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		exitCode := 0
		if result != nil {
			exitCode = result.code
			s.registry.broadcastResult(s.id, *result)
		}
		if err != nil {
			log.Errorf("shell exited with error: %v", err)
		}
		// send an event indicating that this session has ended
		s.registry.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
			events.SessionEventID: string(s.id),
		})
		_ = exitCode
	}()

	// wait for the session to end before the shell, kill the shell
	go func() {
		<-s.closeC
		if cmd.ProcessState == nil && cmd.Process != nil {
			log.Infof("killing process: %v PID=%v", cmd.Path, cmd.Process.Pid)
			if err := cmd.Process.Kill(); err != nil {
				log.Error(err)
			}
		}
	}()
	return nil
}

func (s *session) broadcastResult(r execResult) {
	for _, p := range s.parties {
		p.ctx.sendResult(r)
	}
}

func (s *session) String() string {
	return fmt.Sprintf("session(id=%v, parties=%v)", s.id, len(s.parties))
}

func (s *session) removeParty(p *party) error {
	s.Lock()
	defer s.Unlock()
	p.ctx.Infof("%v is leaving", p)

	delete(s.parties, p.id)
	s.writer.deleteWriter(string(p.id))

	return nil
}

// pollAndSyncTerm is a loop inside a goroutite which keeps synchronizing the terminal
// size to what's in the session (so all connected parties have the same terminal size)
func (s *session) pollAndSyncTerm() {
	sessionServer := s.registry.srv.sessionServer
	if sessionServer == nil {
		return
	}
	syncTerm := func() error {
		sess, err := sessionServer.GetSession(s.id)
		if err != nil {
			log.Debugf("syncTerm: no session")
			return err
		}
		winSize, err := s.term.getWinsize()
		if err != nil {
			log.Debugf("syncTerm: no terminal")
			return err
		}
		if int(winSize.Width) == sess.TerminalParams.W && int(winSize.Height) == sess.TerminalParams.H {
			return nil
		}
		log.Debugf("terminal has changed from: %v to %v", sess.TerminalParams, winSize)
		return s.term.setWinsize(sess.TerminalParams)
	}

	tick := time.NewTicker(defaults.TerminalSizeRefreshPeriod)
	defer tick.Stop()
	for {
		if err := syncTerm(); err != nil {
			log.Infof("sync term error: %v", err)
		}
		select {
		case <-s.closeC:
			log.Infof("[SSH] terminal sync stopped")
			return
		case <-tick.C:
		}
	}
}

func (s *session) addParty(p *party) {
	s.parties[p.id] = p
	s.writer.addWriter(string(p.id), p, true)
	p.ctx.addCloser(p)
	s.term.Add(1)
	go func() {
		defer s.term.Add(-1)
		_, err := io.Copy(s.term.pty, p)
		p.ctx.Infof("party.io.copy(%v) closed", p.id)
		if err != nil {
			log.Error(err)
		}
	}()
	go func() {
		for {
			if err := s.upsertSessionParty(s.id, p); err != nil {
				p.ctx.Warningf("failed to upsert session party: %v", err)
			}
			select {
			case <-p.closeC:
				p.ctx.Infof("closed, stopped heartbeat")
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()
}

func (s *session) join(ch ssh.Channel, req *ssh.Request, ctx *ctx) (*party, error) {
	p := newParty(s, ch, ctx)
	s.addParty(p)
	return p, nil
}

func newMultiWriter() *multiWriter {
	return &multiWriter{writers: make(map[string]writerWrapper)}
}

type multiWriter struct {
	sync.RWMutex
	writers map[string]writerWrapper
}

type writerWrapper struct {
	io.Writer
	closeOnError bool
}

func (m *multiWriter) addWriter(id string, w io.Writer, closeOnError bool) {
	m.Lock()
	defer m.Unlock()
	m.writers[id] = writerWrapper{Writer: w, closeOnError: closeOnError}
}

func (m *multiWriter) deleteWriter(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.writers, id)
}

func (m *multiWriter) Write(p []byte) (n int, err error) {
	m.RLock()
	defer m.RUnlock()

	for _, w := range m.writers {
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

func newParty(s *session, ch ssh.Channel, ctx *ctx) *party {
	return &party{
		user:     ctx.teleportUser,
		serverID: s.registry.srv.ID(),
		site:     ctx.conn.RemoteAddr().String(),
		id:       rsession.NewID(),
		ch:       ch,
		ctx:      ctx,
		s:        s,
		closeC:   make(chan bool),
	}
}

type party struct {
	sync.Mutex
	user       string
	serverID   string
	site       string
	id         rsession.ID
	s          *session
	sconn      *ssh.ServerConn
	ch         ssh.Channel
	ctx        *ctx
	closeC     chan bool
	lastActive time.Time
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

func (p *party) Close() error {
	p.ctx.Infof("party[%v].Close()", p.id)
	close(p.closeC)
	return p.s.registry.leaveShell(p)
}

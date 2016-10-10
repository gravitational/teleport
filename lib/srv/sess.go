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
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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
	log.Debugf("sessionRegistry.Close()")
}

// joinShell either joins an existing session or starts a new shell
func (s *sessionRegistry) openSession(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	if ctx.session != nil {
		// emit "joined session" event:
		s.srv.EmitAuditEvent(events.SessionJoinEvent, events.EventFields{
			events.SessionEventID:  string(ctx.session.id),
			events.EventLogin:      ctx.login,
			events.EventUser:       ctx.teleportUser,
			events.LocalAddr:       ctx.conn.LocalAddr().String(),
			events.RemoteAddr:      ctx.conn.RemoteAddr().String(),
			events.SessionServerID: ctx.srv.ID(),
		})
		ctx.Infof("[SESSION] joining session: %v", ctx.session.id)
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
	ctx.Infof("[SESSION] new session %v", sid)

	if err := sess.start(ch, ctx); err != nil {
		sess.Close()
		return trace.Wrap(err)
	}
	return nil
}

// leaveSession removes the given party from this session
func (s *sessionRegistry) leaveSession(party *party) error {
	sess := party.s
	s.Lock()
	defer s.Unlock()

	// remove from in-memory representation of the session:
	if err := sess.removeParty(party); err != nil {
		return trace.Wrap(err)
	}

	// emit "session leave" event (party left the session)
	s.srv.EmitAuditEvent(events.SessionLeaveEvent, events.EventFields{
		events.SessionEventID:  string(sess.id),
		events.EventUser:       party.user,
		events.SessionServerID: party.serverID,
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
			log.Infof("[session.registry] session %v becomes active again", sess.id)
			return
		}
		log.Infof("[session.registry] session %v to be garbage collected", sess.id)

		// no more people left? Need to end the session!
		s.Lock()
		delete(s.sessions, sess.id)
		s.Unlock()

		// send an event indicating that this session has ended
		s.srv.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
			events.SessionEventID: string(sess.id),
			events.EventUser:      party.user,
		})
		if err := sess.Close(); err != nil {
			log.Error(err)
		}

		// mark it as inactive in the DB
		if s.srv.sessionServer != nil {
			False := false
			s.srv.sessionServer.UpdateSession(rsession.UpdateRequest{
				ID:     sess.id,
				Active: &False,
			})
		}
	}
	go lingerAndDie()
	return nil
}

// getParties allows to safely return a list of parties connected to this
// session (as determined by ctx)
func (s *sessionRegistry) getParties(ctx *ctx) (parties []*party) {
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
func (s *sessionRegistry) notifyWinChange(params rsession.TerminalParams, ctx *ctx) error {
	if ctx.session == nil {
		log.Debugf("notifyWinChange(): no session found!")
		return nil
	}
	sid := ctx.session.id

	// report this to the event/audit log:
	s.srv.EmitAuditEvent(events.ResizeEvent, events.EventFields{
		events.SessionEventID: sid,
		events.EventLogin:     ctx.login,
		events.EventUser:      ctx.teleportUser,
		events.TerminalSize:   params.Serialize(),
	})
	err := ctx.session.term.setWinsize(params)
	if err != nil {
		return trace.Wrap(err)
	}

	// notify all connected parties about the change in real time
	// (if they're capable)
	for _, p := range s.getParties(ctx) {
		p.onWindowChanged(&params)
	}

	go func() {
		err := s.srv.sessionServer.UpdateSession(
			rsession.UpdateRequest{ID: sid, TerminalParams: &params})
		if err != nil {
			log.Error(err)
		}
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

	term *terminal

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
}

// newSession creates a new session with a given ID within a given context.
func newSession(id rsession.ID, r *sessionRegistry, context *ctx) (*session, error) {
	rsess := rsession.Session{
		ID: id,
		TerminalParams: rsession.TerminalParams{
			W: teleport.DefaultTerminalWidth,
			H: teleport.DefaultTerminalHeight,
		},
		Login:      context.login,
		Created:    time.Now().UTC(),
		LastActive: time.Now().UTC(),
		ServerID:   context.srv.ID(),
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
		if trace.IsAlreadyExists(err) {
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
		// return nil, trace.Wrap(err)
		// No need to abort. Perhaps the auth server is down?
		// Log the error and continue:
		log.Errorf("failed logging new session: %v", err)
	}

	sess := &session{
		id:        id,
		registry:  r,
		parties:   make(map[rsession.ID]*party),
		writer:    newMultiWriter(),
		login:     context.login,
		closeC:    make(chan bool),
		lingerTTL: defaults.SessionRefreshPeriod * 10,
	}
	return sess, nil
}

// PartyForConnection finds an existing party which owns the given connection
func (r *sessionRegistry) PartyForConnection(sconn *ssh.ServerConn) *party {
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

// This goroutine pushes terminal resize events directly into a connected web client
func (p *party) termSizePusher(ch ssh.Channel) {
	var (
		err error
		n   int
	)
	defer func() {
		if err != nil {
			log.Error(err)
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

// isLingering returns 'true' if every party has left this session
func (s *session) isLingering() bool {
	s.Lock()
	defer s.Unlock()
	return len(s.parties) == 0
}

// Close ends the active session forcing all clients to disconnect and freeing all resources
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		// closing needs to happen asynchronously because the last client
		// (session writer) will try to close this session, causing a deadlock
		// because of closeOnce
		go func() {
			log.Infof("session.Close(%v)", s.id)
			if s.term != nil {
				s.term.Close()
			}
			close(s.closeC)

			// close all writers in our multi-writer
			s.writer.Lock()
			defer s.writer.Unlock()
			for writerName, writer := range s.writer.writers {
				log.Infof("session.close(writer=%v)", writerName)
				closer, ok := io.Writer(writer).(io.WriteCloser)
				if ok {
					closer.Close()
				}
			}
		}()
	})
	return nil
}

// sessionRecorder implements io.Writer to be plugged into the multi-writer
// associated with every session. It forwards session stream to the audit log
type sessionRecorder struct {
	// alog is the audit log to store session chunks
	alog events.IAuditLog
	// sid defines the session to record
	sid rsession.ID
}

func newSessionRecorder(alog events.IAuditLog, sid rsession.ID) *sessionRecorder {
	sr := &sessionRecorder{
		alog: alog,
		sid:  sid,
	}
	return sr
}

// Write takes a chunk and writes it into the audit log
func (r *sessionRecorder) Write(data []byte) (int, error) {
	const (
		minChunkLen   = 4
		maxDelay      = time.Millisecond * 20
		requiredDelay = time.Millisecond * 5
	)
	// terminal recording is a tricky business. the TTY subsystem expects a certain
	// delay when it writes to a virtual console. In our case recording takes no time
	// so we have to emulate a bit of a delay here:
	var start time.Time
	dataLen := len(data)
	if dataLen > minChunkLen {
		start = time.Now()
		defer func() {
			postingDuration := time.Now().Sub(start)
			if postingDuration < requiredDelay {
				delay := time.Millisecond * time.Duration(dataLen)
				if delay > maxDelay {
					delay = maxDelay
				}
				time.Sleep(delay)
			}
		}()
	}
	// post the chunk of bytes to the audit log:
	if err := r.alog.PostSessionChunk(r.sid, bytes.NewReader(data)); err != nil {
		log.Error(err)
	}
	return dataLen, nil
}

// Close() does nothing for session recorder (audit log cannot be closed)
func (r *sessionRecorder) Close() error {
	return nil
}

// start starts a new interactive process (or a shell) in the current session
func (s *session) start(ch ssh.Channel, ctx *ctx) error {
	// create a new "party" (connected client)
	p := newParty(s, ch, ctx)

	// allocate a terminal or take the one previously allocated via a
	// seaprate "allocate TTY" SSH request
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
	cmd, err := prepInteractiveCommand(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.term.run(cmd); err != nil {
		ctx.Errorf("shell command failed: %v", err)
		return trace.ConvertSystemError(err)
	}
	s.addParty(p)

	// emit "new session created" event:
	s.registry.srv.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
		events.SessionEventID:  string(s.id),
		events.SessionServerID: ctx.srv.ID(),
		events.EventLogin:      ctx.login,
		events.EventUser:       ctx.teleportUser,
		events.LocalAddr:       ctx.conn.LocalAddr().String(),
		events.RemoteAddr:      ctx.conn.RemoteAddr().String(),
		events.TerminalSize:    s.term.params.Serialize(),
	})

	// start recording this session
	auditLog := s.registry.srv.alog
	if auditLog != nil {
		s.writer.addWriter("session-recorder",
			newSessionRecorder(auditLog, s.id),
			true)
	}

	// start asynchronous loop of synchronizing session state with
	// the session server (terminal size and activity)
	go s.pollAndSync()

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
		if result != nil {
			s.registry.broadcastResult(s.id, *result)
		}
		if err != nil {
			log.Errorf("shell exited with error: %v", err)
		} else {
			// no error? this means the command exited cleanly: no need
			// for this session to "linger" after this.
			s.SetLingerTTL(time.Duration(0))
		}
	}()

	// wait for the session to end before the shell, kill the shell
	go func() {
		<-s.closeC
		if cmd.Process != nil {
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

// removeParty removes the party from two places:
//   1. from in-memory dictionary inside of this session
//   2. from sessin server's storage
func (s *session) removeParty(p *party) error {
	p.ctx.Infof("session.removeParty(%v)", p)

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
		dbSession, err := db.GetSession(s.id)
		if err != nil {
			log.Error(err)
			return
		}
		if dbSession != nil && dbSession.RemoveParty(p.id) {
			db.UpdateSession(rsession.UpdateRequest{
				ID:      dbSession.ID,
				Parties: &dbSession.Parties,
			})
		}
	}
	if s.registry.srv.sessionServer != nil {
		go storageRemove(s.registry.srv.sessionServer)
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

// pollAndSync is a loop inside a goroutite which keeps synchronizing the terminal
// size to what's in the session (so all connected parties have the same terminal size)
// it also updates 'active' field on the session.
func (s *session) pollAndSync() {
	log.Debugf("[session.registry] start pollAndSync()\b")
	defer log.Debugf("[session.registry] end pollAndSync()\n")

	sessionServer := s.registry.srv.sessionServer
	if sessionServer == nil {
		return
	}
	errCount := 0
	sync := func() error {
		sess, err := sessionServer.GetSession(s.id)
		if sess == nil {
			return trace.Wrap(err)
		}
		var active = true
		sessionServer.UpdateSession(rsession.UpdateRequest{
			ID:      sess.ID,
			Active:  &active,
			Parties: nil,
		})
		winSize, err := s.term.getWinsize()
		if err != nil {
			return err
		}
		termSizeChanged := (int(winSize.Width) != sess.TerminalParams.W ||
			int(winSize.Height) != sess.TerminalParams.H)
		if termSizeChanged {
			log.Debugf("terminal has changed from: %v to %v", sess.TerminalParams, winSize)
			err = s.term.setWinsize(sess.TerminalParams)
		}
		return err
	}

	tick := time.NewTicker(defaults.TerminalSizeRefreshPeriod)
	defer tick.Stop()
	for {
		if err := sync(); err != nil {
			log.Infof("sync term error: %v", err)
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
			log.Infof("[SSH] terminal sync stopped")
			return
		case <-tick.C:
		}
	}
}

// addParty is called when a new party joins the session.
func (s *session) addParty(p *party) {
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
	p.ctx.addCloser(p)
	s.term.Add(1)

	// update session on the session server
	storageUpdate := func(db rsession.Service) {
		dbSession, err := db.GetSession(s.id)
		if err != nil {
			log.Error(err)
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
			ID:      dbSession.ID,
			Parties: &dbSession.Parties,
		})
	}
	if s.registry.srv.sessionServer != nil {
		go storageUpdate(s.registry.srv.sessionServer)
	}

	p.ctx.Infof("[SESSION] new party joined: %v", p.String())

	// this goroutine keeps pumping party's input into the session
	go func() {
		defer s.term.Add(-1)
		_, err := io.Copy(s.term.pty, p)
		p.ctx.Infof("party.io.copy(%v) closed", p.id)
		if err != nil {
			log.Error(err)
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

func newParty(s *session, ch ssh.Channel, ctx *ctx) *party {
	return &party{
		user:      ctx.teleportUser,
		serverID:  s.registry.srv.ID(),
		site:      ctx.conn.RemoteAddr().String(),
		id:        rsession.NewID(),
		ch:        ch,
		ctx:       ctx,
		s:         s,
		sconn:     ctx.conn,
		termSizeC: make(chan []byte, 5),
		closeC:    make(chan bool),
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
	termSizeC  chan []byte
	lastActive time.Time
	closeOnce  sync.Once
}

func (p *party) onWindowChanged(params *rsession.TerminalParams) {
	log.Debugf("party(%s).onWindowChanged(%v)", p.id, params.Serialize())

	p.Lock()
	defer p.Unlock()

	// this prefix will be appended to the end of every socker write going
	// to this party:
	prefix := []byte("\x00" + params.Serialize())
	if p.termSizeC != nil && len(p.termSizeC) == 0 {
		p.termSizeC <- prefix
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
		p.ctx.Infof("party[%v].Close()", p.id)
		if err = p.s.registry.leaveSession(p); err != nil {
			p.ctx.Error(err)
		}
		close(p.closeC)
		close(p.termSizeC)
	})
	return err
}

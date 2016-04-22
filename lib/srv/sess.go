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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	rsession "github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
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

// joinShell either joins an existing session or starts a new shell
func (s *sessionRegistry) joinShell(sid rsession.ID, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("joinShell(sid=%v)", sid)

	sess, found := s.lockedFindSession(sid)
	if found {
		ctx.Infof("joining session: %v", sess.id)
		_, err := sess.join(sconn, ch, req, ctx)
		return trace.Wrap(err)
	}

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	sess, err := newSession(sid, s, ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := sess.startShell(sconn, ch, ctx); err != nil {
		defer sess.Close()
		return trace.Wrap(err)
	}
	s.addSession(sess)
	ctx.Infof("created session: %v", sess.id)

	return nil
}

func (s *sessionRegistry) leaveShell(sid, pid rsession.ID) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	if err := sess.leave(pid); err != nil {
		return trace.Wrap(err)
	}
	if len(sess.parties) != 0 {
		return nil
	}
	log.Infof("last party left %v, removing from server", sess)
	delete(s.sessions, sess.id)
	if err := sess.Close(); err != nil {
		log.Errorf("failed to close: %v", err)
		return err
	}
	return nil
}

func (s *sessionRegistry) notifyWinChange(sid rsession.ID, params rsession.TerminalParams, ctx *ctx) error {
	log.Infof("notifyWinChange(%v)", sid)
	sess, found := s.findSession(sid)
	if !found {
		return trace.NotFound("session %v not found", sid)
	}
	err := sess.term.setWinsize(params)
	if err != nil {
		return trace.Wrap(err)
	}
	if s.srv.sessionServer == nil {
		return nil
	}
	go func() {
		err := s.srv.sessionServer.UpdateSession(
			rsession.UpdateRequest{ID: sid, TerminalParams: &params})
		if err != nil {
			log.Error(err)
		}
		// report this to the event/audit log:
		ctx.emit(&events.TerminalResized{
			SessionID: string(sid),
			Width:     params.W,
			Height:    params.H,
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

func (s *sessionRegistry) lockedFindSession(id rsession.ID) (*session, bool) {
	s.Lock()
	defer s.Unlock()
	return s.findSession(id)
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

type session struct {
	sync.Mutex
	id          rsession.ID
	eid         lunk.EventID
	registry    *sessionRegistry
	writer      *multiWriter
	parties     map[rsession.ID]*party
	term        *terminal
	chunkWriter *chunkWriter
	closeC      chan bool
	login       string
	closeOnce   sync.Once
}

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
	if err := r.srv.elog.LogSession(rsess); err != nil {
		log.Warn("failed to log session event: %v", err)
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

func (s *session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeC)
	})
	var err error
	if s.term != nil {
		err = s.term.Close()
	}
	if s.chunkWriter != nil {
		err = s.chunkWriter.Close()
	}
	return trace.Wrap(err)
}

func (s *session) upsertSessionParty(sid rsession.ID, p *party) error {
	if s.registry.srv.sessionServer == nil {
		return nil
	}
	return s.registry.srv.sessionServer.UpsertParty(sid, rsession.Party{
		ID:         p.id,
		User:       p.user,
		ServerID:   p.serverID,
		RemoteAddr: p.site,
		LastActive: p.getLastActive(),
	}, defaults.ActivePartyTTL)
}

// startShell starts a new shell process in the current session
func (s *session) startShell(sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) error {
	s.eid = ctx.eid
	ctx.session = s
	// create a new "party" (connected client)
	p := newParty(s, sconn, ch, ctx)

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
	// start recording the session (if enabled)
	sessionRecorder := ctx.srv.rec
	recordID := ""
	if sessionRecorder != nil {
		sid := string(s.id)
		// create a 'writer' wrapper which converts io.Write([]byte) calls to recorder.WriteChunks([]chunk)
		cw, err := sessionRecorder.GetChunkWriter(sid)
		if err != nil {
			ctx.Errorf("failed to create recorder: %v", err)
			return trace.Wrap(err)
		}
		s.chunkWriter = newChunkWriter(sid, s, cw)
		recordID = s.chunkWriter.recordID
		s.writer.addWriter("capture", s.chunkWriter, false)
	}
	// emit 'new session' event into the audit log:
	shellCmd := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " "))
	ctx.emit(events.NewShellSession(string(s.id), sconn, shellCmd, recordID))
	s.addParty(p)

	// start terminal size syncing loop:
	go s.pollAndSyncTerm()

	// Pipe session to shell and visa-versa capturing input and output
	s.term.Add(1)
	go func() {
		// notify terminal about a copy process going on
		defer s.term.Add(-1)
		_, err := io.Copy(s.writer, s.term.pty)
		if err != nil {
			log.Error(err)
		}
		log.Infof("session.io.copy() closed")
		cmd.Process = nil
	}()

	// wait for the shell to complete:
	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		exitCode := 0
		if result != nil {
			exitCode = result.code
			s.registry.broadcastResult(s.id, *result)
		}
		// send an event indicating that this session has ended
		ctx.emit(&events.SessionEnded{string(s.id), exitCode, ""})
		if err != nil {
			log.Errorf("shell exited with error: %v", err)
		}
	}()

	// wait for the session to end before the shell, kill the shell
	go func() {
		<-s.closeC
		if cmd.Process != nil {
			log.Infof("session ended. killing process: %v PID=%v", cmd.Path, cmd.Process.Pid)
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

func (s *session) leave(id rsession.ID) error {
	p, ok := s.parties[id]
	if !ok {
		return trace.NotFound("party %v not found", id)
	}
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

func (s *session) join(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) (*party, error) {
	p := newParty(s, sconn, ch, ctx)
	s.addParty(p)
	// replay last chunk to the new party so they'll see what's going on:
	io.Copy(p, s.chunkWriter)
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

func newParty(s *session, sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) *party {
	return &party{
		user:     ctx.teleportUser,
		serverID: s.registry.srv.ID(),
		site:     sconn.RemoteAddr().String(),
		id:       rsession.NewID(),
		sconn:    sconn,
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
	return p.s.registry.leaveShell(p.s.id, p.id)
}

func newChunkWriter(recordID string, sess *session, cw recorder.ChunkWriteCloser) *chunkWriter {
	return &chunkWriter{
		recordID: recordID,
		w:        cw,
		sess:     sess,
	}
}

type chunkWriter struct {
	sync.Mutex
	before    time.Time
	recordID  string
	w         recorder.ChunkWriteCloser
	sess      *session
	lastChunk *recorder.Chunk
}

func (l *chunkWriter) Write(b []byte) (int, error) {
	diff := time.Duration(0)
	if l.before.IsZero() {
		l.before = time.Now()
	} else {
		now := time.Now()
		diff = now.Sub(l.before)
		l.before = now
	}
	chunk := recorder.Chunk{
		Delay:          diff,
		Data:           b,
		ServerID:       l.sess.registry.srv.ID(),
		TerminalParams: l.sess.term.getTerminalParams(),
	}
	if err := l.w.WriteChunks([]recorder.Chunk{chunk}); err != nil {
		return 0, err
	}

	l.Lock()
	defer l.Unlock()
	l.lastChunk = &chunk

	return len(b), nil
}

// Read implements io.Reader for chunkWriter. Reads the last chunk's data
func (l *chunkWriter) Read(b []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	if l.lastChunk == nil || l.lastChunk.Data == nil {
		return 0, io.EOF
	}
	return copy(b, l.lastChunk.Data), io.EOF
}

func (l *chunkWriter) Close() error {
	return l.w.Close()
}

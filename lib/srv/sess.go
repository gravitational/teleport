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
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	rsession "github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/ssh"
)

type sessionRegistry struct {
	sync.Mutex
	sessions map[string]*session
	srv      *Server
}

func (s *sessionRegistry) newShell(sid string, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("newShell(sid=%v)", sid)

	sess, err := newSession(sid, s, ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := sess.start(sconn, ch, ctx); err != nil {
		return trace.Wrap(err)
	}
	s.sessions[sess.id] = sess
	ctx.Infof("created session: %v", sess.id)
	return nil
}

func (s *sessionRegistry) joinShell(sid string, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("joinShell(sid=%v)", sid)

	sess, found := s.lockedFindSession(sid)
	if found {
		ctx.Infof("joining session: %v", sess.id)
		_, err := sess.join(sconn, ch, req, ctx)
		return trace.Wrap(err)
	}

	// This logic allows concurrent request to create a new session
	// to fail, what is ok because we should never have this condition
	return trace.Wrap(s.newShell(sid, sconn, ch, req, ctx))
}

func (s *sessionRegistry) leaveShell(sid, pid string) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return trace.Wrap(
			teleport.NotFound(fmt.Sprintf("session %v not found", sid)))
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

func (s *sessionRegistry) notifyWinChange(sid string, params rsession.TerminalParams) error {
	log.Infof("notifyWinChange(%v)", sid)

	sess, found := s.findSession(sid)
	if !found {
		return trace.Wrap(
			teleport.NotFound(fmt.Sprintf("session %v not found", sid)))
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
			log.Infof("notifyWinChange(%v): %v", sid, err)
		}
	}()
	return nil
}

func (s *sessionRegistry) broadcastResult(sid string, r execResult) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return trace.Wrap(
			teleport.NotFound(fmt.Sprintf("session %v not found", sid)))
	}
	sess.broadcastResult(r)
	return nil
}

func (s *sessionRegistry) findSession(id string) (*session, bool) {
	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	return sess, true
}

func (s *sessionRegistry) lockedFindSession(id string) (*session, bool) {
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
		sessions: make(map[string]*session),
	}
}

type session struct {
	id          string
	eid         lunk.EventID
	registry    *sessionRegistry
	writer      *multiWriter
	parties     map[string]*party
	term        *terminal
	chunkWriter *chunkWriter
	closeC      chan bool
	login       string
	closeOnce   sync.Once
}

func newSession(id string, r *sessionRegistry, context *ctx) (*session, error) {
	rsess := rsession.Session{
		ID:             id,
		TerminalParams: rsession.TerminalParams{W: 100, H: 100},
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
		rsess.TerminalParams.W = int(winsize.Width)
		rsess.TerminalParams.H = int(winsize.Height)
	}
	err := r.srv.sessionServer.CreateSession(rsess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess := &session{
		id:       id,
		registry: r,
		parties:  make(map[string]*party),
		writer:   newMultiWriter(),
		login:    context.login,
		closeC:   make(chan bool),
	}
	go sess.pollAndSyncTerm()
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

func (s *session) upsertSessionParty(sid string, p *party) error {
	if s.registry.srv.sessionServer == nil {
		return nil
	}
	return s.registry.srv.sessionServer.UpsertParty(sid, rsession.Party{
		ID:         p.id,
		User:       p.user,
		ServerAddr: p.serverAddr,
		RemoteAddr: p.site,
		LastActive: p.getLastActive(),
	}, rsession.DefaultActivePartyTTL)
}

func setCmdUser(cmd *exec.Cmd, username string) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	osUser, err := user.Lookup(username)
	if err != nil {
		return trace.Wrap(err)
	}
	curUser, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}

	if username != curUser.Username {
		uid, err := strconv.Atoi(osUser.Uid)
		if err != nil {
			return trace.Wrap(err)
		}
		gid, err := strconv.Atoi(osUser.Gid)
		if err != nil {
			return trace.Wrap(err)
		}

		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		cmd.Dir = osUser.HomeDir
	} else {
		cmd.Dir = curUser.HomeDir
	}

	return nil
}

func (s *session) start(sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) error {
	s.eid = ctx.eid
	p := newParty(s, sconn, ch, ctx)
	if p.ctx.getTerm() != nil {
		s.term = p.ctx.getTerm()
		p.ctx.setTerm(nil)
	} else {
		var err error
		if s.term, err = newTerminal(); err != nil {
			ctx.Infof("handleShell failed to create term: %v", err)
			return trace.Wrap(err)
		}
	}
	cmd := exec.Command(s.registry.srv.shell)
	// TODO(klizhentas) figure out linux user policy for launching shells,
	// what user and environment should we use to execute the shell? the simplest
	// answer is to use current user and env, however  what if we are root?
	cmd.Env = []string{
		"TERM=xterm",
		"HOME=" + os.Getenv("HOME"),
		"USER=" + sconn.User(),
	}

	err := setCmdUser(cmd, sconn.User())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.term.run(cmd); err != nil {
		p.ctx.Infof("failed to start shell: %v", err)
		return trace.Wrap(err)
	}
	p.ctx.Infof("starting shell input/output streaming")

	if s.registry.srv.rec != nil {
		w, err := newChunkWriter(s.id, s.registry.srv.rec, s.registry.srv.addr.Addr)
		if err != nil {
			p.ctx.Errorf("failed to create recorder: %v", err)
			return trace.Wrap(err)
		}
		s.chunkWriter = w
		s.registry.srv.emit(ctx.eid, events.NewShellSession(s.id, sconn, s.registry.srv.shell, w.rid))
		s.writer.addWriter("capture", w, false)
	} else {
		s.registry.srv.emit(ctx.eid, events.NewShellSession(s.id, sconn, s.registry.srv.shell, ""))
	}
	s.addParty(p)

	// Pipe session to shell and visa-versa capturing input and output
	go func() {
		written, err := io.Copy(s.writer, s.term.pty)
		p.ctx.Infof("shell to channel copy closed, bytes written: %v, err: %v",
			written, err)
	}()

	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		if err != nil {
			p.ctx.Errorf("wait failed: %v", err)
			return
		}
		if result != nil {
			s.registry.broadcastResult(s.id, *result)
			p.ctx.Infof("result broadcasted")
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

func (s *session) leave(id string) error {
	p, ok := s.parties[id]
	if !ok {
		return trace.Wrap(
			teleport.NotFound(fmt.Sprintf("party %v not found", id)))
	}
	p.ctx.Infof("%v is leaving", p)
	delete(s.parties, p.id)
	s.writer.deleteWriter(p.id)
	return nil
}

const pollingPeriod = time.Second

func (s *session) syncTerm(sessionServer rsession.Service) error {
	sess, err := sessionServer.GetSession(s.id)
	if err != nil {
		log.Infof("syncTerm: no session")
		return trace.Wrap(err)
	}
	winSize, err := s.term.getWinsize()
	if err != nil {
		log.Infof("syncTerm: no terminal")
		return trace.Wrap(err)
	}
	if int(winSize.Width) == sess.TerminalParams.W && int(winSize.Height) == sess.TerminalParams.H {
		log.Infof("terminal not changed: %v", sess.TerminalParams)
		return nil
	}
	log.Infof("terminal has changed from: %v to %v", sess.TerminalParams, winSize)
	err = s.term.setWinsize(sess.TerminalParams)
	return trace.Wrap(err)
}

func (s *session) pollAndSyncTerm() {
	if s.registry.srv.sessionServer == nil {
		return
	}
	sessionServer := s.registry.srv.sessionServer
	tick := time.NewTicker(pollingPeriod)
	defer tick.Stop()
	for {
		if err := s.syncTerm(sessionServer); err != nil {
			log.Infof("sync term error: %v", err)
		}
		select {
		case <-s.closeC:
			log.Infof("closed, stopped heartbeat")
			return
		case <-tick.C:
		}
	}
}

func (s *session) addParty(p *party) {
	s.parties[p.id] = p
	s.writer.addWriter(p.id, p, true)
	p.ctx.addCloser(p)
	go func() {
		written, err := io.Copy(s.term.pty, p)
		p.ctx.Infof("channel to shell copy closed, bytes written: %v, err: %v",
			written, err)
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

const partyTTL = 10 * time.Second

func (s *session) join(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) (*party, error) {
	p := newParty(s, sconn, ch, ctx)
	s.addParty(p)
	return p, nil
}

type joinSubsys struct {
	srv *Server
	sid string
}

func parseJoinSubsys(name string, srv *Server) (*joinSubsys, error) {
	return &joinSubsys{
		srv: srv,
		sid: strings.TrimPrefix(name, "join:"),
	}, nil
}

func (j *joinSubsys) String() string {
	return fmt.Sprintf("joinSubsys(sid=%v)", j.sid)
}

func (j *joinSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	if err := j.srv.reg.joinShell(j.sid, sconn, ch, req, ctx); err != nil {
		return trace.Wrap(err)
	}
	finished := make(chan bool)
	ctx.addCloser(closerFunc(func() error {
		close(finished)
		ctx.Infof("shutting down subsystem")
		return nil
	}))
	<-finished
	return nil
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
		user:       ctx.teleportUser,
		serverAddr: s.registry.srv.addr.Addr,
		site:       sconn.RemoteAddr().String(),
		id:         uuid.New(),
		sconn:      sconn,
		ch:         ch,
		ctx:        ctx,
		s:          s,
		closeC:     make(chan bool),
	}
}

type party struct {
	sync.Mutex
	user       string
	serverAddr string
	site       string
	id         string
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
	p.ctx.Infof("closing")
	close(p.closeC)
	return p.s.registry.leaveShell(p.s.id, p.id)
}

func newChunkWriter(sessionID string, rec recorder.Recorder, serverAddr string) (*chunkWriter, error) {
	cw, err := rec.GetChunkWriter(sessionID)
	if err != nil {
		return nil, err
	}
	return &chunkWriter{
		w:          cw,
		rid:        sessionID,
		serverAddr: serverAddr,
	}, nil
}

type chunkWriter struct {
	before     time.Time
	rid        string
	w          recorder.ChunkWriteCloser
	serverAddr string
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
	cs := []recorder.Chunk{
		recorder.Chunk{Delay: diff, Data: b, ServerAddr: l.serverAddr},
	}
	if err := l.w.WriteChunks(cs); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (l *chunkWriter) Close() error {
	return l.w.Close()
}

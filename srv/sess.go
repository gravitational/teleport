package srv

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/events"
	"github.com/gravitational/teleport/recorder"
	rsession "github.com/gravitational/teleport/session"

	"github.com/gravitational/teleport/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type sessionRegistry struct {
	sync.Mutex
	sessions map[string]*session
	srv      *Server
}

func (s *sessionRegistry) newShell(sid string, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v newShell(%v)", ctx, string(req.Payload))

	sess := newSession(sid, s)
	if err := sess.start(sconn, ch, ctx); err != nil {
		return err
	}
	s.sessions[sess.id] = sess
	log.Infof("%v created session: %v", ctx, sess.id)
	return nil
}

func (s *sessionRegistry) joinShell(sid string, sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v joinShell(%v)", ctx, string(req.Payload))
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		log.Infof("%v creating new session: %v", ctx, sid)
		return s.newShell(sid, sconn, ch, req, ctx)
	}
	log.Infof("%v joining session: %v", ctx, sess.id)
	sess.join(sconn, ch, req, ctx)
	return nil
}

func (s *sessionRegistry) leaveShell(sid, pid string) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return fmt.Errorf("session %v not found", sid)
	}
	if err := sess.leave(pid); err != nil {
		log.Errorf("failed to leave session: %v", err)
		return err
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

func (s *sessionRegistry) broadcastResult(sid string, r execResult) error {
	s.Lock()
	defer s.Unlock()

	sess, found := s.findSession(sid)
	if !found {
		return fmt.Errorf("session %v not found", sid)
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

func newSessionRegistry(srv *Server) *sessionRegistry {
	return &sessionRegistry{
		srv:      srv,
		sessions: make(map[string]*session),
	}
}

type session struct {
	id      string
	eid     lunk.EventID
	r       *sessionRegistry
	writer  *multiWriter
	parties map[string]*party
	t       *term
	cw      *chunkWriter
	closeC  chan bool
}

func newSession(id string, r *sessionRegistry) *session {
	return &session{
		id:      id,
		r:       r,
		parties: make(map[string]*party),
		writer:  newMultiWriter(),
	}
}

func (s *session) Close() error {
	var err error
	if s.t != nil {
		err = s.t.Close()
	}
	if s.cw != nil {
		err = s.cw.Close()
	}
	return err
}

func (s *session) upsertSessionParty(sid string, p *party, ttl time.Duration) error {
	if s.r.srv.se == nil {
		return nil
	}
	return s.r.srv.se.UpsertParty(sid, rsession.Party{
		ID:         p.id,
		User:       p.user,
		ServerAddr: p.serverAddr,
		Site:       p.site,
		LastActive: p.getLastActive(),
	}, ttl)
}

func (s *session) start(sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) error {
	s.eid = ctx.eid
	p := newParty(s, sconn, ch, ctx)
	if p.ctx.getTerm() != nil {
		s.t = p.ctx.getTerm()
		p.ctx.setTerm(nil)
	} else {
		var err error
		if s.t, err = newTerm(); err != nil {
			log.Infof("handleShell failed to create term: %v", err)
			return err
		}
	}
	cmd := exec.Command(s.r.srv.shell)
	// TODO(klizhentas) figure out linux user policy for launching shells,
	// what user and environment should we use to execute the shell? the simplest
	// answer is to use current user and env, however  what if we are root?
	cmd.Env = []string{"TERM=xterm", fmt.Sprintf("HOME=%v", os.Getenv("HOME"))}
	if err := s.t.run(cmd); err != nil {
		log.Infof("%v failed to start shell: %v", p.ctx, err)
		return err
	}
	log.Infof("%v starting shell input/output streaming", p.ctx)

	if s.r.srv.rec != nil {
		w, err := newChunkWriter(s.r.srv.rec)
		if err != nil {
			log.Errorf("failed to create recorder: %v", err)
			return err
		}
		s.cw = w
		s.r.srv.emit(ctx.eid, events.NewShellSession(s.id, sconn, s.r.srv.shell, w.rid))
		s.writer.addWriter("capture", w)
	} else {
		s.r.srv.emit(ctx.eid, events.NewShellSession(s.id, sconn, s.r.srv.shell, ""))
	}
	s.addParty(p)

	// Pipe session to shell and visa-versa capturing input and output
	go func() {
		written, err := io.Copy(s.writer, s.t.pty)
		log.Infof("%v shell to channel copy closed, bytes written: %v, err: %v",
			p.ctx, written, err)
	}()

	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		if err != nil {
			log.Errorf("%v wait failed: %v", p.ctx, err)
		}
		if result != nil {
			s.r.broadcastResult(s.id, *result)
			log.Infof("%v result broadcasted", p.ctx)
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
		return fmt.Errorf("failed to find party: %v", id)
	}
	log.Infof("%v is leaving %v", p, s)
	delete(s.parties, p.id)
	s.writer.deleteWriter(p.id)
	return nil
}

func (s *session) addParty(p *party) {
	s.parties[p.id] = p
	s.writer.addWriter(p.id, p)
	p.ctx.addCloser(p)
	go func() {
		written, err := io.Copy(s.t.pty, p)
		log.Infof("%v channel to shell copy closed, bytes written: %v, err: %v",
			p.ctx, written, err)
	}()
	go func() {
		for {
			select {
			case <-p.closeC:
				log.Infof("%v closed, stopped heartbeat", p)
				return
			case <-time.After(1 * time.Second):
			}
			if err := s.upsertSessionParty(s.id, p, 10*time.Second); err != nil {
				log.Warningf("%v failed to upsert session party: %v", p, err)
			}
			log.Infof("%v upserted session party", p)
		}
	}()
}

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
		log.Errorf("error: %v", err)
		return err
	}
	finished := make(chan bool)
	ctx.addCloser(closerFunc(func() error {
		close(finished)
		log.Infof("%v shutting down subsystem", ctx)
		return nil
	}))
	<-finished
	return nil
}

func newMultiWriter() *multiWriter {
	return &multiWriter{writers: make(map[string]io.Writer)}
}

type multiWriter struct {
	sync.RWMutex
	writers map[string]io.Writer
}

func (m *multiWriter) addWriter(id string, w io.Writer) {
	m.Lock()
	defer m.Unlock()
	m.writers[id] = w
}

func (m *multiWriter) deleteWriter(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.writers, id)
}

func (t *multiWriter) Write(p []byte) (n int, err error) {
	t.RLock()
	defer t.RUnlock()

	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
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
		user:       sconn.User(),
		serverAddr: s.r.srv.addr.Addr,
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
	log.Infof("%v closing", p)
	close(p.closeC)
	return p.s.r.leaveShell(p.s.id, p.id)
}

func newChunkWriter(rec recorder.Recorder) (*chunkWriter, error) {
	id := uuid.New()
	cw, err := rec.GetChunkWriter(id)
	if err != nil {
		return nil, err
	}
	return &chunkWriter{
		w:   cw,
		rid: id,
	}, nil
}

type chunkWriter struct {
	before time.Time
	rid    string
	w      recorder.ChunkWriteCloser
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
	cs := []recorder.Chunk{recorder.Chunk{Delay: diff, Data: b}}
	if err := l.w.WriteChunks(cs); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (l *chunkWriter) Close() error {
	return l.w.Close()
}

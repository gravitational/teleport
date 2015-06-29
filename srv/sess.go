package srv

import (
	"bytes"

	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"code.google.com/p/go-uuid/uuid"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/events"
)

type shellSession struct {
	sync.Mutex
	id      string
	eid     lunk.EventID
	s       *Server
	writer  *multiWriter
	parties map[string]*shellParty
	t       *term
}

func newShellSession(id string, s *Server) *shellSession {
	return &shellSession{
		id:      id,
		s:       s,
		parties: make(map[string]*shellParty),
		writer:  newMultiWriter(),
	}
}

func (s *shellSession) Close() error {
	if s.t != nil {
		return s.t.Close()
	}
	return nil
}

func (s *shellSession) start(sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) error {
	s.eid = ctx.eid
	p := newShellParty(s, sconn, ch, ctx)
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
	cmd := exec.Command(s.s.shell)
	// TODO(klizhentas) figure out linux user policy for launching shells,
	// what user and environment should we use to execute the shell? the simplest
	// answer is to use current user and env, however  what if we are root?
	cmd.Env = []string{"TERM=xterm", fmt.Sprintf("HOME=%v", os.Getenv("HOME"))}
	if err := s.t.run(cmd); err != nil {
		log.Infof("%v failed to start shell: %v", p.ctx, err)
		return err
	}
	log.Infof("%v starting shell input/output streaming", p.ctx)

	// Pipe session to shell and visa-versa capturing input and output
	out := &bytes.Buffer{}

	// TODO(klizhentas) implement capturing as a thread safe factored out feature
	// what is important is that writes and reads to buffer should be protected
	// out contains captured command output
	s.writer.addWriter("capture", out)

	s.addParty(p)

	go func() {
		written, err := io.Copy(s.writer, s.t.pty)
		log.Infof("%v shell to channel copy closed, bytes written: %v, err: %v",
			p.ctx, written, err)
	}()

	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		if err != nil {
			log.Errorf("%v wait failed: %v", p.ctx, err)
			s.s.emit(ctx.eid, events.NewShell(sconn, s.s.shell, out, -1, err))
		}
		if result != nil {
			log.Infof("%v result collected: %v", p.ctx, result)
			s.s.emit(ctx.eid, events.NewShell(sconn, s.s.shell, out, result.code, nil))
			s.broadcastResult(*result)
		}
	}()

	return nil
}

func (s *shellSession) broadcastResult(r execResult) {
	s.Lock()
	defer s.Unlock()
	for _, p := range s.parties {
		p.ctx.sendResult(r)
	}
}

func (s *shellSession) String() string {
	return fmt.Sprintf("shellSession(id=%v, parties=%v)", s.id, len(s.parties))
}

func (s *shellSession) leave(id string) error {
	s.Lock()
	defer s.Unlock()

	p, ok := s.parties[id]
	if !ok {
		return fmt.Errorf("failed to find party: %v", id)
	}
	log.Infof("%v is leaving %v", p, s.id)
	delete(s.parties, p.id)
	s.writer.deleteWriter(p.id)
	if len(s.parties) != 0 {
		return nil
	}
	log.Infof("%v last party left, removing from server", s)
	s.s.removeShell(s.id)
	if err := s.Close(); err != nil {
		log.Errorf("failed to close: %v", s.t)
		return err
	}
	return nil
}

func (s *shellSession) addParty(p *shellParty) {
	s.Lock()
	defer s.Unlock()
	s.parties[p.id] = p
	s.writer.addWriter(p.id, p)
	p.ctx.addCloser(p)
	go func() {
		written, err := io.Copy(s.t.pty, p.ch)
		log.Infof("%v channel to shell copy closed, bytes written: %v, err: %v",
			p.ctx, written, err)
	}()
}

func (s *shellSession) join(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) (*shellParty, error) {
	p := newShellParty(s, sconn, ch, ctx)
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
	if err := j.srv.joinShell(j.sid, sconn, ch, req, ctx); err != nil {
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

func newShellParty(s *shellSession, sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) *shellParty {
	return &shellParty{
		id:    uuid.New(),
		sconn: sconn,
		ch:    ch,
		ctx:   ctx,
		s:     s,
	}
}

type shellParty struct {
	id    string
	s     *shellSession
	sconn *ssh.ServerConn
	ch    ssh.Channel
	ctx   *ctx
}

func (p *shellParty) Write(bytes []byte) (int, error) {
	return p.ch.Write(bytes)
}

func (p *shellParty) String() string {
	return fmt.Sprintf("%v party(id=%v)", p.ctx, p.id)
}

func (p *shellParty) Close() error {
	log.Infof("%v closing", p)
	return p.s.leave(p.id)
}

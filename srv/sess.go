package srv

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/events"
)

type shellSession struct {
	id     string
	s      *Server
	sconn  *ssh.ServerConn
	ch     ssh.Channel
	ctx    *ctx
	writer *multiWriter
}

func newShellSession(
	s *Server, sconn *ssh.ServerConn, ch ssh.Channel, ctx *ctx) *shellSession {
	return &shellSession{
		id:     uuid.New(),
		s:      s,
		sconn:  sconn,
		ch:     ch,
		ctx:    ctx,
		writer: &multiWriter{},
	}
}

func (s *shellSession) Start() error {
	if s.ctx.getTerm() == nil {
		t, err := newTerm()
		if err != nil {
			log.Infof("handleShell failed to create term: %v", err)
			return err
		}
		s.ctx.setTerm(t)
	}
	t := s.ctx.getTerm()
	cmd := exec.Command(s.s.shell)
	// TODO(klizhentas) figure out linux user policy for launching shells,
	// what user and environment should we use to execute the shell? the simplest
	// answer is to use current user and env, however  what if we are root?
	cmd.Env = []string{"TERM=xterm", fmt.Sprintf("HOME=%v", os.Getenv("HOME"))}
	if err := t.run(cmd); err != nil {
		log.Infof("%v failed to start shell: %v", s.ctx, err)
		return err
	}
	log.Infof("%v starting shell input/output streaming", s.ctx)

	// Pipe session to shell and visa-versa capturing input and output

	out := &bytes.Buffer{}
	// TODO(klizhentas) implement capturing as a thread safe factored out feature
	// what is important is that writes and reads to buffer should be protected
	// out contains captured command output
	s.writer.addWriters(s.ch, out)

	go func() {
		written, err := io.Copy(s.writer, t.pty)
		log.Infof("%v shell to channel copy closed, bytes written: %v, err: %v",
			s.ctx, written, err)
	}()
	go func() {
		written, err := io.Copy(t.pty, s.ch)
		log.Infof("%v channel to shell copy closed, bytes written: %v, err: %v",
			s.ctx, written, err)
	}()
	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		if err != nil {
			log.Errorf("%v wait failed: %v", s.ctx, err)
			s.ctx.emit(events.NewShell(s.sconn, s.s.shell, out, -1, err))
		}
		if result != nil {
			log.Infof("%v result collected: %v", s.ctx, result)
			s.ctx.emit(events.NewShell(s.sconn, s.s.shell, out, result.code, nil))
			s.ctx.sendResult(*result)
		}
	}()
	return nil
}

func (s *shellSession) Join(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	t := s.ctx.getTerm()
	go func() {
		written, err := io.Copy(t.pty, ch)
		log.Infof("%v channel to shell copy closed, bytes written: %v, err: %v",
			s.ctx, written, err)
	}()
	s.writer.addWriters(ch)
	return nil
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
	if err := j.srv.joinShell(sconn, ch, req, ctx, j.sid); err != nil {
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

type multiWriter struct {
	sync.Mutex
	writers []io.Writer
}

func (t *multiWriter) addWriters(w ...io.Writer) {
	t.Lock()
	defer t.Unlock()
	t.writers = append(t.writers, w...)
}

func (t *multiWriter) Write(p []byte) (n int, err error) {
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

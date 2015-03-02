package srv

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/kr/pty"
	"github.com/mailgun/log"
	"golang.org/x/crypto/ssh"
)

// term provides handy functions for managing PTY, usch as resizing windows
// execing processes with PTY and cleaning up
type term struct {
	pty  *os.File
	tty  *os.File
	err  error
	done bool
}

type ptyReq struct {
	Env   string
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
}

type winChangeReq struct {
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
}

func parsePTYReq(req *ssh.Request) (*ptyReq, error) {
	var r ptyReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Infof("failed to parse PTY request: %v", err)
		return nil, err
	}
	return &r, nil
}

func newTerm() (*term, error) {
	// Create new PTY
	pty, tty, err := pty.Open()
	if err != nil {
		log.Infof("could not start pty (%s)", err)
		return nil, err
	}
	return &term{pty: pty, tty: tty, err: err}, nil
}

func reqPTY(req *ssh.Request) (*term, error) {
	var r ptyReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Infof("failed to parse PTY request: %v", err)
		return nil, err
	}
	log.Infof("Parsed pty request pty(enn=%v, w=%v, h=%v)", r.Env, r.W, r.H)
	t, err := newTerm()
	if err != nil {
		log.Infof("failed to create term: %v", err)
		return nil, err
	}
	t.setWinsize(r.W, r.H)
	return t, nil
}

func (t *term) reqWinChange(req *ssh.Request) error {
	var r winChangeReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		log.Infof("failed to parse window change request: %v", err)
		return err
	}
	log.Infof("Parsed window change request: %#v", r)
	t.setWinsize(r.W, r.H)
	return nil
}

func (t *term) setWinsize(w, h uint32) {
	log.Infof("window resize %dx%d", w, h)
	ws := &winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, t.pty.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

func (t *term) closeTTY() {
	if err := t.tty.Close(); err != nil {
		log.Infof("failed to close TTY: %v", err)
	}
	t.tty = nil
}

func (t *term) run(c *exec.Cmd) error {
	defer t.closeTTY()
	c.Stdout = t.tty
	c.Stdin = t.tty
	c.Stderr = t.tty
	c.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}
	return c.Start()
}

func (t *term) Close() error {
	var err error
	if e := t.pty.Close(); e != nil {
		err = e
	}
	if t.tty != nil {
		if e := t.tty.Close(); e != nil {
			err = e
		}
	}
	return err
}

// winsize stores the Height and Width of a terminal.
type winsize struct {
	Height uint16
	Width  uint16
	x      uint16
	y      uint16
}

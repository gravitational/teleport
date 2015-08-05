package srv

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/events"
)

type execResult struct {
	command string
	code    int
}

type execReq struct {
	Command string
}

type execFn struct {
	cmdName string
	cmd     *exec.Cmd
	ctx     *ctx

	// TODO(klizhentas) implement capturing as a thread safe factored out feature
	// what is important is that writes and reads to buffer should be protected
	// out contains captured command output
	out *bytes.Buffer
}

func parseExecRequest(req *ssh.Request, ctx *ctx) (*execFn, error) {
	var e execReq
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		return nil, fmt.Errorf("failed to parse exec request, error: %v", err)
	}
	return &execFn{
		ctx:     ctx,
		cmdName: e.Command,
		out:     &bytes.Buffer{},
	}, nil
}

func (e *execFn) String() string {
	return fmt.Sprintf("Exec(cmd=%v)", e.cmdName)
}

// execute is a blocking execution of a command
func (e *execFn) start(shell string, ch ssh.Channel) (*execResult, error) {
	e.cmd = exec.Command(shell, []string{"-c", e.cmdName}...)
	// we capture output to the buffer
	e.cmd.Stdout = io.MultiWriter(e.out, ch)
	e.cmd.Stderr = io.MultiWriter(e.out, ch)
	// TODO(klizhentas) figure out the way to see if stdin is ever needed.
	// e.cmd.Stdin = ch leads to the following problem:
	// e.cmd.Wait() never returns  because stdin never gets closed or never reached
	// see cmd.Stdin comments
	e.cmd.Stdin = nil
	if err := e.cmd.Start(); err != nil {
		log.Infof("%v %v start failure err: %v", e.ctx, e, err)
		return e.collectStatus(e.cmd, err)
	}
	log.Infof("%v %v started", e.ctx, e)
	return nil, nil
}

func (e *execFn) collectStatus(cmd *exec.Cmd, err error) (*execResult, error) {
	status, err := collectStatus(e.cmd, err)
	if err != nil {
		e.ctx.emit(events.NewExec(e.cmdName, e.out, -1, err))
	} else {
		e.ctx.emit(events.NewExec(e.cmdName, e.out, status.code, err))
	}
	return status, err
}

func (e *execFn) wait() (*execResult, error) {
	if e.cmd.Process == nil {
		log.Errorf("%v no process", e)
	}
	return e.collectStatus(e.cmd, e.cmd.Wait())
}

func collectStatus(cmd *exec.Cmd, err error) (*execResult, error) {
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			return &execResult{code: status.ExitStatus(), command: cmd.Path}, nil
		}
		return nil, err
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return nil, fmt.Errorf("unspoorted exit status: %T(%v)", cmd.ProcessState.Sys(), cmd.ProcessState.Sys())
	}
	return &execResult{code: status.ExitStatus(), command: cmd.Path}, nil
}

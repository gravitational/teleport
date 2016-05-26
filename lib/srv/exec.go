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
	"net"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/kardianos/osext"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// execResult is used internally to send the result of a command execution from
// a goroutine to SSH request handler and back to the calling client
type execResult struct {
	command string

	// returned exec code
	code int

	// stderr output
	stderr []byte
}

type execReq struct {
	Command string
}

// execResponse prepares the response to a 'exec' SSH request, i.e. executing
// a command after making an SSH connection and delivering the result back.
type execResponse struct {
	cmdName string
	cmd     *exec.Cmd
	ctx     *ctx
	isSCP   bool
}

// parseExecRequest parses SSH exec request
func parseExecRequest(req *ssh.Request, ctx *ctx) (*execResponse, error) {
	var e execReq
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		return nil, fmt.Errorf("failed to parse exec request, error: %v", err)
	}
	// is this scp request?
	isSCP := false
	args := strings.Split(e.Command, " ")
	if len(args) > 0 {
		_, f := filepath.Split(args[0])
		if f == "scp" {
			isSCP = true
			// for 'scp' requests, we'll fork ourselves with scp parameters:
			teleportBin, err := osext.Executable()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			e.Command = fmt.Sprintf("%s scp --remote-addr=%s --local-addr=%s %v",
				teleportBin,
				ctx.conn.RemoteAddr().String(),
				ctx.conn.LocalAddr().String(),
				strings.Join(args[1:], " "))
		}
	}
	return &execResponse{
		ctx:     ctx,
		cmdName: e.Command,
		isSCP:   isSCP,
	}, nil
}

func (e *execResponse) String() string {
	return fmt.Sprintf("Exec(cmd=%v)", e.cmdName)
}

// prepareShell configures exec.Cmd object for launching shell for an SSH user
func prepareShell(ctx *ctx) (*exec.Cmd, error) {
	// determine shell for the given OS user:
	shellCommand, err := utils.GetLoginShell(ctx.login)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	// in test mode short-circuit to /bin/sh
	if ctx.isTestStub {
		shellCommand = "/bin/sh"
	}
	c, err := prepareCommand(ctx, shellCommand)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this configures shell to run in 'login' mode
	c.Args[0] = "-" + filepath.Base(shellCommand)
	return c, nil
}

// prepareCommand configures exec.Cmd for executing a given command within an SSH
// session.
//
func prepareCommand(ctx *ctx, args ...string) (*exec.Cmd, error) {
	osUserName := ctx.login
	// configure UID & GID of the requested OS user:
	osUser, err := user.Lookup(osUserName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uid, err := strconv.Atoi(osUser.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gid, err := strconv.Atoi(osUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// try to determine the host name of the 1st available proxy to set a nicer
	// session URL. fall back to <proxyhost> placeholder
	proxyHost := "<proxyhost>"
	if ctx.srv != nil {
		proxies, err := ctx.srv.authService.GetProxies()
		if err != nil {
			log.Error(err)
		}
		if len(proxies) > 0 {
			proxyHost = proxies[0].Hostname
		}
	}
	var c *exec.Cmd
	if len(args) == 1 {
		c = exec.Command(args[0])
	} else {
		c = exec.Command(args[0], args[1:]...)
	}
	c.Env = []string{
		"TERM=xterm",
		"LANG=en_US.UTF-8",
		"HOME=" + osUser.HomeDir,
		"USER=" + osUserName,
		"SSH_TELEPORT_USER=" + ctx.teleportUser,
		fmt.Sprintf("SSH_SESSION_WEBPROXY_ADDR=%s:3080", proxyHost),
	}
	shell, err := utils.GetLoginShell(ctx.login)
	if err != nil {
		log.Warn(err)
	} else {
		if ctx.isTestStub {
			shell = "/bin/sh"
		}
		c.Env = append(c.Env, "SHELL="+shell)
	}
	c.Dir = osUser.HomeDir
	c.SysProcAttr = &syscall.SysProcAttr{}

	// execute the command under requested user's UID:GID
	c.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

	// apply environment variables passed from the client
	for n, v := range ctx.env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", n, v))
	}
	// apply SSH_xx environment variables
	remoteHost, remotePort, err := net.SplitHostPort(ctx.conn.RemoteAddr().String())
	if err != nil {
		log.Warn(err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.conn.LocalAddr().String())
		if err != nil {
			log.Warn(err)
		} else {
			c.Env = append(c.Env,
				fmt.Sprintf("SSH_CLIENT=%s %s %s", remoteHost, remotePort, localPort),
				fmt.Sprintf("SSH_CONNECTION=%s %s %s %s", remoteHost, remotePort, localHost, localPort))
		}
	}
	if ctx.session != nil {
		if ctx.session.term != nil {
			c.Env = append(c.Env, fmt.Sprintf("SSH_TTY=%s", ctx.session.term.tty.Name()))
		}
		if ctx.session.id != "" {
			c.Env = append(c.Env, fmt.Sprintf("SSH_SESSION_ID=%s", ctx.session.id))
		}
	}
	return c, nil
}

// start launches the given command returns (nil, nil) if successful. execResult is only used
// to communicate an error while launching
func (e *execResponse) start(ch ssh.Channel) (*execResult, error) {
	var err error
	parts := strings.Split(e.cmdName, " ")
	e.cmd, err = prepareCommand(e.ctx, parts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.cmd.Stderr = ch.Stderr()
	e.cmd.Stdout = ch

	inputWriter, err := e.cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		io.Copy(inputWriter, ch)
		inputWriter.Close()
	}()

	if err := e.cmd.Start(); err != nil {
		e.ctx.Warningf("%v start failure err: %v", e, err)
		return e.collectStatus(e.cmd, trace.ConvertSystemError(err))
	}
	e.ctx.Infof("%v started", e)

	return nil, nil
}

func (e *execResponse) wait() (*execResult, error) {
	if e.cmd.Process == nil {
		e.ctx.Errorf("no process")
	}
	err := e.cmd.Wait()
	return e.collectStatus(e.cmd, err)
}

func (e *execResponse) collectStatus(cmd *exec.Cmd, err error) (*execResult, error) {
	status, err := collectStatus(e.cmd, err)
	// report the result of this exec event to the audit logger
	auditLog := e.ctx.srv.alog
	if auditLog == nil {
		return status, err
	}
	fields := events.EventFields{
		events.ExecEventCommand: strings.Join(cmd.Args, " "),
		events.EventUser:        e.ctx.teleportUser,
		events.EventLogin:       e.ctx.login,
		events.LocalAddr:        e.ctx.conn.LocalAddr().String(),
		events.RemoteAddr:       e.ctx.conn.RemoteAddr().String(),
	}
	if err != nil {
		fields[events.ExecEventError] = err.Error()
		if status != nil {
			fields[events.ExecEventCode] = strconv.Itoa(status.code)
		}
	}
	auditLog.EmitAuditEvent(events.ExecEvent, fields)
	return status, err
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
		return nil, fmt.Errorf("unknown exit status: %T(%v)", cmd.ProcessState.Sys(), cmd.ProcessState.Sys())
	}
	return &execResult{code: status.ExitStatus(), command: cmd.Path}, nil
}

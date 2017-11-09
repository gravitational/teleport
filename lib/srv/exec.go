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
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPath          = "/bin:/usr/bin:/usr/local/bin:/sbin"
	defaultEnvPath       = "PATH=" + defaultPath
	defaultTerm          = "xterm"
	defaultLoginDefsPath = "/etc/login.defs"
)

// ExecResult is used internally to send the result of a command execution from
// a goroutine to SSH request handler and back to the calling client
type ExecResult struct {
	// Command is the command that was executed.
	Command string

	// Code is return code that execution of the command resulted in.
	Code int
}

// Exec executes an "exec" request.
type Exec interface {
	// GetCommand returns the command to be executed.
	GetCommand() string

	// SetCommand sets the command to be executed.
	SetCommand(string)

	// Start will start the execution of the command.
	Start(channel ssh.Channel) (*ExecResult, error)

	// Wait will block while the command executes.
	Wait() (*ExecResult, error)
}

// NewExecRequest creates a new local or remote Exec.
func NewExecRequest(ctx *ServerContext, command string) Exec {
	return &LocalExecRequest{
		Ctx:     ctx,
		Command: command,
	}
}

// LocalExecRequest prepares the response to a 'exec' SSH request, i.e. executing
// a command after making an SSH connection and delivering the result back.
type LocalExecRequest struct {
	// Command is the command that will be executed.
	Command string

	// Cmd holds an *exec.Cmd which will be used for local execution.
	Cmd *exec.Cmd

	// Ctx holds the *ServerContext.
	Ctx *ServerContext
}

// GetCommand returns the command string.
func (e *LocalExecRequest) GetCommand() string {
	return e.Command
}

// SetCommand gets the command string.
func (e *LocalExecRequest) SetCommand(command string) {
	e.Command = command
}

// start launches the given command returns (nil, nil) if successful. execResult is only used
// to communicate an error while launching
func (e *LocalExecRequest) Start(channel ssh.Channel) (*ExecResult, error) {
	var err error

	// parse the command to see if the user is trying to run scp
	err = e.parseSecureCopy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// transforms the Command string into *exec.Cmd
	e.Cmd, err = prepareCommand(e.Ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// hook up stdout/err the channel so the user can interact with the command
	e.Cmd.Stderr = channel.Stderr()
	e.Cmd.Stdout = channel
	inputWriter, err := e.Cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		io.Copy(inputWriter, channel)
		inputWriter.Close()
	}()

	if err := e.Cmd.Start(); err != nil {
		e.Ctx.Warningf("%v start failure err: %v", e, err)
		execResult, err := collectLocalStatus(e.Cmd, trace.ConvertSystemError(err))

		// emit the result of execution to the audit log
		emitExecAuditEvent(e.Ctx, strings.Join(e.Cmd.Args, " "), execResult, err)

		return execResult, trace.Wrap(err)
	}
	e.Ctx.Infof("[LOCAL EXEC] Started command: %q", e.Command)

	return nil, nil
}

// Wait will block while the command executes.
func (e *LocalExecRequest) Wait() (*ExecResult, error) {
	if e.Cmd.Process == nil {
		e.Ctx.Errorf("no process")
	}

	// wait for the command to complete
	err := e.Cmd.Wait()
	e.Ctx.Infof("[LOCAL EXEC] Command %q complete", e.Command)

	// figure out if the command successfully exited or if it exited in failure
	execResult, err := collectLocalStatus(e.Cmd, err)

	// emit the result of execution to the audit log
	emitExecAuditEvent(e.Ctx, strings.Join(e.Cmd.Args, " "), execResult, err)

	return execResult, trace.Wrap(err)
}

func (e *LocalExecRequest) String() string {
	return fmt.Sprintf("Exec(Command=%v)", e.Command)
}

// prepareInteractiveCommand configures exec.Cmd object for launching an
// interactive command (or a shell).
func prepareInteractiveCommand(ctx *ServerContext) (*exec.Cmd, error) {
	var (
		err      error
		runShell bool
	)
	// determine shell for the given OS user:
	if ctx.ExecRequest.GetCommand() == "" {
		runShell = true
		cmdName, err := shell.GetLoginShell(ctx.Login)
		ctx.ExecRequest.SetCommand(cmdName)
		if err != nil {
			log.Error(err)
			return nil, trace.Wrap(err)
		}
		// in test mode short-circuit to /bin/sh
		if ctx.IsTestStub {
			ctx.ExecRequest.SetCommand("/bin/sh")
		}
	}
	c, err := prepareCommand(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this configures shell to run in 'login' mode. from openssh source:
	// "If we have no command, execute the shell.  In this case, the shell
	// name to be passed in argv[0] is preceded by '-' to indicate that
	// this is a login shell."
	// https://github.com/openssh/openssh-portable/blob/master/session.c
	if runShell {
		c.Args = []string{"-" + filepath.Base(ctx.ExecRequest.GetCommand())}
	}
	return c, nil
}

// prepareCommand configures exec.Cmd for executing a given command within an SSH
// session.
//
// 'cmd' is the string passed as parameter to 'ssh' command, like "ls -l /"
//
// If 'cmd' does not have any spaces in it, it gets executed directly, otherwise
// it is passed to user's shell for interpretation
func prepareCommand(ctx *ServerContext) (*exec.Cmd, error) {
	osUserName := ctx.Login
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

	// get user's shell:
	shell, err := shell.GetLoginShell(ctx.Login)
	if err != nil {
		log.Warn(err)
	}
	if ctx.IsTestStub {
		shell = "/bin/sh"
	}

	// try and get the public address from the first available proxy. if public_address
	// is not set, fall back to the hostname of the first proxy we get back.
	proxyHost := "<proxyhost>:3080"
	if ctx.srv != nil {
		proxies, err := ctx.srv.GetAuthService().GetProxies()
		if err != nil {
			log.Errorf("Unexpected response from authService.GetProxies(): %v", err)
		}

		if len(proxies) > 0 {
			proxyHost = proxies[0].GetPublicAddr()
			if proxyHost == "" {
				proxyHost = fmt.Sprintf("%v:%v", proxies[0].GetHostname(), defaults.HTTPListenPort)
				log.Debugf("public_address not set for proxy, returning proxyHost: %q", proxyHost)
			}
		}
	}

	// by default, execute command using user's shell like openssh does:
	// https://github.com/openssh/openssh-portable/blob/master/session.c
	c := exec.Command(shell, "-c", ctx.ExecRequest.GetCommand())

	clusterName, err := ctx.srv.GetAuthService().GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.Env = []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(osUser.Uid, defaultLoginDefsPath),
		"HOME=" + osUser.HomeDir,
		"USER=" + osUserName,
		"SHELL=" + shell,
		teleport.SSHTeleportUser + "=" + ctx.TeleportUser,
		teleport.SSHSessionWebproxyAddr + "=" + proxyHost,
		teleport.SSHTeleportHostUUID + "=" + ctx.srv.ID(),
		teleport.SSHTeleportClusterName + "=" + clusterName,
	}
	c.Dir = osUser.HomeDir
	c.SysProcAttr = &syscall.SysProcAttr{}

	// execute the command under requested user's UID:GID
	me, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if me.Uid != osUser.Uid || me.Gid != osUser.Gid {
		userGroups, err := osUser.GroupIds()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		groups := make([]uint32, 0)
		for _, sgid := range userGroups {
			igid, err := strconv.Atoi(sgid)
			if err != nil {
				log.Warnf("Cannot interpret user group: '%v'", sgid)
			} else {
				groups = append(groups, uint32(igid))
			}
		}
		if len(groups) == 0 {
			groups = append(groups, uint32(gid))
		}
		c.SysProcAttr.Credential = &syscall.Credential{
			Uid:    uint32(uid),
			Gid:    uint32(gid),
			Groups: groups,
		}
		c.SysProcAttr.Setsid = true
	}

	// apply environment variables passed from the client
	for n, v := range ctx.env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", n, v))
	}
	// apply SSH_xx environment variables
	remoteHost, remotePort, err := net.SplitHostPort(ctx.Conn.RemoteAddr().String())
	if err != nil {
		log.Warn(err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.Conn.LocalAddr().String())
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
			c.Env = append(c.Env, fmt.Sprintf("SSH_TTY=%s", ctx.session.term.TTY().Name()))
		}
		if ctx.session.id != "" {
			c.Env = append(c.Env, fmt.Sprintf("%s=%s", teleport.SSHSessionID, ctx.session.id))
		}
	}

	// if the server allows reading in of ~/.tsh/environment read it in
	// and pass environment variables along to new session
	if ctx.srv.PermitUserEnvironment() {
		filename := filepath.Join(osUser.HomeDir, ".tsh", "environment")
		userEnvs, err := utils.ReadEnvironmentFile(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.Env = append(c.Env, userEnvs...)
	}
	return c, nil
}

func (e *LocalExecRequest) parseSecureCopy() error {
	// split up command by space to grab the first word
	args := strings.Split(e.GetCommand(), " ")

	if len(args) > 0 {
		_, f := filepath.Split(args[0])

		// is this scp request?
		if f == "scp" {
			// for 'scp' requests, we'll launch ourselves with scp parameters:
			teleportBin, err := osext.Executable()
			if err != nil {
				return trace.Wrap(err)
			}
			e.Command = fmt.Sprintf("%s scp --remote-addr=%s --local-addr=%s %v",
				teleportBin,
				e.Ctx.Conn.RemoteAddr().String(),
				e.Ctx.Conn.LocalAddr().String(),
				strings.Join(args[1:], " "))
		}
	}

	return nil
}

func collectLocalStatus(cmd *exec.Cmd, err error) (*ExecResult, error) {
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			return &ExecResult{Code: status.ExitStatus(), Command: cmd.Path}, nil
		}
		return nil, err
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return nil, fmt.Errorf("unknown exit status: %T(%v)", cmd.ProcessState.Sys(), cmd.ProcessState.Sys())
	}
	return &ExecResult{Code: status.ExitStatus(), Command: cmd.Path}, nil
}

func emitExecAuditEvent(ctx *ServerContext, cmd string, status *ExecResult, err error) {
	// report the result of this exec event to the audit logger
	auditLog := ctx.srv.GetAuditLog()
	if auditLog == nil {
		log.Warnf("No audit log")
		return
	}
	fields := events.EventFields{
		events.ExecEventCommand: cmd,
		events.EventUser:        ctx.TeleportUser,
		events.EventLogin:       ctx.Login,
		events.LocalAddr:        ctx.Conn.LocalAddr().String(),
		events.RemoteAddr:       ctx.Conn.RemoteAddr().String(),
		events.EventNamespace:   ctx.srv.GetNamespace(),
	}
	if err != nil {
		fields[events.ExecEventError] = err.Error()
		if status != nil {
			fields[events.ExecEventCode] = strconv.Itoa(status.Code)
		}
	}
	auditLog.EmitAuditEvent(events.ExecEvent, fields)
}

// getDefaultEnvPath returns the default value of PATH environment variable for
// new logins (prior to shell) based on login.defs. Returns a strings which
// looks like "PATH=/usr/bin:/bin"
func getDefaultEnvPath(uid string, loginDefsPath string) string {
	envPath := defaultEnvPath
	envSuPath := defaultEnvPath

	// open file, if it doesn't exist return a default path and move on
	f, err := os.Open(loginDefsPath)
	if err != nil {
		log.Infof("Unable to open %q: %v: returning default path: %q", loginDefsPath, err, defaultEnvPath)
		return defaultEnvPath
	}
	defer f.Close()

	// read path to login.defs file /etc/login.defs line by line:
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip comments and empty lines:
		if line == "" || line[0] == '#' {
			continue
		}

		// look for a line that starts with ENV_SUPATH or ENV_PATH
		fields := strings.Fields(line)
		if len(fields) > 1 {
			if fields[0] == "ENV_PATH" {
				envPath = fields[1]
			}
			if fields[0] == "ENV_SUPATH" {
				envSuPath = fields[1]
			}
		}
	}

	// if any error occurs while reading the file, return the default value
	err = scanner.Err()
	if err != nil {
		log.Warnf("Unable to read %q: %v: returning default path: %q", loginDefsPath, err, defaultEnvPath)
		return defaultEnvPath
	}

	// if requesting path for uid 0 and no ENV_SUPATH is given, fallback to
	// ENV_PATH first, then the default path.
	if uid == "0" {
		if envSuPath == defaultEnvPath {
			return envPath
		}
		return envSuPath
	}
	return envPath
}

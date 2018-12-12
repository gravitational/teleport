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
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
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
func NewExecRequest(ctx *ServerContext, command string) (Exec, error) {
	// It doesn't matter what mode the cluster is in, if this is a Teleport node
	// return a local *localExec.
	if ctx.srv.Component() == teleport.ComponentNode {
		return &localExec{
			Ctx:     ctx,
			Command: command,
		}, nil
	}

	// When in recording mode, return an *remoteExec which will execute the
	// command on a remote host. This is used by in-memory forwarding nodes.
	if ctx.ClusterConfig.GetSessionRecording() == services.RecordAtProxy {
		return &remoteExec{
			ctx:     ctx,
			command: command,
			session: ctx.RemoteSession,
		}, nil
	}

	// Otherwise return a *localExec which will execute locally on the server.
	// used by the regular Teleport nodes.
	return &localExec{
		Ctx:     ctx,
		Command: command,
	}, nil
}

// localExec prepares the response to a 'exec' SSH request, i.e. executing
// a command after making an SSH connection and delivering the result back.
type localExec struct {
	// Command is the command that will be executed.
	Command string

	// Cmd holds an *exec.Cmd which will be used for local execution.
	Cmd *exec.Cmd

	// Ctx holds the *ServerContext.
	Ctx *ServerContext
}

// GetCommand returns the command string.
func (e *localExec) GetCommand() string {
	return e.Command
}

// SetCommand gets the command string.
func (e *localExec) SetCommand(command string) {
	e.Command = command
}

// Start launches the given command returns (nil, nil) if successful.
// ExecResult is only used to communicate an error while launching.
func (e *localExec) Start(channel ssh.Channel) (*ExecResult, error) {
	var err error

	// parse the command to see if the user is trying to run scp
	err = e.transformSecureCopy()
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
		// copy from the channel (client) into stdin of the process
		io.Copy(inputWriter, channel)
		inputWriter.Close()
	}()

	if err := e.Cmd.Start(); err != nil {
		e.Ctx.Warningf("%v start failure err: %v", e, err)
		execResult, err := collectLocalStatus(e.Cmd, trace.ConvertSystemError(err))

		// emit the result of execution to the audit log
		emitExecAuditEvent(e.Ctx, e.GetCommand(), execResult, err)

		return execResult, trace.Wrap(err)
	}
	e.Ctx.Infof("[LOCAL EXEC] Started command: %q", e.Command)

	return nil, nil
}

// Wait will block while the command executes.
func (e *localExec) Wait() (*ExecResult, error) {
	if e.Cmd.Process == nil {
		e.Ctx.Errorf("no process")
	}

	// wait for the command to complete, then figure out if the command
	// successfully exited or if it exited in failure
	execResult, err := collectLocalStatus(e.Cmd, e.Cmd.Wait())

	// emit the result of execution to the audit log
	emitExecAuditEvent(e.Ctx, e.GetCommand(), execResult, err)

	return execResult, trace.Wrap(err)
}

func (e *localExec) String() string {
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
		cmdName, err := shell.GetLoginShell(ctx.Identity.Login)
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

func (e *localExec) transformSecureCopy() error {
	// split up command by space to grab the first word. if we don't have anything
	// it's an interactive shell the user requested and not scp, return
	args := strings.Split(e.GetCommand(), " ")
	if len(args) == 0 {
		return nil
	}

	// see the user is not requesting scp, return
	_, f := filepath.Split(args[0])
	if f != teleport.SCP {
		return nil
	}

	// for scp requests update the command to execute to launch teleport with
	// scp parameters just like openssh does.
	teleportBin, err := osext.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	e.Command = fmt.Sprintf("%s scp --remote-addr=%s --local-addr=%s %v",
		teleportBin,
		e.Ctx.Conn.RemoteAddr().String(),
		e.Ctx.Conn.LocalAddr().String(),
		strings.Join(args[1:], " "))

	return nil
}

// prepareCommand configures exec.Cmd for executing a given command within an SSH
// session.
//
// 'cmd' is the string passed as parameter to 'ssh' command, like "ls -l /"
//
// If 'cmd' does not have any spaces in it, it gets executed directly, otherwise
// it is passed to user's shell for interpretation
func prepareCommand(ctx *ServerContext) (*exec.Cmd, error) {
	osUserName := ctx.Identity.Login
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
	shell, err := shell.GetLoginShell(ctx.Identity.Login)
	if err != nil {
		log.Warn(err)
	}
	if ctx.IsTestStub {
		shell = "/bin/sh"
	}

	// by default, execute command using user's shell like openssh does:
	// https://github.com/openssh/openssh-portable/blob/master/session.c
	c := exec.Command(shell, "-c", ctx.ExecRequest.GetCommand())

	clusterName, err := ctx.srv.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.Env = []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(osUser.Uid, defaultLoginDefsPath),
		"HOME=" + osUser.HomeDir,
		"USER=" + osUserName,
		"SHELL=" + shell,
		teleport.SSHTeleportUser + "=" + ctx.Identity.TeleportUser,
		teleport.SSHSessionWebproxyAddr + "=" + ctx.ProxyPublicAddress(),
		teleport.SSHTeleportHostUUID + "=" + ctx.srv.ID(),
		teleport.SSHTeleportClusterName + "=" + clusterName.GetClusterName(),
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
	// if a terminal was allocated, apply terminal type variable
	if ctx.session != nil {
		c.Env = append(c.Env, fmt.Sprintf("TERM=%v", ctx.session.term.GetTermType()))
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

// remoteExec is used to run an "exec" SSH request and return the result.
type remoteExec struct {
	command string
	session *ssh.Session
	ctx     *ServerContext
}

// GetCommand returns the command string.
func (e *remoteExec) GetCommand() string {
	return e.command
}

// SetCommand gets the command string.
func (e *remoteExec) SetCommand(command string) {
	e.command = command
}

// Start launches the given command returns (nil, nil) if successful.
// ExecResult is only used to communicate an error while launching.
func (r *remoteExec) Start(ch ssh.Channel) (*ExecResult, error) {
	// hook up stdout/err the channel so the user can interact with the command
	r.session.Stdout = ch
	r.session.Stderr = ch.Stderr()
	inputWriter, err := r.session.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		// copy from the channel (client) into stdin of the process
		io.Copy(inputWriter, ch)
		inputWriter.Close()
	}()

	err = r.session.Start(r.command)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// Wait will block while the command executes then return the result as well
// as emit an event to the Audit Log.
func (r *remoteExec) Wait() (*ExecResult, error) {
	// block until the command is finished and then figure out if the command
	// successfully exited or if it exited in failure
	execResult, err := r.collectRemoteStatus(r.session.Wait())

	// emit the result of execution to the audit log
	emitExecAuditEvent(r.ctx, r.command, execResult, err)

	return execResult, trace.Wrap(err)
}

func (r *remoteExec) collectRemoteStatus(err error) (*ExecResult, error) {
	if err == nil {
		return &ExecResult{
			Code:    teleport.RemoteCommandSuccess,
			Command: r.GetCommand(),
		}, nil
	}

	// if we got an ssh.ExitError, return the status code
	if exitErr, ok := err.(*ssh.ExitError); ok {
		return &ExecResult{
			Code:    exitErr.ExitStatus(),
			Command: r.GetCommand(),
		}, err
	}

	// if we don't know what type of error occurred, return a generic 255 command
	// failed code
	return &ExecResult{
		Code:    teleport.RemoteCommandFailure,
		Command: r.GetCommand(),
	}, err
}

func emitExecAuditEvent(ctx *ServerContext, cmd string, status *ExecResult, execErr error) {
	// Report the result of this exec event to the audit logger.
	auditLog := ctx.srv.GetAuditLog()
	if auditLog == nil {
		log.Warnf("No audit log")
		return
	}

	var eventType string

	// Create common fields for event.
	fields := events.EventFields{
		events.EventUser:      ctx.Identity.TeleportUser,
		events.EventLogin:     ctx.Identity.Login,
		events.LocalAddr:      ctx.Conn.LocalAddr().String(),
		events.RemoteAddr:     ctx.Conn.RemoteAddr().String(),
		events.EventNamespace: ctx.srv.GetNamespace(),
	}
	if execErr != nil {
		fields[events.ExecEventError] = execErr.Error()
		if status != nil {
			fields[events.ExecEventCode] = strconv.Itoa(status.Code)
		}
	}

	// Parse the exec command to find out if it was SCP or not.
	path, action, isSCP, err := parseSecureCopy(cmd)
	if err != nil {
		log.Warnf("Unable to emit audit event: %v.", err)
		return
	}

	// Update appropriate fields based off if the request was SCP or not.
	if isSCP {
		eventType = events.SCPEvent
		fields[events.SCPPath] = path
		fields[events.SCPAction] = action
	} else {
		eventType = events.ExecEvent
		fields[events.ExecEventCommand] = cmd
	}

	// Emit the event.
	auditLog.EmitAuditEvent(eventType, fields)
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

// parseSecureCopy will parse a command and return if it's secure copy or not.
func parseSecureCopy(path string) (string, string, bool, error) {
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return "", "", false, trace.BadParameter("no executable found")
	}

	// Look for the -t flag, it indicates that an upload occurred. The other
	// flags do no matter for now.
	action := events.SCPDownload
	if utils.SliceContainsStr(parts, "-t") {
		action = events.SCPUpload
	}

	// Exract the name of the Teleport executable on disk.
	teleportPath, err := os.Executable()
	if err != nil {
		return "", "", false, trace.Wrap(err)
	}
	_, teleportBinary := filepath.Split(teleportPath)

	// Extract the name of the executable that was run. The command was secure
	// copy if the executable was "scp" or "teleport".
	_, executable := filepath.Split(parts[0])
	switch executable {
	case teleport.SCP, teleportBinary:
		return parts[len(parts)-1], action, true, nil
	default:
		return "", "", false, nil
	}
}

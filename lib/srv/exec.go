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
	"bytes"
	"encoding/json"
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
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPath          = "/bin:/usr/bin:/usr/local/bin:/sbin"
	defaultEnvPath       = "PATH=" + defaultPath
	defaultTerm          = "xterm"
	defaultLoginDefsPath = "/etc/login.defs"
)

// execCommand contains the payload to "teleport exec" will will be used to
// construct and execute a exec.Cmd.
type execCommand struct {
	// Path the the full path to the binary to execute.
	Path string `json:"path"`

	// Args is the list of arguments to pass to the command.
	Args []string `json:"args"`

	// Env is a list of environment variables to pass to the command.
	Env []string `json:"env"`

	// Dir is the working/home directory of the command.
	Dir string `json:"dir"`

	// Uid is the UID under which to spawn the command.
	Uid uint32 `json:"uid"`

	// Gid it the GID under which to spawn the command.
	Gid uint32 `json:"gid"`

	// Groups is the list of supplementary groups.
	Groups []uint32 `json:"groups"`

	// SetCreds controls if the process credentials will be set.
	SetCreds bool `json:"set_creds"`

	// Terminal is if a TTY has been allocated for the session.
	Terminal bool `json:"term"`

	// PAM contains metadata needed to launch a PAM context.
	PAM *pamCommand `json:"pam"`
}

// pamCommand contains the payload to launch a PAM context.
type pamCommand struct {
	// Enabled indicates that PAM has been enabled on this host.
	Enabled bool `json:"enabled"`

	// ServiceName is the name service whose policy will be loaded.
	ServiceName string `json:"service_name"`

	// Username is the host login.
	Username string `json:"username"`
}

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
	Wait() *ExecResult

	// Continue will resume execution of the process after it completes its
	// pre-processing routine (placed in a cgroup).
	Continue()
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

	// sessionContext holds the BPF session context used to lookup and interact
	// with BPF sessions.
	sessionContext *bpf.SessionContext

	// contw is one end of a pipe that is used to signal to the child process
	// that it has been placed in a cgroup and it can continue.
	contw *os.File
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
	// Parse the command to see if it is scp.
	err := e.transformSecureCopy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and marshal command to execute.
	cmdmsg, err := prepareCommand(e.Ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmdbytes, err := json.Marshal(cmdmsg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a pipe used to signal to the process it's safe to continue.
	// Used to make the process wait until it's been placed in a cgroup by the
	// parent process.
	contr, contw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.contw = contw

	// Create pipe and write bytes to pipe. The child process will read the
	// command to execute from this pipe.
	cmdr, cmdw, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = io.Copy(cmdw, bytes.NewReader(cmdbytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = cmdw.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Re-execute Teleport and pass along the allocated PTY as well as the
	// command reader from where Teleport will know how to re-spawn itself.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable}
	if strings.HasSuffix(executable, ".test") {
		args = append(args, "-test.run=TestHelperProcess")
	} else {
		args = append(args, "exec")
		if log.GetLevel() == log.DebugLevel {
			args = append(args, "-d")
		}
	}

	e.Cmd = &exec.Cmd{
		Path: executable,
		Args: args,
		Dir:  cmdmsg.Dir,
		ExtraFiles: []*os.File{
			cmdr,
			contr,
		},
	}

	// Pass in environment variable that will be used by the helper function to
	// know to re-exec Teleport.
	if strings.HasSuffix(executable, ".test") {
		e.Cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	}

	// Connect stdout and stderr to the channel so the user can interact with
	// the command.
	e.Cmd.Stderr = channel.Stderr()
	e.Cmd.Stdout = channel

	// Copy from the channel (client input) into stdin of the process.
	inputWriter, err := e.Cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		io.Copy(inputWriter, channel)
		inputWriter.Close()
	}()

	// Start the command.
	err = e.Cmd.Start()
	if err != nil {
		e.Ctx.Warningf("Local command %v failed to start: %v", e.GetCommand(), err)

		// Emit the result of execution to the audit log
		emitExecAuditEvent(e.Ctx, e.GetCommand(), err)

		return &ExecResult{
			Command: e.GetCommand(),
			Code:    exitCode(err),
		}, trace.ConvertSystemError(err)
	}

	e.Ctx.Infof("Started local command execution: %q", e.Command)

	return nil, nil
}

// Wait will block while the command executes.
func (e *localExec) Wait() *ExecResult {
	if e.Cmd.Process == nil {
		e.Ctx.Errorf("no process")
	}

	// Block until the command is finished executing.
	err := e.Cmd.Wait()
	if err != nil {
		e.Ctx.Debugf("Local command failed: %v.", err)
	} else {
		e.Ctx.Debugf("Local command successfully executed.")
	}

	// Emit the result of execution to the Audit Log.
	emitExecAuditEvent(e.Ctx, e.GetCommand(), err)

	execResult := &ExecResult{
		Command: e.GetCommand(),
		Code:    exitCode(err),
	}

	return execResult
}

// Continue will resume execution of the process after it completes its
// pre-processing routine (placed in a cgroup).
func (e *localExec) Continue() {
	e.contw.Close()
}

func (e *localExec) String() string {
	return fmt.Sprintf("Exec(Command=%v)", e.Command)
}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command.
func RunCommand() (int, error) {
	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(uintptr(3), "/proc/self/fd/3")
	if cmdfd == nil {
		return teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}
	contfd := os.NewFile(uintptr(4), "/proc/self/fd/4")
	if cmdfd == nil {
		return teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
	}

	// Read in the command payload.
	var b bytes.Buffer
	_, err := b.ReadFrom(cmdfd)
	if err != nil {
		return teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	var c execCommand
	err = json.Unmarshal(b.Bytes(), &c)
	if err != nil {
		return teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	cmd := exec.Cmd{
		Path: c.Path,
		Args: c.Args,
		Dir:  c.Dir,
		Env:  c.Env,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid: true,
		},
	}

	// If a terminal was requested, file descriptor 4 and 5 always point to the
	// PTY and TTY. Extract them and set the controlling TTY. Otherwise, connect
	// std{in,out,err} directly.
	if c.Terminal {
		pty := os.NewFile(uintptr(5), "/proc/self/fd/5")
		tty := os.NewFile(uintptr(6), "/proc/self/fd/6")
		if pty == nil || tty == nil {
			return teleport.RemoteCommandFailure, trace.BadParameter("pty and tty not found")
		}

		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty

		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Ctty = int(tty.Fd())
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Only set process credentials if requested. See comment in the
	// "prepareCommand" function for more details.
	if c.SetCreds {
		cmd.SysProcAttr.Credential = &syscall.Credential{
			Uid:    c.Uid,
			Gid:    c.Gid,
			Groups: c.Groups,
		}
	}

	// Reading from the continue file descriptor will block until it's closed. It
	// won't be closed until the parent has placed it in a cgroup.
	var r bytes.Buffer
	r.ReadFrom(contfd)

	// If PAM is enabled, open a PAM context.
	var pamContext *pam.PAM
	if c.PAM.Enabled {
		pamContext, err = pam.Open(&pam.Config{
			ServiceName: c.PAM.ServiceName,
			Username:    c.PAM.Username,
			Stdin:       os.Stdin,
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
		})
		if err != nil {
			return teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()
	}

	// Start the command.
	err = cmd.Start()
	if err != nil {
		return teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait for it to exit.
	err = cmd.Wait()
	if err != nil {
		return exitCode(err), trace.Wrap(err)
	}
	return exitCode(err), nil
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
	teleportBin, err := os.Executable()
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

// prepareCommand prepares a command execution payload.
func prepareCommand(ctx *ServerContext) (*execCommand, error) {
	var c execCommand

	// Get the login shell for the user (or fallback to the default).
	shellPath, err := shell.GetLoginShell(ctx.Identity.Login)
	if err != nil {
		log.Debugf("Failed to get login shell for %v: %v. Using default: %v.",
			ctx.Identity.Login, err, shell.DefaultShell)
	}
	if ctx.IsTestStub {
		shellPath = "/bin/sh"
	}

	// If a term was allocated before (this means it was an exec request with an
	// PTY explicitly allocated) or no command was given (which means an
	// interactive session was requested), then make sure "teleport exec"
	// executes through a PTY.
	if ctx.termAllocated || ctx.ExecRequest.GetCommand() == "" {
		c.Terminal = true
	}

	// If no command was given, configure a shell to run in 'login' mode.
	// Otherwise, execute a command through bash.
	if ctx.ExecRequest.GetCommand() == "" {
		// Overwrite whatever was in the exec command (probably empty) with the shell.
		ctx.ExecRequest.SetCommand(shellPath)

		// Set the path to the path of the shell.
		c.Path = shellPath

		// Configure the shell to run in 'login' mode. From OpenSSH source:
		// "If we have no command, execute the shell. In this case, the shell
		// name to be passed in argv[0] is preceded by '-' to indicate that
		// this is a login shell."
		// https://github.com/openssh/openssh-portable/blob/master/session.c
		c.Args = []string{"-" + filepath.Base(shellPath)}
	} else {
		// Execute commands like OpenSSH does:
		// https://github.com/openssh/openssh-portable/blob/master/session.c
		c.Path = shellPath
		c.Args = []string{shellPath, "-c", ctx.ExecRequest.GetCommand()}
	}

	clusterName, err := ctx.srv.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Lookup the UID and GID for the user.
	osUser, err := user.Lookup(ctx.Identity.Login)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uid, err := strconv.Atoi(osUser.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.Uid = uint32(uid)
	gid, err := strconv.Atoi(osUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.Gid = uint32(gid)

	// Set the home directory for the user.
	c.Dir = osUser.HomeDir

	// Lookup supplementary groups for the user.
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
	c.Groups = groups

	// Only set process credentials if the UID/GID of the requesting user are
	// different than the process (Teleport).
	//
	// Note, the above is important because setting the credentials struct
	// triggers calling of the SETUID and SETGID syscalls during process start.
	// If the caller does not have permission to call those two syscalls (for
	// example, if Teleport is started from a shell), this will prevent the
	// process from spawning shells with the error: "operation not permitted". To
	// workaround this, the credentials struct is only set if the credentials
	// are different from the process itself. If the credentials are not, simply
	// pick up the ambient credentials of the process.
	if strconv.Itoa(os.Getuid()) != osUser.Uid || strconv.Itoa(os.Getgid()) != osUser.Gid {
		c.SetCreds = true
		log.Debugf("Creating process with UID %v, GID: %v, and Groups: %v.",
			uid, gid, groups)
	} else {
		log.Debugf("Credential process with ambient credentials UID %v, GID: %v, Groups: %v.",
			uid, gid, groups)
	}

	// Create environment for user.
	c.Env = []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(osUser.Uid, defaultLoginDefsPath),
		"HOME=" + osUser.HomeDir,
		"USER=" + ctx.Identity.Login,
		"SHELL=" + shellPath,
		teleport.SSHTeleportUser + "=" + ctx.Identity.TeleportUser,
		teleport.SSHSessionWebproxyAddr + "=" + ctx.ProxyPublicAddress(),
		teleport.SSHTeleportHostUUID + "=" + ctx.srv.ID(),
		teleport.SSHTeleportClusterName + "=" + clusterName.GetClusterName(),
	}

	// Apply environment variables passed in from client.
	for n, v := range ctx.env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", n, v))
	}

	// Apply SSH_* environment variables.
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

	// If a terminal was allocated, set terminal type variable.
	if ctx.session != nil {
		c.Env = append(c.Env, fmt.Sprintf("TERM=%v", ctx.session.term.GetTermType()))
	}

	// If the command is being prepared for local execution, check if PAM should
	// be called.
	if ctx.srv.Component() == teleport.ComponentNode {
		conf, err := ctx.srv.GetPAM()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		c.PAM = &pamCommand{
			Enabled:     conf.Enabled,
			ServiceName: conf.ServiceName,
			Username:    ctx.Identity.Login,
		}
	}

	// If the server allows reading in of ~/.tsh/environment read it in
	// and pass environment variables along to new session.
	if ctx.srv.PermitUserEnvironment() {
		filename := filepath.Join(osUser.HomeDir, ".tsh", "environment")
		userEnvs, err := utils.ReadEnvironmentFile(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		c.Env = append(c.Env, userEnvs...)
	}

	return &c, nil
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

// Wait will block while the command executes.
func (r *remoteExec) Wait() *ExecResult {
	// Block until the command is finished executing.
	err := r.session.Wait()
	if err != nil {
		r.ctx.Debugf("Remote command failed: %v.", err)
	} else {
		r.ctx.Debugf("Remote command successfully executed.")
	}

	// Emit the result of execution to the Audit Log.
	emitExecAuditEvent(r.ctx, r.command, err)

	return &ExecResult{
		Command: r.GetCommand(),
		Code:    exitCode(err),
	}
}

// Continue does nothing for remote command execution.
func (r *remoteExec) Continue() {
	return
}

func emitExecAuditEvent(ctx *ServerContext, cmd string, execErr error) {
	// Report the result of this exec event to the audit logger.
	auditLog := ctx.srv.GetAuditLog()
	if auditLog == nil {
		log.Warnf("No audit log")
		return
	}

	var event events.Event

	// Create common fields for event.
	fields := events.EventFields{
		events.EventUser:      ctx.Identity.TeleportUser,
		events.EventLogin:     ctx.Identity.Login,
		events.LocalAddr:      ctx.Conn.LocalAddr().String(),
		events.RemoteAddr:     ctx.Conn.RemoteAddr().String(),
		events.EventNamespace: ctx.srv.GetNamespace(),
		// Due to scp being inherently vulnerable to command injection, always
		// make sure the full command and exit code is recorded for accountability.
		// For more details, see the following.
		//
		// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=327019
		// https://bugzilla.mindrot.org/show_bug.cgi?id=1998
		events.ExecEventCode:    strconv.Itoa(exitCode(execErr)),
		events.ExecEventCommand: cmd,
	}
	if execErr != nil {
		fields[events.ExecEventError] = execErr.Error()
	}

	// Parse the exec command to find out if it was SCP or not.
	path, action, isSCP, err := parseSecureCopy(cmd)
	if err != nil {
		log.Warnf("Unable to emit audit event: %v.", err)
		return
	}

	// Update appropriate fields based off if the request was SCP or not.
	if isSCP {
		fields[events.SCPPath] = path
		fields[events.SCPAction] = action
		switch action {
		case events.SCPActionUpload:
			if execErr != nil {
				event = events.SCPUploadFailure
			} else {
				event = events.SCPUpload
			}
		case events.SCPActionDownload:
			if execErr != nil {
				event = events.SCPDownloadFailure
			} else {
				event = events.SCPDownload
			}
		}
	} else {
		if execErr != nil {
			event = events.ExecFailure
		} else {
			event = events.Exec
		}
	}

	// Emit the event.
	auditLog.EmitAuditEvent(event, fields)
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
	action := events.SCPActionDownload
	if utils.SliceContainsStr(parts, "-t") {
		action = events.SCPActionUpload
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

// exitCode extracts and returns the exit code from the error.
func exitCode(err error) int {
	// If no error occurred, return 0 (success).
	if err == nil {
		return teleport.RemoteCommandSuccess
	}

	switch v := err.(type) {
	// Local execution.
	case *exec.ExitError:
		waitStatus, ok := v.Sys().(syscall.WaitStatus)
		if !ok {
			return teleport.RemoteCommandFailure
		}
		return waitStatus.ExitStatus()
	// Remote execution.
	case *ssh.ExitError:
		return v.ExitStatus()
	// An error occurred, but the type is unknown, return a generic 255 code.
	default:
		log.Debugf("Unknown error returned when executing command: %T: %v.", err, err)
		return teleport.RemoteCommandFailure
	}
}

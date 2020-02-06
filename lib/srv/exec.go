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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/sshutils"
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
// construct and execute a shell.
type execCommand struct {
	// Command is the command to execute. If a interactive session is being
	// requested, will be empty.
	Command string `json:"command"`

	// Username is the username associated with the Teleport identity.
	Username string `json:"username"`

	// Login is the local *nix account.
	Login string `json:"login"`

	// Roles is the list of Teleport roles assigned to the Teleport identity.
	Roles []string `json:"roles"`

	// ClusterName is the name of the Teleport cluster.
	ClusterName string `json:"cluster_name"`

	// Terminal indicates if a TTY has been allocated for the session. This is
	// typically set if either an shell was requested or a TTY was explicitly
	// allocated for a exec request.
	Terminal bool `json:"term"`

	// RequestType is the type of request: either "exec" or "shell". This will
	// be used to control where to connect std{out,err} based on the request
	// type: "exec" or "shell".
	RequestType string `json:"request_type"`

	// PAM indicates if PAM support was requested by the node.
	PAM bool `json:"pam"`

	// ServiceName is the name of the PAM service requested if PAM is enabled.
	ServiceName string `json:"service_name"`

	// Environment is a list of environment variables to add to the defaults.
	Environment []string `json:"environment"`

	// PermitUserEnvironment is set to allow reading in ~/.tsh/environment
	// upon login.
	PermitUserEnvironment bool `json:"permit_user_environment"`

	// IsTestStub is used by tests to mock the shell.
	IsTestStub bool `json:"is_test_stub"`
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

	// PID returns the PID of the Teleport process that was re-execed.
	PID() int
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

	// Create the command that will actually execute.
	e.Cmd, err = configureCommand(e.Ctx)
	if err != nil {
		return nil, trace.Wrap(err)
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
	e.Ctx.contw.Close()

	// Set to nil so the close in the context doesn't attempt to re-close.
	e.Ctx.contw = nil
}

// PID returns the PID of the Teleport process that was re-execed.
func (e *localExec) PID() int {
	return e.Cmd.Process.Pid
}

func (e *localExec) String() string {
	return fmt.Sprintf("Exec(Command=%v)", e.Command)
}

// RunAndExit will run the requested command and then exit.
func RunAndExit() {
	w, code, err := RunCommand()
	if err != nil {
		s := fmt.Sprintf("Failed to launch shell: %v.\r\n", err)
		io.Copy(w, bytes.NewBufferString(s))
	}
	os.Exit(code)
}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command.
func RunCommand() (io.Writer, int, error) {
	// errorWriter is used to return any error message back to the client. By
	// default it writes to stdout, but if a TTY is allocated, it will write
	// to it instead.
	errorWriter := os.Stdout

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(uintptr(3), "/proc/self/fd/3")
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}
	contfd := os.NewFile(uintptr(4), "/proc/self/fd/4")
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
	}

	// Read in the command payload.
	var b bytes.Buffer
	_, err := b.ReadFrom(cmdfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	var c execCommand
	err = json.Unmarshal(b.Bytes(), &c)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	var tty *os.File
	var pty *os.File

	// If a terminal was requested, file descriptor 4 and 5 always point to the
	// PTY and TTY. Extract them and set the controlling TTY. Otherwise, connect
	// std{in,out,err} directly.
	if c.Terminal {
		pty = os.NewFile(uintptr(5), "/proc/self/fd/5")
		tty = os.NewFile(uintptr(6), "/proc/self/fd/6")
		if pty == nil || tty == nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("pty and tty not found")
		}
		errorWriter = tty
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	var pamEnvironment []string
	if c.PAM {
		// Connect std{in,out,err} to the TTY if it's a shell request, otherwise
		// discard std{out,err}. If this was not done, things like MOTD would be
		// printed for "exec" requests.
		var stdin io.Reader
		var stdout io.Writer
		var stderr io.Writer
		if c.RequestType == sshutils.ShellRequest {
			stdin = tty
			stdout = tty
			stderr = tty
		} else {
			stdin = os.Stdin
			stdout = ioutil.Discard
			stderr = ioutil.Discard
		}

		// Set Teleport specific environment variables that PAM modules like
		// pam_script.so can pick up to potentially customize the account/session.
		os.Setenv("TELEPORT_USERNAME", c.Username)
		os.Setenv("TELEPORT_LOGIN", c.Login)
		os.Setenv("TELEPORT_ROLES", strings.Join(c.Roles, " "))

		// Open the PAM context.
		pamContext, err := pam.Open(&pam.Config{
			ServiceName: c.ServiceName,
			Login:       c.Login,
			Stdin:       stdin,
			Stdout:      stdout,
			Stderr:      stderr,
		})
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()

		// Save off any environment variables that come from PAM.
		pamEnvironment = pamContext.Environment()
	}

	// Build the actual command that will launch the shell.
	cmd, err := buildCommand(&c, tty, pty, pamEnvironment)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait until the continue signal is received from Teleport signaling that
	// the child process has been placed in a cgroup.
	err = waitForContinue(contfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Start the command.
	err = cmd.Start()
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait for the command to exit. It doesn't make sense to print an error
	// message here because the shell has successfully started. If an error
	// occured during shell execution or the shell exits with an error (like
	// running exit 2), the shell will print an error if appropriate and return
	// an exit code.
	err = cmd.Wait()
	return ioutil.Discard, exitCode(err), trace.Wrap(err)
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

// configureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
func configureCommand(ctx *ServerContext) (*exec.Cmd, error) {
	// Marshal the parts needed from the *ServerContext into a *execCommand.
	cmdmsg, err := ctx.ExecCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmdbytes, err := json.Marshal(cmdmsg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Write command bytes to pipe. The child process will read the command
	// to execute from this pipe.
	_, err = io.Copy(ctx.cmdw, bytes.NewReader(cmdbytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = ctx.cmdw.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Set to nil so the close in the context doesn't attempt to re-close.
	ctx.cmdw = nil

	// Find the Teleport executable and it's directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	executableDir, _ := filepath.Split(executable)

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, teleport.ExecSubCommand}

	// Build the "teleport exec" command.
	return &exec.Cmd{
		Path: executable,
		Args: args,
		Dir:  executableDir,
		ExtraFiles: []*os.File{
			ctx.cmdr,
			ctx.contr,
		},
	}, nil
}

// buildCommand construct a command that will execute the users shell. This
// function is run by Teleport while it's re-executing.
func buildCommand(c *execCommand, tty *os.File, pty *os.File, pamEnvironment []string) (*exec.Cmd, error) {
	var cmd exec.Cmd

	// Lookup the UID and GID for the user.
	localUser, err := user.Lookup(c.Login)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uid, err := strconv.Atoi(localUser.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gid, err := strconv.Atoi(localUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Lookup supplementary groups for the user.
	userGroups, err := localUser.GroupIds()
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

	// Get the login shell for the user (or fallback to the default).
	shellPath, err := shell.GetLoginShell(c.Login)
	if err != nil {
		log.Debugf("Failed to get login shell for %v: %v. Using default: %v.",
			c.Login, err, shell.DefaultShell)
	}
	if c.IsTestStub {
		shellPath = "/bin/sh"
	}

	// If no command was given, configure a shell to run in 'login' mode.
	// Otherwise, execute a command through the shell.
	if c.Command == "" {
		// Set the path to the path of the shell.
		cmd.Path = shellPath

		// Configure the shell to run in 'login' mode. From OpenSSH source:
		// "If we have no command, execute the shell. In this case, the shell
		// name to be passed in argv[0] is preceded by '-' to indicate that
		// this is a login shell."
		// https://github.com/openssh/openssh-portable/blob/master/session.c
		cmd.Args = []string{"-" + filepath.Base(shellPath)}
	} else {
		// Execute commands like OpenSSH does:
		// https://github.com/openssh/openssh-portable/blob/master/session.c
		cmd.Path = shellPath
		cmd.Args = []string{shellPath, "-c", c.Command}
	}

	// Create default environment for user.
	cmd.Env = []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(localUser.Uid, defaultLoginDefsPath),
		"HOME=" + localUser.HomeDir,
		"USER=" + c.Login,
		"SHELL=" + shellPath,
	}

	// Add in Teleport specific environment variables.
	cmd.Env = append(cmd.Env, c.Environment...)

	// If the server allows reading in of ~/.tsh/environment read it in
	// and pass environment variables along to new session.
	if c.PermitUserEnvironment {
		filename := filepath.Join(localUser.HomeDir, ".tsh", "environment")
		userEnvs, err := utils.ReadEnvironmentFile(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cmd.Env = append(cmd.Env, userEnvs...)
	}

	// If any additional environment variables come from PAM, apply them as well.
	cmd.Env = append(cmd.Env, pamEnvironment...)

	// Set the home directory for the user.
	cmd.Dir = localUser.HomeDir

	// If a terminal was requested, connect std{in,out,err} to the TTY and set
	// the controlling TTY. Otherwise, connect std{in,out,err} to
	// os.Std{in,out,err}.
	if c.Terminal {
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    int(tty.Fd()),
		}
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	}

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
	if strconv.Itoa(os.Getuid()) != localUser.Uid || strconv.Itoa(os.Getgid()) != localUser.Gid {
		cmd.SysProcAttr.Credential = &syscall.Credential{
			Uid:    uint32(uid),
			Gid:    uint32(gid),
			Groups: groups,
		}

		log.Debugf("Creating process with UID %v, GID: %v, and Groups: %v.",
			uid, gid, groups)
	} else {
		log.Debugf("Credential process with ambient credentials UID %v, GID: %v, Groups: %v.",
			uid, gid, groups)
	}

	return &cmd, nil
}

// waitForContinue will wait 10 seconds for the continue signal, if not
// received, it will stop waiting and exit.
func waitForContinue(contfd *os.File) error {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Reading from the continue file descriptor will block until it's closed. It
		// won't be closed until the parent has placed it in a cgroup.
		var r bytes.Buffer
		r.ReadFrom(contfd)

		// Continue signal has been processed, signal to continue execution.
		cancel()
	}()

	// Wait for 10 seconds and then timeout if no continue signal has been sent.
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return trace.BadParameter("timed out waiting for continue signal")
	case <-ctx.Done():
	}
	return nil
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

// PID returns an invalid PID for remotExec.
func (r *remoteExec) PID() int {
	return 0
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

// errorAndExit writes the error to the io.Writer (stdout or a TTY) and
// exits with the given code.
func errorAndExit(w io.Writer, code int, err error) {
	s := fmt.Sprintf("Failed to launch shell: %v.\r\n", err)
	if err != nil {
		io.Copy(w, bytes.NewBufferString(s))
	}
	os.Exit(code)
}

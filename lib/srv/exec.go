/*
Copyright 2015-2020 Gravitational, Inc.

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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	services "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPath          = "/bin:/usr/bin:/usr/local/bin:/sbin"
	defaultEnvPath       = "PATH=" + defaultPath
	defaultRootPath      = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	defaultEnvRootPath   = "PATH=" + defaultRootPath
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
	if services.IsRecordAtProxy(ctx.ClusterConfig.GetSessionRecording()) == true {
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
	// Parse the command to see if it is scp.
	err := e.transformSecureCopy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the command that will actually execute.
	e.Cmd, err = ConfigureCommand(e.Ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Connect stdout and stderr to the channel so the user can interact with
	// the command.
	e.Cmd.Stderr = io.MultiWriter(os.Stderr, channel.Stderr())
	e.Cmd.Stdout = io.MultiWriter(os.Stdout, channel)

	// Copy from the channel (client input) into stdin of the process.
	inputWriter, err := e.Cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	go func() {
		if _, err := io.Copy(inputWriter, channel); err != nil {
			e.Ctx.Warnf("Failed to forward data from SSH channel to local command %q stdin: %v", e.GetCommand(), err)
		}
		inputWriter.Close()
	}()

	e.Ctx.Infof("Started local command execution: %q", e.Command)

	return nil, nil
}

// Wait will block while the command executes.
func (e *localExec) Wait() *ExecResult {
	if e.Cmd.Process == nil {
		e.Ctx.Error("No process.")
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
	e.Command = fmt.Sprintf("%s scp --remote-addr=%q --local-addr=%q %v",
		teleportBin,
		e.Ctx.ServerConn.RemoteAddr().String(),
		e.Ctx.ServerConn.LocalAddr().String(),
		strings.Join(args[1:], " "))

	return nil
}

// waitForContinue will wait 10 seconds for the continue signal, if not
// received, it will stop waiting and exit.
func waitForContinue(contfd *os.File) error {
	waitCh := make(chan error, 1)
	go func() {
		// Reading from the continue file descriptor will block until it's closed. It
		// won't be closed until the parent has placed it in a cgroup.
		buf := make([]byte, 1)
		_, err := contfd.Read(buf)
		if err == io.EOF {
			err = nil
		}
		waitCh <- err
	}()

	// Wait for 10 seconds and then timeout if no continue signal has been sent.
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		return trace.BadParameter("timed out waiting for continue signal")
	case err := <-waitCh:
		return err
	}
}

// remoteExec is used to run an "exec" SSH request and return the result.
type remoteExec struct {
	command string
	session *ssh.Session
	ctx     *ServerContext
}

// String describes this remote exec value
func (e *remoteExec) String() string {
	return fmt.Sprintf("RemoteExec(Command=%v)", e.command)
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
func (e *remoteExec) Start(ch ssh.Channel) (*ExecResult, error) {
	// hook up stdout/err the channel so the user can interact with the command
	e.session.Stdout = ch
	e.session.Stderr = ch.Stderr()
	inputWriter, err := e.session.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		// copy from the channel (client) into stdin of the process
		if _, err := io.Copy(inputWriter, ch); err != nil {
			e.ctx.Warnf("Failed copying data from SSH channel to remote command stdin: %v", err)
		}
		inputWriter.Close()
	}()

	err = e.session.Start(e.command)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// Wait will block while the command executes.
func (e *remoteExec) Wait() *ExecResult {
	// Block until the command is finished executing.
	err := e.session.Wait()
	if err != nil {
		e.ctx.Debugf("Remote command failed: %v.", err)
	} else {
		e.ctx.Debugf("Remote command successfully executed.")
	}

	// Emit the result of execution to the Audit Log.
	emitExecAuditEvent(e.ctx, e.command, err)

	return &ExecResult{
		Command: e.GetCommand(),
		Code:    exitCode(err),
	}
}

// Continue does nothing for remote command execution.
func (e *remoteExec) Continue() {}

// PID returns an invalid PID for remotExec.
func (e *remoteExec) PID() int {
	return 0
}

func emitExecAuditEvent(ctx *ServerContext, cmd string, execErr error) {
	// Create common fields for event.
	serverMeta := events.ServerMetadata{
		ServerID:        ctx.srv.HostUUID(),
		ServerNamespace: ctx.srv.GetNamespace(),
	}

	sessionMeta := events.SessionMetadata{
		SessionID: string(ctx.SessionID()),
		WithMFA:   ctx.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified],
	}

	userMeta := events.UserMetadata{
		User:         ctx.Identity.TeleportUser,
		Login:        ctx.Identity.Login,
		Impersonator: ctx.Identity.Impersonator,
	}

	connectionMeta := events.ConnectionMetadata{
		RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		LocalAddr:  ctx.ServerConn.LocalAddr().String(),
	}

	commandMeta := events.CommandMetadata{
		Command: cmd,
		// Due to scp being inherently vulnerable to command injection, always
		// make sure the full command and exit code is recorded for accountability.
		// For more details, see the following.
		//
		// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=327019
		// https://bugzilla.mindrot.org/show_bug.cgi?id=1998
		ExitCode: strconv.Itoa(exitCode(execErr)),
	}

	if execErr != nil {
		commandMeta.Error = execErr.Error()
	}

	// Parse the exec command to find out if it was SCP or not.
	path, action, isSCP, err := parseSecureCopy(cmd)
	if err != nil {
		log.Warnf("Unable to emit audit event: %v.", err)
		return
	}

	// Update appropriate fields based off if the request was SCP or not.
	if isSCP {
		scpEvent := &events.SCP{
			Metadata: events.Metadata{
				Type:        events.SCPEvent,
				ClusterName: ctx.ClusterName,
			},
			ServerMetadata:     serverMeta,
			SessionMetadata:    sessionMeta,
			UserMetadata:       userMeta,
			ConnectionMetadata: connectionMeta,
			CommandMetadata:    commandMeta,
			Path:               path,
			Action:             action,
		}

		switch action {
		case events.SCPActionUpload:
			if execErr != nil {
				scpEvent.Code = events.SCPUploadFailureCode
			} else {
				scpEvent.Code = events.SCPUploadCode
			}
		case events.SCPActionDownload:
			if execErr != nil {
				scpEvent.Code = events.SCPDownloadFailureCode
			} else {
				scpEvent.Code = events.SCPDownloadCode
			}
		}
		if err := ctx.srv.EmitAuditEvent(ctx.srv.Context(), scpEvent); err != nil {
			log.WithError(err).Warn("Failed to emit scp event.")
		}
	} else {
		execEvent := &events.Exec{
			Metadata: events.Metadata{
				Type: events.ExecEvent,
			},
			ServerMetadata:     serverMeta,
			SessionMetadata:    sessionMeta,
			UserMetadata:       userMeta,
			ConnectionMetadata: connectionMeta,
			CommandMetadata:    commandMeta,
		}
		if execErr != nil {
			execEvent.Code = events.ExecFailureCode
		} else {
			execEvent.Code = events.ExecCode
		}
		if err := ctx.srv.EmitAuditEvent(ctx.srv.Context(), execEvent); err != nil {
			log.WithError(err).Warn("Failed to emit exec event.")
		}
	}
}

// getDefaultEnvPath returns the default value of PATH environment variable for
// new logins (prior to shell) based on login.defs. Returns a string which
// looks like "PATH=/usr/bin:/bin"
func getDefaultEnvPath(uid string, loginDefsPath string) string {
	envPath := defaultEnvPath
	envRootPath := defaultEnvRootPath

	// open file, if it doesn't exist return a default path and move on
	f, err := os.Open(loginDefsPath)
	if err != nil {
		if uid == "0" {
			log.Infof("Unable to open %q: %v: returning default su path: %q", loginDefsPath, err, defaultEnvRootPath)
			return defaultEnvRootPath
		}
		log.Infof("Unable to open %q: %v: returning default path: %q", loginDefsPath, err, defaultEnvPath)
		return defaultEnvPath
	}
	defer f.Close()

	// read path from login.defs file (/etc/login.defs) line by line:
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// skip comments and empty lines:
		if line == "" || line[0] == '#' {
			continue
		}

		// look for a line that starts with ENV_PATH or ENV_SUPATH
		fields := strings.Fields(line)
		if len(fields) > 1 {
			if fields[0] == "ENV_PATH" {
				envPath = fields[1]
			}
			if fields[0] == "ENV_SUPATH" {
				envRootPath = fields[1]
			}
		}
	}

	// if any error occurs while reading the file, return the default value
	err = scanner.Err()
	if err != nil {
		if uid == "0" {
			log.Warnf("Unable to open %q: %v: returning default su path: %q", loginDefsPath, err, defaultEnvRootPath)
			return defaultEnvRootPath
		}
		log.Warnf("Unable to read %q: %v: returning default path: %q", loginDefsPath, err, defaultEnvPath)
		return defaultEnvPath
	}

	// if requesting path for uid 0 and no ENV_SUPATH is given, fallback to
	// ENV_PATH first, then the default path.
	if uid == "0" {
		return envRootPath
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

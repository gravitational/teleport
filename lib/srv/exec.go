/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
	Start(ctx context.Context, channel ssh.Channel) (*ExecResult, error)

	// Wait will block while the command executes.
	Wait() *ExecResult

	// WaitForChild blocks until the child process has completed any required
	// setup operations before proceeding with execution.
	WaitForChild() error

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

	// If this is a registered OpenSSH node or proxy recoding mode is
	// enabled, execute the command on a remote host. This is used by
	// in-memory forwarding nodes.
	if types.IsOpenSSHNodeSubKind(ctx.ServerSubKind) || services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
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
func (e *localExec) Start(ctx context.Context, channel ssh.Channel) (*ExecResult, error) {
	logger := e.Ctx.Logger.With("command", e.GetCommand())

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

	// Connect stdout and stderr to the channel so the user can interact with the command.
	e.Cmd.Stderr = channel.Stderr()
	e.Cmd.Stdout = channel

	// Copy from the channel (client input) into stdin of the process.
	inputWriter, err := e.Cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start the command.
	err = e.Cmd.Start()
	if err != nil {
		logger.WarnContext(ctx, "Local command failed to start", "error", err)

		// Emit the result of execution to the audit log
		emitExecAuditEvent(e.Ctx, e.GetCommand(), err)

		return &ExecResult{
			Command: e.GetCommand(),
			Code:    exitCode(err),
		}, trace.ConvertSystemError(err)
	}
	// Close our half of the write pipe since it is only to be used by the child process.
	// Not closing prevents being signaled when the child closes its half.
	if err := e.Ctx.readyw.Close(); err != nil {
		logger.WarnContext(ctx, "Failed to close parent process ready signal write fd", "error", err)
	}
	e.Ctx.readyw = nil

	go func() {
		if _, err := io.Copy(inputWriter, channel); err != nil {
			logger.WarnContext(ctx, "Failed to forward data from SSH channel to local command", "error", err)
		}
		inputWriter.Close()
	}()

	logger.InfoContext(ctx, "Started local command execution")

	return nil, nil
}

// Wait will block while the command executes.
func (e *localExec) Wait() *ExecResult {
	if e.Cmd.Process == nil {
		e.Ctx.Logger.ErrorContext(e.Ctx.CancelContext(), "No process")
	}

	// Block until the command is finished executing.
	err := e.Cmd.Wait()
	if err != nil {
		e.Ctx.Logger.DebugContext(e.Ctx.CancelContext(), "Local command failed", "error", err)
	} else {
		e.Ctx.Logger.DebugContext(e.Ctx.CancelContext(), "Local command successfully executed")
	}

	// Emit the result of execution to the Audit Log.
	emitExecAuditEvent(e.Ctx, e.GetCommand(), err)

	execResult := &ExecResult{
		Command: e.GetCommand(),
		Code:    exitCode(err),
	}

	return execResult
}

func (e *localExec) WaitForChild() error {
	err := waitForSignal(e.Ctx.readyr, 20*time.Second)
	closeErr := e.Ctx.readyr.Close()
	// Set to nil so the close in the context doesn't attempt to re-close.
	e.Ctx.readyr = nil
	return trace.NewAggregate(err, closeErr)
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
	isSCPCmd, err := checkSCPAllowed(e.Ctx, e.GetCommand())
	if err != nil {
		e.Ctx.GetServer().EmitAuditEvent(e.Ctx.CancelContext(), &apievents.SFTP{
			Metadata: apievents.Metadata{
				Code: events.SCPDisallowedCode,
				Type: events.SCPEvent,
				Time: time.Now(),
			},
			UserMetadata:   e.Ctx.Identity.GetUserMetadata(),
			ServerMetadata: e.Ctx.GetServer().TargetMetadata(),
			Error:          err.Error(),
		})
		return trace.Wrap(err)
	}
	if !isSCPCmd {
		return nil
	}
	_, scpArgs, ok := strings.Cut(e.GetCommand(), " ")
	if !ok {
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
		scpArgs,
	)

	return nil
}

// checkSCPAllowed will return false if the command is not a SCP command,
// and if it is it will return true and potentially an error if file
// copying is not allowed.
func checkSCPAllowed(scx *ServerContext, command string) (bool, error) {
	// split up command by space to grab the first word. if we don't have anything
	// it's an interactive shell the user requested and not scp, return
	args := strings.Split(command, " ")
	if len(args) == 0 {
		return false, nil
	}
	// see the user is not requesting scp, return
	if _, f := filepath.Split(args[0]); f != teleport.SCP {
		return false, nil
	}

	return true, trace.Wrap(scx.CheckFileCopyingAllowed())
}

// waitForSignal will wait 10 seconds for the other side of the pipe to signal, if not
// received, it will stop waiting and exit.
func waitForSignal(fd *os.File, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		// Reading from the file descriptor will block until it's closed.
		_, err := fd.Read(make([]byte, 1))
		if errors.Is(err, io.EOF) {
			err = nil
		}
		waitCh <- err
	}()

	// Timeout if no signal has been sent within the provided duration.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		return trace.LimitExceeded("timed out waiting for continue signal")
	case err := <-waitCh:
		return err
	}
}

// remoteExec is used to run an "exec" SSH request and return the result.
type remoteExec struct {
	command string
	session *tracessh.Session
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
func (e *remoteExec) Start(ctx context.Context, ch ssh.Channel) (*ExecResult, error) {
	if _, err := checkSCPAllowed(e.ctx, e.GetCommand()); err != nil {
		e.ctx.GetServer().EmitAuditEvent(context.WithoutCancel(ctx), &apievents.SFTP{
			Metadata: apievents.Metadata{
				Code: events.SCPDisallowedCode,
				Type: events.SCPEvent,
				Time: time.Now(),
			},
			UserMetadata:   e.ctx.Identity.GetUserMetadata(),
			ServerMetadata: e.ctx.GetServer().TargetMetadata(),
			Error:          err.Error(),
		})
		return nil, trace.Wrap(err)
	}

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
			e.ctx.Logger.WarnContext(ctx, "Failed copying data from SSH channel to remote command stdin", "error", err)
		}
		inputWriter.Close()
	}()

	err = e.session.Start(ctx, e.command)
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
		e.ctx.Logger.DebugContext(e.ctx.CancelContext(), "Remote command failed", "error", err)
	} else {
		e.ctx.Logger.DebugContext(e.ctx.CancelContext(), "Remote command successfully executed")
	}

	// Emit the result of execution to the Audit Log.
	emitExecAuditEvent(e.ctx, e.command, err)

	return &ExecResult{
		Command: e.GetCommand(),
		Code:    exitCode(err),
	}
}

func (e *remoteExec) WaitForChild() error { return nil }

// Continue does nothing for remote command execution.
func (e *remoteExec) Continue() {}

// PID returns an invalid PID for remotExec.
func (e *remoteExec) PID() int {
	return 0
}

// emitExecAuditEvent emits either an SCP or exec event based on the
// command run.
//
// Note: to ensure that the event is recorded ctx.session must be used
// instead of ctx.srv.
func emitExecAuditEvent(ctx *ServerContext, cmd string, execErr error) {
	// Create common fields for event.
	serverMeta := ctx.GetServer().TargetMetadata()
	sessionMeta := ctx.GetSessionMetadata()
	userMeta := ctx.Identity.GetUserMetadata()

	connectionMeta := apievents.ConnectionMetadata{
		RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		LocalAddr:  ctx.ServerConn.LocalAddr().String(),
	}

	commandMeta := apievents.CommandMetadata{
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
		ctx.Logger.WarnContext(ctx.srv.Context(), "Unable to parse scp command", "error", err)
		return
	}

	// Update appropriate fields based off if the request was SCP or not.
	if isSCP {
		scpEvent := &apievents.SCP{
			Metadata: apievents.Metadata{
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
		if err := ctx.session.emitAuditEvent(ctx.srv.Context(), scpEvent); err != nil {
			ctx.Logger.WarnContext(ctx.srv.Context(), "Failed to emit scp event", "error", err)
		}
	} else {
		execEvent := &apievents.Exec{
			Metadata: apievents.Metadata{
				Type:        events.ExecEvent,
				ClusterName: ctx.ClusterName,
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
		if err := ctx.session.emitAuditEvent(ctx.srv.Context(), execEvent); err != nil {
			ctx.Logger.WarnContext(ctx.srv.Context(), "Failed to emit exec event", "error", err)
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
	f, err := utils.OpenFileAllowingUnsafeLinks(loginDefsPath)
	if err != nil {
		if uid == "0" {
			slog.InfoContext(context.Background(), "Unable to open login.defs, returning default su path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvRootPath)
			return defaultEnvRootPath
		}
		slog.InfoContext(context.Background(), "Unable to open login.defs, returning default path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvPath)
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
			slog.WarnContext(context.Background(), "Unable to read login.defs, returning default su path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvRootPath)
			return defaultEnvRootPath
		}
		slog.WarnContext(context.Background(), "Unable to read login.defs, returning default path", "login_defs_path", loginDefsPath, "error", err, "default_path", defaultEnvPath)
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
	if slices.Contains(parts, "-t") {
		action = events.SCPActionUpload
	}

	// Extract the name of the Teleport executable on disk.
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

	var execExitErr *exec.ExitError
	var sshExitErr *ssh.ExitError
	switch {
	// Local execution.
	case errors.As(err, &execExitErr):
		waitStatus, ok := execExitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return teleport.RemoteCommandFailure
		}
		return waitStatus.ExitStatus()
	// Remote execution.
	case errors.As(err, &sshExitErr):
		return sshExitErr.ExitStatus()
	// An error occurred, but the type is unknown, return a generic 255 code.
	default:
		slog.DebugContext(context.Background(), "Unknown error returned when executing command", "error", err)
		return teleport.RemoteCommandFailure
	}
}

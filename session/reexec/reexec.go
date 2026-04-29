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

package reexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	ocselinux "github.com/opencontainers/selinux/go-selinux"
	"golang.org/x/sys/unix"

	apiconstants "github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/session/auditd"
	"github.com/gravitational/teleport/session/envutils"
	"github.com/gravitational/teleport/session/host"
	"github.com/gravitational/teleport/session/logconstants"
	"github.com/gravitational/teleport/session/loginuid"
	"github.com/gravitational/teleport/session/networking"
	"github.com/gravitational/teleport/session/networking/x11"
	"github.com/gravitational/teleport/session/pam"
	"github.com/gravitational/teleport/session/pam/pamcfg"
	"github.com/gravitational/teleport/session/reexec/internal/logutils"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
	"github.com/gravitational/teleport/session/reexec/reexecsftp"
	"github.com/gravitational/teleport/session/selinux"
	"github.com/gravitational/teleport/session/shell"
	"github.com/gravitational/teleport/session/uacc"
	"github.com/gravitational/teleport/session/uds"
)

const (
	// procLoginuid is the path to the current process's loginuid.
	procLoginuid = "/proc/self/loginuid"
	// procSessionID is the path to the current process's session ID.
	procSessionID = "/proc/self/sessionid"
)

// FileFD is a file descriptor passed down from a parent process when
// Teleport is re-executing itself.
type FileFD = uintptr

// FileFDs used by all re-exec subcommands.
const (
	// CommandFile is used to pass the command and arguments that the
	// child process should execute from the parent process.
	CommandFile FileFD = 3 + iota
	// LogFile is used to emit logs from the child process to the parent
	// process.
	LogFile
	// ContinueFile is used to communicate to the child process that
	// it can continue after the parent process starts monitoring the
	// child's audit login session ID when Enhanced Session Recording
	// is enabled. Otherwise it isn't used.
	ContinueFile
	// ReadyFile is used to communicate to the parent process that the
	// child has changed its auid and is ready to be monitored when
	// Enhanced Session Recording is enabled. Otherwise it isn't used.
	ReadyFile
	// TerminateFile is used to communicate to the child process that
	// the interactive terminal should be killed as the client ended the
	// SSH session and without termination the terminal process will be assigned
	// to pid 1 and "live forever". Killing the shell should not prevent processes
	// preventing SIGHUP to be reassigned (ex. processes running with nohup).
	TerminateFile
	// FirstExtraFile is the first file descriptor that will be valid when
	// extra files are passed to child processes without a terminal.
	FirstExtraFile
)

// FileFDs for terminal based exec sessions.
const (
	// TTYFile is a TTY the parent process passes to the child process.
	TTYFile = FirstExtraFile + iota
)

// FileFDs for non-terminal based exec sessions.
const (
	// StdinFile is used to capture the stdin stream of the shell (grandchild) process.
	StdinFile = FirstExtraFile + iota
	// StdoutFile is used to capture the stdout stream of the shell (grandchild) process.
	StdoutFile
	// StderrFile is used to capture the stderr stream of the shell (grandchild) process.
	StderrFile
)

// FileFDs for SFTP sessions.
const (
	// FileTransferOutFile is used to pass write transfer data to the sftp (grandchild) process.
	FileTransferOutFile = FirstExtraFile + iota
	// FileTransferInFile is used to pass read transfer data from the sftp (grandchild) process.
	FileTransferInFile
	// AuditInFile is used to read audit events from the sftp (grandchild) process.
	AuditInFile
)

// FileFDs for networking sessions.
const (
	// ListenerFile is a unix datagram socket listener.
	ListenerFile = FirstExtraFile + iota
)

func fdName(f FileFD) string {
	return fmt.Sprintf("/proc/self/fd/%d", f)
}

// ExecCommand contains the payload to "teleport exec" which will be used to
// construct and execute a shell.
type ExecCommand struct {
	Stdin  io.Reader `json:"-"`
	Stdout io.Writer `json:"-"`
	Stderr io.Writer `json:"-"`

	// LogConfig is the log configuration for the child process.
	LogConfig ExecLogConfig `json:"log_config"`

	// Command is the command to execute. If an interactive session is being
	// requested, will be empty. If a subsystem is requested, it will contain
	// the subsystem name.
	Command string `json:"command"`

	// DestinationAddress is the target address to dial to.
	DestinationAddress string `json:"dst_addr"`

	// Username is the username associated with the Teleport identity.
	Username string `json:"username"`

	// Login is the local *nix account.
	Login string `json:"login"`

	// Roles is the list of Teleport roles assigned to the Teleport identity.
	Roles []string `json:"roles"`

	// ClusterName is the name of the Teleport cluster.
	ClusterName string `json:"cluster_name"`

	// Terminal indicates if a TTY has been allocated for the session. This is
	// typically set if either a shell was requested or a TTY was explicitly
	// allocated for an exec request.
	Terminal bool `json:"term"`

	// TerminalName is the name of TTY terminal, ex: /dev/tty1.
	// Currently, this field is used by auditd.
	TerminalName string `json:"terminal_name"`

	// ClientAddress contains IP address of the connected client.
	// Currently, this field is used by auditd.
	ClientAddress string `json:"client_address"`

	// RequestType is the type of request: either "exec" or "shell". This will
	// be used to control where to connect std{out,err} based on the request
	// type: "exec", "shell" or "subsystem".
	RequestType string `json:"request_type"`

	// PAMConfig is the configuration data that needs to be passed to the child and then to PAM modules.
	PAMConfig *PAMConfig `json:"pam_config,omitempty"`

	// Environment is a list of environment variables to add to the defaults.
	Environment []string `json:"environment"`

	// PermitUserEnvironment is set to allow reading in ~/.tsh/environment
	// upon login.
	PermitUserEnvironment bool `json:"permit_user_environment"`

	// IsTestStub is used by tests to mock the shell.
	IsTestStub bool `json:"is_test_stub"`

	// UserCreatedByTeleport is true when the system user was created by Teleport user auto-provision.
	UserCreatedByTeleport bool

	// UaccMetadata contains metadata needed for user accounting.
	UaccMetadata UaccMetadata `json:"uacc_meta"`

	// SetSELinuxContext is true when the SELinux context should be set
	// for the child.
	SetSELinuxContext bool `json:"set_selinux_context"`

	// RecordWithBPF is true when Enhanced Session Recording should
	// record the session.
	RecordWithBPF bool `json:"bpf_recording"`
}

// ExecLogConfig represents all the logging configuration data that
// needs to be passed to the child.
type ExecLogConfig struct {
	// Level is the log level to use.
	Level slog.Level
	// Format defines the output format. Possible values are 'text' and 'json'.
	Format string
	// ExtraFields lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller].
	ExtraFields []string
	// EnableColors dictates if output should be colored when Format is set to "text".
	EnableColors bool
	// Padding to use for various components when Format is set to "text".
	Padding int
}

// PAMConfig represents all the configuration data that needs to be passed to the child.
type PAMConfig struct {
	// UsePAMAuth specifies whether to trigger the "auth" PAM modules from the
	// policy.
	UsePAMAuth bool `json:"use_pam_auth"`

	// ServiceName is the name of the PAM service requested if PAM is enabled.
	ServiceName string `json:"service_name"`

	// Environment represents env variables to pass to PAM.
	Environment map[string]string `json:"environment"`
}

// UaccMetadata contains information the child needs from the parent for user accounting.
type UaccMetadata struct {
	// RemoteAddr is the address of the remote host.
	RemoteAddr NetAddr `json:"remote_addr"`

	// UtmpPath is the path of the system utmp database.
	UtmpPath string `json:"utmp_path,omitempty"`

	// WtmpPath is the path of the system wtmp log.
	WtmpPath string `json:"wtmp_path,omitempty"`

	// BtmpPath is the path of the system btmp log.
	BtmpPath string `json:"btmp_path,omitempty"`

	// WtmpdbPath is the path of the system wtmpdb database.
	WtmpdbPath string `json:"wtmpdb_path,omitempty"`
}

// NetAddrFromAddr returns NetAddr from golang standard net.Addr
func NetAddrFromAddr(a net.Addr) NetAddr {
	return NetAddr{AddrNetwork: a.Network(), Addr: a.String()}
}

type NetAddr struct {
	AddrNetwork string `json:"network"`
	Addr        string `json:"addr"`
}

var _ net.Addr = (*NetAddr)(nil)

// Network implements [net.Addr].
func (n *NetAddr) Network() string {
	return n.AddrNetwork
}

// String implements [net.Addr].
func (n *NetAddr) String() string {
	return n.Addr
}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command. This function may change
// system state related to the process and/or thread for PAM and SELinux.
// The process should exit after this function returns so the potentially
// modified process and/or thread isn't used with a non-standard state.
// Returns exitErr if the exec/shell command runs and exits successfully
// or err if command fails to run.
func RunCommand() (exitErr error, err error) {
	ctx := context.Background()

	// SIGQUIT is used by teleport to initiate graceful shutdown, waiting for
	// existing exec sessions to close before ending the process. For this to
	// work when closing the entire teleport process group, exec sessions must
	// ignore SIGQUIT signals.
	signal.Ignore(syscall.SIGQUIT)

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, fdName(CommandFile))
	if cmdfd == nil {
		return nil, trace.BadParameter("command pipe not found")
	}
	logfd := os.NewFile(LogFile, fdName(LogFile))
	if logfd == nil {
		return nil, trace.BadParameter("log pipe not found")
	}
	contfd := os.NewFile(ContinueFile, fdName(ContinueFile))
	if contfd == nil {
		return nil, trace.BadParameter("continue pipe not found")
	}
	readyfd := os.NewFile(ReadyFile, fdName(ReadyFile))
	if readyfd == nil {
		return nil, trace.BadParameter("ready pipe not found")
	}
	terminatefd := os.NewFile(TerminateFile, fdName(TerminateFile))
	if terminatefd == nil {
		return nil, trace.BadParameter("terminate pipe not found")
	}

	// Read in the command payload.
	var c ExecCommand
	if err := json.NewDecoder(cmdfd).Decode(&c); err != nil {
		return nil, trace.Wrap(err)
	}

	// If BPF is enabled, ensure that the ready file is closed if a
	// failure causes execution to terminate prior to the audit session
	// ID actually being changed to unblock the parent process.
	if c.RecordWithBPF {
		defer func() {
			if readyfd != nil {
				_ = readyfd.Close()
			}
		}()
	}

	initLogger("reexec", logfd, c.LogConfig)

	auditdMsg := auditd.Message{
		SystemUser:   c.Login,
		TeleportUser: c.Username,
		ConnAddress:  c.ClientAddress,
		TTYName:      c.TerminalName,
	}

	if err := auditd.SendEvent(auditd.AuditUserLogin, auditd.Success, auditdMsg); err != nil {
		slog.DebugContext(ctx, "failed to send user start event to auditd", "error", err)
	}

	defer func() {
		if err != nil {
			if errors.Is(err, user.UnknownUserError(c.Login)) {
				if err := auditd.SendEvent(auditd.AuditUserErr, auditd.Failed, auditdMsg); err != nil {
					slog.DebugContext(ctx, "failed to send UserErr event to auditd", "error", err)
				}
				return
			}
		}

		if err := auditd.SendEvent(auditd.AuditUserEnd, auditd.Success, auditdMsg); err != nil {
			slog.DebugContext(ctx, "failed to send UserEnd event to auditd", "error", err)
		}
	}()

	// If Enhanced Session Recording is enabled, take note of what the
	// loginuid is set to before a PAM context is opened. We will need
	// to write to the loginuid file if PAM hasn't already.
	var loginUIDBytes []byte
	if c.RecordWithBPF {
		loginUIDBytes, err = os.ReadFile(procLoginuid)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var tty *os.File
	if c.Terminal {
		// If this is an interactive session, use the tty file for grandchild stdio.
		tty = os.NewFile(TTYFile, fdName(TTYFile))
		if tty == nil {
			return nil, trace.BadParameter("tty not found")
		}
		c.Stdin = tty
		c.Stdout = tty
		c.Stderr = tty
	} else if c.RequestType == "subsystem" && c.Command == "sftp" {
		// std{in/out} is not used by the SFTP sub process, just collect stderr.
		c.Stdin = bytes.NewReader(nil)
		c.Stdout = io.Discard
		// Propagate sftp subprocess errors to the parent process.
		c.Stderr = os.Stderr

	} else {
		// If this is a normal, non-interactive exec session, use the stdio pipes provided as extra files.
		c.Stdin = os.NewFile(StdinFile, fdName(StdinFile))
		if c.Stdin == nil {
			return nil, trace.BadParameter("stdin not found")
		}
		c.Stdout = os.NewFile(StdoutFile, fdName(StdoutFile))
		if c.Stdout == nil {
			return nil, trace.BadParameter("stdout not found")
		}
		c.Stderr = os.NewFile(StderrFile, fdName(StderrFile))
		if c.Stderr == nil {
			return nil, trace.BadParameter("stderr not found")
		}
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	var pamEnvironment []string
	if c.PAMConfig != nil {
		slog.DebugContext(ctx, "Opening PAM context")

		cfg := &pamcfg.PAMConfig{
			ServiceName: c.PAMConfig.ServiceName,
			UsePAMAuth:  c.PAMConfig.UsePAMAuth,
			Login:       c.Login,
			// Set Teleport specific environment variables that PAM modules
			// like pam_script.so can pick up to potentially customize the
			// account/session.
			Env: c.PAMConfig.Environment,
			// Connect std{in,out,err} to the TTY if a terminal has been allocated.
			Stdin:  c.Stdin,
			Stdout: c.Stdout,
			Stderr: c.Stderr,
		}

		// Discard std{out,err} for non-interactive requests. Otherwise, things like
		// MOTD would be printed.
		if !c.Terminal {
			cfg.Stdout = io.Discard
			cfg.Stderr = io.Discard
		}

		// Open the PAM context.
		pamContext, err := pam.Open(cfg)
		if err != nil {
			// Format the PAM error to be user friendly.
			return nil, trace.Errorf("failed to open PAM context: %v", err.Error())
		}
		defer pamContext.Close()

		// Save off any environment variables that come from PAM.
		pamEnvironment = pamContext.Environment()
	}

	uaccHandler := uacc.NewUserAccountHandler(uacc.UaccConfig{
		UtmpFile:   c.UaccMetadata.UtmpPath,
		WtmpFile:   c.UaccMetadata.WtmpPath,
		BtmpFile:   c.UaccMetadata.BtmpPath,
		WtmpdbFile: c.UaccMetadata.WtmpdbPath,
	})

	localUser, err := user.Lookup(c.Login)
	if err != nil {
		if uaccErr := uaccHandler.FailedLogin(c.Login, &c.UaccMetadata.RemoteAddr); uaccErr != nil {
			slog.DebugContext(ctx, "unable to write failed login attempt to uacc", "error", uaccErr)
		}
		return nil, trace.Wrap(err)
	}

	// Ensure this process has a unique audit login session ID (auid) set
	// so Enhanced Session Recording can track events correctly.
	if c.RecordWithBPF {
		if err := setAuditSessionID(ctx, c, loginUIDBytes, localUser, readyfd); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if c.Terminal {
		uaccSession, err := uaccHandler.OpenSession(tty, c.Login, &c.UaccMetadata.RemoteAddr)
		if err == nil {
			defer func() {
				if closeErr := uaccSession.Close(); closeErr != nil {
					slog.DebugContext(ctx, "failed to close uacc session", "error", closeErr)
				}
			}()
		} else {
			// uacc support is best-effort, only enable it if OpenSession is successful.
			// Currently, there is no way to log this error out-of-band with the
			// command output, so for now we essentially ignore it.
			slog.DebugContext(ctx, "failed to open uacc session", "error", err)
		}
	}

	// Build the actual command that will launch the shell.
	cmd, err := BuildCommand(&c, localUser, pamEnvironment)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wait until the continue signal is received from Teleport signaling that
	// Teleport is monitoring this session if Enhanced Session Recording is enabled.
	if c.RecordWithBPF {
		err = WaitForSignal(ctx, contfd, 10*time.Second)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		slog.DebugContext(ctx, "Received continue signal")
	}

	// If we're planning on changing credentials, we should first park an
	// innocuous process with the same UID and then check the user database
	// again, to avoid it getting deleted under our nose.
	parkerCtx, parkerCancel := context.WithCancel(context.Background())
	defer parkerCancel()

	osPack := newOsWrapper()
	if c.UserCreatedByTeleport {
		// Parker is only needed when the user was created by Teleport.
		err := osPack.startNewParker(
			parkerCtx,
			cmd.SysProcAttr.Credential,
			c.Login, &systemUser{u: localUser})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := setNeutralOOMScore(); err != nil {
		slog.WarnContext(ctx, "failed to adjust OOM score", "error", err)
	}

	// Set SELinux context for the child process if SELinux support is
	// enabled so the child process will be running with the correct SELinux
	// user, role and domain.
	if c.SetSELinuxContext {
		seContext, err := selinux.UserContext(c.Login)
		if err != nil {
			return nil, trace.Wrap(err, "failed to get SELinux context of login user")
		}

		// SetExecLabel changes the SELinux exec context for the
		// calling thread only, so we need to ensure that is the
		// thread that will create the child. We don't ever unlock
		// the thread as we're exiting after the child exits, and
		// we want to avoid another goroutine getting denied due to
		// running on this thread with a different (likely much more
		// restrictive) SELinux context.
		runtime.LockOSThread()
		if err := ocselinux.SetExecLabel(seContext); err != nil {
			return nil, trace.Wrap(err, "failed to set SELinux context")
		}
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Started command")

	parkerCancel()

	exitErr = waitForShell(terminatefd, cmd)
	return trace.Wrap(exitErr), nil
}

// setAuditSessionID ensures the audit login session ID is updated by
// either PAM if PAM is configured or us otherwise.
func setAuditSessionID(ctx context.Context, c ExecCommand, preLoginUID []byte, localUser *user.User, readyfd *os.File) error {
	// Depending of the PAM service, PAM may write to /proc/self/loginuid
	// if the 'pam_loginuid.so' module is enabled. We always want to
	// write to /proc/self/loginuid to ensure the kernel will update the
	// audit session ID for the next child process, but PAM may or may
	// not write to it. Even if 'pam_loginuid.so' is enabled, it won't
	// write to /proc/self/loginuid if the UID is the same as what's
	// currently in /proc/self/loginuid. In any case we can detect if
	// we need to write to /proc/self/loginuid ourselves by seeing if
	// /proc/self/loginuid changed after a PAM context was opened.
	writeLoginuid := true
	if c.PAMConfig != nil {
		postLoginuid, err := os.ReadFile(procLoginuid)
		if err != nil {
			return trace.Wrap(err)
		}
		if !bytes.Equal(preLoginUID, postLoginuid) {
			writeLoginuid = false
		}
	}

	if writeLoginuid {
		oldID, err := os.ReadFile(procSessionID)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := loginuid.Write(localUser.Uid); err != nil {
			return trace.Errorf("failed to write to loginuid: %w", err)
		}

		newID, err := os.ReadFile(procSessionID)
		if err != nil {
			return trace.Wrap(err)
		}

		slog.DebugContext(ctx, "Audit login session IDs", "old", string(oldID), "new", string(newID))
		// If the audit login session IDs are the same, the session ID
		// was not changed and ESR logging will not work correctly.
		if bytes.Equal(oldID, newID) {
			return trace.Errorf("audit login session ID was not changed")
		}
	}

	// Let the parent process know the audit login session ID has changed.
	if err := readyfd.Close(); err != nil {
		return trace.Errorf("failed to close audit login session ID: %w", err)
	}

	return nil
}

// waitForShell waits either for the command to return or the kill signal from the parent Teleport process.
func waitForShell(terminatefd *os.File, cmd *exec.Cmd) error {
	terminateChan := make(chan error)

	go func() {
		buf := make([]byte, 1)
		// Wait for the terminate file descriptor to be closed. The FD will be closed when Teleport
		// parent process wants to terminate the remote command and all childs.
		_, err := terminatefd.Read(buf)
		if errors.Is(err, io.EOF) {
			// Kill the shell process
			err = trace.Errorf("shell process has been killed: %w", cmd.Process.Kill())
		} else {
			err = trace.Errorf("failed to read from terminate file: %w", err)
		}
		terminateChan <- err
	}()

	go func() {
		// Wait for the command to exit. It doesn't make sense to print an error
		// message here because the shell has successfully started. If an error
		// occurred during shell execution or the shell exits with an error (like
		// running exit 2), the shell will print an error if appropriate and return
		// an exit code.
		err := cmd.Wait()

		terminateChan <- err
	}()

	// Wait only for the first error.
	// If the command returns then we don't need to wait for the error from cmd.Process.Kill().
	// If the command is being killed, then we don't care about the error code.
	err := <-terminateChan
	return err
}

// osWrapper wraps system calls, so we can replace them in tests.
type osWrapper struct {
	LookupGroup    func(name string) (*user.Group, error)
	LookupUser     func(username string) (*user.User, error)
	CommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func newOsWrapper() *osWrapper {
	return &osWrapper{
		LookupGroup:    user.LookupGroup,
		LookupUser:     user.Lookup,
		CommandContext: exec.CommandContext,
	}
}

// userInfo wraps user.User data into an interface, so we can override
// returned results in tests.
type userInfo interface {
	GID() string
	UID() string
	GroupIds() ([]string, error)
}

type systemUser struct {
	u *user.User
}

func (s *systemUser) GID() string {
	return s.u.Gid
}

func (s *systemUser) UID() string {
	return s.u.Uid
}

func (s *systemUser) GroupIds() ([]string, error) {
	return s.u.GroupIds()
}

// startNewParker starts a new parker process only if the requested user has been created
// by Teleport. Otherwise, does nothing.
func (o *osWrapper) startNewParker(ctx context.Context, credential *syscall.Credential,
	loginAsUser string, localUser userInfo,
) error {
	if credential == nil {
		// Empty credential, no reason to start the parker.
		return nil
	}

	group, err := o.LookupGroup(apiconstants.TeleportDropGroup)
	if err != nil {
		if isUnknownGroupError(err, apiconstants.TeleportDropGroup) {
			// The service group doesn't exist. Auto-provision is disabled, do nothing.
			return nil
		}
		return trace.Wrap(err)
	}

	groups, err := localUser.GroupIds()
	if err != nil {
		return trace.Wrap(err)
	}

	found := slices.Contains(groups, group.Gid)

	if !found {
		// Check if the new user guid matches the TeleportDropGroup. If not
		// this user hasn't been created by Teleport, and we don't need the parker.
		return nil
	}

	if err := o.newParker(ctx, *credential); err != nil {
		return trace.Wrap(err)
	}

	localUserCheck, err := o.LookupUser(loginAsUser)
	if err != nil {
		return trace.Wrap(err)
	}
	if localUser.UID() != localUserCheck.Uid || localUser.GID() != localUserCheck.Gid {
		return trace.BadParameter("user %q has been changed", loginAsUser)
	}

	return nil
}

const rootDirectory = "/"

func RunNetworking() (code int, err error) {
	// SIGQUIT is used by teleport to initiate graceful shutdown, waiting for
	// existing exec sessions to close before ending the process. For this to
	// work when closing the entire teleport process group, exec sessions must
	// ignore SIGQUIT signals.
	signal.Ignore(syscall.SIGQUIT)

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, fdName(CommandFile))
	if cmdfd == nil {
		return reexecconstants.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}
	logfd := os.NewFile(LogFile, fdName(LogFile))
	if logfd == nil {
		return reexecconstants.RemoteCommandFailure, trace.BadParameter("log pipe not found")
	}
	terminatefd := os.NewFile(TerminateFile, fdName(TerminateFile))
	if terminatefd == nil {
		return reexecconstants.RemoteCommandFailure, trace.BadParameter("terminate pipe not found")
	}

	// Read in the command payload.
	var c ExecCommand
	if err := json.NewDecoder(cmdfd).Decode(&c); err != nil {
		return reexecconstants.RemoteCommandFailure, trace.Wrap(err)
	}

	initLogger("networking", logfd, c.LogConfig)

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used for
	// networking requests.
	var pamEnvironment []string
	if c.PAMConfig != nil {
		// Open the PAM context.
		pamContext, err := pam.Open(&pamcfg.PAMConfig{
			ServiceName: c.PAMConfig.ServiceName,
			Login:       c.Login,
			Stdin:       os.Stdin,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
			// Set Teleport specific environment variables that PAM modules
			// like pam_script.so can pick up to potentially customize the
			// account/session.
			Env: c.PAMConfig.Environment,
		})
		if err != nil {
			return reexecconstants.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()

		pamEnvironment = pamContext.Environment()
	}

	// Once the PAM stack is called with parent process permissions, set the process uid
	// and gid to the requested user. This way, the user's networking requests will be
	// done with the user's permissions.
	localUser, err := user.Lookup(c.Login)
	if err != nil {
		return reexecconstants.RemoteCommandFailure, trace.NotFound("%s", err)
	}

	cred, err := host.GetHostUserCredential(localUser)
	if err != nil {
		return reexecconstants.RemoteCommandFailure, trace.Wrap(err)
	}

	if os.Getuid() != int(cred.Uid) || os.Getgid() != int(cred.Gid) {
		if !cred.NoSetGroups {
			groups := make([]int, len(cred.Groups))
			for i, g := range cred.Groups {
				groups[i] = int(g)
			}
			if err := unix.Setgroups(groups); err != nil {
				return reexecconstants.RemoteCommandFailure, trace.Wrap(err, "failed to set groups for networking process")
			}
		}
		if err := unix.Setgid(int(cred.Gid)); err != nil {
			return reexecconstants.RemoteCommandFailure, trace.Wrap(err, "failed to set gid for networking process")
		}
		if err := unix.Setuid(int(cred.Uid)); err != nil {
			return reexecconstants.RemoteCommandFailure, trace.Wrap(err, "failed to set uid for networking process")
		}
	}

	// Create a minimal default environment for the user.
	workingDir := rootDirectory

	hasAccess, err := checkHomeDir(localUser)
	if hasAccess && err == nil {
		workingDir = localUser.HomeDir
	}

	os.Setenv("HOME", localUser.HomeDir)
	os.Setenv("USER", c.Login)

	// Apply any additional environment variables from PAM.
	for _, kv := range pamEnvironment {
		key, value, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			return reexecconstants.RemoteCommandFailure, trace.BadParameter("bad environment variable from PAM, expected format \"key=value\" but got %q", kv)
		}
		if err := os.Setenv(key, value); err != nil {
			return reexecconstants.RemoteCommandFailure, trace.Wrap(err)
		}
	}

	// Ensure that the working directory is one that the local user has access to.
	if err := os.Chdir(workingDir); err != nil {
		return reexecconstants.RemoteCommandFailure, trace.Wrap(err, "failed to set working directory for networking process: %s", workingDir)
	}

	ffd := os.NewFile(ListenerFile, "listener")
	if ffd == nil {
		return reexecconstants.RemoteCommandFailure, trace.BadParameter("missing socket fd")
	}

	parentConn, err := uds.FromFile(ffd)
	_ = ffd.Close()
	if err != nil {
		return reexecconstants.RemoteCommandFailure, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Maintain a list of file paths to cleanup at the end of the process. This
	// ensures that file cleanup is handled by the child in cases where the parent
	// fails to cleanup due to filesystem namespace discrepancy (pam_namespace)
	var filePathsToCleanup []string
	defer func() {
		for _, path := range filePathsToCleanup {
			os.Remove(path)
		}
	}()

	// parentConn is a datagram Unix socket, which is not connection oriented
	// and thus won't unblock when the parent closes its side of the connection.
	// Instead we use an interrupt signal from the parent process to unblock.
	go func() {
		_, _ = terminatefd.Read(make([]byte, 1))
		parentConn.Close()
	}()

	for {
		buf := make([]byte, 1024)
		fbuf := make([]*os.File, 1)
		n, fn, err := uds.ReadWithFDs(parentConn, buf, fbuf)
		if err != nil {
			if isOKNetworkError(err) {
				// parent connection closed, process should exit.
				return reexecconstants.RemoteCommandSuccess, nil
			}
			slog.ErrorContext(ctx, "error reading networking request from parent", "err", err)
			continue
		}

		if fn == 0 {
			slog.ErrorContext(ctx, "networking request missing control file")
			continue
		}

		requestConn, err := uds.FromFile(fbuf[0])
		_ = fbuf[0].Close()
		if err != nil {
			slog.ErrorContext(ctx, "failed to get a connection from control file", "err", err)
			continue
		}

		var req networking.Request
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			requestConn.Write([]byte(trace.Wrap(err, "error parsing networking request").Error()))
			_ = requestConn.Close()
			continue
		}

		// Some PAM modules (e.g. pam_namespace) do not behave properly in multithreaded contexts.
		// Therefore we favor handling requests and cleanup on the main PAM thread for requests that
		// are expected to be impacted (unix socket listeners).
		switch req.Operation {
		case networking.NetworkingOperationDial, networking.NetworkingOperationListen:
			switch req.Network {
			case "tcp":
				// There are currently no known issues with tcp listen/dial in a multithreaded PAM context.
				go handleNetworkingRequest(ctx, requestConn, req)
			default:
				// Note: we don't currently support non-tcp network forwarding, so this branch is not
				// currently reached. If in the future we add unix socket forwarding similar to OpenSSH's
				// direct-streamlocal@openssh.com extension, we should revisit this multithreading limitation
				// to prevent performance degradation.
				filePaths := handleNetworkingRequest(ctx, requestConn, req)
				filePathsToCleanup = append(filePathsToCleanup, filePaths...)
			}
		case networking.NetworkingOperationListenAgent, networking.NetworkingOperationListenX11:
			// Agent and X11 forwarding requests should occur very rarely, so handling
			// them in the main thread should have negligible performance impact.
			cleanupFilePaths := handleNetworkingRequest(ctx, requestConn, req)
			filePathsToCleanup = append(filePathsToCleanup, cleanupFilePaths...)
		}
	}
}

func handleNetworkingRequest(ctx context.Context, conn *net.UnixConn, req networking.Request) []string {
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	go func() {
		defer cancel()
		_, _ = conn.Read(make([]byte, 1))
	}()

	netFile, filePaths, err := createNetworkingFile(ctx, req)
	if err != nil {
		conn.Write([]byte(trace.Wrap(err, "failed to create networking file").Error()))
		return nil
	}
	defer netFile.Close()

	if _, _, err := uds.WriteWithFDs(conn, nil, []*os.File{netFile}); err != nil {
		conn.Write([]byte(trace.Wrap(err, "failed to write networking file to control conn").Error()))
		return nil
	}
	return filePaths
}

func createNetworkingFile(ctx context.Context, req networking.Request) (*os.File, []string, error) {
	switch req.Operation {
	case networking.NetworkingOperationDial:
		var d net.Dialer
		conn, err := d.DialContext(ctx, req.Network, req.Address)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		defer conn.Close()

		connFD, err := getConnFile(conn)
		return connFD, nil, trace.Wrap(err)

	case networking.NetworkingOperationListen:
		listener, err := net.Listen(req.Network, req.Address)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		defer listener.Close()

		listenerFD, err := getListenerFile(listener)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return listenerFD, []string{listener.Addr().String()}, trace.Wrap(err)

	case networking.NetworkingOperationListenAgent:
		// Create a temp directory to hold the agent socket.
		sockDir, err := os.MkdirTemp("", "teleport-")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		sockPath := filepath.Join(sockDir, "agent.sock")

		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			os.RemoveAll(sockDir)
			return nil, nil, trace.Wrap(err)
		}
		defer listener.Close()

		listenerFD, err := getListenerFile(listener)
		if err != nil {
			os.RemoveAll(sockDir)
			return nil, nil, trace.Wrap(err)
		}

		return listenerFD, []string{sockPath, sockDir}, nil

	case networking.NetworkingOperationListenX11:
		listener, display, err := x11.OpenNewXServerListener(req.X11Request.DisplayOffset, req.X11Request.MaxDisplay, req.X11Request.ScreenNumber)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		defer listener.Close()

		removeCmd := x11.NewXAuthCommand(ctx, req.X11Request.XauthFile)
		if err := removeCmd.RemoveEntries(display); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		addCmd := x11.NewXAuthCommand(ctx, req.X11Request.XauthFile)
		if err := addCmd.AddEntry(x11.XAuthEntry{
			Display: display,
			Proto:   req.X11Request.AuthProtocol,
			Cookie:  req.X11Request.AuthCookie,
		}); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		listenerFD, err := getListenerFile(listener)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return listenerFD, []string{listener.Addr().String()}, trace.Wrap(err)

	default:
		return nil, nil, trace.BadParameter("unsupported networking operation %q", req.Operation)
	}
}

func getListenerFile(listener net.Listener) (*os.File, error) {
	switch l := listener.(type) {
	case *net.UnixListener:
		// Unlinking the socket here will cause the parent process to open a new
		// socket in the parent namespace, which may be inaccessible for the user.
		// Instead we close the listener without unlinking the socket, and cleanup
		// the socket in the child namepsace once the process is closed.
		l.SetUnlinkOnClose(false)
		listenerFD, err := l.File()
		return listenerFD, trace.Wrap(err)
	case *net.TCPListener:
		listenerFD, err := l.File()
		return listenerFD, trace.Wrap(err)
	default:
		return nil, trace.Errorf("expected listener to be of type *net.UnixListener or *net.TCPListener, but was %T", l)
	}
}

func getConnFile(conn net.Conn) (*os.File, error) {
	switch c := conn.(type) {
	case *net.UnixConn:
		connFD, err := c.File()
		return connFD, trace.Wrap(err)
	case *net.TCPConn:
		connFD, err := c.File()
		return connFD, trace.Wrap(err)
	default:
		return nil, trace.Errorf("expected connection to be of type *net.UnixConn or *net.TCPConn, but was %T", conn)
	}
}

// runCheckHomeDir checks if the active user's $HOME dir exists and is accessible.
func runCheckHomeDir() (code int) {
	code = reexecconstants.RemoteCommandSuccess
	if err := hasAccessibleHomeDir(); err != nil {
		switch {
		case trace.IsNotFound(err), trace.IsBadParameter(err):
			code = reexecconstants.HomeDirNotFound
		case trace.IsAccessDenied(err):
			code = reexecconstants.HomeDirNotAccessible
		default:
			code = reexecconstants.RemoteCommandFailure
		}
	}

	return code
}

// runPark does nothing, forever.
func runPark() (code int) {
	// Do not replace this with an empty select because there are no other
	// goroutines running so it will panic.
	for {
		time.Sleep(time.Hour)
	}
}

// RunAndExit will run the requested command and then exit. This wrapper
// allows Run{Command,Networking} to use defers and makes sure error messages
// are consistent across both.
func RunAndExit(commandType string) {
	var code int
	var err error

	switch commandType {
	case reexecconstants.ExecSubCommand:
		var execErr error
		execErr, err = RunCommand()
		if err != nil {
			code = reexecconstants.RemoteCommandFailure
		} else {
			code = exitCode(execErr)
		}
	case reexecconstants.NetworkingSubCommand:
		code, err = RunNetworking()
	case reexecconstants.CheckHomeDirSubCommand:
		code = runCheckHomeDir()
	case reexecconstants.ParkSubCommand:
		code = runPark()
	case reexecconstants.TrueSubCommand:
		// nothing to do
	case reexecconstants.SFTPSubCommand:
		initLogger("sftp", os.Stderr, ExecLogConfig{})
		err = reexecsftp.RunSFTP(slog.Default())
		if err != nil {
			code = 1
		}
	default:
		code, err = reexecconstants.RemoteCommandFailure, fmt.Errorf("unknown command type: %v", commandType)
	}
	if err != nil {
		// Write the error to stderr, where it can be seen by the parent teleport process and
		// propagated to the client.
		if code == reexecconstants.RemoteCommandFailure {
			fmt.Fprintf(os.Stderr, "Failed to launch: %v.\n", err)
		}

		// The "operation not permitted" error is expected from a variety of operations if the
		// teleport process is running as a non-root user and is trying to spawn a process for
		// a different OS user.
		if strings.Contains(err.Error(), "operation not permitted") {
			slog.ErrorContext(context.Background(), "Failed to launch subprocess, is Teleport running as root?", "command_type", commandType, "err", err)
		} else {
			slog.ErrorContext(context.Background(), "Failed to launch subprocess", "command_type", commandType, "err", err)
		}
	}
	os.Exit(code)
}

// MaybeReexec checks if the command-line arguments are those of a Teleport
// reexec command, and if so, runs the logic for the command (terminating the
// process at the end). Should be the first thing called in the main function
// for the Teleport binary or in the TestMain for packages that rely on
// reexecution.
func MaybeReexec() {
	if IsReexec() {
		RunAndExit(os.Args[1])
	}
}

// TODO(espadolini): remove IsReexec and RunAndExit in favor of requiring MaybeReexec, after enterprise is updated

// IsReexec determines if the current process is a teleport reexec command.
// Used by tests to reroute the execution to RunAndExit.
func IsReexec() bool {
	if len(os.Args) < 2 {
		return false
	}

	switch os.Args[1] {
	case reexecconstants.ExecSubCommand,
		reexecconstants.NetworkingSubCommand,
		reexecconstants.CheckHomeDirSubCommand,
		reexecconstants.ParkSubCommand,
		reexecconstants.TrueSubCommand,
		reexecconstants.SFTPSubCommand:
		return true
	default:
		return false
	}
}

// openFileAsUser opens a file as the given user to ensure proper access checks. This is unsafe and should not be used outside of
// bootstrapping reexec commands.
func openFileAsUser(localUser *user.User, path string) (file *os.File, err error) {
	if os.Args[1] != reexecconstants.ExecSubCommand {
		return nil, trace.Errorf("opening files as a user is only possible in a reexec context")
	}

	uid, err := strconv.Atoi(localUser.Uid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gid, err := strconv.Atoi(localUser.Gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	prevUID := os.Geteuid()
	prevGID := os.Getegid()

	defer func() {
		gidErr := syscall.Setegid(prevGID)
		uidErr := syscall.Seteuid(prevUID)
		if uidErr != nil || gidErr != nil {
			file.Close()
			slog.ErrorContext(context.Background(), "cannot proceed with invalid effective credentials", "uid_err", uidErr, "gid_err", gidErr, "error", err)
			os.Exit(reexecconstants.UnexpectedCredentials)
		}
	}()

	if err := syscall.Setegid(gid); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := syscall.Seteuid(uid); err != nil {
		return nil, trace.Wrap(err)
	}

	file, err = os.Open(path)
	return file, trace.ConvertSystemError(err)
}

func readUserEnv(localUser *user.User, path string) ([]string, error) {
	file, err := openFileAsUser(localUser, path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	envs, err := envutils.ReadEnvironment(context.Background(), file)
	return envs, trace.Wrap(err)
}

// BuildCommand constructs a command that will execute the user's shell. This
// function is run by Teleport while it's re-executing.
func BuildCommand(c *ExecCommand, localUser *user.User, pamEnvironment []string) (*exec.Cmd, error) {
	var cmd exec.Cmd
	isReexec := false

	// Get the login shell for the user (or fallback to the default).
	shellPath, err := shell.GetLoginShell(c.Login)
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to get login shell", "login", c.Login, "error", err)
	}
	if c.IsTestStub {
		shellPath = "/bin/sh"
	}

	// If a subsystem was requested, handle the known subsystems or error out;
	// if it's a normal command execution, and if no command was given,
	// configure a shell to run in 'login' mode. Otherwise, execute a command
	// through the shell.
	if c.RequestType == "subsystem" {
		switch c.Command {
		case "sftp":
			executable, err := os.Executable()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cmd.Path = executable
			cmd.Args = []string{executable, reexecconstants.SFTPSubCommand}
			isReexec = true
		default:
			return nil, trace.BadParameter("unsupported subsystem execution request %q", c.Command)
		}
	} else if c.Command == "" {
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
	env := &envutils.SafeEnv{
		"LANG=en_US.UTF-8",
		GetDefaultEnvPath(localUser.Uid),
		"HOME=" + localUser.HomeDir,
		"USER=" + c.Login,
		"SHELL=" + shellPath,
	}

	// Add in Teleport specific environment variables.
	env.AddFullTrusted(c.Environment...)

	// If any additional environment variables come from PAM, apply them as well.
	env.AddFullTrusted(pamEnvironment...)

	// If the server allows reading in of ~/.tsh/environment read it in
	// and pass environment variables along to new session.
	// User controlled values are added last to ensure administrator controlled sources take priority (duplicates ignored)
	if c.PermitUserEnvironment {
		path := filepath.Join(localUser.HomeDir, ".tsh", "environment")
		userEnvs, err := readUserEnv(localUser, path)
		if err != nil {
			if !trace.IsNotFound(err) {
				slog.WarnContext(context.Background(), "Could not read user environment", "error", err)
			}
		} else {
			env.AddFullUnique(userEnvs...)
		}
	}

	// after environment is fully built, set it to cmd
	cmd.Env = *env

	// set stdio. If a terminal was requested, the stdio fields all point to the same tty file.
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr

	if c.Terminal {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			// Note: leaving Ctty empty will default it to stdin fd, which is
			// set to our tty above.
		}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	}

	// Pass extra files for SFTP to grandchild.
	if c.RequestType == "subsystem" && c.Command == "sftp" {
		out := os.NewFile(FileTransferOutFile, "FileTransferOutFile")
		if out == nil {
			return nil, trace.NotFound("FileTransferOutFile file not found")
		}
		in := os.NewFile(FileTransferInFile, "FileTransferInFile")
		if in == nil {
			return nil, trace.NotFound("FileTransferInFile file not found")
		}
		audit := os.NewFile(AuditInFile, "AuditInFile")
		if audit == nil {
			return nil, trace.NotFound("AuditInFile file not found")
		}
		cmd.ExtraFiles = []*os.File{out, in, audit}
	}

	// Set the command's cwd to the user's $HOME, or "/" if
	// they don't have an existing home dir.
	// TODO (atburke): Generalize this to support Windows.
	hasAccess, err := checkHomeDir(localUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if hasAccess {
		cmd.Dir = localUser.HomeDir
	} else {
		// Write failure to find home dir to stdout, same as OpenSSH.
		msg := fmt.Sprintf("Could not set shell's cwd to home directory %q, defaulting to %q\n", localUser.HomeDir, rootDirectory)
		if _, err := cmd.Stdout.Write([]byte(msg)); err != nil {
			return nil, trace.Wrap(err)
		}
		cmd.Dir = rootDirectory
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
	if err := host.MaybeSetCommandCredentialAsUser(context.Background(), &cmd, localUser, slog.Default()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform OS-specific tweaks to the command.
	if isReexec {
		CommandOSTweaks(&cmd)
	} else {
		userCommandOSTweaks(&cmd)
	}

	return &cmd, nil
}

func coerceHomeDirError(usr *user.User, err error) error {
	if os.IsNotExist(err) {
		return trace.NotFound("home directory %q not found for user %q", usr.HomeDir, usr.Name)
	}

	if os.IsPermission(err) {
		return trace.AccessDenied("%q does not have permission to access %q", usr.Name, usr.HomeDir)
	}

	return err
}

// accessibleHomeDirMu is locked by [hasAccessibleHomeDir] to avoid race
// conditions between different goroutines while manipulating the global state
// of the process' working directory. This should be made into a more general
// global lock if we ever end up relying on this sort of temporary chdir in more
// places (but we really should not).
var accessibleHomeDirMu sync.Mutex

// hasAccessibleHomeDir checks if the current user has access to an existing home directory.
func hasAccessibleHomeDir() error {
	// this should usually be fetching a cached value
	currentUser, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}

	fi, err := os.Stat(currentUser.HomeDir)
	if err != nil {
		return trace.Wrap(coerceHomeDirError(currentUser, err))
	}

	if !fi.IsDir() {
		return trace.BadParameter("%q is not a directory", currentUser.HomeDir)
	}

	accessibleHomeDirMu.Lock()
	defer accessibleHomeDirMu.Unlock()

	cwd, err := os.Open(".")
	if err != nil {
		return trace.Wrap(err)
	}
	defer cwd.Close()

	// make sure we return to the original working directory; we ought to panic
	// if this fails but nothing should actually depend on the working directory
	// (which is why we can afford to just change it without additional
	// synchronization here) so we just let it slide
	defer cwd.Chdir()

	// attemping to cd into the target directory is the easiest, cross-platform way to test
	// whether or not the current user has access
	if err := os.Chdir(currentUser.HomeDir); err != nil {
		return trace.Wrap(coerceHomeDirError(currentUser, err))
	}

	return nil
}

// checkHomeDir checks if the user's home directory exists and is accessible to the user. Only catastrophic
// errors will be returned, which means a missing, inaccessible, or otherwise invalid home directory will result
// in a return of (false, nil)
func checkHomeDir(localUser *user.User) (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, trace.Wrap(err)
	}

	// don't spawn a subcommand if already running as the user in question
	if currentUser.Uid == localUser.Uid {
		if err := hasAccessibleHomeDir(); err != nil {
			if trace.IsNotFound(err) || trace.IsAccessDenied(err) || trace.IsBadParameter(err) {
				return false, nil
			}

			return false, trace.Wrap(err)
		}

		return true, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, trace.Wrap(err)
	}

	credential, err := host.GetHostUserCredential(localUser)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Build the "teleport exec" command.
	cmd := &exec.Cmd{
		Path: executable,
		Args: []string{executable, reexecconstants.CheckHomeDirSubCommand},
		Env:  []string{"HOME=" + localUser.HomeDir},
		Dir:  rootDirectory,
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:     true,
			Credential: credential,
		},
	}

	// Perform OS-specific tweaks to the command.
	CommandOSTweaks(cmd)

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState.ExitCode() == reexecconstants.RemoteCommandFailure {
			return false, trace.Wrap(err)
		}

		return false, nil
	}

	return true, nil
}

// Spawns a process with the given credentials, outliving the context.
func (o *osWrapper) newParker(ctx context.Context, credential syscall.Credential) error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := o.CommandContext(ctx, executable, reexecconstants.ParkSubCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &credential,
	}

	// Perform OS-specific tweaks to the command.
	parkerCommandOSTweaks(cmd)

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	// the process will get killed when the context ends, but we still need to
	// Wait on it
	go cmd.Wait()

	return nil
}

// WaitForSignal will wait for the other side of the pipe to signal, if not
// received, it will stop waiting and exit.
func WaitForSignal(ctx context.Context, fd *os.File, timeout time.Duration) error {
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
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "got context error while waiting for continue signal")
	case <-timer.C:
		return trace.LimitExceeded("timed out waiting for continue signal")
	case err := <-waitCh:
		return trace.Wrap(err)
	}
}

// TODO(espadolini): pass slog records in a fixed format to the parent process
// rather than handling the formatting here, so we can get rid of
// internal/logutils
func initLogger(name string, logWriter *os.File, cfg ExecLogConfig) {
	fields, err := logutils.ValidateFields(cfg.ExtraFields)
	if err != nil {
		return
	}

	switch cfg.Format {
	case "text", "":
		logger := slog.New(logutils.NewSlogTextHandler(logWriter, logutils.SlogTextHandlerConfig{
			Level:            cfg.Level,
			EnableColors:     cfg.EnableColors,
			ConfiguredFields: fields,
			Padding:          cfg.Padding,
		}))
		slog.SetDefault(logger.With(logconstants.ComponentKey, name))
	case "json":
		logger := slog.New(logutils.NewSlogJSONHandler(logWriter, logutils.SlogJSONHandlerConfig{
			Level:            cfg.Level,
			ConfiguredFields: fields,
		}))
		slog.SetDefault(logger.With(logconstants.ComponentKey, name))
	default:
		return
	}
}

// isUseOfClosedNetworkError is [utils.IsUseOfClosedNetworkError].
func isUseOfClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), apiconstants.UseOfClosedNetworkConnection)
}

// isFailedToSendCloseNotifyError is [utils.IsFailedToSendCloseNotifyError].
func isFailedToSendCloseNotifyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), apiconstants.FailedToSendCloseNotify)
}

// isOKNetworkError is [utils.IsOKNetworkError].
func isOKNetworkError(err error) bool {
	// trace.Aggregate contains at least one error and all the errors are
	// non-nil
	var a trace.Aggregate
	if errors.As(trace.Unwrap(err), &a) {
		for _, err := range a.Errors() {
			if !isOKNetworkError(err) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, io.EOF) || isUseOfClosedNetworkError(err) || isFailedToSendCloseNotifyError(err)
}

// CommandExecutor is wrapper around *exec.Cmd that handles creating and closing pipes
// used to communicate with child process when reexecuting teleport
type CommandExecutor struct {
	*exec.Cmd

	ctx context.Context

	// cont is used to send the continue signal from the parent process
	// to the child process.
	cont *os.File

	// ready is used to send the ready signal from the child process
	// to the parent process. If ESR is enabled, the child signals after
	// the audit session login ID (auid) is received.
	ready *os.File

	// killShell is used to send kill signal to the child process
	// to terminate the shell.
	killShell *os.File

	childFiles  []*os.File
	parentFiles []io.Closer

	bpfEnabled bool
	logger     *slog.Logger
}

func (e *CommandExecutor) childToParentPipe(fd FileFD) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if e.childFiles, err = addFile(e.childFiles, w, fd); err != nil {
		r.Close()
		w.Close()
		return nil, trace.Wrap(err)
	}
	e.parentFiles = append(e.parentFiles, r)
	return r, nil
}

func (e *CommandExecutor) parentToChildPipe(fd FileFD) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if e.childFiles, err = addFile(e.childFiles, r, fd); err != nil {
		r.Close()
		w.Close()
		return nil, trace.Wrap(err)
	}
	e.parentFiles = append(e.parentFiles, w)
	return w, nil
}

func addFile(slice []*os.File, file *os.File, fd FileFD) ([]*os.File, error) {
	idx := int(fd)
	if idx >= len(slice) {
		slice = slices.Grow(slice, idx+1-len(slice))
		clear(slice[len(slice) : idx+1])
		slice = slice[:idx+1]
	}
	if slice[idx] != nil {
		return nil, trace.BadParameter("file already exists")
	}
	slice[idx] = file
	return slice, nil
}

func (e *CommandExecutor) Close() error {
	var errs []error
	for _, closer := range e.parentFiles {
		if err := closer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	}
	for _, closer := range e.childFiles {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

func (e *CommandExecutor) Start() error {
	if err := e.Cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	for i, file := range e.childFiles {
		if file == nil {
			continue
		}
		if err := file.Close(); err != nil {
			e.logger.WarnContext(e.ctx, "Failed to close child fd", "error", err, "fd", i)
		}
	}
	e.childFiles = nil
	return nil
}

// The child does not signal until it completes PAM setup, which can take an arbitrary
// amount of time, so we use a reasonably long timeout to avoid dubious lockouts.
const childReadyWaitTimeout = 3 * time.Minute

func (e *CommandExecutor) WaitForChild() error {
	if e.ready == nil {
		return nil
	}
	var waitErr error
	if e.bpfEnabled {
		if waitErr = WaitForSignal(e.ctx, e.ready, childReadyWaitTimeout); waitErr != nil {
			e.logger.ErrorContext(e.ctx, "Child process never became ready.", "error", waitErr)
		}
	}

	closeErr := e.ready.Close()
	e.ready = nil

	return trace.NewAggregate(waitErr, closeErr)
}

// Continue will resume execution of the process after it completes its
// pre-processing routine if Enhanced Session Recording is enabled.
// Otherwise, this method is a no-op.
func (e *CommandExecutor) Continue() error {
	if e.cont == nil {
		return nil
	}
	err := e.cont.Close()
	e.cont = nil
	return trace.Wrap(err)
}

// Kill will send signal to the child process that it should terminate the command
func (e *CommandExecutor) Kill() error {
	if e.killShell == nil {
		return nil
	}
	err := e.killShell.Close()
	e.killShell = nil
	return trace.Wrap(err)
}

// ConfigureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
// Context passed to this function is used only for logging and waiting for
// the ready signal from child, the returned command will not be terminated
// when it's done
func ConfigureCommand(ctx context.Context, logger *slog.Logger, childLogWriter io.Writer, command *ExecCommand, execType string, extraFiles map[FileFD]*os.File) (_ *CommandExecutor, err error) {
	executor := &CommandExecutor{
		ctx:    ctx,
		logger: logger,
	}
	defer func() {
		if err != nil {
			if closeErr := executor.Close(); closeErr != nil {
				err = trace.NewAggregate(err, closeErr)
			}
			executor = nil
		}
	}()

	logFileWriter, canReuseLogWriter := childLogWriter.(*os.File)
	if !canReuseLogWriter {
		// Create a pipe so we can pass the writing side as an *os.File to the child process.
		// Then we can copy from the reading side to the log writer (e.g. syslog, log file w/ concurrency protection).
		r, err := executor.childToParentPipe(LogFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Copy logs from the child process to the parent process over
		// the pipe until it is closed by the child context.
		go func() {
			if _, err := io.Copy(childLogWriter, r); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				slog.ErrorContext(ctx, "Failed to copy logs over pipe", "error", err)
			}
		}()
	}
	cmd, err := executor.parentToChildPipe(CommandFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.cont, err = executor.parentToChildPipe(ContinueFile); err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.killShell, err = executor.parentToChildPipe(TerminateFile); err != nil {
		return nil, trace.Wrap(err)
	}
	if executor.ready, err = executor.childToParentPipe(ReadyFile); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a os.Pipe and start copying over the payload to execute. While the
	// pipe buffer is quite large (64k) some users have run into the pipe
	// blocking writes on much smaller buffers (7k) leading to Teleport being
	// unable to run some exec commands.
	//
	// To not depend on the OS implementation of a pipe, instead the copy should
	// be non-blocking. The io.Copy will be closed when either when the child
	// process has fully read in the payload or the process exits with an error
	// (and closes all child file descriptors).
	//
	// See the below for details.
	//
	//   https://man7.org/linux/man-pages/man7/pipe.7.html
	buffer := &bytes.Buffer{}
	if err := json.NewEncoder(buffer).Encode(command); err != nil {
		return nil, trace.Wrap(err)
	}
	go copyCommand(ctx, cmd, buffer)

	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build env for `teleport exec`.
	env := &envutils.SafeEnv{}
	env.AddExecEnvironment()

	// The channel/request type determines the subcommand to execute.
	var subCommand string
	switch execType {
	case reexecconstants.NetworkingSubCommand:
		subCommand = reexecconstants.NetworkingSubCommand

		// Unset XAUTHORITY for the networking command as the SSH session
		// process given to the user will not have it set which can cause
		// issues with the X11 forwarding.
		env.Remove(x11.XAuthFileEnvVar)
	default:
		subCommand = reexecconstants.ExecSubCommand
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, subCommand}

	executor.bpfEnabled = command.RecordWithBPF

	childFiles := slices.Clone(executor.childFiles)

	if canReuseLogWriter {
		childFiles, err = addFile(childFiles, logFileWriter, LogFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for fd, file := range extraFiles {
		childFiles, err = addFile(childFiles, file, fd)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Build the "teleport exec" command.
	executor.Cmd = &exec.Cmd{
		Stdin:      childFiles[0],
		Stdout:     childFiles[1],
		Stderr:     childFiles[2],
		Path:       executable,
		Args:       args,
		Env:        *env,
		ExtraFiles: childFiles[3:],
	}

	// Perform OS-specific tweaks to the command.
	CommandOSTweaks(executor.Cmd)

	return executor, nil
}

// copyCommand will copy the provided command to the child process over the
// pipe attached to the context.
func copyCommand(ctx context.Context, cmdw *os.File, buffer *bytes.Buffer) {
	// Write command bytes to pipe. The child process will read the command
	// to execute from this pipe.
	if _, err := io.Copy(cmdw, buffer); err != nil {
		slog.ErrorContext(ctx, "Failed to copy command over pipe", "error", err)
	}

	if err := cmdw.Close(); err != nil {
		slog.ErrorContext(ctx, "Failed to close command pipe", "error", err)
	}
}

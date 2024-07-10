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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auditd"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/networking"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/envutils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

// FileFD is a file descriptor passed down from a parent process when
// Teleport is re-executing itself.
type FileFD = uintptr

const (
	// CommandFile is used to pass the command and arguments that the
	// child process should execute from the parent process.
	CommandFile FileFD = 3 + iota
	// ContinueFile is used to communicate to the child process that
	// it can continue after the parent process assigns a cgroup to the
	// child process.
	ContinueFile
	// ReadyFile is used to communicate to the parent process that
	// the child has completed any setup operations that must occur before
	// the child is placed into its cgroup.
	ReadyFile
	// TerminateFile is used to communicate to the child process that
	// the interactive terminal should be killed as the client ended the
	// SSH session and without termination the terminal process will be assigned
	// to pid 1 and "live forever". Killing the shell should not prevent processes
	// preventing SIGHUP to be reassigned (ex. processes running with nohup).
	TerminateFile
	// ErrorFile is used to communicate any errors terminating the child process
	// to the parent process
	ErrorFile
	// PTYFile is a PTY the parent process passes to the child process.
	PTYFile
	// TTYFile is a TTY the parent process passes to the child process.
	TTYFile

	// FirstExtraFile is the first file descriptor that will be valid when
	// extra files are passed to child processes without a terminal.
	FirstExtraFile FileFD = ErrorFile + 1
)

func fdName(f FileFD) string {
	return fmt.Sprintf("/proc/self/fd/%d", f)
}

// ExecCommand contains the payload to "teleport exec" which will be used to
// construct and execute a shell.
type ExecCommand struct {
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

	// ExtraFilesLen is the number of extra files that are inherited from
	// the parent process. These files start at file descriptor 3 of the
	// child process, and are only valid for processes without a terminal.
	ExtraFilesLen int `json:"extra_files_len"`
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
	// The hostname of the node.
	Hostname string `json:"hostname"`

	// RemoteAddr is the address of the remote host.
	RemoteAddr [4]int32 `json:"remote_addr"`

	// UtmpPath is the path of the system utmp database.
	UtmpPath string `json:"utmp_path,omitempty"`

	// WtmpPath is the path of the system wtmp log.
	WtmpPath string `json:"wtmp_path,omitempty"`

	// BtmpPath is the path of the system btmp log.
	BtmpPath string `json:"btmp_path,omitempty"`
}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command.
func RunCommand() (errw io.Writer, code int, err error) {
	// SIGQUIT is used by teleport to initiate graceful shutdown, waiting for
	// existing exec sessions to close before ending the process. For this to
	// work when closing the entire teleport process group, exec sessions must
	// ignore SIGQUIT signals.
	signal.Ignore(syscall.SIGQUIT)

	// errorWriter is used to return any error message back to the client. By
	// default, it writes to stdout, but if a TTY is allocated, it will write
	// to it instead.
	errorWriter := os.Stdout

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, fdName(CommandFile))
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}
	contfd := os.NewFile(ContinueFile, fdName(ContinueFile))
	if contfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
	}
	readyfd := os.NewFile(ReadyFile, fdName(ReadyFile))
	if readyfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("ready pipe not found")
	}

	// Ensure that the ready signal is sent if a failure causes execution
	// to terminate prior to actually becoming ready to unblock the parent process.
	defer func() {
		if readyfd == nil {
			return
		}

		_ = readyfd.Close()
	}()

	termiantefd := os.NewFile(TerminateFile, fdName(TerminateFile))
	if termiantefd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("terminate pipe not found")
	}

	// Read in the command payload.
	var c ExecCommand
	if err := json.NewDecoder(cmdfd).Decode(&c); err != nil {
		return io.Discard, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	auditdMsg := auditd.Message{
		SystemUser:   c.Login,
		TeleportUser: c.Username,
		ConnAddress:  c.ClientAddress,
		TTYName:      c.TerminalName,
	}

	if err := auditd.SendEvent(auditd.AuditUserLogin, auditd.Success, auditdMsg); err != nil {
		// Currently, this logs nothing. Related issue https://github.com/gravitational/teleport/issues/17318
		log.WithError(err).Debugf("failed to send user start event to auditd: %v", err)
	}

	defer func() {
		if err != nil {
			if errors.Is(err, user.UnknownUserError(c.Login)) {
				if err := auditd.SendEvent(auditd.AuditUserErr, auditd.Failed, auditdMsg); err != nil {
					log.WithError(err).Debugf("failed to send UserErr event to auditd: %v", err)
				}
				return
			}
		}

		if err := auditd.SendEvent(auditd.AuditUserEnd, auditd.Success, auditdMsg); err != nil {
			log.WithError(err).Debugf("failed to send UserEnd event to auditd: %v", err)
		}
	}()

	var tty *os.File
	var pty *os.File
	uaccEnabled := false

	// If a terminal was requested, file descriptors 6 and 7 always point to the
	// PTY and TTY. Extract them and set the controlling TTY. Otherwise, connect
	// std{in,out,err} directly.
	if c.Terminal {
		pty = os.NewFile(PTYFile, fdName(PTYFile))
		tty = os.NewFile(TTYFile, fdName(TTYFile))
		if pty == nil || tty == nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("pty and tty not found")
		}
		errorWriter = tty
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	var pamEnvironment []string
	if c.PAMConfig != nil {
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
			stdout = io.Discard
			stderr = io.Discard
		}

		// Open the PAM context.
		pamContext, err := pam.Open(&servicecfg.PAMConfig{
			ServiceName: c.PAMConfig.ServiceName,
			UsePAMAuth:  c.PAMConfig.UsePAMAuth,
			Login:       c.Login,
			// Set Teleport specific environment variables that PAM modules
			// like pam_script.so can pick up to potentially customize the
			// account/session.
			Env:    c.PAMConfig.Environment,
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		})
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()

		// Save off any environment variables that come from PAM.
		pamEnvironment = pamContext.Environment()
	}

	// Alert the parent process that the child process has completed any setup operations,
	// and that we are now waiting for the continue signal before proceeding. This is needed
	// to ensure that PAM changing the cgroup doesn't bypass enhanced recording.
	if err := readyfd.Close(); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	readyfd = nil

	localUser, err := user.Lookup(c.Login)
	if err != nil {
		if uaccErr := uacc.LogFailedLogin(c.UaccMetadata.BtmpPath, c.Login, c.UaccMetadata.Hostname, c.UaccMetadata.RemoteAddr); uaccErr != nil {
			log.WithError(uaccErr).Debug("uacc unsupported.")
		}
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	if c.Terminal {
		err = uacc.Open(c.UaccMetadata.UtmpPath, c.UaccMetadata.WtmpPath, c.Login, c.UaccMetadata.Hostname, c.UaccMetadata.RemoteAddr, tty)
		// uacc support is best-effort, only enable it if Open is successful.
		// Currently, there is no way to log this error out-of-band with the
		// command output, so for now we essentially ignore it.
		if err == nil {
			uaccEnabled = true
		} else {
			log.WithError(err).Debug("uacc unsupported.")
		}
	}

	// Build the actual command that will launch the shell.
	cmd, err := buildCommand(&c, localUser, tty, pty, pamEnvironment)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait until the continue signal is received from Teleport signaling that
	// the child process has been placed in a cgroup.
	err = waitForSignal(contfd, 10*time.Second)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
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
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
	}

	if err := setNeutralOOMScore(); err != nil {
		log.WithError(err).Warnf("failed to adjust OOM score")
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	parkerCancel()

	err = waitForShell(termiantefd, cmd)

	if uaccEnabled {
		uaccErr := uacc.Close(c.UaccMetadata.UtmpPath, c.UaccMetadata.WtmpPath, tty)
		if uaccErr != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(uaccErr)
		}
	}

	return io.Discard, exitCode(err), trace.Wrap(err)
}

// waitForShell waits either for the command to return or the kill signal from the parent Teleport process.
func waitForShell(termiantefd *os.File, cmd *exec.Cmd) error {
	terminateChan := make(chan error)

	go func() {
		buf := make([]byte, 1)
		// Wait for the terminate file descriptor to be closed. The FD will be closed when Teleport
		// parent process wants to terminate the remote command and all childs.
		_, err := termiantefd.Read(buf)
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

	group, err := o.LookupGroup(types.TeleportServiceGroup)
	if err != nil {
		if isUnknownGroupError(err, types.TeleportServiceGroup) {
			// The service group doesn't exist. Auto-provision is disabled, do nothing.
			return nil
		}
		return trace.Wrap(err)
	}

	groups, err := localUser.GroupIds()
	if err != nil {
		return trace.Wrap(err)
	}

	found := false
	for _, localUserGroup := range groups {
		if localUserGroup == group.Gid {
			found = true
			break
		}
	}

	if !found {
		// Check if the new user guid matches the TeleportServiceGroup. If not
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

func RunNetworking() (errw io.Writer, code int, err error) {
	// errorWriter is used to return any error message back to the client.
	// Use stderr so that it's not forwarded to the remote client.
	errorWriter := os.Stderr

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, fdName(CommandFile))
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}

	// Parent receives any errors on the sixth file descriptor.
	errfd := os.NewFile(ErrorFile, fdName(ErrorFile))
	if errfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("error pipe not found")
	}

	defer func() {
		writeChildError(errfd, err)
	}()

	// Read in the command payload.
	var c ExecCommand
	if err := json.NewDecoder(cmdfd).Decode(&c); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	var pamEnvironment []string
	if c.PAMConfig != nil {
		// Open the PAM context.
		pamContext, err := pam.Open(&servicecfg.PAMConfig{
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
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()

		pamEnvironment = pamContext.Environment()
	}

	// Once the PAM stack is called with parent process permissions, set the process uid
	// and gid to the requested user. This way, the user's networking requests will be
	// done with the user's permissions.
	localUser, err := user.Lookup(c.Login)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.NotFound(err.Error())
	}

	cred, err := getCmdCredential(localUser)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	if !cred.NoSetGroups {
		groups := make([]int, len(cred.Groups))
		for i, g := range cred.Groups {
			groups[i] = int(g)
		}
		if err := unix.Setgroups(groups); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err, "failed to set groups for networking process")
		}
	}
	if err := unix.Setgid(int(cred.Gid)); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err, "failed to set gid for networking process")
	}
	if err := unix.Setuid(int(cred.Uid)); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err, "failed to set uid for networking process")
	}

	// Create a minimal default environment for the user.
	os.Setenv("HOME", localUser.HomeDir)
	os.Setenv("USER", c.Login)

	// Apply any additional environment variables from PAM.
	for _, kv := range pamEnvironment {
		kvSplit := strings.SplitN(strings.TrimSpace(kv), "=", 2)
		if len(kvSplit) != 2 {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("bad environment variable from PAM, expected format \"key=value\" but got %q", kv)
		}
		if err := os.Setenv(kvSplit[0], kvSplit[1]); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
	}

	if _, code, _ := runCheckHomeDir(); code == teleport.HomeDirNotFound {
		os.Setenv("HOME", "/")
	}

	// Ensure that the working directory is one that the local user has access to.
	if err := unix.Chdir(os.Getenv("HOME")); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err, "failed to set working directory for networking process")
	}

	// Ensure that the working directory is one that the local user has access to.
	if err := unix.Chdir("/"); err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err, "failed to set working directory for networking process")
	}

	// build forwarder from first extra file that was passed to command
	ffd := os.NewFile(FirstExtraFile, "listener")
	if ffd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("missing socket fd")
	}

	parentConn, err := uds.FromFile(ffd)
	ffd.Close()
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		buf := make([]byte, 1024)
		fbuf := make([]*os.File, 1)
		n, fn, err := uds.ReadWithFDs(parentConn, buf, fbuf)
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return errorWriter, teleport.RemoteCommandSuccess, nil
			}
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		if fn == 0 {
			slog.ErrorContext(ctx, "Networking request requires a control file.")
			continue
		}

		controlConn, err := uds.FromFile(fbuf[0])
		_ = fbuf[0].Close()
		if err != nil {
			slog.ErrorContext(ctx, "Failed to get a connection from control file.")
			continue
		}

		// Starting a new goroutine takes us out of the current PAM context,
		// so we handle requests synchronously.
		handleNetworkingRequest(ctx, controlConn, buf[:n])
	}
}

func handleNetworkingRequest(ctx context.Context, conn *net.UnixConn, payload []byte) {
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	go func() {
		defer cancel()
		_, _ = conn.Read(make([]byte, 1))
	}()

	var req networking.Request
	if err := json.Unmarshal(payload, &req); err != nil {
		slog.With("error", err).ErrorContext(ctx, "Error parsing networking request.")
		return
	}

	log := slog.With("request", req)
	log.Debug("Handling networking request")

	netFile, err := createNetworkingFile(ctx, req)
	if err != nil {
		log.With("error", err).ErrorContext(ctx, "Error creating networking file.")
		conn.Write([]byte(err.Error()))
		return
	}
	defer netFile.Close()

	if _, _, err := uds.WriteWithFDs(conn, nil, []*os.File{netFile}); err != nil {
		log.With("error", err.Error()).ErrorContext(ctx, "Failed to write networking file to control conn.")
	}
}

func createNetworkingFile(ctx context.Context, req networking.Request) (*os.File, error) {
	switch req.Operation {
	case networking.NetworkingOperationDial:
		var d net.Dialer
		conn, err := d.DialContext(ctx, req.Network, req.Address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer conn.Close()

		connFD, err := getConnFile(conn)
		return connFD, trace.Wrap(err)

	case networking.NetworkingOperationListen:
		listener, err := newListener(ctx, req.Network, req.Address)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer listener.Close()

		listenerFD, err := getListenerFile(listener)
		return listenerFD, trace.Wrap(err)

	case networking.NetworkingOperationListenAgent:
		// Create a temp directory to hold the agent socket.
		sockDir, err := os.MkdirTemp(os.TempDir(), "teleport-")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Update the agent forwarding socket dir with more restrictive permissions.
		if err = os.Chmod(sockDir, teleport.PrivateDirMode); err != nil {
			return nil, trace.Wrap(err)
		}

		socketPath := filepath.Join(sockDir, fmt.Sprintf("teleport-%v.socket", os.Getpid()))

		listener, err := newListener(ctx, "unix", socketPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer listener.Close()

		listenerFD, err := getListenerFile(listener)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return listenerFD, trace.Wrap(err)

	case networking.NetworkingOperationListenX11:
		listener, display, err := x11.OpenNewXServerListener(req.X11Request.DisplayOffset, req.X11Request.MaxDisplay, req.X11Request.ScreenNumber)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer listener.Close()

		// Setup the user's local xauth file to interface with the local x11 listener.
		removeCmd := x11.NewXAuthCommand(ctx, "")
		if err := removeCmd.RemoveEntries(display); err != nil {
			return nil, trace.Wrap(err)
		}

		addCmd := x11.NewXAuthCommand(ctx, "")
		if err := addCmd.AddEntry(x11.XAuthEntry{
			Display: display,
			Proto:   req.X11Request.AuthProtocol,
			Cookie:  req.X11Request.AuthCookie,
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		listenerFD, err := getListenerFile(listener)
		return listenerFD, trace.Wrap(err)

	default:
		return nil, trace.BadParameter("unsupported networking operation %q", req.Operation)
	}
}

func getListenerFile(listener net.Listener) (*os.File, error) {
	switch l := listener.(type) {
	case *net.UnixListener:
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

// newListener creates a new network listener with address reuse disabled.
func newListener(ctx context.Context, network string, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, addr string, conn syscall.RawConn) error {
			var err error
			err2 := conn.Control(func(descriptor uintptr) {
				// Disable address reuse to prevent socket replacement.
				err = syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
			})
			return trace.NewAggregate(err2, err)
		},
	}

	listener, err := lc.Listen(ctx, network, addr)
	return listener, trace.Wrap(err)
}

// runCheckHomeDir check's if the active user's $HOME dir exists.
func runCheckHomeDir() (errw io.Writer, code int, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return io.Discard, teleport.HomeDirNotFound, nil
	}
	if !utils.IsDir(home) {
		return io.Discard, teleport.HomeDirNotFound, nil
	}
	return io.Discard, teleport.RemoteCommandSuccess, nil
}

// runPark does nothing, forever.
func runPark() (errw io.Writer, code int, err error) {
	// Do not replace this with an empty select because there are no other
	// goroutines running so it will panic.
	for {
		time.Sleep(time.Hour)
	}
}

// RunAndExit will run the requested command and then exit. This wrapper
// allows Run{Command,Forward} to use defers and makes sure error messages
// are consistent across both.
func RunAndExit(commandType string) {
	var w io.Writer
	var code int
	var err error

	switch commandType {
	case teleport.ExecSubCommand:
		w, code, err = RunCommand()
	case teleport.NetworkingSubCommand:
		w, code, err = RunNetworking()
	case teleport.CheckHomeDirSubCommand:
		w, code, err = runCheckHomeDir()
	case teleport.ParkSubCommand:
		w, code, err = runPark()
	default:
		w, code, err = os.Stderr, teleport.RemoteCommandFailure, fmt.Errorf("unknown command type: %v", commandType)
	}
	if err != nil {
		s := fmt.Sprintf("Failed to launch: %v.\r\n", trace.DebugReport(err))
		io.Copy(w, bytes.NewBufferString(s))
	}
	os.Exit(code)
}

// IsReexec determines if the current process is a teleport reexec command.
// Used by tests to reroute the execution to RunAndExit.
func IsReexec() bool {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case teleport.ExecSubCommand, teleport.NetworkingSubCommand,
			teleport.CheckHomeDirSubCommand, teleport.ParkSubCommand, teleport.SFTPSubCommand:
			return true
		}
	}

	return false
}

// buildCommand constructs a command that will execute the users shell. This
// function is run by Teleport while it's re-executing.
func buildCommand(c *ExecCommand, localUser *user.User, tty *os.File, pty *os.File, pamEnvironment []string) (*exec.Cmd, error) {
	var cmd exec.Cmd
	isReexec := false

	// Get the login shell for the user (or fallback to the default).
	shellPath, err := shell.GetLoginShell(c.Login)
	if err != nil {
		log.Debugf("Failed to get login shell for %v: %v.", c.Login, err)
	}
	if c.IsTestStub {
		shellPath = "/bin/sh"
	}

	// If a subsystem was requested, handle the known subsystems or error out;
	// if it's a normal command execution, and if no command was given,
	// configure a shell to run in 'login' mode. Otherwise, execute a command
	// through the shell.
	if c.RequestType == sshutils.SubsystemRequest {
		switch c.Command {
		case teleport.SFTPSubsystem:
			executable, err := os.Executable()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cmd.Path = executable
			cmd.Args = []string{executable, teleport.SFTPSubCommand}
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
		getDefaultEnvPath(localUser.Uid, defaultLoginDefsPath),
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
		filename := filepath.Join(localUser.HomeDir, ".tsh", "environment")
		userEnvs, err := envutils.ReadEnvironmentFile(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		env.AddFullUnique(userEnvs...)
	}

	// after environment is fully built, set it to cmd
	cmd.Env = *env

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
			// Note: leaving Ctty empty will default it to stdin fd, which is
			// set to our tty above.
		}
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		// If a terminal was not requested, and extra files were specified
		// to be passed to the child, open them so that they can be passed
		// to the grandchild.
		if c.ExtraFilesLen > 0 {
			cmd.ExtraFiles = make([]*os.File, c.ExtraFilesLen)
			for i := 0; i < c.ExtraFilesLen; i++ {
				fd := FirstExtraFile + uintptr(i)
				f := os.NewFile(fd, strconv.Itoa(int(fd)))
				if f == nil {
					return nil, trace.NotFound("extra file %d not found", fd)
				}
				cmd.ExtraFiles[i] = f
			}
		}
	}

	// Set the command's cwd to the user's $HOME, or "/" if
	// they don't have an existing home dir.
	// TODO (atburke): Generalize this to support Windows.
	exists, err := CheckHomeDir(localUser)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if exists {
		cmd.Dir = localUser.HomeDir
	} else if !exists {
		// Write failure to find home dir to stdout, same as OpenSSH.
		msg := fmt.Sprintf("Could not set shell's cwd to home directory %q, defaulting to %q\n", localUser.HomeDir, string(os.PathSeparator))
		if _, err := cmd.Stdout.Write([]byte(msg)); err != nil {
			return nil, trace.Wrap(err)
		}
		cmd.Dir = string(os.PathSeparator)
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
	credential, err := getCmdCredential(localUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if os.Getuid() != int(credential.Uid) || os.Getgid() != int(credential.Gid) {
		cmd.SysProcAttr.Credential = credential
		log.Debugf("Creating process with UID %v, GID: %v, and Groups: %v.",
			credential.Uid, credential.Gid, credential.Groups)
	} else {
		log.Debugf("Creating process with ambient credentials UID %v, GID: %v, Groups: %v.",
			credential.Uid, credential.Gid, credential.Groups)
	}

	// Perform OS-specific tweaks to the command.
	if isReexec {
		reexecCommandOSTweaks(&cmd)
	} else {
		userCommandOSTweaks(&cmd)
	}

	return &cmd, nil
}

// ConfigureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
func ConfigureCommand(ctx *ServerContext, extraFiles ...*os.File) (*exec.Cmd, error) {
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
	cmdmsg, err := ctx.ExecCommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cmdmsg.Terminal {
		cmdmsg.ExtraFilesLen = len(extraFiles)
	}

	go copyCommand(ctx, cmdmsg)

	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	executableDir, _ := filepath.Split(executable)

	// The channel/request type determines the subcommand to execute (execution or
	// port forwarding).
	var subCommand string
	switch ctx.ExecType {
	case teleport.NetworkingSubCommand:
		subCommand = teleport.NetworkingSubCommand
	default:
		subCommand = teleport.ExecSubCommand
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, subCommand}

	// build env for `teleport exec`
	env := &envutils.SafeEnv{}
	if subCommand == teleport.ExecSubCommand {
		env.AddExecEnvironment()
	}

	// Build the "teleport exec" command.
	cmd := &exec.Cmd{
		Path: executable,
		Args: args,
		Dir:  executableDir,
		Env:  *env,
		ExtraFiles: []*os.File{
			ctx.cmdr,
			ctx.contr,
			ctx.readyw,
			ctx.killShellr,
			ctx.errw,
		},
	}
	// Add extra files if applicable.
	if len(extraFiles) > 0 {
		cmd.ExtraFiles = append(cmd.ExtraFiles, extraFiles...)
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	return cmd, nil
}

// copyCommand will copy the provided command to the child process over the
// pipe attached to the context.
func copyCommand(ctx *ServerContext, cmdmsg *ExecCommand) {
	defer func() {
		err := ctx.cmdw.Close()
		if err != nil {
			log.Errorf("Failed to close command pipe: %v.", err)
		}

		// Set to nil so the close in the context doesn't attempt to re-close.
		ctx.cmdw = nil
	}()

	// Write command bytes to pipe. The child process will read the command
	// to execute from this pipe.
	if err := json.NewEncoder(ctx.cmdw).Encode(cmdmsg); err != nil {
		log.Errorf("Failed to copy command over pipe: %v.", err)
		return
	}
}

// CheckHomeDir checks if the user's home dir exists
func CheckHomeDir(localUser *user.User) (bool, error) {
	if fi, err := os.Stat(localUser.HomeDir); err == nil {
		return fi.IsDir(), nil
	}

	// In some environments, the user's home directory exists but isn't visible to
	// root, e.g. /home is mounted to an nfs export with root_squash enabled.
	// In case we are in that scenario, re-exec teleport as the user to check
	// if the home dir actually does exist.
	executable, err := os.Executable()
	if err != nil {
		return false, trace.Wrap(err)
	}

	credential, err := getCmdCredential(localUser)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Build the "teleport exec" command.
	cmd := &exec.Cmd{
		Path: executable,
		Args: []string{executable, teleport.CheckHomeDirSubCommand},
		Env:  []string{"HOME=" + localUser.HomeDir},
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:     true,
			Credential: credential,
		},
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState.ExitCode() == teleport.HomeDirNotFound {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return true, nil
}

// Spawns a process with the given credentials, outliving the context.
func (o *osWrapper) newParker(ctx context.Context, credential syscall.Credential) error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := o.CommandContext(ctx, executable, teleport.ParkSubCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &credential,
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	// the process will get killed when the context ends, but we still need to
	// Wait on it
	go cmd.Wait()

	return nil
}

// getCmdCredentials parses the uid, gid, and groups of the
// given user into a credential object for a command to use.
func getCmdCredential(localUser *user.User) (*syscall.Credential, error) {
	uid, err := strconv.ParseUint(localUser.Uid, 10, 32)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gid, err := strconv.ParseUint(localUser.Gid, 10, 32)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if runtime.GOOS == "darwin" {
		// on macOS we should rely on the list of groups managed by the system
		// (the use of setgroups is "highly discouraged", as per the setgroups
		// man page in macOS 13.5)
		return &syscall.Credential{
			Uid:         uint32(uid),
			Gid:         uint32(gid),
			NoSetGroups: true,
		}, nil
	}

	// Lookup supplementary groups for the user.
	userGroups, err := localUser.GroupIds()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groups := make([]uint32, 0)
	for _, sgid := range userGroups {
		igid, err := strconv.ParseUint(sgid, 10, 32)
		if err != nil {
			log.Warnf("Cannot interpret user group: '%v'", sgid)
		} else {
			groups = append(groups, uint32(igid))
		}
	}
	if len(groups) == 0 {
		groups = append(groups, uint32(gid))
	}

	return &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: groups,
	}, nil
}

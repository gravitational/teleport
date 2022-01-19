/*
Copyright 2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/sirupsen/logrus"
)

// ExecCommand contains the payload to "teleport exec" which will be used to
// construct and execute a shell.
type ExecCommand struct {
	// Command is the command to execute. If an interactive session is being
	// requested, will be empty.
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
	// typically set if either an shell was requested or a TTY was explicitly
	// allocated for a exec request.
	Terminal bool `json:"term"`

	// RequestType is the type of request: either "exec" or "shell". This will
	// be used to control where to connect std{out,err} based on the request
	// type: "exec" or "shell".
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

	// UaccMetadata contains metadata needed for user accounting.
	UaccMetadata UaccMetadata `json:"uacc_meta"`

	// X11Config contains an xauth entry to be added to the command user's xauthority.
	X11Config *X11Config `json:"x11_config,omitempty"`
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

// X11Config contains information used by the child process to set up x11 forwarding.
type X11Config struct {
	// XAuthEntry contains xauth data used for x11 forwarding.
	XAuthEntry *x11.XAuthEntry `json:"xauth_entry,omitempty"`
	// XServerUnixSocket is the name of an open xserver unix socket used for x11 forwarding.
	XServerUnixSocket string `json:"xserver_unix_socket"`
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
	if contfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
	}

	// Read in the command payload.
	var b bytes.Buffer
	_, err := b.ReadFrom(cmdfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	var c ExecCommand
	err = json.Unmarshal(b.Bytes(), &c)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	var tty *os.File
	var pty *os.File
	uaccEnabled := false

	// If a terminal was requested, file descriptors 5 and 6 always point to the
	// PTY and TTY. Extract them and set the controlling TTY. Otherwise, connect
	// std{in,out,err} directly.
	if c.Terminal {
		pty = os.NewFile(uintptr(6), "/proc/self/fd/6")
		tty = os.NewFile(uintptr(7), "/proc/self/fd/7")
		if pty == nil || tty == nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("pty and tty not found")
		}
		errorWriter = tty
		err = uacc.Open(c.UaccMetadata.UtmpPath, c.UaccMetadata.WtmpPath, c.Login, c.UaccMetadata.Hostname, c.UaccMetadata.RemoteAddr, tty)
		// uacc support is best-effort, only enable it if Open is successful.
		// Currently there is no way to log this error out-of-band with the
		// command output, so for now we essentially ignore it.
		if err == nil {
			uaccEnabled = true
		}
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
			stdout = ioutil.Discard
			stderr = ioutil.Discard
		}

		// Open the PAM context.
		pamContext, err := pam.Open(&pam.Config{
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

	localUser, err := user.Lookup(c.Login)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Build the actual command that will launch the shell.
	cmd, err := buildCommand(&c, localUser, tty, pty, pamEnvironment)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait until the continue signal is received from Teleport signaling that
	// the child process has been placed in a cgroup.
	err = waitForContinue(contfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	if c.X11Config != nil {
		// Open x11rdy fd to signal parent process once x11 forwarding is set up.
		x11rdyfd := os.NewFile(uintptr(5), "/proc/self/fd/5")
		if x11rdyfd == nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
		}

		// Set the open xserver unix socket's owner to the localuser
		// to prevent a potential privilege escalation vulnerability.
		if err := os.Chown(c.X11Config.XServerUnixSocket, int(cmd.SysProcAttr.Credential.Uid), int(cmd.SysProcAttr.Credential.Gid)); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		// Update localUser's xauth database to include the xauth
		// entry expected for x11 forwarding.
		removeCmd := x11.NewXAuthCommand(context.Background(), "")
		addCmd := x11.NewXAuthCommand(context.Background(), "")

		// Copy the re-exec command's io and user environment fields.
		cpyCmdFields := func(cmd *exec.Cmd, params *exec.Cmd) {
			cmd.Stdout = params.Stdout
			cmd.Stdin = params.Stdin
			cmd.Stderr = params.Stderr
			cmd.SysProcAttr = params.SysProcAttr
			cmd.Env = params.Env
			cmd.Dir = params.Dir
		}
		cpyCmdFields(removeCmd.Cmd, cmd)
		cpyCmdFields(addCmd.Cmd, cmd)

		if err := removeCmd.RemoveEntries(c.X11Config.XAuthEntry.Display); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		if err := addCmd.AddEntry(c.X11Config.XAuthEntry); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		// Set $DISPLAY so that XServer requests are sent to the X11 socket opened above.
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", x11.DisplayEnv, c.X11Config.XAuthEntry.Display))

		// Write a single byte to signal since Close
		// only closes the fd, not the underlying pipe.
		if _, err := x11rdyfd.Write([]byte{0}); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		x11rdyfd.Close()
	}

	// Start the command.
	err = cmd.Start()
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// Wait for the command to exit. It doesn't make sense to print an error
	// message here because the shell has successfully started. If an error
	// occurred during shell execution or the shell exits with an error (like
	// running exit 2), the shell will print an error if appropriate and return
	// an exit code.
	err = cmd.Wait()

	if uaccEnabled {
		uaccErr := uacc.Close(c.UaccMetadata.UtmpPath, c.UaccMetadata.WtmpPath, tty)
		if uaccErr != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(uaccErr)
		}
	}

	return ioutil.Discard, exitCode(err), trace.Wrap(err)
}

// RunForward reads in the command to run from the parent process (over a
// pipe) then port forwards.
func RunForward() (io.Writer, int, error) {
	// errorWriter is used to return any error message back to the client.
	// Use stderr so that it's not forwarded to the remote client.
	errorWriter := os.Stderr

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(uintptr(3), "/proc/self/fd/3")
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}

	// Read in the command payload.
	var b bytes.Buffer
	_, err := b.ReadFrom(cmdfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	var c ExecCommand
	err = json.Unmarshal(b.Bytes(), &c)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	if c.PAMConfig != nil {
		// Open the PAM context.
		pamContext, err := pam.Open(&pam.Config{
			ServiceName: c.PAMConfig.ServiceName,
			Login:       c.Login,
			Stdin:       os.Stdin,
			Stdout:      ioutil.Discard,
			Stderr:      ioutil.Discard,
			// Set Teleport specific environment variables that PAM modules
			// like pam_script.so can pick up to potentially customize the
			// account/session.
			Env: c.PAMConfig.Environment,
		})
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		defer pamContext.Close()
	}

	// Connect to the target host.
	conn, err := net.Dial("tcp", c.DestinationAddress)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}
	defer conn.Close()

	// Start copy routines that copy from channel to stdin pipe and from stdout
	// pipe to channel.
	errorCh := make(chan error, 2)
	go func() {
		defer conn.Close()
		defer os.Stdout.Close()
		defer os.Stdin.Close()

		_, err := io.Copy(os.Stdout, conn)
		errorCh <- err
	}()
	go func() {
		defer conn.Close()
		defer os.Stdout.Close()
		defer os.Stdin.Close()

		_, err := io.Copy(conn, os.Stdin)
		errorCh <- err
	}()

	// Block until copy is complete in either direction. The other direction
	// will get cleaned up automatically.
	if err = <-errorCh; err != nil && err != io.EOF {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	return ioutil.Discard, teleport.RemoteCommandSuccess, nil
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
	case teleport.ForwardSubCommand:
		w, code, err = RunForward()
	default:
		w, code, err = os.Stderr, teleport.RemoteCommandFailure, fmt.Errorf("unknown command type: %v", commandType)
	}
	if err != nil {
		s := fmt.Sprintf("Failed to launch: %v.\r\n", err)
		io.Copy(w, bytes.NewBufferString(s))
	}
	os.Exit(code)
}

// buildCommand constructs a command that will execute the users shell. This
// function is run by Teleport while it's re-executing.
func buildCommand(c *ExecCommand, localUser *user.User, tty *os.File, pty *os.File, pamEnvironment []string) (*exec.Cmd, error) {
	var cmd exec.Cmd

	// Get UID and GID.
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

	// Ensure that the user has a home directory.
	// TODO: Generalize this to support Windows.
	homeDir := string(os.PathSeparator)
	if utils.IsDir(localUser.HomeDir) {
		homeDir = localUser.HomeDir
	}

	// Create default environment for user.
	cmd.Env = []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(localUser.Uid, defaultLoginDefsPath),
		"HOME=" + homeDir,
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
	cmd.Dir = homeDir

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
		log.Debugf("Creating process with ambient credentials UID %v, GID: %v, Groups: %v.",
			uid, gid, groups)
	}

	// Perform OS-specific tweaks to the command.
	userCommandOSTweaks(&cmd)

	return &cmd, nil
}

// ConfigureCommand creates a command fully configured to execute. This
// function is used by Teleport to re-execute itself and pass whatever data
// is need to the child to actually execute the shell.
func ConfigureCommand(ctx *ServerContext) (*exec.Cmd, error) {
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
	cmdbytes, err := json.Marshal(cmdmsg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go copyCommand(ctx, cmdbytes)

	// Find the Teleport executable and its directory on disk.
	executable, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	executableDir, _ := filepath.Split(executable)

	// The channel type determines the subcommand to execute (execution or
	// port forwarding).
	subCommand := teleport.ExecSubCommand
	if ctx.ChannelType == teleport.ChanDirectTCPIP {
		subCommand = teleport.ForwardSubCommand
	}

	// Build the list of arguments to have Teleport re-exec itself. The "-d" flag
	// is appended if Teleport is running in debug mode.
	args := []string{executable, subCommand}

	// Build the "teleport exec" command.
	cmd := &exec.Cmd{
		Path: executable,
		Args: args,
		Dir:  executableDir,
		ExtraFiles: []*os.File{
			ctx.cmdr,
			ctx.contr,
			ctx.x11rdyw,
		},
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	return cmd, nil
}

// copyCommand will copy the provided command to the child process over the
// pipe attached to the context.
func copyCommand(ctx *ServerContext, cmdbytes []byte) {
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
	_, err := io.Copy(ctx.cmdw, bytes.NewReader(cmdbytes))
	if err != nil {
		log.Errorf("Failed to copy command over pipe: %v.", err)
		return
	}
}

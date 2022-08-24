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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/sirupsen/logrus"
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
	// X11File is used to communicate to the parent process that the child
	// process has set up X11 forwarding.
	X11File
	// PTYFile is a PTY the parent process passes to the child process.
	PTYFile
	// TTYFile is a TTY the parent process passes to the child process.
	TTYFile

	// FirstExtraFile is the first file descriptor that will be valid when
	// extra files are passed to child processes without a terminal.
	FirstExtraFile FileFD = X11File + 1
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
	X11Config X11Config `json:"x11_config"`

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

// X11Config contains information used by the child process to set up X11 forwarding.
type X11Config struct {
	// XAuthEntry contains xauth data used for X11 forwarding.
	XAuthEntry x11.XAuthEntry `json:"xauth_entry,omitempty"`
	// XServerUnixSocket is the name of an open XServer unix socket used for X11 forwarding.
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

// RunForward reads in the command to run from the parent process (over a
// pipe) then port forwards.
func RunForward() (errw io.Writer, code int, err error) {
	// errorWriter is used to return any error message back to the client.
	// Use stderr so that it's not forwarded to the remote client.
	errorWriter := os.Stderr

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, "/proc/self/fd/3")
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}

	// Read in the command payload.
	var b bytes.Buffer
	_, err = b.ReadFrom(cmdfd)
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

	return io.Discard, teleport.RemoteCommandSuccess, nil
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
	case teleport.ForwardSubCommand:
		w, code, err = RunForward()
	case teleport.CheckHomeDirSubCommand:
		w, code, err = runCheckHomeDir()
	case teleport.ParkSubCommand:
		w, code, err = runPark()
	default:
		w, code, err = os.Stderr, teleport.RemoteCommandFailure, fmt.Errorf("unknown command type: %v", commandType)
	}
	if err != nil {
		s := fmt.Sprintf("Failed to launch: %v.\r\n", err)
		io.Copy(w, bytes.NewBufferString(s))
	}
	os.Exit(code)
}

// IsReexec determines if the current process is a teleport reexec command.
// Used by tests to reroute the execution to RunAndExit.
func IsReexec() bool {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case teleport.ExecSubCommand, teleport.ForwardSubCommand, teleport.CheckHomeDirSubCommand,
			teleport.ParkSubCommand, teleport.SFTPSubCommand:
			return true
		}
	}

	return false
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

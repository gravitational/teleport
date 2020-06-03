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
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/shell"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/sirupsen/logrus"
)

// execCommand contains the payload to "teleport exec" which will be used to
// construct and execute a shell.
type execCommand struct {
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

		// Open the PAM context.
		pamContext, err := pam.Open(&pam.Config{
			ServiceName: c.ServiceName,
			Login:       c.Login,
			// Set Teleport specific environment variables that PAM modules
			// like pam_script.so can pick up to potentially customize the
			// account/session.
			Env: map[string]string{
				"TELEPORT_USERNAME": c.Username,
				"TELEPORT_LOGIN":    c.Login,
				"TELEPORT_ROLES":    strings.Join(c.Roles, " "),
			},
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
	// occurred during shell execution or the shell exits with an error (like
	// running exit 2), the shell will print an error if appropriate and return
	// an exit code.
	err = cmd.Wait()
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
	var c execCommand
	err = json.Unmarshal(b.Bytes(), &c)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// If PAM is enabled, open a PAM context. This has to be done before anything
	// else because PAM is sometimes used to create the local user used to
	// launch the shell under.
	if c.PAM {
		// Set Teleport specific environment variables that PAM modules like
		// pam_script.so can pick up to potentially customize the account/session.
		os.Setenv("TELEPORT_USERNAME", c.Username)
		os.Setenv("TELEPORT_LOGIN", c.Login)
		os.Setenv("TELEPORT_ROLES", strings.Join(c.Roles, " "))

		// Open the PAM context.
		pamContext, err := pam.Open(&pam.Config{
			ServiceName: c.ServiceName,
			Login:       c.Login,
			Stdin:       os.Stdin,
			Stdout:      ioutil.Discard,
			Stderr:      ioutil.Discard,
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
	// Marshal the parts needed from the *ServerContext into an *execCommand.
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
		},
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	return cmd, nil
}

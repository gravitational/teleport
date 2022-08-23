// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package srv

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func init() {
	// errors in open/openat are signaled by returning -1, we don't really care
	// about the specifics anyway so we can just ignore the error value
	//
	// we're opening with O_PATH rather than O_RDONLY because the binary might
	// not actually be readable (but only executable)
	fd1, _ := syscall.Open("/proc/self/exe", unix.O_PATH|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	fd2, _ := syscall.Open("/proc/self/exe", unix.O_PATH|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)

	// this can happen if both calls failed (returning -1) or if we're
	// running in a version of qemu-user that's affected by this bug:
	// https://gitlab.com/qemu-project/qemu/-/issues/927
	// (hopefully they'll also add special handling for execve on /proc/self/exe
	// if they ever fix that bug)
	if fd1 == fd2 {
		return
	}

	// closing -1 is harmless, no need to check here
	syscall.Close(fd1)
	syscall.Close(fd2)

	// if one Open has failed but not the other we can't really
	// trust the availability of "/proc/self/exe"
	if fd1 == -1 || fd2 == -1 {
		return
	}

	reexecPath = "/proc/self/exe"
}

// reexecPath specifies a path to execute on reexec, overriding Path in the cmd
// passed to reexecCommandOSTweaks, if not empty.
var reexecPath string

func reexecCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// Linux only: when parent process (node) dies unexpectedly without
	// cleaning up child processes, send a signal for graceful shutdown
	// to children.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGQUIT

	// replace the path on disk (which might not exist, or refer to an
	// upgraded version of teleport) with reexecPath, which contains
	// some path that refers to the specific binary we're running
	if reexecPath != "" {
		cmd.Path = reexecPath
	}
}

func userCommandOSTweaks(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// Linux only: when parent process (this process) dies unexpectedly, kill
	// the child process instead of orphaning it.
	// SIGKILL because we don't control the child process and it could choose
	// to ignore other signals.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command.
func RunCommand() (errw io.Writer, code int, err error) {
	// errorWriter is used to return any error message back to the client. By
	// default it writes to stdout, but if a TTY is allocated, it will write
	// to it instead.
	errorWriter := os.Stdout

	// Parent sends the command payload in the third file descriptor.
	cmdfd := os.NewFile(CommandFile, "/proc/self/fd/3")
	if cmdfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("command pipe not found")
	}
	contfd := os.NewFile(ContinueFile, "/proc/self/fd/4")
	if contfd == nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
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

	var tty *os.File
	var pty *os.File
	uaccEnabled := false

	// If a terminal was requested, file descriptors 6 and 7 always point to the
	// PTY and TTY. Extract them and set the controlling TTY. Otherwise, connect
	// std{in,out,err} directly.
	if c.Terminal {
		pty = os.NewFile(PTYFile, "/proc/self/fd/6")
		tty = os.NewFile(TTYFile, "/proc/self/fd/7")
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
			stdout = io.Discard
			stderr = io.Discard
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
	defer func() {
		for _, f := range cmd.ExtraFiles {
			if err := f.Close(); err != nil {
				log.WithError(err).Warn("Error closing extra file.")
			}
		}
	}()

	// Wait until the continue signal is received from Teleport signaling that
	// the child process has been placed in a cgroup.
	err = waitForContinue(contfd)
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	// If we're planning on changing credentials, we should first park an
	// innocuous process with the same UID and then check the user database
	// again, to avoid it getting deleted under our nose.
	parkerCtx, parkerCancel := context.WithCancel(context.Background())
	defer parkerCancel()
	if cmd.SysProcAttr.Credential != nil {
		if err := newParker(parkerCtx, *cmd.SysProcAttr.Credential); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		localUserCheck, err := user.Lookup(c.Login)
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		if localUser.Uid != localUserCheck.Uid || localUser.Gid != localUserCheck.Gid {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("user %q has been changed", c.Login)
		}
	}

	if c.X11Config.XServerUnixSocket != "" {
		// Set the open XServer unix socket's owner to the localuser
		// to prevent a potential privilege escalation vulnerability.
		uid, err := strconv.Atoi(localUser.Uid)
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		gid, err := strconv.Atoi(localUser.Gid)
		if err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
		if err := os.Chown(c.X11Config.XServerUnixSocket, uid, gid); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		// Update localUser's xauth database for X11 forwarding. We set
		// cmd.SysProcAttr.Setsid, cmd.Env, and cmd.Dir so that the xauth command
		// acts as if called within the following shell/exec, so that the
		// xauthority files is put into the correct place ($HOME/.Xauthority)
		// with the right permissions.
		removeCmd := x11.NewXAuthCommand(context.Background(), "")
		removeCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		removeCmd.Env = cmd.Env
		removeCmd.Dir = cmd.Dir
		if err := removeCmd.RemoveEntries(c.X11Config.XAuthEntry.Display); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		addCmd := x11.NewXAuthCommand(context.Background(), "")
		addCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		addCmd.Env = cmd.Env
		addCmd.Dir = cmd.Dir
		if err := addCmd.AddEntry(c.X11Config.XAuthEntry); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		// Set $DISPLAY so that XServer requests forwarded to the X11 unix listener.
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", x11.DisplayEnv, c.X11Config.XAuthEntry.Display.String()))

		// Open x11rdy fd to signal parent process once X11 forwarding is set up.
		x11rdyfd := os.NewFile(X11File, "/proc/self/fd/5")
		if x11rdyfd == nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.BadParameter("continue pipe not found")
		}

		// Write a single byte to signal to the parent process that X11 forwarding is set up.
		if _, err := x11rdyfd.Write([]byte{0}); err != nil {
			if err2 := x11rdyfd.Close(); err2 != nil {
				return errorWriter, teleport.RemoteCommandFailure, trace.NewAggregate(err, err2)
			}
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}

		if err := x11rdyfd.Close(); err != nil {
			return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
		}
	}

	// Start the command.
	err = cmd.Start()
	if err != nil {
		return errorWriter, teleport.RemoteCommandFailure, trace.Wrap(err)
	}

	parkerCancel()

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

	return io.Discard, exitCode(err), trace.Wrap(err)
}

// buildCommand constructs a command that will execute the users shell. This
// function is run by Teleport while it's re-executing.
func buildCommand(c *ExecCommand, localUser *user.User, tty *os.File, pty *os.File, pamEnvironment []string) (*exec.Cmd, error) {
	var cmd exec.Cmd

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
	exists, err := checkHomeDir(localUser)
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
	userCommandOSTweaks(&cmd)

	return &cmd, nil
}

// checkHomeDir checks if the user's home dir exists
func checkHomeDir(localUser *user.User) (bool, error) {
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
func newParker(ctx context.Context, credential syscall.Credential) error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.CommandContext(ctx, executable, teleport.ParkSubCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &credential,
	}

	// Perform OS-specific tweaks to the command.
	reexecCommandOSTweaks(cmd)

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	// the process will get killed when the context ends but we still need to
	// Wait on it
	go cmd.Wait()

	return nil
}

// getCmdCredentials parses the uid, gid, and groups of the
// given user into a credential object for a command to use.
func getCmdCredential(localUser *user.User) (*syscall.Credential, error) {
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

	return &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: groups,
	}, nil
}

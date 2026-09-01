//go:build linux

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
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/utils/testutils"
	"github.com/gravitational/teleport/session/host"
	"github.com/gravitational/teleport/session/reexec"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

func TestOSCommandPrep(t *testing.T) {
	testutils.RequireRoot(t)

	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	scx.Identity.AccessPermit = &decisionpb.SSHAccessPermit{}

	// because CheckHomeDir now inspects access to the home directory as the actual user after a rexec,
	// we need to setup a real, non-root user with a valid home directory in order for this test to
	// exercise the correct paths
	tempHome := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(tempHome), 0o777))

	username := "test-os-command-prep"
	scx.Identity.Login = username
	_, err := host.UserAdd(username, nil, host.UserOpts{
		Home: tempHome,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		// change homedir back so user deletion doesn't fail
		changeHomeDir(t, username, tempHome)
		_, err := host.UserDel(username)
		require.NoError(t, err)
	})

	usr, err := user.Lookup(username)
	require.NoError(t, err)

	uid, err := strconv.Atoi(usr.Uid)
	require.NoError(t, err)

	require.NoError(t, os.Chown(tempHome, uid, -1))
	expectedEnv := []string{
		"LANG=en_US.UTF-8",
		reexec.GetDefaultEnvPath(usr.Uid),
		fmt.Sprintf("HOME=%s", usr.HomeDir),
		fmt.Sprintf("USER=%s", username),
		"SHELL=/bin/sh",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"TERM=xterm",
		fmt.Sprintf("SSH_TTY=%v", scx.party.s.term.TTYName()),
		"SSH_SESSION_ID=xxx",
		"TELEPORT_SESSION=xxx",
		"SSH_TELEPORT_HOST_UUID=testID",
		"SSH_TELEPORT_CLUSTER_NAME=localhost",
		"SSH_TELEPORT_USER=teleportUser",
	}

	// Empty command (simple shell).
	execCmd, err := scx.ExecCommand()
	require.NoError(t, err)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	cmd, err := reexec.BuildCommand(execCmd, usr, nil)
	require.NoError(t, err)

	require.NotNil(t, cmd)
	require.Equal(t, "/bin/sh", cmd.Path)
	require.Equal(t, []string{"-sh"}, cmd.Args)
	require.Equal(t, usr.HomeDir, cmd.Dir)
	require.Equal(t, expectedEnv, cmd.Env)
	require.Equal(t, syscall.SIGKILL, cmd.SysProcAttr.Pdeathsig)

	// Non-empty command (exec a prog).
	scx.execRequest.SetCommand("ls -lh /etc")
	execCmd, err = scx.ExecCommand()
	require.NoError(t, err)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	cmd, err = reexec.BuildCommand(execCmd, usr, nil)
	require.NoError(t, err)

	require.NotNil(t, cmd)
	require.Equal(t, "/bin/sh", cmd.Path)
	require.Equal(t, []string{"/bin/sh", "-c", "ls -lh /etc"}, cmd.Args)
	require.Equal(t, usr.HomeDir, cmd.Dir)
	require.Equal(t, expectedEnv, cmd.Env)
	require.Equal(t, syscall.SIGKILL, cmd.SysProcAttr.Pdeathsig)

	// Command without args.
	scx.execRequest.SetCommand("top")
	execCmd, err = scx.ExecCommand()
	require.NoError(t, err)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	cmd, err = reexec.BuildCommand(execCmd, usr, nil)
	require.NoError(t, err)

	require.Equal(t, "/bin/sh", cmd.Path)
	require.Equal(t, []string{"/bin/sh", "-c", "top"}, cmd.Args)
	require.Equal(t, syscall.SIGKILL, cmd.SysProcAttr.Pdeathsig)

	// Missing home directory - HOME should still be set to the given
	// home dir, but the command should set its CWD to root instead.
	changeHomeDir(t, username, "/wrong/place")
	usr.HomeDir = "/wrong/place"
	root := string(os.PathSeparator)
	expectedEnv[2] = "HOME=/wrong/place"
	cmd, err = reexec.BuildCommand(execCmd, usr, nil)
	require.NoError(t, err)

	require.Equal(t, root, cmd.Dir)
	require.Equal(t, expectedEnv, cmd.Env)
}

func TestConfigureCommand(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	scx.Identity.AccessPermit = &decisionpb.SSHAccessPermit{}

	unexpectedKey := "FOO"
	unexpectedValue := "BAR"
	// environment values in the server context should not be forwarded
	scx.SetEnv(unexpectedKey, unexpectedValue)

	cmd, err := scx.ConfigureCommand(nil)
	require.NoError(t, err)

	require.NotNil(t, cmd)
	require.Equal(t, "/proc/self/exe", cmd.Path)
	require.NotContains(t, cmd.Env, unexpectedKey+"="+unexpectedValue)
}

// TestContinue tests if the process continues once the continue signal
// has been sent.
func TestContinue(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	scx.Identity.AccessPermit = &decisionpb.SSHAccessPermit{}

	// Configure Session Context to re-exec "ls".
	var err error
	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)
	scx.execRequest.SetCommand(lsPath)

	r, w, err := os.Pipe()
	require.NoError(t, err)

	defer r.Close()
	defer w.Close()

	// Create an exec.Cmd to execute through Teleport.
	cmd, err := scx.ConfigureCommand(map[reexec.FileFD]*os.File{
		reexec.StdinFile:  r,
		reexec.StdoutFile: w,
		reexec.StderrFile: w,
	})
	require.NoError(t, err)

	// Create a channel that will be used to signal that execution is complete.
	cmdDone := make(chan error, 1)

	// Re-execute Teleport and run "ls". Signal over the context when execution
	// is complete.
	go func() {
		if err := cmd.Start(); err != nil {
			cmdDone <- err
		}

		cmdDone <- cmd.Wait()
	}()

	// Signal to child that it may execute the requested program.
	err = cmd.Continue()
	require.NoError(t, err)

	// Program should have executed now. If the complete signal has not come
	// over the context, something failed.
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for process to finish.")
	case err := <-cmdDone:
		require.NoError(t, err)
	}
}

func changeHomeDir(t *testing.T, username, home string) {
	usermodBin, err := exec.LookPath("usermod")
	assert.NoError(t, err, "usermod binary must be present")

	cmd := exec.Command(usermodBin, "--home", home, username)
	_, err = cmd.CombinedOutput()
	assert.NoError(t, err, "changing home should not error")
	assert.Equal(t, 0, cmd.ProcessState.ExitCode(), "changing home should exit 0")
}

// TestSFTPCommand exercises the SFTP subprocess as the user running the test,
// whose home directory exists.
func TestSFTPCommand(t *testing.T) {
	t.Parallel()

	client := newSFTPSubprocessClient(t, "")

	assertCanReadAbsolutePath(t, client)

	// Relative paths resolve against the home directory, which is what clients
	// rely on after stripping "~" (see sftputils.ExpandHomeDir).
	usr, err := user.Current()
	require.NoError(t, err)
	if _, err := os.Stat(usr.HomeDir); err != nil {
		// Covered by TestRootSFTPCommandMissingHomeDir instead.
		t.Skipf("home directory %q is not usable: %v", usr.HomeDir, err)
	}

	resolvedHome, err := filepath.EvalSymlinks(usr.HomeDir)
	require.NoError(t, err)

	wd, err := client.Getwd()
	require.NoError(t, err)
	require.Equal(t, resolvedHome, wd)
}

// TestRootSFTPCommandMissingHomeDir is the regression test for SFTP failing
// entirely when the user's home directory does not exist.
func TestRootSFTPCommandMissingHomeDir(t *testing.T) {
	testutils.RequireRoot(t)

	// Create the user with a real home directory, then point it somewhere that
	// doesn't exist.
	tempHome := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(tempHome), 0o777))

	login := testutils.GenerateLocalUsername(t)
	_, err := host.UserAdd(login, nil, host.UserOpts{Home: tempHome})
	require.NoError(t, err)

	// userdel removes the home directory, and refuses with exit status 12 if
	// the user doesn't own it
	usr, err := user.Lookup(login)
	require.NoError(t, err)
	uid, err := strconv.Atoi(usr.Uid)
	require.NoError(t, err)
	require.NoError(t, os.Chown(tempHome, uid, -1))

	t.Cleanup(func() {
		changeHomeDir(t, login, tempHome)
		host.UserDel(login)
	})

	missingHome := filepath.Join(tempHome, "does-not-exist")
	require.NoDirExists(t, missingHome)
	changeHomeDir(t, login, missingHome)

	client := newSFTPSubprocessClient(t, login)
	// Reaching this point means the subcommand did not bail early on non-existent home dir.

	t.Run("absolute paths work per-request", func(t *testing.T) {
		assertCanReadAbsolutePath(t, client)
	})
	t.Run("relative paths fail per-request without killing the session", func(t *testing.T) {
		// Clients strip "~" and rely on the server being rooted at the home
		// directory, so these must report a missing path rather than silently
		// resolving somewhere else, e.g. "~/file" landing in "/file".
		for _, relPath := range []string{".", "file.txt"} {
			_, err := client.Open(relPath)
			require.ErrorIs(t, err, os.ErrNotExist, "opening %q", relPath)
		}

		// Nothing should break after, the process should handle the above.
		assertCanReadAbsolutePath(t, client)
	})
}

// assertCanReadAbsolutePath checks that absolute access works regardless of whether home dir exists.
// Example: `tsh scp user@host:/etc/foo .`
func assertCanReadAbsolutePath(t *testing.T, client *sftp.Client) {
	t.Helper()

	dir := t.TempDir()
	// The transfer runs as the target user, so the file must be reachable.
	require.NoError(t, os.Chmod(filepath.Dir(dir), 0o755))
	require.NoError(t, os.Chmod(dir, 0o755))

	srcPath := filepath.Join(dir, "foo")
	contents := []byte("bar\n")
	require.NoError(t, os.WriteFile(srcPath, contents, 0o644))

	f, err := client.Open(srcPath)
	require.NoError(t, err)
	defer f.Close()

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, contents, got)
}

// newSFTPSubprocessClient runs the SFTP subsystem for the given login
func newSFTPSubprocessClient(t *testing.T, login string) *sftp.Client {
	t.Helper()

	srv := newMockServer(t)
	scx := newTestServerContext(t, srv, nil, &decisionpb.SSHAccessPermit{})
	if login != "" {
		scx.Identity.Login = login
	}

	scx.execRequest.SetCommand(reexecconstants.SFTPSubCommand)
	const subsystemRequestType = "subsystem"
	require.NoError(t, scx.SetSSHRequest(&ssh.Request{Type: subsystemRequestType}))

	cmdMsg, err := scx.ExecCommand()
	require.NoError(t, err)
	require.Equal(t, subsystemRequestType, cmdMsg.RequestType)
	require.Equal(t, reexecconstants.SFTPSubCommand, cmdMsg.Command)

	// Two pipes for the SSH channel plus one for audit events.
	chReadR, chReadW, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { chReadW.Close() })

	chWriteR, chWriteW, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { chWriteR.Close() })

	auditR, auditW, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { auditR.Close() })

	executor, err := scx.ConfigureCommand(map[reexec.FileFD]*os.File{
		reexec.StdinFile:  chReadR,
		reexec.StdoutFile: chWriteW,
		reexec.StderrFile: auditW,
	})
	require.NoError(t, err)

	// Stderr is assigned directly rather than through the extra files map,
	// since StderrFile is taken by the audit pipe.
	executor.Stderr = io.Discard

	require.NoError(t, executor.Start())
	require.NoError(t, executor.Continue())
	t.Cleanup(func() {
		executor.Kill()
		executor.Close()
	})

	// Mimic [sftpSubsys.Start]
	require.NoError(t, chReadR.Close())
	require.NoError(t, chWriteW.Close())
	require.NoError(t, auditW.Close())

	go func() {
		// Drain audit events
		io.Copy(io.Discard, auditR)
	}()

	// null byte means there is no request for this session.
	_, err = chReadW.Write([]byte{0x0})
	require.NoError(t, err)

	// Temporarily set a deadline to reduce test time if misconfigured.
	require.NoError(t, chWriteR.SetReadDeadline(time.Now().Add(15*time.Second)))
	client, err := sftp.NewClientPipe(chWriteR, chReadW)
	require.NoError(t, err, "SFTP process failed to start.")
	require.NoError(t, chWriteR.SetReadDeadline(time.Time{}))
	t.Cleanup(func() { client.Close() })
	return client
}

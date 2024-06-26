//go:build linux
// +build linux

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
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
)

func TestOSCommandPrep(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	usr, err := user.Current()
	require.NoError(t, err)

	expectedEnv := []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(strconv.Itoa(os.Geteuid()), defaultLoginDefsPath),
		fmt.Sprintf("HOME=%s", usr.HomeDir),
		fmt.Sprintf("USER=%s", usr.Username),
		"SHELL=/bin/sh",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"TERM=xterm",
		fmt.Sprintf("SSH_TTY=%v", scx.session.term.TTY().Name()),
		"SSH_SESSION_ID=xxx",
		"SSH_TELEPORT_HOST_UUID=testID",
		"SSH_TELEPORT_CLUSTER_NAME=localhost",
		"SSH_TELEPORT_USER=teleportUser",
	}

	// Empty command (simple shell).
	execCmd, err := scx.ExecCommand()
	require.NoError(t, err)

	cmd, err := buildCommand(execCmd, usr, nil, nil, nil)
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

	cmd, err = buildCommand(execCmd, usr, nil, nil, nil)
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

	cmd, err = buildCommand(execCmd, usr, nil, nil, nil)
	require.NoError(t, err)

	require.Equal(t, "/bin/sh", cmd.Path)
	require.Equal(t, []string{"/bin/sh", "-c", "top"}, cmd.Args)
	require.Equal(t, syscall.SIGKILL, cmd.SysProcAttr.Pdeathsig)

	if os.Geteuid() != 0 {
		t.Skip("skipping portion of test which must run as root")
	}

	// Missing home directory - HOME should still be set to the given
	// home dir, but the command should set it's CWD to root instead.
	usr.HomeDir = "/wrong/place"
	root := string(os.PathSeparator)
	expectedEnv[2] = "HOME=/wrong/place"
	cmd, err = buildCommand(execCmd, usr, nil, nil, nil)
	require.NoError(t, err)

	require.Equal(t, root, cmd.Dir)
	require.Equal(t, expectedEnv, cmd.Env)
}

func TestConfigureCommand(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	unexpectedKey := "FOO"
	unexpectedValue := "BAR"
	// environment values in the server context should not be forwarded
	scx.SetEnv(unexpectedKey, unexpectedValue)

	cmd, err := ConfigureCommand(scx)
	require.NoError(t, err)

	require.NotNil(t, cmd)
	require.Equal(t, "/proc/self/exe", cmd.Path)
	require.NotContains(t, cmd.Env, unexpectedKey+"="+unexpectedValue)
}

func TestRootConfigureCommand(t *testing.T) {
	utils.RequireRoot(t)

	login := utils.GenerateLocalUsername(t)
	_, err := host.UserAdd(login, nil, "", "", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := host.UserDel(login)
		require.NoError(t, err)
	})

	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)
	scx.Identity.Login = login
	scx.ExecType = teleport.TCPIPForwardRequest

	u, err := user.Lookup(login)
	require.NoError(t, err)
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	require.NoError(t, err)
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	require.NoError(t, err)

	cmd, err := ConfigureCommand(scx)
	require.NoError(t, err)
	// Verify that the configured command will run as the expected user.
	assert.Equal(t, uint32(uid), cmd.SysProcAttr.Credential.Uid)
	assert.Equal(t, uint32(gid), cmd.SysProcAttr.Credential.Gid)
}

// TestContinue tests if the process hangs if a continue signal is not sent
// and makes sure the process continues once it has been sent.
func TestContinue(t *testing.T) {
	srv := newMockServer(t)
	scx := newExecServerContext(t, srv)

	// Configure Session Context to re-exec "ls".
	var err error
	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)
	scx.execRequest.SetCommand(lsPath)

	// Create an exec.Cmd to execute through Teleport.
	cmd, err := ConfigureCommand(scx)
	require.NoError(t, err)

	// Create a channel that will be used to signal that execution is complete.
	cmdDone := make(chan error, 1)

	// Re-execute Teleport and run "ls". Signal over the context when execution
	// is complete.
	go func() {
		if err := cmd.Start(); err != nil {
			cmdDone <- err
		}

		// Close the read half of the pipe to unblock the ready signal.
		closeErr := scx.readyw.Close()
		cmdDone <- trace.NewAggregate(closeErr, cmd.Wait())
	}()

	// Wait for the process. Since the continue pipe has not been closed, the
	// process should not have exited yet.
	select {
	case err := <-cmdDone:
		t.Fatalf("Process exited before continue with error %v", err)
	case <-time.After(5 * time.Second):
	}

	// Wait for the child process to indicate its completed initialization.
	require.NoError(t, scx.execRequest.WaitForChild())

	// Signal to child that it may execute the requested program.
	scx.execRequest.Continue()

	// Program should have executed now. If the complete signal has not come
	// over the context, something failed.
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for process to finish.")
	case err := <-cmdDone:
		require.NoError(t, err)
	}
}

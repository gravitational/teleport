/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func isGitDirCheck(cmd *exec.Cmd) bool {
	return slices.Equal([]string{"git", "rev-parse", "--is-inside-work-tree"}, cmd.Args)
}
func isGitListRemoteURL(cmd *exec.Cmd) bool {
	return slices.Equal([]string{"git", "ls-remote", "--get-url"}, cmd.Args)
}
func isGitConfigGetCoreSSHCommand(cmd *exec.Cmd) bool {
	return slices.Equal([]string{"git", "config", "--local", "--default", "", "--get", "core.sshcommand"}, cmd.Args)
}

type fakeGitCommandRunner struct {
	dirCheckError  error
	coreSSHCommand string
	remoteURL      string
	verifyCommand  func(cmd *exec.Cmd) error
}

func (f fakeGitCommandRunner) run(cmd *exec.Cmd) error {
	switch {
	case isGitDirCheck(cmd):
		return f.dirCheckError
	case isGitConfigGetCoreSSHCommand(cmd):
		fmt.Fprintln(cmd.Stdout, f.coreSSHCommand)
		return nil
	case isGitListRemoteURL(cmd):
		fmt.Fprintln(cmd.Stdout, f.remoteURL)
		return nil
	default:
		if f.verifyCommand != nil {
			return trace.Wrap(f.verifyCommand(cmd))
		}
		return trace.NotFound("unknown command")
	}
}

func TestGitConfigCommand(t *testing.T) {
	tests := []struct {
		name                string
		cmd                 *gitConfigCommand
		fakeRunner          fakeGitCommandRunner
		checkError          require.ErrorAssertionFunc
		checkOutputContains string
	}{
		{
			name: "not a git dir",
			cmd:  &gitConfigCommand{},
			fakeRunner: fakeGitCommandRunner{
				dirCheckError: trace.BadParameter("not a git dir"),
			},
			checkError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "the current directory is not a Git repository")
			},
		},
		{
			name: "check",
			cmd:  &gitConfigCommand{},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: makeGitCoreSSHCommand("tsh", "org"),
			},
			checkError:          require.NoError,
			checkOutputContains: "is configured with Teleport for GitHub organization \"org\"",
		},
		{
			name: "check not configured",
			cmd:  &gitConfigCommand{},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: "",
			},
			checkError:          require.NoError,
			checkOutputContains: "is not configured",
		},
		{
			name: "update success",
			cmd: &gitConfigCommand{
				action: gitConfigActionUpdate,
			},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: makeGitCoreSSHCommand("tsh", "org"),
				remoteURL:      "git@github.com:gravitational/teleport.git",
				verifyCommand: func(cmd *exec.Cmd) error {
					expect := []string{
						"git", "config", "--local",
						"--replace-all", "core.sshcommand",
						"\"tsh\" git ssh --github-org gravitational",
					}
					if !slices.Equal(expect, cmd.Args) {
						return trace.CompareFailed("expect %v but got %v", expect, cmd.Args)
					}
					return nil
				},
			},
			checkError: require.NoError,
		},
		{
			name: "update failed missing url",
			cmd: &gitConfigCommand{
				action: gitConfigActionUpdate,
			},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: makeGitCoreSSHCommand("tsh", "org"),
				remoteURL:      "",
			},
			checkError: require.Error,
		},
		{
			name: "reset no-op",
			cmd: &gitConfigCommand{
				action: gitConfigActionReset,
			},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: "",
			},
			checkError: require.NoError,
		},
		{
			name: "reset no-op",
			cmd: &gitConfigCommand{
				action: gitConfigActionReset,
			},
			fakeRunner: fakeGitCommandRunner{
				coreSSHCommand: makeGitCoreSSHCommand("tsh", "org"),
				verifyCommand: func(cmd *exec.Cmd) error {
					expect := []string{
						"git", "config", "--local",
						"--unset-all", "core.sshcommand",
					}
					if !slices.Equal(expect, cmd.Args) {
						return trace.CompareFailed("expect %v but got %v", expect, cmd.Args)
					}
					return nil
				},
			},
			checkError: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cf := &CLIConf{
				Context:          context.Background(),
				OverrideStdout:   &buf,
				executablePath:   "tsh",
				cmdRunner:        tt.fakeRunner.run,
				lookPathOverride: "git",
			}
			tt.checkError(t, tt.cmd.run(cf))
			require.Contains(t, buf.String(), tt.checkOutputContains)
		})
	}
}

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
	"context"
	"os/exec"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGitCloneCommand(t *testing.T) {
	tests := []struct {
		name          string
		cmd           *gitCloneCommand
		verifyCommand func(*exec.Cmd) error
		checkError    require.ErrorAssertionFunc
	}{
		{
			name: "success",
			cmd: &gitCloneCommand{
				repository: "git@github.com:gravitational/teleport.git",
			},
			verifyCommand: func(cmd *exec.Cmd) error {
				expect := []string{
					"git", "clone",
					"--config", "core.sshcommand=\"tsh\" git ssh --github-org gravitational",
					"git@github.com:gravitational/teleport.git",
				}
				if !slices.Equal(expect, cmd.Args) {
					return trace.CompareFailed("expect %v but got %v", expect, cmd.Args)
				}
				return nil
			},
			checkError: require.NoError,
		},
		{
			name: "success with target dir",
			cmd: &gitCloneCommand{
				repository: "git@github.com:gravitational/teleport.git",
				directory:  "target_dir",
			},
			verifyCommand: func(cmd *exec.Cmd) error {
				expect := []string{
					"git", "clone",
					"--config", "core.sshcommand=\"tsh\" git ssh --github-org gravitational",
					"git@github.com:gravitational/teleport.git",
					"target_dir",
				}
				if !slices.Equal(expect, cmd.Args) {
					return trace.CompareFailed("expect %v but got %v", expect, cmd.Args)
				}
				return nil
			},
			checkError: require.NoError,
		},
		{
			name: "invalid URL",
			cmd: &gitCloneCommand{
				repository: "not-a-git-ssh-url",
			},
			checkError: require.Error,
		},
		{
			name: "unsupported Git service",
			cmd: &gitCloneCommand{
				repository: "git@gitlab.com:group/project.git",
			},
			checkError: require.Error,
		},
		{
			name: "git fails",
			cmd: &gitCloneCommand{
				repository: "git@github.com:gravitational/teleport.git",
			},
			verifyCommand: func(cmd *exec.Cmd) error {
				return trace.BadParameter("some git error")
			},
			checkError: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("some git error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := &CLIConf{
				Context:          context.Background(),
				executablePath:   "tsh",
				cmdRunner:        tt.verifyCommand,
				lookPathOverride: "git",
			}
			tt.checkError(t, tt.cmd.run(cf))
		})
	}
}

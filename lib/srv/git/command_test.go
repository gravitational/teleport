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

package git

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestParseSSHCommand(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		checkError require.ErrorAssertionFunc
		wantOutput *Command
	}{
		{
			name:       "git-upload-pack",
			input:      "git-upload-pack 'my-org/my-repo.git'",
			checkError: require.NoError,
			wantOutput: &Command{
				SSHCommand: "git-upload-pack 'my-org/my-repo.git'",
				Service:    "git-upload-pack",
				Repository: "my-org/my-repo.git",
			},
		},
		{
			name:       "git-upload-pack with double quote",
			input:      "git-upload-pack \"my-org/my-repo.git\"",
			checkError: require.NoError,
			wantOutput: &Command{
				SSHCommand: "git-upload-pack \"my-org/my-repo.git\"",
				Service:    "git-upload-pack",
				Repository: "my-org/my-repo.git",
			},
		},
		{
			name:       "git-upload-pack with args",
			input:      "git-upload-pack --strict 'my-org/my-repo.git'",
			checkError: require.NoError,
			wantOutput: &Command{
				SSHCommand: "git-upload-pack --strict 'my-org/my-repo.git'",
				Service:    "git-upload-pack",
				Repository: "my-org/my-repo.git",
			},
		},
		{
			name:       "git-upload-pack with args after repo",
			input:      "git-upload-pack --strict 'my-org/my-repo.git' --timeout=60",
			checkError: require.NoError,
			wantOutput: &Command{
				SSHCommand: "git-upload-pack --strict 'my-org/my-repo.git' --timeout=60",
				Service:    "git-upload-pack",
				Repository: "my-org/my-repo.git",
			},
		},
		{
			name:       "missing quote",
			input:      "git-upload-pack 'my-org/my-repo.git",
			checkError: require.Error,
		},
		{
			name:       "git-receive-pack",
			input:      "git-receive-pack 'my-org/my-repo.git'",
			checkError: require.NoError,
			wantOutput: &Command{
				SSHCommand: "git-receive-pack 'my-org/my-repo.git'",
				Service:    "git-receive-pack",
				Repository: "my-org/my-repo.git",
			},
		},
		{
			name:       "missing args",
			input:      "git-receive-pack",
			checkError: require.Error,
		},
		{
			name:       "unsupported",
			input:      "git-cat-file",
			checkError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := ParseSSHCommand(tt.input)
			tt.checkError(t, err)
			require.Equal(t, tt.wantOutput, output)
		})
	}
}

func Test_checkSSHCommand(t *testing.T) {
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  "my-org",
		Organization: "my-org",
	})
	require.NoError(t, err)

	tests := []struct {
		name       string
		server     types.Server
		sshCommand string
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "success",
			server:     server,
			sshCommand: "git-upload-pack 'my-org/my-repo.git'",
			checkError: require.NoError,
		},
		{
			name:       "org does not match",
			server:     server,
			sshCommand: "git-upload-pack 'some-other-org/my-repo.git'",
			checkError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), i...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := ParseSSHCommand(tt.sshCommand)
			require.NoError(t, err)
			tt.checkError(t, checkSSHCommand(tt.server, command))
		})
	}
}

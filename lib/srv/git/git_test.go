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

func TestCheckSSHCommand(t *testing.T) {
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
			name:       "success command with double quotes",
			server:     server,
			sshCommand: "git-upload-pack \"my-org/my-repo.git\"",
			checkError: require.NoError,
		},
		{
			name:       "org does not match",
			server:     server,
			sshCommand: "git-upload-pack 'some-other-org/my-repo.git'",
			checkError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err), i...)
			},
		},
		{
			name:       "invalid command",
			server:     server,
			sshCommand: "not-git-command",
			checkError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), i...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkError(t, CheckSSHCommand(tt.server, tt.sshCommand))
		})
	}
}

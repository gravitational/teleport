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
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseGitSSHURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		wantOut   *gitSSHURL
	}{
		{
			name:  "github ssh format",
			input: "org-1234567@github.com:some-org/some-repo.git",
			wantOut: &gitSSHURL{
				Protocol: "ssh",
				Host:     "github.com",
				User:     "org-1234567",
				Path:     "some-org/some-repo.git",
				Port:     22,
			},
		},
		{
			name:      "github ssh format invalid path",
			input:     "org-1234567@github.com:missing-org",
			wantError: true,
		},
		{
			name:  "ssh schema format",
			input: "ssh://git@github.com/some-org/some-repo.git",
			wantOut: &gitSSHURL{
				Protocol: "ssh",
				Host:     "github.com",
				User:     "git",
				Path:     "/some-org/some-repo.git",
			},
		},
		{
			name:      "unsupported format",
			input:     "https://github.com/gravitational/teleport.git",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			out, err := parseGitSSHURL(tt.input)
			t.Log(out, err)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOut, out)
		})
	}
}

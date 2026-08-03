// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBotResourceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scope     string
		botName   string
		want      string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "unscoped",
			botName:   "name",
			want:      "bot-name",
			assertErr: require.NoError,
		},
		{
			name:      "unscoped with spaces",
			botName:   "name with spaces",
			want:      "bot-name-with-spaces",
			assertErr: require.NoError,
		},
		{
			name:      "scoped",
			scope:     "/staging",
			botName:   "name",
			want:      "bot-++staging+name",
			assertErr: require.NoError,
		},
		{
			// A scope is a prefix of its children, so the encoded separator must
			// keep the two apart.
			name:      "scoped, child scope",
			scope:     "/staging/eu",
			botName:   "name",
			want:      "bot-++staging+eu+name",
			assertErr: require.NoError,
		},
		{
			// An empty segment has no encoding that preserves sort order.
			name:      "unencodable scope",
			scope:     "/staging//eu",
			botName:   "name",
			assertErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BotResourceName(tt.scope, tt.botName)
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

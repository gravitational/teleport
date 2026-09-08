/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package identity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_validateTokenValue(t *testing.T) {
	testCases := map[string]struct {
		wantErr string
		scoped  bool
		value   string
	}{
		"scoped: with SQN": {
			scoped: true,
			value:  "/staging/team-a::bot-join-token",
		},
		"scoped: token value empty": {
			scoped:  true,
			value:   "",
			wantErr: "join token cannot be empty",
		},
		"scoped: with malformed SQN": {
			scoped:  true,
			value:   "/staging/team-a/::bot-join-token::",
			wantErr: "join token must be a valid SQN when in scoped mode",
		},
		"scoped: without SQN": {
			scoped:  true,
			value:   "bot-join-token",
			wantErr: "join token must be a valid SQN when in scoped mode",
		},

		"unscoped: with SQN": {
			scoped: false,
			value:  "/staging/team-a::bot-join-token",
		},
		"unscoped: without SQN": {
			scoped: false,
			value:  "bot-join-token",
		},
		"unscoped: token value empty": {
			scoped:  false,
			value:   "",
			wantErr: "join token cannot be empty",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateTokenValue(tc.scoped, tc.value)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

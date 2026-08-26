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

package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestUserTokenUnmarshal(t *testing.T) {
	t.Parallel()

	created, err := time.Parse(time.RFC3339, "2020-01-14T18:52:39.523076855Z")
	require.NoError(t, err)

	type testCase struct {
		description string
		input       string
		expected    types.UserToken
	}

	testCases := []testCase{
		{
			description: "simple case",
			input: `
        {
          "kind": "user_token",
          "version": "v3",
          "metadata": {
            "name": "tokenId"
          },
          "spec": {
            "user": "example@example.com",
            "created": "2020-01-14T18:52:39.523076855Z",
            "url": "https://localhost"
          }
        }
      `,
			expected: &types.UserTokenV3{
				Kind:    types.KindUserToken,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "tokenId",
					Namespace: defaults.Namespace,
				},
				Spec: types.UserTokenSpecV3{
					Created: created,
					User:    "example@example.com",
					URL:     "https://localhost",
				},
			},
		},
	}

	for _, tc := range testCases {
		comment := fmt.Sprintf("test case %q", tc.description)
		out, err := UnmarshalUserToken([]byte(tc.input))
		require.NoError(t, err, comment)
		require.Empty(t, cmp.Diff(tc.expected, out))
		data, err := MarshalUserToken(out)
		require.NoError(t, err, comment)
		out2, err := UnmarshalUserToken(data)
		require.NoError(t, err, comment)
		require.Empty(t, cmp.Diff(tc.expected, out2))
	}
}

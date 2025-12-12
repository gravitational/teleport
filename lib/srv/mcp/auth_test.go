/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_rewriteAuthDetails(t *testing.T) {
	tests := []struct {
		name  string
		input *types.Rewrite
		want  rewriteAuthDetails
	}{
		{
			name:  "nil",
			input: nil,
			want:  rewriteAuthDetails{},
		},
		{
			name: "auth header",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer abcdef",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
			},
		},
		{
			name: "auth header with external traits",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{external.abcdef}}",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
			},
		},
		{
			name: "jwt",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "X-API-Key",
					Value: "{{internal.jwt}}",
				}},
			},
			want: rewriteAuthDetails{
				hasJWTTrait: true,
			},
		},
		{
			name: "id token",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{internal.id_token}}",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
				hasIDTokenTrait:   true,
			},
		},
		{
			name: "multiple ",
			input: &types.Rewrite{
				Headers: []*types.Header{
					{
						Name:  "foo",
						Value: "bar",
					},
					{
						Name:  "Authorization",
						Value: "Bearer {{internal.id_token}}",
					},
					{
						Name:  "X-API-Key",
						Value: "{{internal.jwt}}",
					},
				},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
				hasIDTokenTrait:   true,
				hasJWTTrait:       true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := newRewriteAuthDetails(tt.input)
			require.Equal(t, tt.want, actual)
		})
	}
}

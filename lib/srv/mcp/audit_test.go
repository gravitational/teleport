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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_headersForAudit(t *testing.T) {
	tests := []struct {
		name  string
		input http.Header
		want  http.Header
	}{
		{
			name:  "no header",
			input: nil,
			want:  nil,
		},
		{
			name: "remove reserved header",
			input: http.Header{
				"X-Forwarded-For": []string{"hello"},
				"Foo":             []string{"bar"},
				"Alice":           []string{"bob", "charlie"},
			},
			want: http.Header{
				"Foo":   []string{"bar"},
				"Alice": []string{"bob", "charlie"},
			},
		},
		{
			name: "redact secrets",
			input: http.Header{
				"Authorization": []string{"Bearer secret"},
				"X-Api-Key":     []string{"api-key"},
				"Alice":         []string{"bob", "charlie"},
			},
			want: http.Header{
				"Authorization": []string{"<REDACTED>"},
				"X-Api-Key":     []string{"<REDACTED>"},
				"Alice":         []string{"bob", "charlie"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, headersForAudit(tt.input))
		})
	}
}

func Test_guessEgressAuthType(t *testing.T) {
	tests := []struct {
		name         string
		inputHeader  http.Header
		inputRewrite *types.Rewrite
		expect       string
	}{
		{
			name:   "no header or rewrite",
			expect: "unknown",
		},
		{
			name: "user defined",
			inputHeader: http.Header{
				"Authorization": []string{"Bearer secret"},
			},
			inputRewrite: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Some-Other-Header",
					Value: "some-other-value",
				}},
			},
			expect: "user-defined",
		},
		{
			name: "app defined",
			inputRewrite: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer abcdef",
				}},
			},
			expect: "app-defined",
		},
		{
			name: "app defined with Teleport JWT",
			inputRewrite: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{internal.jwt}}",
				}},
			},
			expect: "app-jwt",
		},
		{
			name: "app defined with Teleport OIDC token",
			inputRewrite: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{internal.id_token}}",
				}},
			},
			expect: "app-oidc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, guessEgressAuthType(tt.inputHeader, tt.inputRewrite))
		})
	}
}

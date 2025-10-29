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

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

package vnet

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIsHTTPSTunnelApp(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		expect require.BoolAssertionFunc
	}{
		{
			name:   "TCP app",
			uri:    "tcp://localhost:5432",
			expect: require.False,
		},
		{
			name:   "HTTP app",
			uri:    "http://localhost:8080",
			expect: require.True,
		},
		{
			name:   "HTTPS app",
			uri:    "https://localhost:8443",
			expect: require.True,
		},
		{
			name:   "LLM app",
			uri:    "llm://",
			expect: require.True,
		},
		{
			name:   "MCP app",
			uri:    "mcp+http://localhost:8080",
			expect: require.False,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := &types.AppV3{Spec: types.AppSpecV3{URI: tc.uri}}
			tc.expect(t, IsHTTPSTunnelApp(app))
		})
	}
}

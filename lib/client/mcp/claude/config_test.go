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

package claude

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPServer_json(t *testing.T) {
	input := `{
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-filesystem",
    "/Users/username/Desktop",
    "/Users/username/Downloads"
  ]
}`
	var s MCPServer
	err := json.Unmarshal([]byte(input), &s)
	require.NoError(t, err)
	require.Equal(t, "npx", s.Command)
	require.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/Users/username/Desktop", "/Users/username/Downloads"}, s.Args)

	data, err := json.MarshalIndent(s, "", "  ")
	require.NoError(t, err)
	require.Equal(t, input, string(data))
}

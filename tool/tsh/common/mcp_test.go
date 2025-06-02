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

package common

import (
	"cmp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
)

func Test_isLocalMCPServerFromTeleport(t *testing.T) {
	nameCheck := func(localName string) bool {
		return strings.HasPrefix(localName, "test-")
	}
	startsWithArgs := []string{"test", "arg"}
	cf := &CLIConf{
		executablePath: "test-binary",
	}
	cfWithCustomHome := &CLIConf{
		executablePath: "test-binary",
		HomePath:       "/foo/.tsh",
	}

	tests := []struct {
		name      string
		cf        *CLIConf
		mcpServer claude.MCPServer
		check     require.BoolAssertionFunc
	}{
		{
			name: "test-is-from-teleport",
			mcpServer: claude.MCPServer{
				Command: "test-binary",
				Args:    append(startsWithArgs, "some", "more", "args"),
			},
			check: require.True,
		},
		{
			name: "test-not-same-binary",
			mcpServer: claude.MCPServer{
				Command: "npx",
				Args:    append(startsWithArgs, "some", "more", "args"),
			},
			check: require.False,
		},
		{
			name: "not-same-prefix",
			mcpServer: claude.MCPServer{
				Command: "test-binary",
				Args:    append(startsWithArgs, "some", "more", "args"),
			},
			check: require.False,
		},
		{
			name: "test-not-same-args",
			mcpServer: claude.MCPServer{
				Command: "test-binary",
				Args:    []string{"some", "other", "args"},
			},
			check: require.False,
		},
		{
			name: "test-home-path-match",
			cf:   cfWithCustomHome,
			mcpServer: claude.MCPServer{
				Command: "test-binary",
				Args:    append(startsWithArgs, "some", "more", "args"),
				Envs: map[string]string{
					types.HomeEnvVar: "/foo/.tsh",
				},
			},
			check: require.True,
		},
		{
			name: "test-home-path-no-match",
			cf:   cfWithCustomHome,
			mcpServer: claude.MCPServer{
				Command: "test-binary",
				Args:    append(startsWithArgs, "some", "more", "args"),
				Envs: map[string]string{
					types.HomeEnvVar: "/bar/.tsh",
				},
			},
			check: require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, isLocalMCPServerFromTeleport(
				cmp.Or(tt.cf, cf), tt.name, tt.mcpServer,
				nameCheck, startsWithArgs,
			))
		})
	}
}

func Test_mcpConfigFileFlags(t *testing.T) {
	tests := []struct {
		name       string
		flags      *mcpConfigFileFlags
		checkIsSet require.BoolAssertionFunc
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "empty",
			flags:      &mcpConfigFileFlags{},
			checkIsSet: require.False,
			checkError: require.Error,
		},
		{
			name: "claude",
			flags: &mcpConfigFileFlags{
				claude: true,
			},
			checkIsSet: require.True,
			checkError: require.NoError,
		},
		{
			name: "json-file",
			flags: &mcpConfigFileFlags{
				jsonFile: "some-file",
			},
			checkIsSet: require.True,
			checkError: require.NoError,
		},
		{
			name: "both set",
			flags: &mcpConfigFileFlags{
				claude:   true,
				jsonFile: "some-file",
			},
			checkIsSet: require.True,
			checkError: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkIsSet(t, tt.flags.isSet())
			tt.checkError(t, tt.flags.check())
		})
	}
}

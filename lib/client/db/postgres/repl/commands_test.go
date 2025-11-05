// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package repl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
)

func TestCommandExecution(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	for name, tt := range map[string]struct {
		line          string
		commandResult string
		expectedArgs  string
		expectUnknown bool
		commandExit   bool
	}{
		"execute":                           {line: "\\test", commandResult: "test"},
		"execute with additional arguments": {line: "\\test a b", commandResult: "test", expectedArgs: "a b"},
		"execute with exit":                 {line: "\\test", commandExit: true},
		"execute with leading and trailing whitespace": {line: "   \\test   ", commandResult: "test"},
		"unknown command with semicolon":               {line: "\\test;", expectUnknown: true},
		"unknown command":                              {line: "\\wrong", expectUnknown: true},
		"with special characters":                      {line: "\\special_chars_!@#$%^&*()}", expectUnknown: true},
		"empty command":                                {line: "\\", expectUnknown: true},
	} {
		t.Run(name, func(t *testing.T) {
			repl, err := New(ctx, &dbrepl.NewREPLConfig{Client: nil, ServerConn: nil, Route: clientproto.RouteToDatabase{}})
			require.NoError(t, err)
			instance := repl.(*REPL)
			// Reset available commands and add a test command so we can assert
			// the command execution flow without relying in commands
			// implementation or test server capabilities.
			instance.commands = map[string]*command{
				"test": {
					ExecFunc: func(_ *REPL, args string) (string, bool) {
						assert.Equal(t, tt.expectedArgs, args)
						return tt.commandResult, tt.commandExit
					},
				},
			}
			reply, shouldExit := instance.processCommand(tt.line)
			if tt.expectUnknown {
				require.True(t, strings.HasPrefix(reply, "Unknown command"))
			} else {
				require.Equal(t, tt.commandResult, reply)
			}
			require.Equal(t, tt.commandExit, shouldExit)
		})
	}
}

func TestCommands(t *testing.T) {
	t.Parallel()
	availableCmds := initCommands()
	for cmdName, tc := range map[string]struct {
		repl               *REPL
		args               string
		expectExit         bool
		assertCommandReply require.ValueAssertionFunc
	}{
		"q": {expectExit: true},
		"teleport": {
			repl: &REPL{teleportVersion: teleport.Version},
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Contains(t, val, teleport.Version, "expected \\teleport command to include current Teleport version")
			},
		},
		"?": {
			repl: &REPL{commands: availableCmds},
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				for cmd := range availableCmds {
					require.Contains(t, val, cmd, "expected \\? command to include information about \\%s", cmd)
				}
			},
		},
		"session": {
			repl: &REPL{route: clientproto.RouteToDatabase{
				ServiceName: "service",
				Username:    "username",
				Database:    "database",
			}},
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Contains(t, val, "service", "expected \\session command to contain service name")
				require.Contains(t, val, "username", "expected \\session command to contain username")
				require.Contains(t, val, "database", "expected \\session command to contain database name")
			},
		},
	} {
		t.Run(cmdName, func(t *testing.T) {
			cmd, ok := availableCmds[cmdName]
			require.True(t, ok, "expected command %q to be available at commands", cmdName)
			reply, exit := cmd.ExecFunc(tc.repl, tc.args)
			if tc.expectExit {
				require.True(t, exit, "expected command to exit the REPL")
				return
			}
			tc.assertCommandReply(t, reply)
		})
	}
}

func FuzzCommands(f *testing.F) {
	f.Add("q")
	f.Add("?")
	f.Add("session")
	f.Add("teleport")

	repl := &REPL{commands: make(map[string]*command)}
	f.Fuzz(func(t *testing.T, line string) {
		require.NotPanics(t, func() {
			_, _ = repl.processCommand(line)
		})
	})
}

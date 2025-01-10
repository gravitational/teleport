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
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
)

func TestCommandExecution(t *testing.T) {
	ctx := context.Background()

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
			commandArgsChan := make(chan string, 1)
			instance, tc := StartWithServer(t, ctx, WithSkipREPLRun())
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			runErrChan := make(chan error)
			go func() {
				runErrChan <- instance.Run(ctx)
			}()

			// Consume the REPL banner.
			_ = readUntilNextLead(t, tc)

			// Reset available commands and add a test command so we can assert
			// the command execution flow without relying in commands
			// implementation or test server capabilities.
			instance.commands = map[string]*command{
				"test": {
					ExecFunc: func(r *REPL, args string) (string, bool) {
						commandArgsChan <- args
						return tt.commandResult, tt.commandExit
					},
				},
			}

			writeLine(t, tc, tt.line)
			if tt.expectUnknown {
				reply := readUntilNextLead(t, tc)
				require.True(t, strings.HasPrefix(strings.ToLower(reply), "unknown command"))
				return
			}

			select {
			case args := <-commandArgsChan:
				require.Equal(t, tt.expectedArgs, args)
			case <-time.After(time.Second):
				require.Fail(t, "expected to command args from test server but got nothing")
			}

			// When the command exits, the REPL and the connections will be
			// closed.
			if tt.commandExit {
				require.EventuallyWithT(t, func(t *assert.CollectT) {
					var buf []byte
					_, err := tc.conn.Read(buf[0:])
					assert.ErrorIs(t, err, io.EOF)
				}, 5*time.Second, time.Millisecond)

				select {
				case err := <-runErrChan:
					require.NoError(t, err, "expected the REPL instance exit gracefully")
				case <-time.After(5 * time.Second):
					require.Fail(t, "expected REPL run to terminate but got nothing")
				}
				return
			}

			reply := readUntilNextLead(t, tc)
			require.Equal(t, tt.commandResult, reply)

			// Terminate the REPL run session and wait for the Run results.
			cancel()
			select {
			case err := <-runErrChan:
				require.ErrorIs(t, err, context.Canceled, "expected the REPL instance to finish running with error due to cancelation")
			case <-time.After(5 * time.Second):
				require.Fail(t, "expected REPL run to terminate but got nothing")
			}
		})
	}
}

func TestCommands(t *testing.T) {
	availableCmds := initCommands()
	for cmdName, tc := range map[string]struct {
		repl               *REPL
		args               string
		expectExit         bool
		assertCommandReply require.ValueAssertionFunc
	}{
		"q": {expectExit: true},
		"teleport": {
			assertCommandReply: func(t require.TestingT, val interface{}, _ ...interface{}) {
				require.Contains(t, val, teleport.Version, "expected \\teleport command to include current Teleport version")
			},
		},
		"?": {
			repl: &REPL{commands: availableCmds},
			assertCommandReply: func(t require.TestingT, val interface{}, _ ...interface{}) {
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
			assertCommandReply: func(t require.TestingT, val interface{}, _ ...interface{}) {
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

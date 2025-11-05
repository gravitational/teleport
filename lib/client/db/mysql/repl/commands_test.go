// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"maps"
	"slices"
	"testing"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	clientproto "github.com/gravitational/teleport/api/client/proto"
)

func TestCommands(t *testing.T) {
	t.Parallel()
	parser, err := newParser()
	require.NoError(t, err)
	commands := parser.commands
	require.NotEmpty(t, commands.byName)
	require.ElementsMatch(t,
		slices.Collect(maps.Values(commands.byName)),
		slices.Collect(maps.Values(commands.byShortcut)),
	)

	for name, cmd := range commands.byName {
		require.NotNil(t, cmd)
		require.NotEmpty(t, cmd.name)
		require.NotZero(t, cmd.shortcut)
		require.NotEmpty(t, cmd.description)
		require.NotNil(t, cmd.execFunc)

		require.Equal(t, cmd.name, name, "expected command to be registered by name")
		byShortcut, ok := commands.byShortcut[cmd.shortcut]
		require.True(t, ok)
		require.Equal(t, cmd, byShortcut, "expected commands from name and shortcut indices to match")
	}

	repl := &REPL{
		parser: parser,
		myConn: &fakeMySQLConn{
			exec: func(command string, args ...any) (*mysql.Result, error) {
				resultSet, err := mysql.BuildSimpleTextResultset([]string{"current_database"}, [][]any{{"example"}})
				if err != nil {
					return nil, err
				}
				resultSet.Values = make([][]mysql.FieldValue, 1)
				resultSet.Values[0], err = resultSet.RowDatas[0].Parse(resultSet.Fields, false, nil)
				return &mysql.Result{
					Resultset: resultSet,
				}, err
			},
		},
		route: clientproto.RouteToDatabase{
			ServiceName: "test-service",
			Username:    "test-user",
			Database:    "test-database",
		},
		teleportVersion: "19.0.0-dev",
	}
	tests := []struct {
		desc               string
		cmdName            string
		args               string
		expectExit         bool
		assertCommandReply require.ValueAssertionFunc
	}{
		{
			desc:    "delimiter command has an empty reply",
			cmdName: "delimiter",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Empty(t, val)
			},
		},
		{
			desc:    "delimiter fails regex",
			cmdName: "delimiter",
			args:    "abc",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Equal(t, `DELIMITER "abc" does not match regex used for validation "^(;|[/]{2}|[$]{2})$"`, val)
			},
		},
		{
			desc:    "help",
			cmdName: "help",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				for name := range commands.byName {
					require.Contains(t, val, name, `expected help command to include information about the %q command`, name)
				}
			},
		},
		{
			desc:       "quit",
			cmdName:    "quit",
			expectExit: true,
		},
		{
			desc:    "status",
			cmdName: "status",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Contains(t, val, "test-service", "expected session command to contain service name")
				require.Contains(t, val, "test-user", "expected session command to contain username")
				require.Contains(t, val, "test-database", "expected session command to contain database name")
			},
		},
		{
			desc:    "teleport",
			cmdName: "teleport",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Contains(t, val, "v19.0.0-dev", "expected teleport command to include current Teleport version")
			},
		},
		{
			desc:    "use requires an arg",
			cmdName: "use",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Equal(t, "USE must be followed by a database name", val)
			},
		},
		{
			desc:    "use with an unquoted argument",
			cmdName: "use",
			args:    "example",
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Equal(t, `Default database changed to "example"`, val)
			},
		},
		{
			desc:    "use with a quoted argument",
			cmdName: "use",
			args:    `"example"`,
			assertCommandReply: func(t require.TestingT, val any, _ ...any) {
				require.Equal(t, `Default database changed to "example"`, val)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cmd, ok := commands.byName[test.cmdName]
			require.True(t, ok, "expected command to be registered")
			reply, exit := cmd.execFunc(repl, test.args)
			if test.expectExit {
				require.True(t, exit, "expected command to exit the REPL")
				return
			}
			test.assertCommandReply(t, reply)
		})
	}
}

type fakeMySQLConn struct {
	exec func(command string, args ...any) (*mysql.Result, error)
}

func (c *fakeMySQLConn) Execute(command string, args ...any) (*mysql.Result, error) {
	return c.exec(command, args...)
}

func (c *fakeMySQLConn) ExecuteSelectStreaming(command string, result *mysql.Result, perRowCallback client.SelectPerRowCallback, perResultCallback client.SelectPerResultCallback) error {
	return trace.NotImplemented("ExecuteSelectStreaming not implemented")
}

func (c *fakeMySQLConn) UseDB(dbName string) error {
	return nil
}

func (c *fakeMySQLConn) GetServerVersion() string {
	return "1.2.3 Fake MySQL Server"
}

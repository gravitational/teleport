/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package cli

import (
	"testing"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDatabaseTunnelCommand tests that the DatabaseTunnelCommand
// properly parses its arguments and applies as expected onto a BotConfig.
func TestDatabaseTunnelCommand(t *testing.T) {
	mockAction := configMutatorMock{}
	mockAction.On("action", mock.Anything).Return(nil)

	app, subcommand := buildMinimalKingpinApp("start")
	cmd := NewDatabaseTunnelCommand(subcommand, mockAction.action)

	// Note: various flags here are already tested as part of sharedStartArgs.
	command, err := app.Parse([]string{
		"start",
		"database-tunnel",
		"--token=foo",
		"--join-method=github",
		"--proxy-server=example.com:443",
		"--listen=tcp://0.0.0.0:8000",
		"--service=foo",
		"--username=bar",
		"--database=baz",
	})
	require.NoError(t, err)

	match, err := cmd.TryRun(command)
	require.NoError(t, err)
	require.True(t, match)

	mockAction.AssertCalled(t, "action", mock.Anything)

	// Convert these args to a BotConfig and check it.
	cfg, err := LoadConfigWithMutators(&GlobalArgs{}, cmd)
	require.NoError(t, err)

	require.Len(t, cfg.Services, 1)

	// It must configure a db tunnel service
	svc := cfg.Services[0]
	db, ok := svc.(*config.DatabaseTunnelService)
	require.True(t, ok)

	require.Equal(t, "tcp://0.0.0.0:8000", db.Listen)
	require.Equal(t, "foo", db.Service)
	require.Equal(t, "bar", db.Username)
	require.Equal(t, "baz", db.Database)
}

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package database

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
)

func TestSQLServerErrors(t *testing.T) {
	pinger := &SQLServerPinger{}

	for _, tt := range []struct {
		desc      string
		err       error
		isErrFunc func(error) bool
	}{
		{
			desc:      "ConnectionRefusedError",
			err:       errors.New("mssql: login error: unable to open tcp connection with host 'sqlserver.ad.teleport.dev:1433': dial tcp 0.0.0.0:1433: i/o timeout"),
			isErrFunc: pinger.IsConnectionRefusedError,
		},
		{
			desc:      "InvalidDatabaseUserError",
			err:       errors.New("mssql: login error: authentication failed"),
			isErrFunc: pinger.IsInvalidDatabaseUserError,
		},
		{
			desc:      "InvalidDatabaseNameError",
			err:       errors.New("mssql: login error: mssql: login error: Cannot open database \"wrong\" that was requested by the login. Using the user default database \"master\" instead"),
			isErrFunc: pinger.IsInvalidDatabaseNameError,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			require.True(t, tt.isErrFunc(tt.err))
		})
	}
}

func TestSQLServerPing(t *testing.T) {
	mockClt := setupMockClient(t)
	testServer, err := sqlserver.NewTestServer(common.TestServerConfig{
		AuthClient: mockClt,
	})
	require.NoError(t, err)

	go func() {
		t.Logf("MSSQL test server running at %s port", testServer.Port())
		testServer.Serve()
	}()
	t.Cleanup(func() {
		testServer.Close()
	})

	port, err := strconv.Atoi(testServer.Port())
	require.NoError(t, err)

	pinger := &SQLServerPinger{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, pinger.Ping(ctx, PingParams{
		Host:         "localhost",
		Port:         port,
		Username:     "alice",
		DatabaseName: "master",
	}))
}

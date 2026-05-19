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
	"strconv"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/db/common"
	libmysql "github.com/gravitational/teleport/lib/srv/db/mysql"
)

func TestMySQLErrors(t *testing.T) {
	p := MySQLPinger{}

	tests := []struct {
		name               string
		pingErr            error
		wantConnRefusedErr bool
		wantDBUserErr      bool
		wantDBNameErr      bool
	}{
		{
			name:               "connection refused string",
			pingErr:            mysql.NewError(mysql.ER_UNKNOWN_ERROR, "Connection Refused"),
			wantConnRefusedErr: true,
		},
		{
			name:               "connection refused host not allowed",
			pingErr:            mysql.NewError(mysql.ER_HOST_NOT_PRIVILEGED, "some message about host"),
			wantConnRefusedErr: true,
		},
		{
			name:               "connection refused host blocked",
			pingErr:            mysql.NewError(mysql.ER_HOST_IS_BLOCKED, "some message about host"),
			wantConnRefusedErr: true,
		},
		{
			name:          "invalid database user access denied",
			pingErr:       mysql.NewError(mysql.ER_ACCESS_DENIED_ERROR, "some message about access denied"),
			wantDBUserErr: true,
		},
		{
			name:          "invalid database user",
			pingErr:       mysql.NewError(mysql.ER_USERNAME, "some message"),
			wantDBUserErr: true,
		},
		{
			name:          "invalid database name access denied",
			pingErr:       mysql.NewError(mysql.ER_DBACCESS_DENIED_ERROR, "some message about access denied to database"),
			wantDBNameErr: true,
		},
		{
			name:          "invalid database name",
			pingErr:       mysql.NewError(mysql.ER_BAD_DB_ERROR, "some message"),
			wantDBNameErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantConnRefusedErr, p.IsConnectionRefusedError(tt.pingErr))
			require.Equal(t, tt.wantDBNameErr, p.IsInvalidDatabaseNameError(tt.pingErr))
			require.Equal(t, tt.wantDBUserErr, p.IsInvalidDatabaseUserError(tt.pingErr))
		})
	}
}

func TestMySQLPing(t *testing.T) {
	mockClt := setupMockClient(t)

	testServer, err := libmysql.NewTestServer(common.TestServerConfig{
		AuthClient: mockClt,
	})
	require.NoError(t, err)

	go func() {
		t.Logf("MySQL Fake server running at %s port", testServer.Port())
		require.NoError(t, testServer.Serve())
	}()
	t.Cleanup(func() {
		testServer.Close()
	})

	port, err := strconv.Atoi(testServer.Port())
	require.NoError(t, err)

	p := MySQLPinger{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	err = p.Ping(ctx, PingParams{
		Host:         "localhost",
		Port:         port,
		Username:     "someuser",
		DatabaseName: "somedb",
	})

	require.NoError(t, err)
}

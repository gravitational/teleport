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

package uacc

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func requireDBEntry(t *testing.T, db *sql.DB, key int64, expectUsername, expectTTY, expectAddr string, expectLoginTime, expectLogoutTime time.Time) {
	var user string
	var tty, remoteHost sql.NullString
	var wtmpType, loginTs, logoutTs sql.NullInt64
	require.NoError(t,
		db.QueryRow("SELECT Type, User, Login, TTY, RemoteHost, Logout FROM wtmp WHERE ID = ?", key).
			Scan(&wtmpType, &user, &loginTs, &tty, &remoteHost, &logoutTs),
	)
	require.Equal(t, int64(userProcess), wtmpType.Int64)
	require.Equal(t, expectUsername, user)
	require.Equal(t, expectTTY, tty.String)
	require.Equal(t, expectAddr, remoteHost.String)
	require.Equal(t, expectLoginTime.UnixMicro(), loginTs.Int64)
	require.Equal(t, expectLogoutTime.UnixMicro(), logoutTs.Int64)
}

func TestWtmpdb(t *testing.T) {
	t.Parallel()
	// Create database.
	dbFile := filepath.Join(t.TempDir(), "wtmp.db")
	wtmpdb, err := NewWtmpdbBackend(dbFile)
	require.NoError(t, err)
	_, err = wtmpdb.db.Exec("CREATE TABLE IF NOT EXISTS wtmp(ID INTEGER PRIMARY KEY, Type INTEGER, User TEXT NOT NULL, Login INTEGER, Logout INTEGER, TTY TEXT, RemoteHost TEXT, Service TEXT) STRICT;")
	require.NoError(t, err)

	// Log in a user.
	remote := &utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        "123.456.789.012",
	}
	loginTime := time.Now()
	key, err := wtmpdb.Login("pts/99", "testuser", remote, loginTime)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	// Check that user was logged.
	requireDBEntry(t, wtmpdb.db, key, "testuser", "pts/99", remote.Addr, loginTime, time.Unix(0, 0))
	isUserLoggedIn, err := wtmpdb.IsUserLoggedIn("testuser")
	require.NoError(t, err)
	require.True(t, isUserLoggedIn)

	// Check that logout is logged.
	logoutTime := loginTime.Add(time.Hour)
	require.NoError(t, wtmpdb.Logout(key, logoutTime))
	requireDBEntry(t, wtmpdb.db, key, "testuser", "pts/99", remote.Addr, loginTime, logoutTime)
	isUserLoggedIn, err = wtmpdb.IsUserLoggedIn("testuser")
	require.NoError(t, err)
	require.False(t, isUserLoggedIn)
}

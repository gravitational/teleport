package uacc

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

func requireDBEntry(t *testing.T, db *sql.DB, key string, expectUsername, expectTTY, expectAddr string, expectLoginTime, expectLogoutTime time.Time) {
	var user string
	var tty, remoteHost sql.NullString
	var wtmpType, loginTs, logoutTs sql.NullInt64
	require.NoError(t,
		db.QueryRow("SELECT Type, User, Login, TTY, RemoteHost, Logout FROM wtmp WHERE ID = ?", key).
			Scan(&wtmpType, &user, &loginTs, &tty, &remoteHost, &logoutTs),
	)
	require.Equal(t, int64(USER_PROCESS), wtmpType.Int64)
	require.Equal(t, expectUsername, user)
	require.Equal(t, expectTTY, tty.String)
	require.Equal(t, expectAddr, remoteHost.String)
	require.Equal(t, expectLoginTime.UnixMicro(), loginTs.Int64)
	require.Equal(t, expectLogoutTime.UnixMicro(), logoutTs.Int64)
}

func TestWtmpdb(t *testing.T) {
	t.Parallel()
	dbFile := filepath.Join(t.TempDir(), "wtmp.db")
	wtmpdb, err := newWtmpdb(dbFile)
	require.NoError(t, err)
	_, err = wtmpdb.db.Exec("CREATE TABLE IF NOT EXISTS wtmp(ID INTEGER PRIMARY KEY, Type INTEGER, User TEXT NOT NULL, Login INTEGER, Logout INTEGER, TTY TEXT, RemoteHost TEXT, Service TEXT) STRICT;")
	require.NoError(t, err)

	remote := &utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        "123.456.789.012",
	}
	loginTime := time.Now()
	key, err := wtmpdb.Login("pts/99", "testuser", remote, loginTime)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	requireDBEntry(t, wtmpdb.db, key, "testuser", "pts/99", remote.Addr, loginTime, time.Unix(0, 0))
	isUserLoggedIn, err := wtmpdb.IsUserLoggedIn("testuser")
	require.NoError(t, err)
	require.True(t, isUserLoggedIn)

	logoutTime := loginTime.Add(time.Hour)
	require.NoError(t, wtmpdb.Logout(key, logoutTime))
	requireDBEntry(t, wtmpdb.db, key, "testuser", "pts/99", remote.Addr, loginTime, logoutTime)
	isUserLoggedIn, err = wtmpdb.IsUserLoggedIn("testuser")
	require.NoError(t, err)
	require.False(t, isUserLoggedIn)
}

//go:build linux

package uacc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertUserPresence(t *testing.T, utmp *UtmpBackend, file, username string, present bool) {
	inFile, err := utmp.IsUserInFile(file, username)
	assert.NoError(t, err)
	assert.Equal(t, present, inFile)
}

func touchFile(t *testing.T, name string) {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	require.NoError(t, err)
	require.NoError(t, file.Close())
}

func makeUtmpBackend(t *testing.T) *UtmpBackend {
	tmpDir := t.TempDir()
	utmpFile := filepath.Join(tmpDir, "utmp")
	wtmpFile := filepath.Join(tmpDir, "wtmp")
	btmpFile := filepath.Join(tmpDir, "btmp")
	touchFile(t, utmpFile)
	touchFile(t, wtmpFile)
	touchFile(t, btmpFile)

	utmp, err := NewUtmpBackend(utmpFile, wtmpFile, btmpFile)
	require.NoError(t, err)
	return utmp
}

func TestUtmp(t *testing.T) {
	t.Parallel()
	utmp := makeUtmpBackend(t)
	remote := &utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        "123.456.789.012:0",
	}
	const goodUser = "good-user"
	assert.NoError(t, utmp.Login("pts/99", goodUser, remote, time.Now()))
	assertUserPresence(t, utmp, utmp.utmpPath, goodUser, true)
	assertUserPresence(t, utmp, utmp.wtmpPath, goodUser, true)
	assertUserPresence(t, utmp, utmp.btmpPath, goodUser, false)
	assert.NoError(t, utmp.Logout("pts/99", time.Now()))

	const badUser = "bad-user"
	assert.NoError(t, utmp.FailedLogin(badUser, remote, time.Now()))
	assertUserPresence(t, utmp, utmp.utmpPath, badUser, false)
	assertUserPresence(t, utmp, utmp.wtmpPath, badUser, false)
	assertUserPresence(t, utmp, utmp.btmpPath, badUser, true)
}

func TestUtmpUsernameLength(t *testing.T) {
	dir := t.TempDir()
	utmpPath := filepath.Join(dir, "utmp")
	wtmpPath := filepath.Join(dir, "wtmp")
	btmpPath := filepath.Join(dir, "btmp")
	touchFile(t, utmpPath)
	touchFile(t, wtmpPath)
	touchFile(t, btmpPath)

	utmp, err := NewUtmpBackend(utmpPath, wtmpPath, btmpPath)

	// A 33 character long username.
	username := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err = utmp.Login("pts/99", username, &utils.NetAddr{Addr: "0.0.0.0:0"}, time.Now())
	require.True(t, trace.IsBadParameter(err))

	// A 32 character long username.
	username = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err = utmp.Login("pts/99", username, &utils.NetAddr{Addr: "0.0.0.0:0"}, time.Now())
	require.False(t, trace.IsBadParameter(err))
}

//go:build linux

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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
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
	t.Parallel()
	utmp := makeUtmpBackend(t)

	// A 33 character long username.
	username := strings.Repeat("a", 33)
	err := utmp.Login("pts/99", username, &utils.NetAddr{Addr: "0.0.0.0:0"}, time.Now())
	require.True(t, trace.IsBadParameter(err))

	// A 32 character long username.
	username = strings.Repeat("a", 32)
	err = utmp.Login("pts/99", username, &utils.NetAddr{Addr: "0.0.0.0:0"}, time.Now())
	require.False(t, trace.IsBadParameter(err))
}

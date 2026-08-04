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

package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

var desktopURI = uri.NewClusterURI("foo").AppendWindowsDesktop("bar")
var login = "admin"

func TestShareDirectory(t *testing.T) {
	basePath := t.TempDir()
	directoryOne := filepath.Join(basePath, "one")
	directoryTwo := filepath.Join(basePath, "two")
	require.NoError(t, os.Mkdir(directoryOne, 0700))
	require.NoError(t, os.Mkdir(directoryTwo, 0700))
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	// Share each directory
	require.NoError(t, session.ShareDirectory(directoryOne, 1))
	require.NoError(t, session.ShareDirectory(directoryTwo, 2))

	// Try to re-use a directory_id
	err = session.ShareDirectory("any_path", 2)
	require.ErrorAs(t, err, new(*trace.AlreadyExistsError))

	// Share invalid directory
	require.ErrorAs(t, session.ShareDirectory("any", 3), new(*trace.NotFoundError))
}

func TestGetDirectory(t *testing.T) {
	path := t.TempDir()
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	_, err = session.GetDirectoryAccess(1)
	require.True(t, trace.IsNotFound(err))

	err = session.ShareDirectory(path, 1)
	require.NoError(t, err)

	access, err := session.GetDirectoryAccess(1)
	require.NoError(t, err)
	_, err = access.Stat("")
	require.NoError(t, err)
}

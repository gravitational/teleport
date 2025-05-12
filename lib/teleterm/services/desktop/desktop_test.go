package desktop

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

var desktopURI = uri.NewClusterURI("foo").AppendWindowsDesktop("bar")
var login = "admin"

func TestAttachDirectory(t *testing.T) {
	path := t.TempDir()
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	// Clean state, share the directory.
	err = session.AttachSharedDirectory(path)
	require.NoError(t, err)

	// Attempt to share another directory.
	err = session.AttachSharedDirectory("any_path")
	require.True(t, trace.IsAlreadyExists(err))
}

func TestGetDirectory(t *testing.T) {
	path := t.TempDir()
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	_, err = session.GetDirectoryAccess()
	require.True(t, trace.IsNotFound(err))

	err = session.AttachSharedDirectory(path)
	require.NoError(t, err)

	access, err := session.GetDirectoryAccess()
	require.NoError(t, err)
	resolvedPath, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	require.Equal(t, resolvedPath, access.basePath)
}

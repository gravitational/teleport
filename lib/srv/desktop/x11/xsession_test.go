package x11

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAvailableXSessions(t *testing.T) {
	_, helperFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(helperFile), "testdata")
	require.NoError(t, os.Setenv("TELEPORT_XSESSIONS_PATH", fixtureDir))
	
	entries, err := GetAvailableXSessions()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, entries["Xfce Session"], "startxfce4")
	require.Equal(t, entries["KDE Plasma"], "start-plasma")
}

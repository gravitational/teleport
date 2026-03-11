package x11

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

func TestStartTeleportExecXSession(t *testing.T) {
	t.Cleanup(func() {
		executablePath = os.Executable
		currentUser = user.Current
	})

	t.Run("empty command", func(t *testing.T) {
		_, err := StartTeleportExecXSession("  ")
		require.Error(t, err)
	})

	t.Run("current user lookup fails", func(t *testing.T) {
		currentUser = func() (*user.User, error) {
			return nil, errors.New("boom")
		}

		_, err := StartTeleportExecXSession("startxfce4")
		require.Error(t, err)
	})

	t.Run("executable lookup fails", func(t *testing.T) {
		currentUser = func() (*user.User, error) {
			return &user.User{Username: "alice"}, nil
		}
		executablePath = func() (string, error) {
			return "", errors.New("boom")
		}

		_, err := StartTeleportExecXSession("startxfce4")
		require.Error(t, err)
	})
}

func TestWaitForPipeClose(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	done := make(chan error, 1)
	go func() {
		done <- waitForPipeClose(r, time.Second)
	}()

	require.NoError(t, w.Close())
	require.NoError(t, <-done)
}

//go:build pam && cgo && linux

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package pam

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestReadStreamEchoOffSuppressesTerminalEcho(t *testing.T) {
	ptm, pts, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ptm.Close())
		require.NoError(t, pts.Close())
	})

	state, err := unix.IoctlGetTermios(int(pts.Fd()), unix.TCGETS)
	require.NoError(t, err)
	require.NotZero(t, state.Lflag&unix.ECHO, "test PTY must start with terminal echo enabled")

	readResultC := make(chan readStreamResult, 1)
	go func() {
		text, err := (&PAM{stdin: pts}).readStream(false)
		readResultC <- readStreamResult{text: text, err: err}
	}()

	// Give readStream a chance to apply echo=false before the user types.
	time.Sleep(50 * time.Millisecond)

	const secret = "test-secret-for-pam\n"
	_, err = ptm.Write([]byte(secret))
	require.NoError(t, err)

	select {
	case result := <-readResultC:
		require.NoError(t, result.err)
		require.Equal(t, secret, result.text)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for readStream")
	}

	require.NoError(t, unix.SetNonblock(int(ptm.Fd()), true))
	echoed, err := io.ReadAll(ptm)
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
		err = nil
	}
	require.NoError(t, err)
	require.NotContains(t, string(echoed), strings.TrimSpace(secret), "PAM echo-off input was echoed to the PTY output")
}

type readStreamResult struct {
	text string
	err  error
}

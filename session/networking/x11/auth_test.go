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

package x11

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestXAuthCommands(t *testing.T) {
	if os.Getenv("TELEPORT_XAUTH_TEST") == "" {
		t.Skip("Skipping test as xauth is not enabled")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	xauthFile := filepath.Join(tmpDir, ".Xauthority")
	display, err := ParseDisplay("unix:10")
	require.NoError(t, err)

	// New xauth file should have no entries
	xauth := NewXAuthCommand(ctx, xauthFile)
	xauthEntry, err := xauth.ReadEntry(display)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, xauthEntry)

	// Add trusted xauth entry
	trustedXauthEntry, err := NewFakeXAuthEntry(display)
	require.NoError(t, err)
	xauth = NewXAuthCommand(ctx, xauthFile)
	err = xauth.AddEntry(*trustedXauthEntry)
	require.NoError(t, err)

	// Read back the xauth entry
	xauth = NewXAuthCommand(ctx, xauthFile)
	xauthEntry, err = xauth.ReadEntry(display)
	require.NoError(t, err)
	require.Equal(t, trustedXauthEntry, xauthEntry)

	// Remove xauth entries
	xauth = NewXAuthCommand(ctx, xauthFile)
	err = xauth.RemoveEntries(xauthEntry.Display)
	require.NoError(t, err)

	xauth = NewXAuthCommand(ctx, xauthFile)
	xauthEntry, err = xauth.ReadEntry(display)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, xauthEntry)

	// TODO(Joerger): Currently this test will only run/pass locally if you have $DISPLAY
	// set. We need to add Xorg to the buildbox and start an XServer in order to run
	// this test in CI. Additionally, Xorg and x11-apps can be used to add more tests
	// and rework current tests which depend on fake display listeners instead.
	localDisplay, err := GetXDisplay()
	if trace.IsBadParameter(err) {
		t.Skip("skipping xauth generate test, DISPLAY isn't set")
	}
	xauth = NewXAuthCommand(ctx, xauthFile)
	err = xauth.GenerateUntrustedCookie(localDisplay, 0)
	require.NoError(t, err)

	xauth = NewXAuthCommand(ctx, xauthFile)
	xauthEntry, err = xauth.ReadEntry(localDisplay)
	require.NoError(t, err)
	require.NotNil(t, xauthEntry)
}

func TestReadAndRewriteXAuthPacket(t *testing.T) {
	t.Parallel()

	realXAuthEntry, err := NewFakeXAuthEntry(Display{})
	require.NoError(t, err)
	realXAuthPacket := mockXAuthPacket(t, realXAuthEntry)

	spoofedXAuthEntry, err := realXAuthEntry.SpoofXAuthEntry()
	require.NoError(t, err)
	spoofedXAuthPacket := mockXAuthPacket(t, spoofedXAuthEntry)

	otherXAuthEntry, err := NewFakeXAuthEntry(Display{})
	require.NoError(t, err)
	otherXAuthPacket := mockXAuthPacket(t, otherXAuthEntry)

	t.Run("match and replace xauth packet", func(t *testing.T) {
		in := bytes.NewBuffer(spoofedXAuthPacket)
		out, err := ReadAndRewriteXAuthPacket(in, spoofedXAuthEntry, realXAuthEntry)
		require.NoError(t, err)
		require.Equal(t, realXAuthPacket, out)
	})

	t.Run("xauth packet doesn't match", func(t *testing.T) {
		in := bytes.NewBuffer(otherXAuthPacket)
		out, err := ReadAndRewriteXAuthPacket(in, spoofedXAuthEntry, realXAuthEntry)
		require.True(t, trace.IsAccessDenied(err))
		require.Empty(t, out)
	})

	t.Run("xauth packet missing xauth data", func(t *testing.T) {
		in := bytes.NewBuffer(mockXAuthPacketInitial(len(mitMagicCookieProto), mitMagicCookieSize))
		out, err := ReadAndRewriteXAuthPacket(in, spoofedXAuthEntry, realXAuthEntry)
		require.Error(t, err)
		require.Empty(t, out)
	})

	t.Run("xauth packet empty", func(t *testing.T) {
		out, err := ReadAndRewriteXAuthPacket(bytes.NewBuffer([]byte{}), spoofedXAuthEntry, realXAuthEntry)
		require.Error(t, err)
		require.Empty(t, out)
	})
}

// mockXAuthPacket creates an xauth packet for the given xauth entry.
func mockXAuthPacket(t *testing.T, entry *XAuthEntry) []byte {
	authData, err := hex.DecodeString(entry.Cookie)
	require.NoError(t, err)

	var xauthPacket []byte
	initPacket := mockXAuthPacketInitial(len(entry.Proto), len(authData))
	xauthPacket = append(xauthPacket, initPacket...)
	xauthPacket = append(xauthPacket, []byte(entry.Proto)...)
	xauthPacket = append(xauthPacket, 0, 0)
	xauthPacket = append(xauthPacket, authData...)
	return xauthPacket
}

// mockXAuthPacketInitial creates the fixed size initial
// portion of an xauth packet, with little endian encoding.
func mockXAuthPacketInitial(authProtoLen, authDataLen int) []byte {
	initData := make([]byte, 12)
	initData[0] = 'l'
	e := binary.LittleEndian
	e.PutUint16(initData[6:8], uint16(authProtoLen))
	e.PutUint16(initData[8:10], uint16(authDataLen))
	return initData
}

// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package x11

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

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

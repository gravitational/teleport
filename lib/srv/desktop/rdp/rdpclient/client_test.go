/*
 * *
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package rdpclient

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

type fakeConn struct {
	bytes.Buffer
	writer bytes.Buffer
}

func (f *fakeConn) Write(p []byte) (n int, err error) {
	return f.writer.Write(p)
}

func (f *fakeConn) Close() error {
	return nil
}

func (f *fakeConn) AddMessage(message tdp.Message) error {
	msg, err := message.Encode()
	if err != nil {
		return err
	}
	_, err = f.Buffer.Write(msg)
	return err
}

func TestClientNew_EOF(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(tdp.ClientUsername{Username: "user"})
	require.NoError(t, err)
	conn := tdp.NewConn(&f)

	_, err = New(createConfig(conn))
	require.EqualError(t, err, "EOF")
}

func TestClientNew_NoKeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(tdp.ClientUsername{Username: "user"})
	require.NoError(t, err)
	err = f.AddMessage(tdp.ClientScreenSpec{
		Width:  100,
		Height: 100,
	})
	require.NoError(t, err)
	err = f.AddMessage(tdp.ClientScreenSpec{
		Width:  100,
		Height: 100,
	})
	require.NoError(t, err)

	conn := tdp.NewConn(&f)

	_, err = New(createConfig(conn))
	require.NoError(t, err)
}

func TestClientNew_KeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(tdp.ClientUsername{Username: "user"})
	require.NoError(t, err)
	err = f.AddMessage(tdp.ClientScreenSpec{
		Width:  100,
		Height: 100,
	})
	require.NoError(t, err)
	err = f.AddMessage(tdp.ClientKeyboardLayout{})
	require.NoError(t, err)
	err = f.AddMessage(tdp.ClientScreenSpec{
		Width:  100,
		Height: 100,
	})
	require.NoError(t, err)

	conn := tdp.NewConn(&f)

	_, err = New(createConfig(conn))
	require.NoError(t, err)
}

func createConfig(conn *tdp.Conn) Config {
	return Config{
		Addr:        "example.com",
		AuthorizeFn: func(login string) error { return nil },
		Conn:        conn,
		Logger:      slog.Default(),
	}
}

func TestRDPClientID(t *testing.T) {
	nilID := rdpClientID{}
	// The MD5 hash of an empty string
	emptyHash := rdpClientID([16]byte{0xD4, 0x1D, 0x8C, 0xD9, 0x8F, 0x00, 0xB2, 0x04, 0xE9, 0x80, 0x09, 0x98, 0xEC, 0xF8, 0x42, 0x7E})

	invalidIDs := []rdpClientID{nilID, emptyHash}
	t.Run("from uuid", func(t *testing.T) {
		// We must continue to attempt to parse strings
		// as UUIDs first.
		newUUID := uuid.New()
		id := newRDPClientID(newUUID.String())
		parsed, err := uuid.FromBytes(id[:])
		require.NoError(t, err)
		// The rdpClientID should just be a UUID under the hood.
		require.Equal(t, newUUID, parsed)
	})

	t.Run("from other string", func(t *testing.T) {
		otherID := "some-other-identifier"
		id := newRDPClientID(otherID)
		// At make sure it's not nil or the empty hash.
		require.NotContains(t, invalidIDs, id)
	})

	t.Run("empty string", func(t *testing.T) {
		// Should match the empty hash.
		require.Equal(t, newRDPClientID(""), emptyHash)
	})

	t.Run("uint32 conversion", func(t *testing.T) {
		// The MD5 hash of an empty string is:
		// d41d8cd9 8f00b204 e9800998 ecf8427e
		// Then represent each 32-bit word as little endian and we get:
		expected := [4]uint32{0xd98c1dd4, 0x04b2008f, 0x980980e9, 0x7e42f8ec}

		// Our [4]uint32 representation of the rdpClientID should
		// match the above. As a bonus, this will serve as a regression
		// test to ensure that we don't switch hash algorithms by mistake.
		got := rdpClientIDToUint32Array[uint32](newRDPClientID(""))
		require.Equal(t, expected, got)
	})
}

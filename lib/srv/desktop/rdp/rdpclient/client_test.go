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
	"io"
	"log/slog"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
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
	conn := tdp.NewConn(&f, tdp.DecoderAdapter(tdpb.DecodePermissive))

	_, err := New(createConfig(conn))
	require.ErrorIs(t, err, io.EOF)
}

func TestClientNew_NoKeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(&tdpb.ClientHello{Username: "user"})
	require.NoError(t, err)

	conn := tdp.NewConn(&f, tdp.DecoderAdapter(tdpb.DecodePermissive))

	_, err = New(createConfig(conn))
	require.NoError(t, err)
}

func TestClientNew_KeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(&tdpb.ClientHello{Username: "user", KeyboardLayout: 1})
	require.NoError(t, err)

	conn := tdp.NewConn(&f, tdp.DecoderAdapter(tdpb.DecodePermissive))

	_, err = New(createConfig(conn))
	require.NoError(t, err)

}

func createConfig(conn *tdp.Conn) Config {
	return Config{
		Addr:        "example.com4",
		AuthorizeFn: func(login string) error { return nil },
		Conn:        conn,
		Logger:      slog.Default(),
		Width:       1,
		Height:      1,
	}
}

func TestEncodeQOIZ(t *testing.T) {
	// this test aim is to verify we get the same data as in tests on Rust side:
	// encode_qoiz_single in encoder.rs
	frames, err := EncodeQOIZ([]byte{0xFF, 0xFF, 0xFF, 0xFF}, 0, 0, 1, 1)
	require.NoError(t, err)
	require.Len(t, frames, 1)
	require.Equal(t, []byte{0, 59, 4, 54, 0, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 11, 1, 0, 1, 0, 32, 0, 0,
		0, 40, 181, 47, 253, 0, 88, 185, 0, 0, 113, 111, 105, 102, 0, 0, 0, 1, 0, 0, 0, 1,
		3, 0, 85, 0, 0, 0, 0, 0, 0, 0, 1}, frames[0].Pdu)

	// test with random data to verify we get correct number of frames from Rust
	data := make([]byte, 4*500*500)
	for i := 0; i < 4*500*500; i += 4 {
		data[i] = byte(rand.IntN(256))
		data[i+1] = byte(rand.IntN(256))
		data[i+2] = byte(rand.IntN(256))
		data[i+3] = 0xFF
	}

	frames, err = EncodeQOIZ(data, 0, 0, 500, 500)
	require.NoError(t, err)
	require.Len(t, frames, 27)
}

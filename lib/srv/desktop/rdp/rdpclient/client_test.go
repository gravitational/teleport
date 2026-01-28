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

	_, err := New(conn, createConfig())
	require.ErrorIs(t, err, io.EOF)
}

func TestClientNew_NoKeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(&tdpb.ClientHello{Username: "user"})
	require.NoError(t, err)

	conn := tdp.NewConn(&f, tdp.DecoderAdapter(tdpb.DecodePermissive))

	_, err = New(conn, createConfig())
	require.NoError(t, err)
}

func TestClientNew_KeyboardLayout(t *testing.T) {
	f := fakeConn{}
	err := f.AddMessage(&tdpb.ClientHello{Username: "user", KeyboardLayout: 1})
	require.NoError(t, err)

	conn := tdp.NewConn(&f, tdp.DecoderAdapter(tdpb.DecodePermissive))

	_, err = New(conn, createConfig())
	require.NoError(t, err)

}

func createConfig() Config {
	return Config{
		Addr:           "example.com",
		AuthorizeFn:    func(login string) error { return nil },
		Logger:         slog.Default(),
		Width:          1,
		Height:         1,
		ClientProtocol: tdpb.ProtocolName,
	}
}

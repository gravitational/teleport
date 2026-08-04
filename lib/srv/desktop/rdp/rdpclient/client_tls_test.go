//go:build desktop_access_rdp

/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"runtime/cgo"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
)

// x224ConnectionConfirmSSL is an X.224 Connection Confirm PDU whose RDP
// negotiation response selects PROTOCOL_SSL. This informs the RDP client
// that it's time to upgrade to TLS.
var x224ConnectionConfirmSSL = []byte{
	0x03, 0x00, 0x00, 0x13, // TPKT header, total length 19
	0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00, // X.224 CC TPDU
	0x02, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00, // RDP_NEG_RSP, PROTOCOL_SSL
}

// startFakeRDPServer listens on localhost and, for the first connection,
// reads the X.224 Connection Request, replies with a Connection Confirm that
// selects TLS, and then hands the connection to afterNegotiation.
func startFakeRDPServer(t *testing.T, afterNegotiation func(net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the X.224 Connection Request. TPKT frames it with a
		// big-endian total length in bytes 2-3.
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		total := int(binary.BigEndian.Uint16(hdr[2:4]))
		if total < len(hdr) {
			return
		}
		if _, err := io.ReadFull(conn, make([]byte, total-len(hdr))); err != nil {
			return
		}

		if _, err := conn.Write(x224ConnectionConfirmSSL); err != nil {
			return
		}

		afterNegotiation(conn)
	}()

	return ln.Addr().String()
}

// newTestClient builds a Client wired up to addr with a live cgo handle,
// returning the client and the buffer that captures TDP messages sent back
// to the browser.
func newTestClient(t *testing.T, addr string) (*Client, *fakeConn) {
	t.Helper()

	f := &fakeConn{}
	require.NoError(t, f.AddMessage(&tdpb.ClientHello{Username: "user"}))

	conn := tdp.NewConn(f, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)
	wrappedConn, hello, err := PrepareConnecton(tdpb.ProtocolName, conn, slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	cfg := createConfig()
	cfg.Addr = addr
	cfg.NLA = false
	cfg.AD = false

	c, err := New(wrappedConn, hello, cfg)
	require.NoError(t, err)

	c.handle = cgo.NewHandle(c)
	t.Cleanup(c.handle.Delete)

	return c, f
}

func TestTLSHandshakeFailureIsReported(t *testing.T) {
	addr := startFakeRDPServer(t, func(conn net.Conn) {
		// The client now expects a TLS ServerHello. Send something that
		// is not a ServerHello, which will cause the handshake to fail.
		_, _ = conn.Write([]byte("this is not a TLS record"))
		// Hold the connection open so the failure is the handshake itself
		// rather than a premature EOF.
		time.Sleep(5 * time.Second)
	})

	c, _ := newTestClient(t, addr)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	err := c.startRustRDP(ctx, []byte("fake-cert"), []byte("fake-key"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "TLS handshake with "+addr+" failed",
		"the TLS failure should be surfaced, got: %v", err)
}

func TestDialTimeoutIsReported(t *testing.T) {
	// This address is guaranteed to be unreachable:
	// https://www.rfc-editor.org/info/rfc5737/#section-3
	c, f := newTestClient(t, "203.0.113.1:3389")

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := c.startRustRDP(ctx, []byte("fake-cert"), []byte("fake-key"))
	require.ErrorContains(t, err, "Connection Timed Out")
	require.Less(t, time.Since(start), 15*time.Second, "should give up after dial timeout")

	// Make sure the error is also written to the client (browser).
	require.Contains(t, f.writer.String(), "Connection Timed Out")
}

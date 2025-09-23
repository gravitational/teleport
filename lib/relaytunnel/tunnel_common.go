// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package relaytunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/protobuf/proto"

	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// yamuxTunnelALPN is the ALPN protocol name for the bespoke protocol used by
// tunnel connections. Future variants of the protocol should use a different
// ALPN protocol name.
const yamuxTunnelALPN = "teleport-relaytunnel"

// The teleport-relaytunnel protocol consists of a "control" yamux stream that's
// opened by the client at the beginning of the session and stays open
// throughout its lifetime, and some "dialing" streams opened by the server
// afterwards.
//
// In the control stream the client sends a ClientHello message, the server
// responds with a ServerHello message which contains a google.rpc.Status, and
// if the status is ok, the stream continues with ClientControl messages sent by
// the client and ServerControl messages sent by the server.
//
// In dialing streams the server sends a DialRequest message, the client
// responds with a DialResponse message containing a google.rpc.Status, and in
// case of success the data of the connection immediately follows on both ends.
//
// Messages are sent in their protobuf wire format, prefixed by a little endian
// 32 bit size. Messages must be smaller than maxMessageSize (128KiB).

const maxMessageSize = 128 * 1024

func readProto(r io.Reader, m proto.Message) error {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf[:])
	if size > maxMessageSize {
		return trace.LimitExceeded("bad size")
	}

	msgBuf := make([]byte, size)
	if _, err := io.ReadFull(r, msgBuf); err != nil {
		if errors.Is(err, io.EOF) {
			return trace.Wrap(io.ErrUnexpectedEOF)
		}
		return trace.Wrap(err)
	}

	if err := proto.Unmarshal(msgBuf, m); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeProto(w io.Writer, m proto.Message) error {
	msgBuf, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	if len(msgBuf) > maxMessageSize {
		return trace.LimitExceeded("bad size")
	}
	var sizeBuf [4]byte
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(msgBuf)))
	if _, err := w.Write(sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.Write(msgBuf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func addrToProto(a net.Addr) *relaytunnelv1alpha.Addr {
	if a == nil {
		return nil
	}

	return &relaytunnelv1alpha.Addr{
		Network: a.Network(),
		Addr:    a.String(),
	}
}

func addrFromProto(a *relaytunnelv1alpha.Addr) net.Addr {
	if a == nil {
		return nil
	}

	return &utils.NetAddr{
		AddrNetwork: a.GetNetwork(),
		Addr:        a.GetAddr(),
	}
}

type yamuxLogger slog.Logger

var _ yamux.Logger = (*yamuxLogger)(nil)

// Printf implements [yamux.Logger].
func (l *yamuxLogger) Printf(format string, args ...any) {
	if f, ok := strings.CutPrefix(format, "[ERR] "); ok {
		//nolint:sloglint // we're adapting a fmt.Printf-like interface
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(f, args...))
	} else if f, ok := strings.CutPrefix(format, "[WARN] "); ok {
		//nolint:sloglint // we're adapting a fmt.Printf-like interface
		(*slog.Logger)(l).WarnContext(context.Background(), fmt.Sprintf(f, args...))
	} else {
		//nolint:sloglint // we're adapting a fmt.Printf-like interface
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(format, args...))
	}
}

// Print implements [yamux.Logger].
func (l *yamuxLogger) Print(args ...any) {
	// the Print method doesn't seem to be used by yamux, it's only implemented
	// here for completeness' sake

	//nolint:sloglint // we're adapting a fmt.Print-like interface
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprint(args...))
}

// Println implements [yamux.Logger].
func (l *yamuxLogger) Println(args ...any) {
	// the Println method doesn't seem to be used by yamux, it's only
	// implemented here for completeness' sake, and the concept of adding a
	// newline at the end of a log message is so weird that we will just avoid
	// doing that and just redirect to Print instead

	l.Print(args...)
}

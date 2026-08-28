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

package relaypeer

import (
	"encoding/binary"
	"errors"
	"io"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	relaypeeringv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaypeering/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

// simpleALPN is the ALPN protocol name for the bespoke protocol used by peer
// connections. Future variants of the protocol should use a different ALPN
// protocol name.
const simpleALPN = "teleport-relaypeer"

// The teleport-relaypeer protocol consists of a DialRequest message sent by the
// client followed by a DialResponse message sent by the server, containing a
// google.rpc.Status. If the status is ok, the data for the connection will then
// follow.
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

func addrToProto(a net.Addr) *relaypeeringv1alpha.Addr {
	if a == nil {
		return nil
	}

	return &relaypeeringv1alpha.Addr{
		Network: a.Network(),
		Addr:    a.String(),
	}
}

func addrFromProto(a *relaypeeringv1alpha.Addr) net.Addr {
	if a == nil {
		return nil
	}

	return &utils.NetAddr{
		AddrNetwork: a.GetNetwork(),
		Addr:        a.GetAddr(),
	}
}

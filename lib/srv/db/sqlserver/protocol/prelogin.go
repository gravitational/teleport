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

package protocol

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
)

// PreLoginPacket represents a Pre-Login packet which is sent by the client
// to set up context for login.
//
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/60f56408-0188-4cd5-8b90-25c6f2423868
type PreLoginPacket struct {
	packet Packet
}

// ReadPreLoginPacket reads Pre-Login packet from the reader.
func ReadPreLoginPacket(r io.Reader) (*PreLoginPacket, error) {
	pkt, err := ReadPacket(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pkt.Type() != PacketTypePreLogin {
		return nil, trace.BadParameter("expected Pre-Login packet, got: %#v", pkt)
	}

	return &PreLoginPacket{
		packet: pkt,
	}, nil
}

// WritePreLoginResponse writes response to the Pre-Login packet to the writer.
func WritePreLoginResponse(w io.Writer) error {
	var buf bytes.Buffer
	if err := mssql.WritePreLoginFields(&buf, preLoginOptions); err != nil {
		return trace.Wrap(err)
	}

	pkt, err := makePacket(PacketTypeResponse, buf.Bytes())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = w.Write(pkt)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

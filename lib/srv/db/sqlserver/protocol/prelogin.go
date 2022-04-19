/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package protocol

import (
	"bytes"
	"io"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/gravitational/trace"
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

	if pkt.Type != PacketTypePreLogin {
		return nil, trace.BadParameter("expected Pre-Login packet, got: %#v", pkt)
	}

	return &PreLoginPacket{
		packet: *pkt,
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

/*
Copyright 2023 Gravitational, Inc.

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
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

// AcceptPacket defines TNS Oracle package
// that is sent by the Oracle Server as a successful
// response on Connect packet.
type AcceptPacket struct {
	*packet
	// ProtocolVersion contains Oracle Server protocol version
	ProtocolVersion uint16
	// ProtocolOptions contains Oracle Server protocol options.
	ProtocolOptions uint16
}

func parseAcceptPacket(basicPacket *packet) (Packet, error) {
	r := bytes.NewReader(basicPacket.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	acceptPaket := &AcceptPacket{
		packet: basicPacket,
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolVersion); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolOptions); err != nil {
		return nil, trace.Wrap(err)
	}
	return acceptPaket, nil
}

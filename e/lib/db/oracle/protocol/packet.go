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

// Packet defines a basic packet interfaces.
type Packet interface {
	// Size returns the Oracle packet size.
	Size() uint32
	// Type returns the Oracle packet type.
	Type() Type
	// Payload returns raw Oracle packet data.
	Payload() []byte
	// writer is used internally to send a packet
	writer
}

type packet struct {
	*Header
	buff   []byte
	reader io.Reader
}

func (b *packet) write(conn *oracleConn) error {
	_, err := conn.Write(b.buff)
	return trace.Wrap(err)
}

// Size return the total size of the packet.
func (b *packet) Size() uint32 {
	return b.PacketSize
}

// Type returns packet type
func (b *packet) Type() Type {
	return b.PacketType
}

// Payload returns all package data payload.
func (b *packet) Payload() []byte {
	return b.buff
}

// DataPacket defines TNS data oracle packet
// that is used a generic transport unit in oracle wire protocol.
type DataPacket struct {
	*packet
}

// Header defines the TNS Oracle packet header.
type Header struct {
	// PacketSize is the size of a packet.
	PacketSize uint32
	// PacketType is a TNS Oracle packet type.
	PacketType Type
	// headerBytes contains raw bytes of the header.
	headerBytes []byte
}

// parsePacket read the package payload and returns Oracle Packet.
func parsePacket(bp *packet, c *oracleConn) (Packet, error) {
	switch bp.PacketType {
	case ACCEPT:
		ac, err := parseAcceptPacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ac, nil
	case RESEND:
		return &ResendPacket{
			packet: bp,
		}, nil
	case DATA:
		return &DataPacket{
			packet: bp,
		}, nil
	case REDIRECT:
		return &RedirectPacket{
			packet: bp,
		}, nil
	case CONNECT:
		cp, err := parseConnectPacket(bp, c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cp, nil
	case REFUSE:
		ac, err := parseRefusePacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ac, nil
	default:
		return bp, nil
	}
}

func readString(r io.Reader) (string, error) {
	// Read string length.
	var strLength uint8
	if err := binary.Read(r, binary.BigEndian, &strLength); err != nil {
		return "", trace.Wrap(err)
	}

	// Read string content.
	buff := bytes.NewBuffer(make([]byte, 0, strLength))
	if _, err := io.CopyN(buff, r, int64(strLength)); err != nil {
		return "", trace.Wrap(err)
	}
	return buff.String(), nil
}

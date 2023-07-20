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
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

// PacketHeader represents a 8-byte packet header.
//
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/7af53667-1b72-4703-8258-7984e838f746
//
// Note: the order of fields in the struct matters as it gets unpacked from the
// binary stream.
type PacketHeader struct {
	Type     uint8
	Status   uint8
	Length   uint16 // network byte order (big-endian)
	SPID     uint16 // network byte order (big-endian)
	PacketID uint8
	Window   uint8
}

// Marshal marshals the packet header to the wire protocol byte representation.
func (h *PacketHeader) Marshal() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, packetHeaderSize))
	if err := binary.Write(buf, binary.BigEndian, h); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

// Packet is a packet interface.
type Packet interface {
	// Bytes returns whole packet bytes.
	Bytes() []byte
	// Data returns packet data without data related to Header.
	Data() []byte
	// Header returns packet Header definition.
	Header() PacketHeader
	// Type returns packet type ID.
	Type() uint8
}

// BasicPacket implements the Packet interfaces allowing to operate on
// PacketHeader and get underlying packet type.
type BasicPacket struct {
	header PacketHeader
	data   []byte
	raw    bytes.Buffer
}

// Bytes returns raw packet bytes.
func (g BasicPacket) Bytes() []byte {
	return g.raw.Bytes()
}

// Data is the packet data bytes without header.
func (g BasicPacket) Data() []byte {
	return g.data
}

// Header is the parsed packet header.
func (g BasicPacket) Header() PacketHeader {
	return g.header
}

// Type is the parsed packet header.
func (g BasicPacket) Type() uint8 {
	return g.header.Type
}

// ReadPacket reads a single full packet from the reader.
func ReadPacket(r io.Reader) (*BasicPacket, error) {
	var buff bytes.Buffer
	tr := io.TeeReader(r, &buff)
	// Read 8-byte packet header.
	var headerBytes [packetHeaderSize]byte
	if _, err := io.ReadFull(tr, headerBytes[:]); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Unmarshal packet header from the binary form.
	var header PacketHeader
	if err := binary.Read(bytes.NewReader(headerBytes[:]), binary.BigEndian, &header); err != nil {
		return nil, trace.Wrap(err)
	}

	// Read packet data. Packet length includes header.
	dataBytes := make([]byte, header.Length-packetHeaderSize)
	if _, err := io.ReadFull(tr, dataBytes); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	p := &BasicPacket{
		header: header,
		data:   dataBytes,
		raw:    buff,
	}
	return p, nil
}

// NewBasicPacket creates a new BasicPacket instance with the specified
// PacketHeader and data.
func NewBasicPacket(header PacketHeader, data []byte) (*BasicPacket, error) {
	headerBytes, err := header.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := bytes.NewBuffer(append(headerBytes, data...))
	return &BasicPacket{
		header: header,
		data:   data,
		raw:    *raw,
	}, nil
}

// ToSQLPacket tries to convert basicPacket to MSServer SQL packet.
func ToSQLPacket(p *BasicPacket) (out Packet, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = trace.BadParameter("failed to convert packet to SQL packet: %v", r)
		}
	}()

	switch p.Type() {
	case PacketTypeRPCRequest:
		sqlBatch, err := toRPCRequest(p)
		if err != nil {
			return p, trace.Wrap(err)
		}
		return sqlBatch, trace.Wrap(err)
	case PacketTypeSQLBatch:
		rpcRequest, err := toSQLBatch(p)
		if err != nil {
			return p, trace.Wrap(err)
		}
		return rpcRequest, trace.Wrap(err)
	}
	return p, trace.Wrap(err)
}

// makePacket prepends header to the provided packet data.
func makePacket(pktType uint8, pktData []byte) ([]byte, error) {
	header := PacketHeader{
		Type:   pktType,
		Status: PacketStatusLast,
		Length: uint16(packetHeaderSize + len(pktData)),
	}

	headerBytes, err := header.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return append(headerBytes, pktData...), nil
}

// IsFinalPacket returns true there are no more packets on the message.
func IsFinalPacket(packet Packet) bool {
	return packet.Header().Status&PacketStatusLast != 0
}

package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

type PacketFlags byte

type PacketHeader struct {
	PacketSize     uint32      // Length of the data
	PacketType     Type        // Packet type
	PacketFlags    PacketFlags // Packet flags
	PacketChecksum uint16      // PacketChecksum (only for basic version)
	HeaderChecksum uint16      // HeaderChecksum (optional)

	// Extended packet header is set for packets read under large SDU protocol (>=315).
	Extended bool

	HeaderBytes []byte
}

const (
	PacketFlagRedirect       PacketFlags = 0x4
	PacketFlagTLSRenegotiate PacketFlags = 0x8
)

func (pf PacketFlags) HasFlag(flag PacketFlags) bool {
	return pf&flag != 0
}

func (pf PacketFlags) ToString() string {
	bin := formatBinary(pf)
	var flags []string
	if pf.HasFlag(PacketFlagRedirect) {
		flags = append(flags, "Redirect")
	}
	if pf.HasFlag(PacketFlagTLSRenegotiate) {
		flags = append(flags, "TLS Renegotiate")
	}

	joined := strings.Join(flags, "|")
	if joined != "" {
		return fmt.Sprintf("PacketFlags(%v, %v)", bin, joined)
	} else {
		return fmt.Sprintf("PacketFlags(%v)", bin)
	}
}

func parseHeader(protocolVersion uint16, r io.Reader) (*PacketHeader, error) {
	data := make([]byte, PacketHeaderSize)
	count, err := r.Read(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if count != PacketHeaderSize {
		return nil, trace.BadParameter("invalid packet size, expected %v got %v", PacketHeaderSize, count)
	}

	var header PacketHeader

	header.HeaderBytes = data // Store the raw bytes for forwarding

	if protocolVersion < TNSVersionMinLargeSdu {
		header.PacketSize = uint32(binary.BigEndian.Uint16(data[0:2]))
		header.PacketChecksum = binary.BigEndian.Uint16(data[2:4])
	} else {
		header.PacketSize = binary.BigEndian.Uint32(data[0:4])
		header.Extended = true
	}

	header.PacketType = Type(data[4])
	header.PacketFlags = PacketFlags(data[5])
	header.HeaderChecksum = binary.BigEndian.Uint16(data[6:8])

	return &header, nil
}

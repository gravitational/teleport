/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// MessageOpCompressed represents parsed OP_COMPRESSED wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_compressed
//
//	struct {
//	    MsgHeader header;           // standard message header
//	    int32  originalOpcode;      // value of wrapped opcode
//	    int32  uncompressedSize;    // size of deflated compressedMessage, excluding MsgHeader
//	    uint8  compressorId;        // ID of compressor that compressed message
//	    char    *compressedMessage; // opcode itself, excluding MsgHeader
//	}
type MessageOpCompressed struct {
	Header            MessageHeader
	OriginalOpcode    wiremessage.OpCode
	UncompressedSize  int32
	CompressorID      wiremessage.CompressorID
	CompressedMessage []byte
	// originalMessage is the decompressed wire message.
	originalMessage Message
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// GetHeader returns the wire message header.
func (m *MessageOpCompressed) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpCompressed) GetBytes() []byte {
	return m.bytes
}

// String returns the message string representation.
func (m *MessageOpCompressed) String() string {
	return m.originalMessage.String()
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpCompressed) MoreToCome(msg Message) bool {
	return m.originalMessage.MoreToCome(msg)
}

// GetDatabase returns database for the wrapped message.
func (m *MessageOpCompressed) GetDatabase() (string, error) {
	return m.originalMessage.GetDatabase()
}

// GetCommand returns the message's command.
func (m *MessageOpCompressed) GetCommand() (string, error) {
	return m.originalMessage.GetCommand()
}

// GetOriginal returns original decompressed message.
func (m *MessageOpCompressed) GetOriginal() Message {
	return m.originalMessage
}

// readOpCompressed converts OP_COMPRESSED wire message bytes to a structured form.
func readOpCompressed(header MessageHeader, payload []byte) (message *MessageOpCompressed, err error) {
	originalOpcode, rem, ok := wiremessage.ReadCompressedOriginalOpCode(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: missing original opcode %v", payload)
	}
	if originalOpcode == wiremessage.OpCompressed { // Just being a little paranoid.
		return nil, trace.BadParameter("malformed OP_COMPRESSED: double-compressed")
	}
	uncompressedSize, rem, ok := wiremessage.ReadCompressedUncompressedSize(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: missing uncompressed size %v", payload)
	}
	compressorID, rem, ok := wiremessage.ReadCompressedCompressorID(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: missing compressor ID %v", payload)
	}
	compressedSize := header.MessageLength - 25 // header (16) + original opcode (4) + uncompressed size (4) + compressor ID (1)
	compressedMessage, _, ok := wiremessage.ReadCompressedCompressedMessage(rem, compressedSize)
	if !ok {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: missing compressed message %v", payload)
	}
	message = &MessageOpCompressed{
		Header:            header,
		OriginalOpcode:    originalOpcode,
		UncompressedSize:  uncompressedSize,
		CompressorID:      compressorID,
		CompressedMessage: compressedMessage,
		bytes:             append(header.bytes[:], payload...),
	}
	if uncompressedSize <= 0 || len(compressedMessage) == 0 {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: invalid message size %v", payload)
	} else if uncompressedSize > int32(defaultMaxMessageSizeBytes) {
		return nil, trace.BadParameter("malformed OP_COMPRESSED: uncompressed size exceeded max %v", payload)
	}

	message.originalMessage, err = decompress(message)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return message, nil
}

// ToWire converts this message to wire protocol message bytes.
func (m *MessageOpCompressed) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpCompressed)
	dst = wiremessage.AppendCompressedOriginalOpCode(dst, m.OriginalOpcode)
	dst = wiremessage.AppendCompressedUncompressedSize(dst, m.UncompressedSize)
	dst = wiremessage.AppendCompressedCompressorID(dst, m.CompressorID)
	dst = wiremessage.AppendCompressedCompressedMessage(dst, m.CompressedMessage)
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}

// decompress returns the original message from the compressed message.
func decompress(message *MessageOpCompressed) (Message, error) {
	// Make the uncompressed message's header.
	header := make([]byte, 0, headerSizeBytes)
	header = wiremessage.AppendHeader(header,
		headerSizeBytes+message.UncompressedSize,
		message.Header.RequestID,
		message.Header.ResponseTo,
		message.OriginalOpcode)

	// Decompress the payload.
	decompressedPayload, err := driver.DecompressPayload(
		message.CompressedMessage,
		driver.CompressionOpts{
			Compressor:       message.CompressorID,
			UncompressedSize: message.UncompressedSize,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the uncompressed message.
	return ReadMessage(bytes.NewReader(append(
		header, decompressedPayload...)))
}

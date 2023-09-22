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
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// Message defines common interface for MongoDB wire protocol messages.
type Message interface {
	// GetHeader returns the wire message header.
	GetHeader() MessageHeader
	// GetBytes returns raw wire message bytes read from the connection.
	GetBytes() []byte
	// ToWire returns the message as wire bytes format.
	ToWire(responseTo int32) []byte
	// MoreToCome is whether sender will send another message right after this one.
	MoreToCome(message Message) bool
	// GetDatabase returns the message's database (for client messages).
	GetDatabase() (string, error)
	// GetCommand returns the message's command (for client messages).
	GetCommand() (string, error)
	// Stringer dumps message in the readable format for logs and audit.
	fmt.Stringer
}

// ReadMessage reads the next MongoDB wire protocol message from the reader.
func ReadMessage(reader io.Reader, maxMessageSize uint32) (Message, error) {
	header, payload, err := readHeaderAndPayload(reader, maxMessageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch header.OpCode {
	case wiremessage.OpMsg:
		return readOpMsg(*header, payload)
	case wiremessage.OpQuery:
		return readOpQuery(*header, payload)
	case wiremessage.OpGetMore:
		return readOpGetMore(*header, payload)
	case wiremessage.OpInsert:
		return readOpInsert(*header, payload)
	case wiremessage.OpUpdate:
		return readOpUpdate(*header, payload)
	case wiremessage.OpDelete:
		return readOpDelete(*header, payload)
	case wiremessage.OpCompressed:
		return readOpCompressed(*header, payload, maxMessageSize)
	case wiremessage.OpReply:
		return readOpReply(*header, payload)
	case wiremessage.OpKillCursors:
		return readOpKillCursors(*header, payload)
	}
	return nil, trace.BadParameter("unknown wire protocol message: %v %v",
		*header, payload)
}

// ReadServerMessage reads wire protocol message from the MongoDB server connection.
func ReadServerMessage(ctx context.Context, conn driver.Connection, maxMessageSize uint32) (Message, error) {
	var wm []byte
	wm, err := conn.ReadWireMessage(ctx, wm)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadMessage(bytes.NewReader(wm), maxMessageSize)
}

func readHeaderAndPayload(reader io.Reader, maxMessageSize uint32) (*MessageHeader, []byte, error) {
	// First read message header which is 16 bytes.
	var header [headerSizeBytes]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	length, requestID, responseTo, opCode, _, ok := wiremessage.ReadHeader(header[:])
	if !ok {
		return nil, nil, trace.BadParameter("failed to read message header %v", header)
	}

	// Check if the payload size will underflow when we extract the header size from it.
	if length < math.MinInt32+headerSizeBytes {
		return nil, nil, trace.BadParameter("invalid header size %v", header)
	}

	payloadLength := uint32(length - headerSizeBytes)
	if payloadLength >= maxMessageSize {
		return nil, nil, trace.BadParameter("exceeded the maximum message size, got length: %d", length)
	}

	if length-headerSizeBytes <= 0 {
		return nil, nil, trace.BadParameter("invalid header %v", header)
	}

	// Then read the entire message body.
	payloadBuff := bytes.NewBuffer(make([]byte, 0, buffAllocCapacity(payloadLength, maxMessageSize)))
	if _, err := io.CopyN(payloadBuff, reader, int64(payloadLength)); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &MessageHeader{
		MessageLength: length,
		RequestID:     requestID,
		ResponseTo:    responseTo,
		OpCode:        opCode,
		bytes:         header,
	}, payloadBuff.Bytes(), nil
}

// DefaultMaxMessageSizeBytes is the default max size of mongoDB message. This
// value is only used if the MongoDB doesn't impose any value. Defaults to
// double size of MongoDB default.
const DefaultMaxMessageSizeBytes = uint32(48000000) * 2

// buffCapacity returns the capacity for the payload buffer.
func buffAllocCapacity(payloadLength, maxMessageSize uint32) uint32 {
	if payloadLength >= maxMessageSize {
		return maxMessageSize
	}
	return payloadLength
}

// MessageHeader represents parsed MongoDB wire protocol message header.
//
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#standard-message-header
type MessageHeader struct {
	MessageLength int32
	RequestID     int32
	ResponseTo    int32
	OpCode        wiremessage.OpCode
	// bytes is the wire message header bytes read from the connection.
	bytes [headerSizeBytes]byte
}

const (
	headerSizeBytes = 16
)

const (
	// IsMasterCommand is legacy handshake command name.
	IsMasterCommand = "isMaster"
	// HelloCommand is the handshake command name.
	HelloCommand = "hello"
)

// IsHandshake returns true if the message is a handshake request.
func IsHandshake(m Message) bool {
	cmd, err := m.GetCommand()
	if err != nil {
		return false
	}

	// Servers must accept alternative casing for IsMasterCommand.
	return strings.EqualFold(cmd, IsMasterCommand) || cmd == HelloCommand
}

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

	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/gravitational/trace"
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
func ReadMessage(reader io.Reader) (Message, error) {
	header, payload, err := readHeaderAndPayload(reader)
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
		return readOpCompressed(*header, payload)
	case wiremessage.OpReply:
		return readOpReply(*header, payload)
	case wiremessage.OpKillCursors:
		return readOpKillCursors(*header, payload)
	}
	return nil, trace.BadParameter("unknown wire protocol message: %v %v",
		*header, payload)
}

// ReadServerMessage reads wire protocol message from the MongoDB server connection.
func ReadServerMessage(ctx context.Context, conn driver.Connection) (Message, error) {
	var wm []byte
	wm, err := conn.ReadWireMessage(ctx, wm)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadMessage(bytes.NewReader(wm))
}

func readHeaderAndPayload(reader io.Reader) (*MessageHeader, []byte, error) {
	// First read message header which is 16 bytes.
	var header [headerSizeBytes]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	length, requestID, responseTo, opCode, _, ok := wiremessage.ReadHeader(header[:])
	if !ok {
		return nil, nil, trace.BadParameter("failed to read message header %v", header)
	}

	if length-headerSizeBytes <= 0 {
		return nil, nil, trace.BadParameter("invalid header %v", header)
	}

	// Then read the entire message body.
	payload := make([]byte, length-headerSizeBytes)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &MessageHeader{
		MessageLength: length,
		RequestID:     requestID,
		ResponseTo:    responseTo,
		OpCode:        opCode,
		bytes:         header,
	}, payload, nil
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

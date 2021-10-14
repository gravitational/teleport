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
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/stretchr/testify/require"
)

// TestOpMsgSingleBody verifies marshal/unmarshal for single-document OP_MSG wire message.
func TestOpMsgSingleBody(t *testing.T) {
	t.Parallel()

	// Make OP_MSG message.
	message := makeTestOpMsg(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get command name.
	command, err := parsed.GetCommand()
	require.NoError(t, err)
	require.Equal(t, "find", command)

	// Verify we can get database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestMalformedOpMsg verifies that malformed OP_MSG wire messages are not let through.
func TestMalformedOpMsg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document bson.D
	}{
		{
			name: "missing $db key",
			document: bson.D{
				{Key: "find", Value: 1},
				{Key: "a", Value: "b"},
			},
		},
		{
			name: "empty $db key",
			document: bson.D{
				{Key: "find", Value: 1},
				{Key: "a", Value: "b"},
				{Key: "$db", Value: ""},
			},
		},
		{
			name: "multiple $db keys",
			document: bson.D{
				{Key: "find", Value: 1},
				{Key: "a", Value: "b"},
				{Key: "$db", Value: "test1"},
				{Key: "$db", Value: "test2"},
			},
		},
		{
			name: "invalid $db value",
			document: bson.D{
				{Key: "find", Value: 1},
				{Key: "a", Value: "b"},
				{Key: "$db", Value: 42},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, err := bson.Marshal(test.document)
			require.NoError(t, err)

			message := makeTestOpMsgWithBody(t, document)
			parsed, err := ReadMessage(bytes.NewReader(message.bytes))
			require.NoError(t, err)

			_, err = parsed.GetDatabase()
			require.Error(t, err)
		})
	}
}

// TestOpMsgDocumentSequence verifies marshal/unmarshal for multi-document OP_MSG wire message.
func TestOpMsgDocumentSequence(t *testing.T) {
	t.Parallel()

	message := &MessageOpMsg{
		BodySection: SectionBody{
			Document: makeTestDocument(t),
		},
		DocumentSequenceSections: []SectionDocumentSequence{
			{
				Identifier: "insert",
				Documents: []bsoncore.Document{
					makeTestDocument(t),
					makeTestDocument(t),
				},
			},
		},
	}

	// Marshal the message to its wire representation.
	message.bytes = message.ToWire(0)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)

	// Make sure we got the same message back.
	message.Header = parsed.GetHeader()
	require.Equal(t, message, parsed)
}

// TestOpReply verifies marshal/unmarshal for OP_REPLY wire message.
func TestOpReply(t *testing.T) {
	t.Parallel()

	// Make OP_REPLY message.
	message := makeTestOpReply(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)
}

// TestOpQuery verifies marshal/unmarshal for OP_QUERY wire message.
func TestOpQuery(t *testing.T) {
	t.Parallel()

	// Make OP_QUERY message.
	message := makeTestOpQuery(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get command name.
	command, err := parsed.GetCommand()
	require.NoError(t, err)
	require.Equal(t, "find", command)

	// Verify we can get the database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestOpGetMore verifies marshal/unmarshal for OP_GET_MORE wire message.
func TestOpGetMore(t *testing.T) {
	t.Parallel()

	// Make OP_GET_MORE message.
	message := makeTestOpGetMore(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get the database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestOpInsert verifies marshal/unmarshal for OP_INSERT wire message.
func TestOpInsert(t *testing.T) {
	t.Parallel()

	// Make OP_INSERT message.
	message := makeTestOpInsert(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get the database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestOpUpdate verifies marshal/unmarshal for OP_UPDATE wire message.
func TestOpUpdate(t *testing.T) {
	t.Parallel()

	// Make OP_UPDATE message.
	message := makeTestOpUpdate(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get the database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestOpDelete verifies marshal/unmarshal for OP_DELETE wire message.
func TestOpDelete(t *testing.T) {
	t.Parallel()

	// Make OP_DELETE message.
	message := makeTestOpDelete(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)

	// Verify we can get the database name.
	database, err := parsed.GetDatabase()
	require.NoError(t, err)
	require.Equal(t, "test", database)
}

// TestOpKillCursors verifies marshal/unmarshal for OP_KILL_CURSORS wire message.
func TestOpKillCursors(t *testing.T) {
	t.Parallel()

	// Make OP_KILL_CURSORS message.
	message := makeTestOpKillCursors(t)

	// Read it back.
	parsed, err := ReadMessage(bytes.NewReader(message.bytes))
	require.NoError(t, err)
	require.Equal(t, message, parsed)
}

// TestOpCompressed verifies marshal/unmarshal for OP_COMPRESSED wire message.
func TestOpCompressed(t *testing.T) {
	t.Parallel()

	// OP_COMPRESSED can wrap any other message type.
	tests := []struct {
		name    string
		message Message
	}{
		{
			name:    "compressed OP_MSG",
			message: makeTestOpMsg(t),
		},
		{
			name:    "compressed OP_QUERY",
			message: makeTestOpQuery(t),
		},
		{
			name:    "compressed OP_GET_MORE",
			message: makeTestOpGetMore(t),
		},
		{
			name:    "compressed OP_INSERT",
			message: makeTestOpInsert(t),
		},
		{
			name:    "compressed OP_UPDATE",
			message: makeTestOpUpdate(t),
		},
		{
			name:    "compressed OP_DELETE",
			message: makeTestOpDelete(t),
		},
		{
			name:    "compressed OP_REPLY",
			message: makeTestOpReply(t),
		},
	}

	// Verify we get the same message back after decompressing.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Wrap the message in OP_COMPRESSED.
			compressedMessage := makeTestOpCompressed(t, test.message)

			// Read it back.
			parsed, err := ReadMessage(bytes.NewReader(compressedMessage.bytes))
			require.NoError(t, err)
			require.Equal(t, compressedMessage, parsed)

			// Make sure we can get the original message.
			originalMessage := parsed.(*MessageOpCompressed).GetOriginal()
			require.Equal(t, test.message, originalMessage)
		})
	}
}

func makeTestOpCompressed(t *testing.T, message Message) *MessageOpCompressed {
	// Marshal the original message to wire representation.
	bytes := message.ToWire(0)

	// Compress the message payload, excluding header.
	compressed, err := driver.CompressPayload(bytes[headerSizeBytes:],
		driver.CompressionOpts{
			Compressor: wiremessage.CompressorZLib,
			ZlibLevel:  wiremessage.DefaultZlibLevel,
		})
	require.NoError(t, err)

	// Wrap the message in OP_COMPRESSED.
	msg := &MessageOpCompressed{
		OriginalOpcode:    message.GetHeader().OpCode,
		UncompressedSize:  int32(len(bytes) - headerSizeBytes),
		CompressorID:      wiremessage.CompressorZLib,
		CompressedMessage: compressed,
		originalMessage:   message,
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpCompressed)
	return msg
}

func makeTestOpMsg(t *testing.T) *MessageOpMsg {
	return makeTestOpMsgWithBody(t, makeTestDocument(t))
}

func makeTestOpMsgWithBody(t *testing.T, doc []byte) *MessageOpMsg {
	msg := &MessageOpMsg{
		Flags: wiremessage.ChecksumPresent,
		BodySection: SectionBody{
			Document: doc,
		},
		Checksum: 123,
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpMsg)
	return msg
}

func makeTestOpQuery(t *testing.T) *MessageOpQuery {
	msg := &MessageOpQuery{
		Flags:                wiremessage.AwaitData,
		FullCollectionName:   "test.collection",
		NumberToSkip:         1,
		NumberToReturn:       1,
		Query:                makeTestDocument(t),
		ReturnFieldsSelector: makeTestDocument(t),
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpQuery)
	return msg
}

func makeTestOpGetMore(t *testing.T) *MessageOpGetMore {
	msg := &MessageOpGetMore{
		Zero:               0,
		FullCollectionName: "test.collection",
		NumberToReturn:     5,
		CursorID:           1234567890,
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpGetMore)
	return msg
}

func makeTestOpInsert(t *testing.T) *MessageOpInsert {
	msg := &MessageOpInsert{
		Flags:              1,
		FullCollectionName: "test.collection",
		Documents: []bsoncore.Document{
			makeTestDocument(t),
			makeTestDocument(t)},
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpInsert)
	return msg
}

func makeTestOpUpdate(t *testing.T) *MessageOpUpdate {
	msg := &MessageOpUpdate{
		Zero:               0,
		FullCollectionName: "test.collection",
		Flags:              1,
		Selector:           makeTestDocument(t),
		Update:             makeTestDocument(t),
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpUpdate)
	return msg
}

func makeTestOpDelete(t *testing.T) *MessageOpDelete {
	msg := &MessageOpDelete{
		Zero:               0,
		FullCollectionName: "test.collection",
		Flags:              1,
		Selector:           makeTestDocument(t),
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpDelete)
	return msg
}

func makeTestOpKillCursors(t *testing.T) *MessageOpKillCursors {
	msg := &MessageOpKillCursors{
		Zero:              0,
		NumberOfCursorIDs: 3,
		CursorIDs:         []int64{1, 2, 3},
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpKillCursors)
	return msg
}

func makeTestOpReply(t *testing.T) *MessageOpReply {
	msg := &MessageOpReply{
		Flags:          wiremessage.QueryFailure,
		CursorID:       1,
		StartingFrom:   1,
		NumberReturned: 1,
		Documents: []bsoncore.Document{
			makeTestDocument(t),
		},
	}
	msg.bytes = msg.ToWire(0)
	msg.Header = makeTestHeader(msg.bytes, wiremessage.OpReply)
	return msg
}

func makeTestHeader(msg []byte, op wiremessage.OpCode) MessageHeader {
	var headerBytes [headerSizeBytes]byte
	copy(headerBytes[:], msg[:headerSizeBytes])
	return MessageHeader{
		MessageLength: int32(len(msg)),
		OpCode:        op,
		bytes:         headerBytes,
	}
}

func makeTestDocument(t *testing.T) []byte {
	document, err := bson.Marshal(bson.D{
		{Key: "find", Value: 1},
		{Key: "a", Value: "b"},
		{Key: "$db", Value: "test"},
	})
	require.NoError(t, err)
	return document
}

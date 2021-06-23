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
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/stretchr/testify/require"
)

// TestOpMsgSingleBody verifies marshal/unmarshal for single-document OP_MSG wire message.
func TestOpMsgSingleBody(t *testing.T) {
	t.Parallel()

	message := &MessageOpMsg{
		Flags: wiremessage.ChecksumPresent,
		Sections: []Section{
			&SectionBody{
				Document: makeTestDocument(t),
			},
		},
		Checksum: 123,
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

// TestOpMsgDocumentSequence verifies marshal/unmarshal for multi-document OP_MSG wire message.
func TestOpMsgDocumentSequence(t *testing.T) {
	t.Parallel()

	message := &MessageOpMsg{
		Sections: []Section{
			&SectionDocumentSequence{
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

	message := &MessageOpReply{
		Flags:          wiremessage.QueryFailure,
		CursorID:       1,
		StartingFrom:   1,
		NumberReturned: 1,
		Documents: []bsoncore.Document{
			makeTestDocument(t),
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

// TestOpQuery verifies marshal/unmarshal for OP_QUERY wire message.
func TestOpQuery(t *testing.T) {
	t.Parallel()

	message := &MessageOpQuery{
		Flags:                wiremessage.AwaitData,
		FullCollectionName:   "db.collection",
		NumberToSkip:         1,
		NumberToReturn:       1,
		Query:                makeTestDocument(t),
		ReturnFieldsSelector: makeTestDocument(t),
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

func makeTestDocument(t *testing.T) []byte {
	document, err := bson.Marshal(bson.M{"a": "b"})
	require.NoError(t, err)
	return document
}

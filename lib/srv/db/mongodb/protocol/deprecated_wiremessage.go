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
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// This file contains logic which has been deprecated from MongoDB's client library, but needs to be supported for
// our backwards compatibility needs. This deprecation started in MongoDB 1.13.0.

// OpmsgWireVersion is the minimum wire version needed to use OP_MSG
const OpmsgWireVersion = 6

// ReadQueryFlags reads OP_QUERY flags from src.
func ReadQueryFlags(src []byte) (flags wiremessage.QueryFlag, rem []byte, ok bool) {
	i32, rem, ok := readInt32(src)
	return wiremessage.QueryFlag(i32), rem, ok
}

// ReadQueryFullCollectionName reads the full collection name from src.
func ReadQueryFullCollectionName(src []byte) (collname string, rem []byte, ok bool) {
	return readCString(src)
}

// ReadQueryNumber is a replacement for ReadQueryNumberToSkip or ReadQueryNumberToSkip. This function reads a 32 bit
// integer from src.
func ReadQueryNumber(src []byte) (nts int32, rem []byte, ok bool) {
	return readInt32(src)
}

// ReadDocument is a replacement for ReadQueryQuery or ReadQueryReturnFieldsSelector.  This function reads a bson
// document from src.
func ReadDocument(src []byte) (rfs bsoncore.Document, rem []byte, ok bool) {
	return bsoncore.ReadDocument(src)
}

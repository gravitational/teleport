/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

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

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// ReadQueryFlags reads OP_QUERY flags from src.
func ReadQueryFlags(src []byte) (flags wiremessage.QueryFlag, rem []byte, ok bool) {
	i32, rem, ok := readi32(src)
	return wiremessage.QueryFlag(i32), rem, ok
}

// ReadQueryFullCollectionName reads the full collection name from src.
func ReadQueryFullCollectionName(src []byte) (collname string, rem []byte, ok bool) {
	return readcstring(src)
}

// ReadQueryNumber is a replacement for ReadQueryNumberToSkip or ReadQueryNumberToSkip. This function reads a 32 bit
// integer from src.
func ReadQueryNumber(src []byte) (nts int32, rem []byte, ok bool) {
	return readi32(src)
}

// ReadDocument is a replacement for ReadQueryQuery or ReadQueryReturnFieldsSelector.  This function reads a bson
// document from src.
func ReadDocument(src []byte) (rfs bsoncore.Document, rem []byte, ok bool) {
	return bsoncore.ReadDocument(src)
}

func readi32(src []byte) (int32, []byte, bool) {
	if len(src) < 4 {
		return 0, src, false
	}

	return int32(src[0]) | int32(src[1])<<8 | int32(src[2])<<16 | int32(src[3])<<24, src[4:], true
}

func readcstring(src []byte) (string, []byte, bool) {
	idx := bytes.IndexByte(src, 0x00)
	if idx < 0 {
		return "", src, false
	}
	return string(src[:idx]), src[idx+1:], true
}

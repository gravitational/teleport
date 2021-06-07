// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

// Package cryptofmt provides constants and convenience methods that define the
// format of ciphertexts and signatures.
package cryptofmt

import (
	"encoding/binary"
	"fmt"

	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

const (
	// NonRawPrefixSize is the prefix size of Tink and Legacy key types.
	NonRawPrefixSize = 5

	// LegacyPrefixSize is the prefix size of legacy key types.
	// The prefix starts with \x00 and followed by a 4-byte key id.
	LegacyPrefixSize = NonRawPrefixSize
	// LegacyStartByte is the first byte of the prefix of legacy key types.
	LegacyStartByte = byte(0)

	// TinkPrefixSize is the prefix size of Tink key types.
	// The prefix starts with \x01 and followed by a 4-byte key id.
	TinkPrefixSize = NonRawPrefixSize
	// TinkStartByte is the first byte of the prefix of Tink key types.
	TinkStartByte = byte(1)

	// RawPrefixSize is the prefix size of Raw key types.
	// Raw prefix is empty.
	RawPrefixSize = 0
	// RawPrefix is the empty prefix of Raw key types.
	RawPrefix = ""
)

// OutputPrefix generates the prefix of ciphertexts produced by the crypto
// primitive obtained from key.  The prefix can be either empty (for RAW-type
// prefix), or consists of a 1-byte indicator of the type of the prefix,
// followed by 4 bytes of the key ID in big endian encoding.
func OutputPrefix(key *tinkpb.Keyset_Key) (string, error) {
	switch key.OutputPrefixType {
	case tinkpb.OutputPrefixType_LEGACY, tinkpb.OutputPrefixType_CRUNCHY:
		return createOutputPrefix(LegacyPrefixSize, LegacyStartByte, key.KeyId), nil
	case tinkpb.OutputPrefixType_TINK:
		return createOutputPrefix(TinkPrefixSize, TinkStartByte, key.KeyId), nil
	case tinkpb.OutputPrefixType_RAW:
		return RawPrefix, nil
	default:
		return "", fmt.Errorf("crypto_format: unknown output prefix type")
	}
}

func createOutputPrefix(size int, startByte byte, keyID uint32) string {
	prefix := make([]byte, size)
	prefix[0] = startByte
	binary.BigEndian.PutUint32(prefix[1:], keyID)
	return string(prefix)
}

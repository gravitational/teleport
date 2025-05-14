// Copyright 2025 Gravitational, Inc
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

package types

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
)

// EncryptOAEP encrypts data using OAEP with the public key and hash present
// in the EncryptionKey receiver.
func (k EncryptionKeyPair) EncryptOAEP(plaintext []byte) ([]byte, error) {
	pub, err := keys.ParsePublicKey(k.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hash := crypto.SHA256
	if k.Hash > 0 {
		hash = crypto.Hash(k.Hash)
	}
	switch pubKey := pub.(type) {
	case *rsa.PublicKey:
		ciphertext, err := rsa.EncryptOAEP(hash.New(), rand.Reader, pubKey, plaintext, nil)
		return ciphertext, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported encryption public key type %T", pub)
	}
}

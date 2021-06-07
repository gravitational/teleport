// Copyright 2018 Google LLC
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

package aead

import (
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
	"github.com/golang/protobuf/proto"
	"github.com/google/tink/go/aead/subtle"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/subtle/random"

	cppb "github.com/google/tink/go/proto/chacha20_poly1305_go_proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

const (
	chaCha20Poly1305KeyVersion = 0
	chaCha20Poly1305TypeURL    = "type.googleapis.com/google.crypto.tink.ChaCha20Poly1305Key"
)

// Common errors.
var errInvalidChaCha20Poly1305Key = fmt.Errorf("chacha20poly1305_key_manager: invalid key")
var errInvalidChaCha20Poly1305KeyFormat = fmt.Errorf("chacha20poly1305_key_manager: invalid key format")

// chaCha20Poly1305KeyManager is an implementation of KeyManager interface.
// It generates new ChaCha20Poly1305Key keys and produces new instances of ChaCha20Poly1305 subtle.
type chaCha20Poly1305KeyManager struct{}

// newChaCha20Poly1305KeyManager creates a new chaCha20Poly1305KeyManager.
func newChaCha20Poly1305KeyManager() *chaCha20Poly1305KeyManager {
	return new(chaCha20Poly1305KeyManager)
}

// Primitive creates an ChaCha20Poly1305 subtle for the given serialized ChaCha20Poly1305Key proto.
func (km *chaCha20Poly1305KeyManager) Primitive(serializedKey []byte) (interface{}, error) {
	if len(serializedKey) == 0 {
		return nil, errInvalidChaCha20Poly1305Key
	}
	key := new(cppb.ChaCha20Poly1305Key)
	if err := proto.Unmarshal(serializedKey, key); err != nil {
		return nil, errInvalidChaCha20Poly1305Key
	}
	if err := km.validateKey(key); err != nil {
		return nil, err
	}
	ret, err := subtle.NewChaCha20Poly1305(key.KeyValue)
	if err != nil {
		return nil, fmt.Errorf("chacha20poly1305_key_manager: cannot create new primitive: %s", err)
	}
	return ret, nil
}

// NewKey creates a new key, ignoring the specification in the given serialized key format
// because the key size and other params are fixed.
func (km *chaCha20Poly1305KeyManager) NewKey(serializedKeyFormat []byte) (proto.Message, error) {
	return km.newChaCha20Poly1305Key(), nil
}

// NewKeyData creates a new KeyData ignoring the specification in the given serialized key format
// because the key size and other params are fixed.
// It should be used solely by the key management API.
func (km *chaCha20Poly1305KeyManager) NewKeyData(serializedKeyFormat []byte) (*tinkpb.KeyData, error) {
	key := km.newChaCha20Poly1305Key()
	serializedKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	return &tinkpb.KeyData{
		TypeUrl:         chaCha20Poly1305TypeURL,
		Value:           serializedKey,
		KeyMaterialType: tinkpb.KeyData_SYMMETRIC,
	}, nil
}

// DoesSupport indicates if this key manager supports the given key type.
func (km *chaCha20Poly1305KeyManager) DoesSupport(typeURL string) bool {
	return typeURL == chaCha20Poly1305TypeURL
}

// TypeURL returns the key type of keys managed by this key manager.
func (km *chaCha20Poly1305KeyManager) TypeURL() string {
	return chaCha20Poly1305TypeURL
}

func (km *chaCha20Poly1305KeyManager) newChaCha20Poly1305Key() *cppb.ChaCha20Poly1305Key {
	keyValue := random.GetRandomBytes(chacha20poly1305.KeySize)
	return &cppb.ChaCha20Poly1305Key{
		Version:  chaCha20Poly1305KeyVersion,
		KeyValue: keyValue,
	}
}

// validateKey validates the given ChaCha20Poly1305Key.
func (km *chaCha20Poly1305KeyManager) validateKey(key *cppb.ChaCha20Poly1305Key) error {
	err := keyset.ValidateKeyVersion(key.Version, chaCha20Poly1305KeyVersion)
	if err != nil {
		return fmt.Errorf("chacha20poly1305_key_manager: %s", err)
	}
	keySize := uint32(len(key.KeyValue))
	if keySize != chacha20poly1305.KeySize {
		return fmt.Errorf("chacha20poly1305_key_manager: keySize != %d", chacha20poly1305.KeySize)
	}
	return nil
}

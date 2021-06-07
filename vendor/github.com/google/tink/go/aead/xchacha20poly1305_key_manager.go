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
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/subtle/random"

	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
	xcppb "github.com/google/tink/go/proto/xchacha20_poly1305_go_proto"
)

const (
	xChaCha20Poly1305KeyVersion = 0
	xChaCha20Poly1305TypeURL    = "type.googleapis.com/google.crypto.tink.XChaCha20Poly1305Key"
)

// Common errors.
var errInvalidXChaCha20Poly1305Key = fmt.Errorf("xchacha20poly1305_key_manager: invalid key")

// xChaCha20Poly1305KeyManager is an implementation of KeyManager interface.
// It generates new XChaCha20Poly1305Key keys and produces new instances of XChaCha20Poly1305 subtle.
type xChaCha20Poly1305KeyManager struct{}

// Assert that xChaCha20Poly1305KeyManager implements the KeyManager interface.
var _ registry.KeyManager = (*xChaCha20Poly1305KeyManager)(nil)

// newXChaCha20Poly1305KeyManager creates a new xChaCha20Poly1305KeyManager.
func newXChaCha20Poly1305KeyManager() *xChaCha20Poly1305KeyManager {
	return new(xChaCha20Poly1305KeyManager)
}

// Primitive creates an XChaCha20Poly1305 subtle for the given serialized XChaCha20Poly1305Key proto.
func (km *xChaCha20Poly1305KeyManager) Primitive(serializedKey []byte) (interface{}, error) {
	if len(serializedKey) == 0 {
		return nil, errInvalidXChaCha20Poly1305Key
	}
	key := new(xcppb.XChaCha20Poly1305Key)
	if err := proto.Unmarshal(serializedKey, key); err != nil {
		return nil, errInvalidXChaCha20Poly1305Key
	}
	if err := km.validateKey(key); err != nil {
		return nil, err
	}
	ret, err := subtle.NewXChaCha20Poly1305(key.KeyValue)
	if err != nil {
		return nil, fmt.Errorf("xchacha20poly1305_key_manager: cannot create new primitive: %s", err)
	}
	return ret, nil
}

// NewKey creates a new key, ignoring the specification in the given serialized key format
// because the key size and other params are fixed.
func (km *xChaCha20Poly1305KeyManager) NewKey(serializedKeyFormat []byte) (proto.Message, error) {
	return km.newXChaCha20Poly1305Key(), nil
}

// NewKeyData creates a new KeyData, ignoring the specification in the given serialized key format
// because the key size and other params are fixed.
// It should be used solely by the key management API.
func (km *xChaCha20Poly1305KeyManager) NewKeyData(serializedKeyFormat []byte) (*tinkpb.KeyData, error) {
	key := km.newXChaCha20Poly1305Key()
	serializedKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	return &tinkpb.KeyData{
		TypeUrl:         xChaCha20Poly1305TypeURL,
		Value:           serializedKey,
		KeyMaterialType: tinkpb.KeyData_SYMMETRIC,
	}, nil
}

// DoesSupport indicates if this key manager supports the given key type.
func (km *xChaCha20Poly1305KeyManager) DoesSupport(typeURL string) bool {
	return typeURL == xChaCha20Poly1305TypeURL
}

// TypeURL returns the key type of keys managed by this key manager.
func (km *xChaCha20Poly1305KeyManager) TypeURL() string {
	return xChaCha20Poly1305TypeURL
}

func (km *xChaCha20Poly1305KeyManager) newXChaCha20Poly1305Key() *xcppb.XChaCha20Poly1305Key {
	keyValue := random.GetRandomBytes(chacha20poly1305.KeySize)
	return &xcppb.XChaCha20Poly1305Key{
		Version:  xChaCha20Poly1305KeyVersion,
		KeyValue: keyValue,
	}
}

// validateKey validates the given XChaCha20Poly1305Key.
func (km *xChaCha20Poly1305KeyManager) validateKey(key *xcppb.XChaCha20Poly1305Key) error {
	err := keyset.ValidateKeyVersion(key.Version, xChaCha20Poly1305KeyVersion)
	if err != nil {
		return fmt.Errorf("xchacha20poly1305_key_manager: %s", err)
	}
	keySize := uint32(len(key.KeyValue))
	if keySize != chacha20poly1305.KeySize {
		return fmt.Errorf("xchacha20poly1305_key_manager: keySize != %d", chacha20poly1305.KeySize)
	}
	return nil
}

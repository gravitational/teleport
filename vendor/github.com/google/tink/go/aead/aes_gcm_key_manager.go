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

	"github.com/golang/protobuf/proto"
	"github.com/google/tink/go/aead/subtle"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/subtle/random"
	gcmpb "github.com/google/tink/go/proto/aes_gcm_go_proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

const (
	aesGCMKeyVersion = 0
	aesGCMTypeURL    = "type.googleapis.com/google.crypto.tink.AesGcmKey"
)

// common errors
var errInvalidAESGCMKey = fmt.Errorf("aes_gcm_key_manager: invalid key")
var errInvalidAESGCMKeyFormat = fmt.Errorf("aes_gcm_key_manager: invalid key format")

// aesGCMKeyManager is an implementation of KeyManager interface.
// It generates new AESGCMKey keys and produces new instances of AESGCM subtle.
type aesGCMKeyManager struct{}

// Assert that aesGCMKeyManager implements the KeyManager interface.
var _ registry.KeyManager = (*aesGCMKeyManager)(nil)

// newAESGCMKeyManager creates a new aesGcmKeyManager.
func newAESGCMKeyManager() *aesGCMKeyManager {
	return new(aesGCMKeyManager)
}

// Primitive creates an AESGCM subtle for the given serialized AESGCMKey proto.
func (km *aesGCMKeyManager) Primitive(serializedKey []byte) (interface{}, error) {
	if len(serializedKey) == 0 {
		return nil, errInvalidAESGCMKey
	}
	key := new(gcmpb.AesGcmKey)
	if err := proto.Unmarshal(serializedKey, key); err != nil {
		return nil, errInvalidAESGCMKey
	}
	if err := km.validateKey(key); err != nil {
		return nil, err
	}
	ret, err := subtle.NewAESGCM(key.KeyValue)
	if err != nil {
		return nil, fmt.Errorf("aes_gcm_key_manager: cannot create new primitive: %s", err)
	}
	return ret, nil
}

// NewKey creates a new key according to specification the given serialized AESGCMKeyFormat.
func (km *aesGCMKeyManager) NewKey(serializedKeyFormat []byte) (proto.Message, error) {
	if len(serializedKeyFormat) == 0 {
		return nil, errInvalidAESGCMKeyFormat
	}
	keyFormat := new(gcmpb.AesGcmKeyFormat)
	if err := proto.Unmarshal(serializedKeyFormat, keyFormat); err != nil {
		return nil, errInvalidAESGCMKeyFormat
	}
	if err := km.validateKeyFormat(keyFormat); err != nil {
		return nil, fmt.Errorf("aes_gcm_key_manager: invalid key format: %s", err)
	}
	keyValue := random.GetRandomBytes(keyFormat.KeySize)
	return &gcmpb.AesGcmKey{
		Version:  aesGCMKeyVersion,
		KeyValue: keyValue,
	}, nil
}

// NewKeyData creates a new KeyData according to specification in the given serialized
// AESGCMKeyFormat.
// It should be used solely by the key management API.
func (km *aesGCMKeyManager) NewKeyData(serializedKeyFormat []byte) (*tinkpb.KeyData, error) {
	key, err := km.NewKey(serializedKeyFormat)
	if err != nil {
		return nil, err
	}
	serializedKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	return &tinkpb.KeyData{
		TypeUrl:         aesGCMTypeURL,
		Value:           serializedKey,
		KeyMaterialType: tinkpb.KeyData_SYMMETRIC,
	}, nil
}

// DoesSupport indicates if this key manager supports the given key type.
func (km *aesGCMKeyManager) DoesSupport(typeURL string) bool {
	return typeURL == aesGCMTypeURL
}

// TypeURL returns the key type of keys managed by this key manager.
func (km *aesGCMKeyManager) TypeURL() string {
	return aesGCMTypeURL
}

// validateKey validates the given AESGCMKey.
func (km *aesGCMKeyManager) validateKey(key *gcmpb.AesGcmKey) error {
	err := keyset.ValidateKeyVersion(key.Version, aesGCMKeyVersion)
	if err != nil {
		return fmt.Errorf("aes_gcm_key_manager: %s", err)
	}
	keySize := uint32(len(key.KeyValue))
	if err := subtle.ValidateAESKeySize(keySize); err != nil {
		return fmt.Errorf("aes_gcm_key_manager: %s", err)
	}
	return nil
}

// validateKeyFormat validates the given AESGCMKeyFormat.
func (km *aesGCMKeyManager) validateKeyFormat(format *gcmpb.AesGcmKeyFormat) error {
	if err := subtle.ValidateAESKeySize(format.KeySize); err != nil {
		return fmt.Errorf("aes_gcm_key_manager: %s", err)
	}
	return nil
}

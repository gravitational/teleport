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

package aead

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/keyset"
	kmsepb "github.com/google/tink/go/proto/kms_envelope_go_proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

const (
	kmsEnvelopeAEADKeyVersion = 0
	kmsEnvelopeAEADTypeURL    = "type.googleapis.com/google.crypto.tink.KmsEnvelopeAeadKey"
)

// kmsEnvelopeAEADKeyManager is an implementation of KeyManager interface.
// It generates new KMSEnvelopeAEADKey keys and produces new instances of KMSEnvelopeAEAD subtle.
type kmsEnvelopeAEADKeyManager struct{}

// newKMSEnvelopeAEADKeyManager creates a new aesGcmKeyManager.
func newKMSEnvelopeAEADKeyManager() *kmsEnvelopeAEADKeyManager {
	return new(kmsEnvelopeAEADKeyManager)
}

// Primitive creates an KMSEnvelopeAEAD subtle for the given serialized KMSEnvelopeAEADKey proto.
func (km *kmsEnvelopeAEADKeyManager) Primitive(serializedKey []byte) (interface{}, error) {
	if len(serializedKey) == 0 {
		return nil, errors.New("kms_envelope_aead_key_manager: invalid key")
	}
	key := new(kmsepb.KmsEnvelopeAeadKey)
	if err := proto.Unmarshal(serializedKey, key); err != nil {
		return nil, errors.New("kms_envelope_aead_key_manager: invalid key")
	}
	if err := km.validateKey(key); err != nil {
		return nil, err
	}
	uri := key.Params.KekUri
	kmsClient, err := registry.GetKMSClient(uri)
	if err != nil {
		return nil, err
	}
	backend, err := kmsClient.GetAEAD(uri)
	if err != nil {
		return nil, errors.New("kms_envelope_aead_key_manager: invalid aead backend")
	}

	return NewKMSEnvelopeAEAD2(key.Params.DekTemplate, backend), nil
}

// NewKey creates a new key according to specification the given serialized KMSEnvelopeAEADKeyFormat.
func (km *kmsEnvelopeAEADKeyManager) NewKey(serializedKeyFormat []byte) (proto.Message, error) {
	if len(serializedKeyFormat) == 0 {
		return nil, errors.New("kms_envelope_aead_key_manager: invalid key format")
	}
	keyFormat := new(kmsepb.KmsEnvelopeAeadKeyFormat)
	if err := proto.Unmarshal(serializedKeyFormat, keyFormat); err != nil {
		return nil, errors.New("kms_envelope_aead_key_manager: invalid key format")
	}
	return &kmsepb.KmsEnvelopeAeadKey{
		Version: kmsEnvelopeAEADKeyVersion,
		Params:  keyFormat,
	}, nil
}

// NewKeyData creates a new KeyData according to specification in the given serialized
// KMSEnvelopeAEADKeyFormat.
// It should be used solely by the key management API.
func (km *kmsEnvelopeAEADKeyManager) NewKeyData(serializedKeyFormat []byte) (*tinkpb.KeyData, error) {
	key, err := km.NewKey(serializedKeyFormat)
	if err != nil {
		return nil, err
	}
	serializedKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	return &tinkpb.KeyData{
		TypeUrl:         kmsEnvelopeAEADTypeURL,
		Value:           serializedKey,
		KeyMaterialType: tinkpb.KeyData_REMOTE,
	}, nil
}

// DoesSupport indicates if this key manager supports the given key type.
func (km *kmsEnvelopeAEADKeyManager) DoesSupport(typeURL string) bool {
	return typeURL == kmsEnvelopeAEADTypeURL
}

// TypeURL returns the key type of keys managed by this key manager.
func (km *kmsEnvelopeAEADKeyManager) TypeURL() string {
	return kmsEnvelopeAEADTypeURL
}

// validateKey validates the given KmsEnvelopeAeadKey.
func (km *kmsEnvelopeAEADKeyManager) validateKey(key *kmsepb.KmsEnvelopeAeadKey) error {
	err := keyset.ValidateKeyVersion(key.Version, kmsEnvelopeAEADKeyVersion)
	if err != nil {
		return fmt.Errorf("kms_envelope_aead_key_manager: %s", err)
	}
	return nil
}

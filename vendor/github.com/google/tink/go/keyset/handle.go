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

package keyset

import (
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"

	"github.com/google/tink/go/core/primitiveset"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/tink"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

var errInvalidKeyset = fmt.Errorf("keyset.Handle: invalid keyset")

// Handle provides access to a Keyset protobuf, to limit the exposure of actual protocol
// buffers that hold sensitive key material.
type Handle struct {
	ks *tinkpb.Keyset
}

// NewHandle creates a keyset handle that contains a single fresh key generated according
// to the given KeyTemplate.
func NewHandle(kt *tinkpb.KeyTemplate) (*Handle, error) {
	ksm := NewManager()
	err := ksm.Rotate(kt)
	if err != nil {
		return nil, fmt.Errorf("keyset.Handle: cannot generate new keyset: %s", err)
	}
	handle, err := ksm.Handle()
	if err != nil {
		return nil, fmt.Errorf("keyset.Handle: cannot get keyset handle: %s", err)
	}
	return handle, nil
}

// NewHandleWithNoSecrets creates a new instance of KeysetHandle using the given keyset which does
// not contain any secret key material.
func NewHandleWithNoSecrets(ks *tinkpb.Keyset) (*Handle, error) {
	if ks == nil {
		return nil, errors.New("keyset.Handle: nil keyset")
	}
	h := &Handle{ks}
	if h.hasSecrets() {
		// If you need to do this, you have to use func insecurecleartextkeyset.Read() instead.
		return nil, errors.New("importing unencrypted secret key material is forbidden")
	}
	return h, nil
}

// Read tries to create a Handle from an encrypted keyset obtained via reader.
func Read(reader Reader, masterKey tink.AEAD) (*Handle, error) {
	encryptedKeyset, err := reader.ReadEncrypted()
	if err != nil {
		return nil, err
	}
	ks, err := decrypt(encryptedKeyset, masterKey)
	if err != nil {
		return nil, err
	}
	return &Handle{ks}, nil
}

// ReadWithNoSecrets tries to create a keyset.Handle from a keyset obtained via reader.
func ReadWithNoSecrets(reader Reader) (*Handle, error) {
	ks, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return NewHandleWithNoSecrets(ks)
}

// Public returns a Handle of the public keys if the managed keyset contains private keys.
func (h *Handle) Public() (*Handle, error) {
	privKeys := h.ks.Key
	pubKeys := make([]*tinkpb.Keyset_Key, len(privKeys))

	for i := 0; i < len(privKeys); i++ {
		if privKeys[i] == nil || privKeys[i].KeyData == nil {
			return nil, errInvalidKeyset
		}
		privKeyData := privKeys[i].KeyData
		pubKeyData, err := publicKeyData(privKeyData)
		if err != nil {
			return nil, fmt.Errorf("keyset.Handle: %s", err)
		}
		pubKeys[i] = &tinkpb.Keyset_Key{
			KeyData:          pubKeyData,
			Status:           privKeys[i].Status,
			KeyId:            privKeys[i].KeyId,
			OutputPrefixType: privKeys[i].OutputPrefixType,
		}
	}
	ks := &tinkpb.Keyset{
		PrimaryKeyId: h.ks.PrimaryKeyId,
		Key:          pubKeys,
	}
	return &Handle{ks}, nil
}

// String returns a string representation of the managed keyset.
// The result does not contain any sensitive key material.
func (h *Handle) String() string {
	return proto.CompactTextString(getKeysetInfo(h.ks))
}

// KeysetInfo returns KeysetInfo representation of the managed keyset.
// The result does not contain any sensitive key material.
func (h *Handle) KeysetInfo() *tinkpb.KeysetInfo {
	return getKeysetInfo(h.ks)
}

// Write encrypts and writes the enclosing keyset.
func (h *Handle) Write(writer Writer, masterKey tink.AEAD) error {
	encrypted, err := encrypt(h.ks, masterKey)
	if err != nil {
		return err
	}
	return writer.WriteEncrypted(encrypted)
}

// WriteWithNoSecrets exports the keyset in h to the given Writer w returning an error if the keyset
// contains secret key material.
func (h *Handle) WriteWithNoSecrets(w Writer) error {
	if h.hasSecrets() {
		return errors.New("exporting unencrypted secret key material is forbidden")
	}

	return w.Write(h.ks)
}

// Primitives creates a set of primitives corresponding to the keys with
// status=ENABLED in the keyset of the given keyset handle, assuming all the
// corresponding key managers are present (keys with status!=ENABLED are skipped).
//
// The returned set is usually later "wrapped" into a class that implements
// the corresponding Primitive-interface.
func (h *Handle) Primitives() (*primitiveset.PrimitiveSet, error) {
	return h.PrimitivesWithKeyManager(nil)
}

// PrimitivesWithKeyManager creates a set of primitives corresponding to
// the keys with status=ENABLED in the keyset of the given keysetHandle, using
// the given key manager (instead of registered key managers) for keys supported
// by it.  Keys not supported by the key manager are handled by matching registered
// key managers (if present), and keys with status!=ENABLED are skipped.
//
// This enables custom treatment of keys, for example providing extra context
// (e.g. credentials for accessing keys managed by a KMS), or gathering custom
// monitoring/profiling information.
//
// The returned set is usually later "wrapped" into a class that implements
// the corresponding Primitive-interface.
func (h *Handle) PrimitivesWithKeyManager(km registry.KeyManager) (*primitiveset.PrimitiveSet, error) {
	if err := Validate(h.ks); err != nil {
		return nil, fmt.Errorf("registry.PrimitivesWithKeyManager: invalid keyset: %s", err)
	}
	primitiveSet := primitiveset.New()
	for _, key := range h.ks.Key {
		if key.Status != tinkpb.KeyStatusType_ENABLED {
			continue
		}
		var primitive interface{}
		var err error
		if km != nil && km.DoesSupport(key.KeyData.TypeUrl) {
			primitive, err = km.Primitive(key.KeyData.Value)
		} else {
			primitive, err = registry.PrimitiveFromKeyData(key.KeyData)
		}
		if err != nil {
			return nil, fmt.Errorf("registry.PrimitivesWithKeyManager: cannot get primitive from key: %s", err)
		}
		entry, err := primitiveSet.Add(primitive, key)
		if err != nil {
			return nil, fmt.Errorf("registry.PrimitivesWithKeyManager: cannot add primitive: %s", err)
		}
		if key.KeyId == h.ks.PrimaryKeyId {
			primitiveSet.Primary = entry
		}
	}
	return primitiveSet, nil
}

// hasSecrets checks if the keyset handle contains any key material considered secret.
// Both symmetric keys and the private key of an assymmetric crypto system are considered secret keys.
// Also returns true when encountering any errors.
func (h *Handle) hasSecrets() bool {
	for _, k := range h.ks.Key {
		if k == nil || k.KeyData == nil {
			continue
		}
		if k.KeyData.KeyMaterialType == tinkpb.KeyData_UNKNOWN_KEYMATERIAL {
			return true
		}
		if k.KeyData.KeyMaterialType == tinkpb.KeyData_ASYMMETRIC_PRIVATE {
			return true
		}
		if k.KeyData.KeyMaterialType == tinkpb.KeyData_SYMMETRIC {
			return true
		}
	}
	return false
}

func publicKeyData(privKeyData *tinkpb.KeyData) (*tinkpb.KeyData, error) {
	if privKeyData.KeyMaterialType != tinkpb.KeyData_ASYMMETRIC_PRIVATE {
		return nil, fmt.Errorf("keyset.Handle: keyset contains a non-private key")
	}
	km, err := registry.GetKeyManager(privKeyData.TypeUrl)
	if err != nil {
		return nil, err
	}
	pkm, ok := km.(registry.PrivateKeyManager)
	if !ok {
		return nil, fmt.Errorf("keyset.Handle: %s does not belong to a PrivateKeyManager", privKeyData.TypeUrl)
	}
	return pkm.PublicKeyData(privKeyData.Value)
}

func decrypt(encryptedKeyset *tinkpb.EncryptedKeyset, masterKey tink.AEAD) (*tinkpb.Keyset, error) {
	if encryptedKeyset == nil || masterKey == nil {
		return nil, fmt.Errorf("keyset.Handle: invalid encrypted keyset")
	}
	decrypted, err := masterKey.Decrypt(encryptedKeyset.EncryptedKeyset, []byte{})
	if err != nil {
		return nil, fmt.Errorf("keyset.Handle: decryption failed: %s", err)
	}
	keyset := new(tinkpb.Keyset)
	if err := proto.Unmarshal(decrypted, keyset); err != nil {
		return nil, errInvalidKeyset
	}
	return keyset, nil
}

func encrypt(keyset *tinkpb.Keyset, masterKey tink.AEAD) (*tinkpb.EncryptedKeyset, error) {
	serializedKeyset, err := proto.Marshal(keyset)
	if err != nil {
		return nil, errInvalidKeyset
	}
	encrypted, err := masterKey.Encrypt(serializedKeyset, []byte{})
	if err != nil {
		return nil, fmt.Errorf("keyset.Handle: encrypted failed: %s", err)
	}
	// get keyset info
	encryptedKeyset := &tinkpb.EncryptedKeyset{
		EncryptedKeyset: encrypted,
		KeysetInfo:      getKeysetInfo(keyset),
	}
	return encryptedKeyset, nil
}

// getKeysetInfo returns a KeysetInfo from a Keyset protobuf.
func getKeysetInfo(keyset *tinkpb.Keyset) *tinkpb.KeysetInfo {
	if keyset == nil {
		panic("keyset.Handle: keyset must be non nil")
	}
	nKey := len(keyset.Key)
	keyInfos := make([]*tinkpb.KeysetInfo_KeyInfo, nKey)
	for i, key := range keyset.Key {
		keyInfos[i] = getKeyInfo(key)
	}
	return &tinkpb.KeysetInfo{
		PrimaryKeyId: keyset.PrimaryKeyId,
		KeyInfo:      keyInfos,
	}
}

// getKeyInfo returns a KeyInfo from a Key protobuf.
func getKeyInfo(key *tinkpb.Keyset_Key) *tinkpb.KeysetInfo_KeyInfo {
	return &tinkpb.KeysetInfo_KeyInfo{
		TypeUrl:          key.KeyData.TypeUrl,
		Status:           key.Status,
		KeyId:            key.KeyId,
		OutputPrefixType: key.OutputPrefixType,
	}
}

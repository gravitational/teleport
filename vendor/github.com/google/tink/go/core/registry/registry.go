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

// Package registry provides a container that for each supported key type holds
// a corresponding KeyManager object, which can generate new keys or
// instantiate the primitive corresponding to given key.
//
// Registry is initialized at startup, and is later used to instantiate
// primitives for given keys or keysets. Keeping KeyManagers for all primitives
// in a single Registry (rather than having a separate KeyManager per
// primitive) enables modular construction of compound primitives from "simple"
// ones, e.g., AES-CTR-HMAC AEAD encryption uses IND-CPA encryption and a MAC.
//
// Note that regular users will usually not work directly with Registry, but
// rather via primitive factories, which in the background query the Registry
// for specific KeyManagers. Registry is public though, to enable
// configurations with custom primitives and KeyManagers.
package registry

import (
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

var (
	keyManagersMu sync.RWMutex
	keyManagers   = make(map[string]KeyManager) // typeURL -> KeyManager
	kmsClientsMu  sync.RWMutex
	kmsClients    = []KMSClient{}
)

// RegisterKeyManager registers the given key manager.
// Does not allow to overwrite existing key managers.
func RegisterKeyManager(km KeyManager) error {
	keyManagersMu.Lock()
	defer keyManagersMu.Unlock()
	typeURL := km.TypeURL()
	if _, existed := keyManagers[typeURL]; existed {
		return fmt.Errorf("registry.RegisterKeyManager: type %s already registered", typeURL)
	}
	keyManagers[typeURL] = km
	return nil
}

// GetKeyManager returns the key manager for the given typeURL if existed.
func GetKeyManager(typeURL string) (KeyManager, error) {
	keyManagersMu.RLock()
	defer keyManagersMu.RUnlock()
	km, existed := keyManagers[typeURL]
	if !existed {
		return nil, fmt.Errorf("registry.GetKeyManager: unsupported key type: %s", typeURL)
	}
	return km, nil
}

// NewKeyData generates a new KeyData for the given key template.
func NewKeyData(kt *tinkpb.KeyTemplate) (*tinkpb.KeyData, error) {
	if kt == nil {
		return nil, fmt.Errorf("registry.NewKeyData: invalid key template")
	}
	km, err := GetKeyManager(kt.TypeUrl)
	if err != nil {
		return nil, err
	}
	return km.NewKeyData(kt.Value)
}

// NewKey generates a new key for the given key template.
func NewKey(kt *tinkpb.KeyTemplate) (proto.Message, error) {
	if kt == nil {
		return nil, fmt.Errorf("registry.NewKey: invalid key template")
	}
	km, err := GetKeyManager(kt.TypeUrl)
	if err != nil {
		return nil, err
	}
	return km.NewKey(kt.Value)
}

// PrimitiveFromKeyData creates a new primitive for the key given in the given KeyData.
func PrimitiveFromKeyData(kd *tinkpb.KeyData) (interface{}, error) {
	if kd == nil {
		return nil, fmt.Errorf("registry.PrimitiveFromKeyData: invalid key data")
	}
	return Primitive(kd.TypeUrl, kd.Value)
}

// Primitive creates a new primitive for the given serialized key using the KeyManager
// identified by the given typeURL.
func Primitive(typeURL string, sk []byte) (interface{}, error) {
	if len(sk) == 0 {
		return nil, fmt.Errorf("registry.Primitive: invalid serialized key")
	}
	km, err := GetKeyManager(typeURL)
	if err != nil {
		return nil, err
	}
	return km.Primitive(sk)
}

// RegisterKMSClient is used to register a new KMS client
func RegisterKMSClient(k KMSClient) {
	kmsClientsMu.Lock()
	defer kmsClientsMu.Unlock()
	kmsClients = append(kmsClients, k)
}

// GetKMSClient fetches a KMSClient by a given URI.
func GetKMSClient(keyURI string) (KMSClient, error) {
	kmsClientsMu.RLock()
	defer kmsClientsMu.RUnlock()
	for _, k := range kmsClients {
		if k.Supported(keyURI) {
			return k, nil
		}
	}
	return nil, fmt.Errorf("KMS client supporting %s not found", keyURI)
}

// ClearKMSClients removes all registered KMS clients.
func ClearKMSClients() {
	kmsClientsMu.Lock()
	defer kmsClientsMu.Unlock()
	kmsClients = []KMSClient{}
}

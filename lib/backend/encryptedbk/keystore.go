/*
Copyright 2015 Gravitational, Inc.

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
package encryptedbk

import (
	"encoding/json"
	"io/ioutil"
	"sync"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
)

type KeyStore interface {
	AddKey(key encryptor.Key) error
	HasKey(keyID string) bool
	GetKey(keyID string) (encryptor.Key, error)
	GetKeys() ([]encryptor.Key, error)
	DeleteKey(keyID string) error
	Close()
}

type BoltKeyStore struct {
	*sync.Mutex
	bolt *boltbk.BoltBackend
}

func NewKeyStore(filename string) (*BoltKeyStore, error) {
	bks := BoltKeyStore{}
	bks.Mutex = &sync.Mutex{}
	var err error
	bks.bolt, err = boltbk.New(filename)
	return &bks, err
}

func (b *BoltKeyStore) AddKey(key encryptor.Key) error {
	b.Lock()
	defer b.Unlock()

	out, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.bolt.UpsertVal([]string{"keys"}, key.ID, out, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func (b *BoltKeyStore) GetKey(id string) (encryptor.Key, error) {
	b.Lock()
	defer b.Unlock()

	return b.getKey(id)
}

func (b *BoltKeyStore) getKey(id string) (encryptor.Key, error) {
	val, err := b.bolt.GetVal([]string{"keys"}, id)
	if err != nil {
		return encryptor.Key{}, err
	}

	var key encryptor.Key
	err = json.Unmarshal(val, &key)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	return key, nil
}

func (b *BoltKeyStore) HasKey(id string) bool {
	b.Lock()
	defer b.Unlock()

	return b.hasKey(id)
}

func (b *BoltKeyStore) hasKey(id string) bool {
	_, err := b.getKey(id)
	return err == nil
}

func (b *BoltKeyStore) DeleteKey(id string) error {
	b.Lock()
	defer b.Unlock()

	if !b.hasKey(id) {
		return &teleport.NotFoundError{}
	}

	err := b.bolt.DeleteKey([]string{"keys"}, id)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *BoltKeyStore) GetKeys() ([]encryptor.Key, error) {
	b.Lock()
	defer b.Unlock()

	ids, err := b.bolt.GetKeys([]string{"keys"})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keys := make([]encryptor.Key, len(ids))
	for i, id := range ids {
		keys[i], err = b.getKey(id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return keys, nil
}

func SaveKeyToFile(key encryptor.Key, filename string) error {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(filename, keyJSON, 0700)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func LoadKeyFromFile(filename string) (encryptor.Key, error) {
	keyJSON, err := ioutil.ReadFile(filename)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	var key encryptor.Key
	err = json.Unmarshal([]byte(keyJSON), &key)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	return key, nil
}

func (b *BoltKeyStore) Close() {
	b.bolt.Close()
}

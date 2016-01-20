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
// package encryptedbk implements encryption layer for any backend.
package encryptedbk

import (
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
)

type EncryptedBackend struct {
	bk        backend.Backend
	encryptor *encryptor.GPGEncryptor
	prefix    []string
	KeyID     string
}

func newEncryptedBackend(backend backend.Backend, key encryptor.Key,
	signKey encryptor.Key, signCheckingKeys []encryptor.Key) (*EncryptedBackend, error) {
	var err error

	ebk := EncryptedBackend{}
	ebk.bk = backend
	ebk.encryptor, err = encryptor.NewGPGEncryptor(key)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	ebk.prefix = []string{rootDir, key.ID}
	ebk.KeyID = key.ID

	if err := ebk.encryptor.SetSignKey(signKey); err != nil {
		return nil, trace.Wrap(err)
	}

	for _, key := range signCheckingKeys {
		if err := ebk.encryptor.AddSignCheckingKey(key); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &ebk, nil
}

// Add special value.
// Encrypted with public key and signed with private key.
func (b *EncryptedBackend) Sign() error {
	return b.UpsertVal([]string{}, "sign", []byte(b.KeyID), 0)
}

// Try to decrypt the special value and verify its sign.
func (b *EncryptedBackend) VerifySign() error {
	val, err := b.GetVal([]string{}, "sign")
	if err != nil {
		return err
	}
	if string(val) != b.KeyID {
		return trace.Errorf("Can't verify sign")
	}
	return nil
}

func (b *EncryptedBackend) DeleteAll() error {
	return b.bk.DeleteBucket(b.prefix[:len(b.prefix)-1], b.prefix[len(b.prefix)-1])
}

func (b *EncryptedBackend) GetKeys(path []string) ([]string, error) {
	return b.bk.GetKeys(append(b.prefix, path...))
}

func (b *EncryptedBackend) DeleteKey(path []string, key string) error {
	return b.bk.DeleteKey(append(b.prefix, path...), key)
}

func (b *EncryptedBackend) DeleteBucket(path []string, bkt string) error {
	return b.bk.DeleteBucket(append(b.prefix, path...), bkt)
}

func (b *EncryptedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	encVal, err := b.encryptor.Encrypt(val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.bk.UpsertVal(append(b.prefix, path...), key, encVal, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *EncryptedBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	encStored, err := b.bk.GetVal(append(b.prefix, path...), key)
	if err != nil && !teleport.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	storedVal := []byte{}
	if len(encStored) > 0 {
		storedVal, err = b.encryptor.Decrypt(encStored)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if string(prevVal) != string(storedVal) {
		return storedVal, &teleport.CompareFailedError{
			Message: "Expected '" + string(prevVal) + "', obtained '" + string(storedVal) + "'",
		}
	}

	encVal, err := b.encryptor.Encrypt(val)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	encStored, e := b.bk.CompareAndSwap(append(b.prefix, path...), key, encVal, ttl, encStored)
	if e != nil && !teleport.IsCompareFailed(e) {
		return nil, e
	}

	storedVal = []byte{}
	if len(encStored) != 0 {
		storedVal, err = b.encryptor.Decrypt(encStored)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if e != nil {
		log.Errorf(string(storedVal) + " " + string(prevVal))
	}

	return storedVal, e
}

func (b *EncryptedBackend) GetVal(path []string, key string) ([]byte, error) {
	encVal, err := b.bk.GetVal(append(b.prefix, path...), key)
	if err != nil {
		if !teleport.IsNotFound(err) {
			err = trace.Wrap(err)
		}
		return nil, err
	}

	val, err := b.encryptor.Decrypt(encVal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return val, nil
}

func (b *EncryptedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	encVal, ttl, err := b.bk.GetValAndTTL(append(b.prefix, path...), key)
	if err != nil {
		if !teleport.IsNotFound(err) {
			err = trace.Wrap(err)
		}
		return nil, 0, err
	}

	val, err := b.encryptor.Decrypt(encVal)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	return val, ttl, nil
}

func (b *EncryptedBackend) AcquireLock(token string, ttl time.Duration) error {
	return b.bk.AcquireLock(token, ttl)
}

func (b *EncryptedBackend) ReleaseLock(token string) error {
	return b.bk.ReleaseLock(token)
}

const (
	rootDir = "data"
)

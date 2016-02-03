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
package encryptor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/mailgun/lemma/secret"
)

type NaClEncryptor struct {
	secret secret.SecretService
}

func NewNaClEncryptor(key Key) (*NaClEncryptor, error) {
	e := NaClEncryptor{}
	conf := secret.Config{}
	conf.KeyBytes = &[secret.SecretKeyLength]byte{}
	copy(conf.KeyBytes[:], key.Value[:])

	var err error
	e.secret, err = secret.New(&conf)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &e, nil
}

func (e NaClEncryptor) Encrypt(data []byte) ([]byte, error) {
	sealedData, err := e.secret.Seal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sealedBytes := secret.SealedBytes{
		Ciphertext: sealedData.CiphertextBytes(),
		Nonce:      sealedData.NonceBytes(),
	}

	sealedBytesJSON, err := json.Marshal(sealedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sealedBytesJSON, nil
}

func (e NaClEncryptor) Decrypt(data []byte) ([]byte, error) {
	sealedBytesJSON := data

	var sealedBytes secret.SealedBytes
	err := json.Unmarshal(sealedBytesJSON, &sealedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	val, err := e.secret.Open(&sealedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return val, nil
}

func GenerateNaClKey(name string) (Key, error) {
	keyValue, err := secret.NewKey()
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	keySha1 := sha256.Sum256(keyValue[:])
	keyHash := hex.EncodeToString(keySha1[:])

	key := Key{
		ID:    keyHash,
		Value: keyValue[:],
		Name:  name,
	}
	return key, nil
}

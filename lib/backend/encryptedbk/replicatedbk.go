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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
)

type ReplicatedBackend struct {
	baseBk           backend.Backend
	ebk              []*EncryptedBackend
	mutex            *sync.Mutex
	keyStore         KeyStore
	signKey          encryptor.Key
	signCheckingKeys []encryptor.Key
	keyGenerator     encryptor.KeyGenerator
}

func NewReplicatedBackend(backend backend.Backend, keysFile string,
	additionalKeys []encryptor.Key,
	keyGenerator encryptor.KeyGenerator) (*ReplicatedBackend, error) {
	var err error
	backend.AcquireLock(bkLock, time.Minute)
	defer backend.ReleaseLock(bkLock)
	repBk := ReplicatedBackend{}
	repBk.mutex = &sync.Mutex{}
	repBk.mutex.Lock()
	defer repBk.mutex.Unlock()
	repBk.baseBk = backend
	repBk.keyGenerator = keyGenerator
	repBk.keyStore, err = NewKeyStore(keysFile)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	for _, key := range additionalKeys {
		if !repBk.keyStore.HasKey(key.ID) {
			err := repBk.keyStore.AddKey(key)
			if err != nil {
				repBk.keyStore.Close()
				return nil, trace.Wrap(err)
			}
		}
	}

	localKeys, err := repBk.keyStore.GetKeys()
	if err != nil {
		repBk.keyStore.Close()
		return nil, trace.Wrap(err)
	}
	log.Infof("Available %v local seal keys:", len(localKeys))
	for _, key := range localKeys {
		log.Infof(key.Name)
		if err := repBk.addSignCheckingKey(key.Public()); err != nil {
			repBk.keyStore.Close()
			return nil, err
		}
		if len(key.PrivateValue) != 0 {
			if err := repBk.setSignKey(key, false); err != nil {
				repBk.keyStore.Close()
				return nil, err
			}
		}
	}

	remoteKeys, err := backend.GetKeys([]string{rootDir})
	if err != nil {
		repBk.keyStore.Close()
		return nil, trace.Wrap(err)
	}
	if len(remoteKeys) != 0 {
		err = repBk.initFromExistingBk(additionalKeys)
	} else {
		err = repBk.initFromEmptyBk()
	}
	if err != nil {
		log.Errorf(err.Error())
		repBk.keyStore.Close()
		return nil, err
	}

	go repBk.refreshKeys()

	log.Infof("Backend was initialized")
	return &repBk, nil
}

func (b *ReplicatedBackend) initFromExistingBk(additionalKeys []encryptor.Key) error {
	log.Infof("Starting with an existing backend. Comparing local and remote keys.")

	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	if len(localKeys) == 0 {
		return trace.Errorf("can't initialize backend: no backend seal keys were provided")
	}

	// first initialize only backends, that can decrypt data
	for _, key := range localKeys {
		bk, err := newEncryptedBackend(b.baseBk, key, b.signKey, b.signCheckingKeys)
		if err != nil {
			return trace.Wrap(err)
		}

		if bk.VerifySign() == nil {
			b.ebk = append(b.ebk, bk)
		}
	}

	if len(b.ebk) == 0 {
		return trace.Errorf("can't initialize backend: no valid backend seal keys were provided")
	}

	if err := b.updateLocalKeysFromCluster(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *ReplicatedBackend) initFromEmptyBk() error {
	log.Infof("Starting with empty backend")

	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	if len(localKeys) == 0 {
		log.Infof("No local backend encrypting keys were found, generating new key 'default key'")
		_, err := b.generateSealKey("default key", false)
		return err
	} else {

		for _, key := range localKeys {
			bk, err := newEncryptedBackend(b.baseBk, key,
				b.signKey, b.signCheckingKeys)
			if err != nil {
				return trace.Wrap(err)
			}
			err = bk.Sign()
			if err != nil {
				return trace.Wrap(err)
			}
			b.ebk = append(b.ebk, bk)
			if len(key.PrivateValue) != 0 {
				if err := bk.VerifySign(); err != nil {
					return trace.Wrap(err)
				}
			}
			if err := b.addSignCheckingKey(key.Public()); err != nil {
				return trace.Wrap(err)
			}
		}

		for _, key := range localKeys {
			if err := b.upsertKeyToPublicKeysList(key); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}
}

func (b *ReplicatedBackend) refreshKeys() {
	for {
		time.Sleep(time.Minute)
		if err := b.updateLocalKeysFromCluster(); err != nil {
			log.Errorf(err.Error())
		}
	}
}

func (b *ReplicatedBackend) GetKeys(path []string) ([]string, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getKeys(path)
}

func (b *ReplicatedBackend) getKeys(path []string) ([]string, error) {
	var e error
	e = trace.Errorf("")
	for _, bk := range b.ebk {
		if bk.VerifySign() == nil {
			var keys []string
			keys, err := bk.GetKeys(path)
			e = err
			if err == nil {
				return keys, nil
			}
		}
	}
	return nil, trace.Errorf("backend can't be accessed because there are no valid decrypting keys. Last error message: %s", e.Error())
}

func (b *ReplicatedBackend) DeleteKey(path []string, key string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.deleteKey(path, key)
}

func (b *ReplicatedBackend) deleteKey(path []string, key string) error {
	var resultErr error
	resultErr = nil

	for _, bk := range b.ebk {
		err := bk.DeleteKey(path, key)
		if err != nil {
			resultErr = err
		}
	}
	return resultErr
}

func (b *ReplicatedBackend) DeleteBucket(path []string, bkt string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var resultErr error
	resultErr = nil

	for _, bk := range b.ebk {
		err := bk.DeleteBucket(path, bkt)
		if err != nil {
			resultErr = err
		}
	}
	return resultErr
}

func (b *ReplicatedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.upsertVal(path, key, val, ttl)
}

func (b *ReplicatedBackend) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.createVal(path, key, val, ttl)
}

func (b *ReplicatedBackend) TouchVal(path []string, key string, ttl time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for _, bk := range b.ebk {
		err = bk.TouchVal(path, key, ttl)
	}

	return trace.Wrap(err)
}

func (b *ReplicatedBackend) upsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	var err error
	for _, bk := range b.ebk {
		err = bk.UpsertVal(path, key, val, ttl)
	}
	return trace.Wrap(err)
}

func (b *ReplicatedBackend) createVal(path []string, key string, val []byte, ttl time.Duration) error {
	var err error
	for _, bk := range b.ebk {
		err = bk.CreateVal(path, key, val, ttl)
	}
	return trace.Wrap(err)
}

func (b *ReplicatedBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)

	storedVal, err := b.getVal(path, key)
	if err != nil {
		if teleport.IsNotFound(err) {
			storedVal = nil
			err = nil
		} else {
			return nil, err
		}
	}

	bothAreEmpty := len(storedVal) == 0 && len(prevVal) == 0

	if bothAreEmpty || reflect.DeepEqual(storedVal, prevVal) {
		return storedVal, b.upsertVal(path, key, val, ttl)
	}

	return storedVal, &teleport.CompareFailedError{}

}

func (b *ReplicatedBackend) GetVal(path []string, key string) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getVal(path, key)
}

func (b *ReplicatedBackend) getVal(path []string, key string) ([]byte, error) {
	err := trace.Errorf("can't decrypt data or check signature: no valid keys")
	for _, bk := range b.ebk {
		if bk.VerifySign() == nil {
			var val []byte
			val, err = bk.GetVal(path, key)
			if err == nil {
				return val, nil
			}
		}
	}
	return nil, err
}

func (b *ReplicatedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getValAndTTL(path, key)
}

func (b *ReplicatedBackend) getValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	err := trace.Errorf("can't decrypt data or check signature: no valid keys")
	for _, bk := range b.ebk {
		if bk.VerifySign() == nil {
			var val []byte
			var ttl time.Duration
			val, ttl, err = bk.GetValAndTTL(path, key)
			if err == nil {
				return val, ttl, nil
			}
		}
	}
	return nil, 0, err
}

func (b *ReplicatedBackend) AcquireLock(token string, ttl time.Duration) error {
	log.Infof("Acquire")
	return b.baseBk.AcquireLock(token, ttl)
}

func (b *ReplicatedBackend) ReleaseLock(token string) error {
	log.Infof("Release")
	return b.baseBk.ReleaseLock(token)
}

func (b *ReplicatedBackend) GenerateSealKey(name string) (encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.generateSealKey(name, true)
}

func (b *ReplicatedBackend) generateSealKey(name string, copyData bool) (encryptor.Key, error) {
	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	for _, key := range localKeys {
		if key.Name == name {
			return encryptor.Key{}, trace.Errorf("key with name '" + name + "' already exists")
		}
	}

	key, err := b.keyGenerator(name)
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	if len(b.signKey.PrivateValue) == 0 {
		if err := b.setSignKey(key, false); err != nil {
			return encryptor.Key{}, trace.Wrap(err)
		}
	}

	if err := b.addSealKey(key, copyData); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	return key, nil
}

func (b *ReplicatedBackend) GetSealKey(id string) (encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.getLocalSealKey(id)
}

func (b *ReplicatedBackend) getLocalSealKey(id string) (encryptor.Key, error) {
	return b.keyStore.GetKey(id)
}

func (b *ReplicatedBackend) GetSealKeys() ([]encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.getLocalSealKeys()
}

func (b *ReplicatedBackend) getLocalSealKeys() ([]encryptor.Key, error) {
	return b.keyStore.GetKeys()
}

func (b *ReplicatedBackend) AddSealKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.addSealKey(key, true)
}

func (b *ReplicatedBackend) addSealKey(key encryptor.Key, copyData bool) error {
	log.Infof("Adding backend seal key '" + key.Name + "'")

	if len(key.Name) == 0 {
		return trace.Errorf("key name is not provided")
	}
	keySha1 := sha256.Sum256(key.PublicValue)
	keyHash := hex.EncodeToString(keySha1[:])

	if !reflect.DeepEqual(key.ID, keyHash) {
		return trace.Errorf("key is corrupted, key id mismatch key value")
	}

	_, err := b.getLocalSealKey(key.ID)
	if err == nil {
		return &teleport.AlreadyExistsError{Message: "Error: Key " + key.ID + " already exists"}
	}
	if !teleport.IsNotFound(err) {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}

	if err := b.keyStore.AddKey(key); err != nil {
		return trace.Wrap(err)
	}

	bk, err := newEncryptedBackend(b.baseBk, key,
		b.signKey, b.signCheckingKeys)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	for _, k := range b.signCheckingKeys {
		err = bk.encryptor.AddSignCheckingKey(k)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = bk.encryptor.SetSignKey(b.signKey)
	if err != nil {
		return trace.Wrap(err)
	}

	if copyData && len(b.ebk) > 0 {
		copied := false
		for _, ebk := range b.ebk {
			if ebk.VerifySign() == nil {
				err = b.copy(b.ebk[0], bk, []string{})
				if err != nil {
					log.Errorf(err.Error())
					bk.DeleteAll()
					b.keyStore.DeleteKey(key.ID)
					return err
				}
				copied = true
			}
		}
		if !copied {
			return trace.Errorf("can't copy: no valid keys to decrypt data of verify signs")
		}
	}

	b.ebk = append(b.ebk, bk)

	if err := b.addSignCheckingKey(key); err != nil {
		return trace.Wrap(err)
	}

	if err := b.upsertKeyToPublicKeysList(key.Public()); err != nil {
		return trace.Wrap(err)
	}

	if err := bk.Sign(); err != nil {
		return trace.Wrap(err)
	}

	if len(key.PrivateValue) > 0 {
		if err := bk.VerifySign(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (b *ReplicatedBackend) DeleteSealKey(id string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, time.Minute)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.deleteSealKey(id, true)
}

func (b *ReplicatedBackend) deleteSealKey(id string, rewriteData bool) error {
	anotherValidKey := false
	var anotherKey encryptor.Key
	for _, bk := range b.ebk {
		if bk.KeyID != id && bk.VerifySign() == nil {
			var err error
			anotherKey, err = b.keyStore.GetKey(bk.KeyID)
			if err == nil {
				anotherValidKey = true
				break
			}
		}
	}

	if !anotherValidKey {
		log.Warningf("Key %s is the last valid key on this server, it can't be deleted", id)
		return trace.Errorf("key %s is the last valid key on this server, it can't be deleted", id)
	}

	if b.signKey.ID == id {
		if err := b.setSignKey(anotherKey, rewriteData); err != nil {
			return trace.Wrap(err)
		}
	}

	deletedLocally := false
	deletedGlobally := false

	err := b.keyStore.DeleteKey(id)
	if err != nil && !teleport.IsNotFound(err) {
		log.Errorf(err.Error())
		return err
	}

	if !teleport.IsNotFound(err) {
		deletedLocally = true
		log.Infof("Key %s was deleted from local keys", id)
	}

	for i, bk := range b.ebk {
		if bk.KeyID == id {
			b.ebk = append(b.ebk[:i], b.ebk[i+1:]...)
			break
		}
	}

	err = b.baseBk.DeleteBucket([]string{rootDir}, id)
	if err == nil {
		deletedGlobally = true
		log.Infof("Key %s was deleted from remote backend keys", id)
	}

	if err := b.deleteClusterPublicKey(id); err == nil {
		deletedGlobally = true
	}

	b.deleteSignCheckingKey(id)

	if !deletedGlobally && !deletedLocally {
		return trace.Errorf("key " + id + " was not found in local and cluster keys")
	}

	return nil
}

func (b *ReplicatedBackend) getClusterPublicSealKeys() ([]encryptor.Key, error) {
	ids, err := b.getKeys([]string{publicKeysPath})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keys := []encryptor.Key{}
	for _, id := range ids {
		keyJSON, err := b.getVal([]string{publicKeysPath}, id)
		if err == nil {
			var key encryptor.Key
			err = json.Unmarshal(keyJSON, &key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			keys = append(keys, key)
		} else {
			log.Errorf(err.Error())
		}
	}

	return keys, nil
}

func (b *ReplicatedBackend) SetSignKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.setSignKey(key, true)
}

func (b *ReplicatedBackend) setSignKey(key encryptor.Key, rewriteData bool) error {
	for _, ebk := range b.ebk {
		err := ebk.encryptor.SetSignKey(key)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	b.signKey = key
	if rewriteData {
		b.rewriteData()
	}
	return nil
}

func (b *ReplicatedBackend) GetSignKey() (encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if len(b.signKey.PrivateValue) == 0 {
		return encryptor.Key{}, trace.Errorf("sign key is not set")
	}

	return b.signKey, nil
}

func (b *ReplicatedBackend) addSignCheckingKey(key encryptor.Key) error {
	for _, ebk := range b.ebk {
		err := ebk.encryptor.AddSignCheckingKey(key)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	b.signCheckingKeys = append(b.signCheckingKeys, key)
	return nil
}

func (b *ReplicatedBackend) deleteSignCheckingKey(id string) error {
	for i := len(b.signCheckingKeys) - 1; i >= 0; i-- {
		if b.signCheckingKeys[i].ID == id {
			b.signCheckingKeys = append(b.signCheckingKeys[:i],
				b.signCheckingKeys[i+1:]...)
		}
	}

	for _, bk := range b.ebk {
		if err := bk.encryptor.DeleteSignCheckingKey(id); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (b *ReplicatedBackend) RewriteData() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.rewriteData()
}

func (b *ReplicatedBackend) rewriteData() error {
	var srcBk *EncryptedBackend = nil
	for _, bk := range b.ebk {
		if bk.VerifySign() == nil {
			srcBk = bk
		}
	}

	if srcBk == nil {
		return trace.Errorf("no valid backend keys to decrypt data")
	}

	for _, bk := range b.ebk {
		if err := b.copy(srcBk, bk, []string{}); err != nil {
			return trace.Wrap(err)
		}
		if err := bk.Sign(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// copy path and all the subpaths from one encrypted backend to another
func (b *ReplicatedBackend) copy(src, dest *EncryptedBackend, path []string) error {
	keys, err := src.GetKeys(path)
	if err == nil && len(keys) != 0 {
		for _, key := range keys {
			err = b.copy(src, dest, append(path, key))
			if err != nil {
				return err
			}
		}
	} else {
		val, ttl, err := src.GetValAndTTL(path[:len(path)-1], path[len(path)-1])
		if err != nil {
			if teleport.IsNotFound(err) {
				return nil
			}
			log.Errorf(err.Error())
			return err
		}
		err = dest.UpsertVal(path[:len(path)-1], path[len(path)-1], val, ttl)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
	}
	return nil
}

func (b *ReplicatedBackend) upsertKeyToPublicKeysList(key encryptor.Key) error {
	keyJSON, err := json.Marshal(key.Public())
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertVal([]string{publicKeysPath}, key.ID, keyJSON, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *ReplicatedBackend) deleteClusterPublicKey(keyID string) error {
	return b.deleteKey([]string{publicKeysPath}, keyID)
}

func (b *ReplicatedBackend) UpdateLocalKeysFromCluster() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.updateLocalKeysFromCluster()
}

func (b *ReplicatedBackend) updateLocalKeysFromCluster() error {
	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		return trace.Wrap(err)
	}

	activeKeys, err := b.getClusterPublicSealKeys()
	if err != nil {
		return trace.Wrap(err)
	}

	// initialize backends from active public keys
	for _, key := range activeKeys {
		alreadyInitialized := false
		for _, bk := range b.ebk {
			if bk.KeyID == key.ID {
				alreadyInitialized = true
				break
			}
		}
		if !alreadyInitialized {
			bk, err := newEncryptedBackend(b.baseBk, key,
				b.signKey, b.signCheckingKeys)
			if err != nil {
				return trace.Wrap(err)
			}
			b.ebk = append(b.ebk, bk)
			if err := b.addSignCheckingKey(key); err != nil {
				return trace.Wrap(err)
			}

		}

		if !b.keyStore.HasKey(key.ID) {
			if err := b.keyStore.AddKey(key); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// delete unused local keys from keystore
	for _, key := range localKeys {
		active := false
		for _, actkey := range activeKeys {
			if actkey.ID == key.ID {
				active = true
				break
			}
		}
		if !active {
			if err := b.deleteSealKey(key.ID, false); err != nil {
				log.Errorf(err.Error())
			}
		}
	}

	return nil

}

const bkLock = "replicated"
const publicKeysPath = "publickeys"

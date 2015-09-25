package encryptedbk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/encryptedbk/encryptor"
)

type ReplicatedBackend interface {
	GetKeys(path []string) ([]string, error)
	UpsertVal(path []string, key string, val []byte, ttl time.Duration) error
	GetVal(path []string, key string) ([]byte, error)
	GetValAndTTL(path []string, key string) ([]byte, time.Duration, error)
	DeleteKey(path []string, key string) error
	DeleteBucket(path []string, bkt string) error
	CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error)

	AcquireLock(token string, ttl time.Duration) error
	ReleaseLock(token string) error

	GetLocalSealKey(id string) (encryptor.Key, error)
	GenerateSealKey(name string) (encryptor.KeyDescription, error)
	GetLocalSealKeys() ([]encryptor.Key, error)
	AddSealKey(key encryptor.Key) error
	DeleteSealKey(id string) error
	GetClusterSealKeys() ([]encryptor.KeyDescription, error)
	UpdateLocalKeysFromCluster() error

	SetSignKey(key encryptor.Key) error
	GetSignKey() encryptor.Key
	RewriteData() error
	//AddSignCheckingKey(key encryptor.Key) error
}

type MasterReplicatedBackend struct {
	baseBk   backend.Backend
	ebk      []*EncryptedBackend
	mutex    *sync.Mutex
	keyStore KeyStore
	//signStore        KeyStore
	signKey          encryptor.Key
	signCheckingKeys []encryptor.Key
}

func NewMasterReplicatedBackend(backend backend.Backend, keysFile string, additionalKeys []encryptor.Key) (*MasterReplicatedBackend, error) {
	var err error
	backend.AcquireLock(bkLock, 0)
	defer backend.ReleaseLock(bkLock)

	repBk := MasterReplicatedBackend{}
	repBk.mutex = &sync.Mutex{}
	repBk.mutex.Lock()
	defer repBk.mutex.Unlock()
	repBk.baseBk = backend
	repBk.keyStore, err = NewKeyStore(keysFile)
	//repBk.signStore, err = NewKeyStore(path.Join(dataDir, "signStore"))
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	remoteKeys, err := backend.GetKeys([]string{rootDir})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	//if len(remoteKeys) == 0 {
	for _, key := range additionalKeys {
		err = repBk.keyStore.AddKey(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	//}

	localKeys, err := repBk.keyStore.GetKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, key := range localKeys {
		if err := repBk.addSignCheckingKey(key.Public()); err != nil {
			return nil, err
		}
		if len(key.PrivateValue) != 0 {
			if err := repBk.setSignKey(key); err != nil {
				return nil, err
			}
		}
	}

	if len(remoteKeys) != 0 {
		/*for _, key := range additionalKeys {
			repBk.addSealKey(key, true)
			if err != nil {
				log.Errorf(err.Error())
			}
		}*/
		err = repBk.initFromExistingBk(additionalKeys)
	} else {
		err = repBk.initFromEmptyBk()
	}
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	//localKeys, err := repBk.GetLocalSealKeys()

	go repBk.regularActions()

	log.Infof(" ", len(repBk.ebk))

	return &repBk, nil
}

func (b *MasterReplicatedBackend) initFromExistingBk(additionalKeys []encryptor.Key) error {
	log.Infof("Starting with an existing backend. Comparing local and remote keys.")

	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	if len(localKeys) == 0 {
		return trace.Errorf("Can't initialize backend: no backend seal keys were provided")
	}

	/*remoteKeys, err := b.baseBk.GetKeys([]string{keysDir})
	if err != nil {
		return trace.Wrap(err)
	}*/

	// first initialize only backends, that can decrypt data
	for _, key := range localKeys {
		bk, err := newEncryptedBackend(b.baseBk, key)
		if err != nil {
			return trace.Wrap(err)
		}

		if bk.VerifySign() == nil {
			b.ebk = append(b.ebk, bk)
		}
	}

	if len(b.ebk) == 0 {
		return trace.Errorf("Can't initialize backend: no valid backend seal keys were provided")
	}

	if err := b.UpdateLocalKeysFromCluster(); err != nil {
		return trace.Wrap(err)
	}

	/*	activeKeys, err := b.getClusterPublicSealKeys()
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
				bk, err := newEncryptedBackend(b.baseBk, key)
				if err != nil {
					return trace.Wrap(err)
				}
				b.ebk = append(b.ebk, bk)
			}
		}

		// delete unused local keys from keystore
		for _, key := range localKeys {
			alreadyInitialized := false
			for _, bk := range b.ebk {
				if bk.KeyID == key.ID {
					alreadyInitialized = true
					break
				}
			}
			if !alreadyInitialized {
				b.keyStore.DeleteKey(key.ID)
			}
		}*/

	/*for _, key := range additionalKeys {
		err := b.addSealKey(key, true)
		if err != nil && !teleport.IsAlredyExists(err) {
			return trace.Wrap(err)
		}
	}*/

	/*publicKeys, err := b.getClusterPublicSealKeys()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, key := range publicKeys {
		alreadyUsed := false
		for _, bk := range b.ebk {
			if bk.KeyID == key.ID {
				alreadyUsed = true
				break
			}
		}
		if !alreadyUsed {

		}
	}*/

	/*for _, remoteKey := range remoteKeys {
		localKeyExists := false
		for _, bk := range b.ebk {
			if remoteKey == bk.KeyID {
				localKeyExists = true
			}
		}

		if !localKeyExists {
			log.Infof("Remote key %s is not provided in the local keys. Backend will work in readonly mode", remoteKey)
			b.readonly = true
		}
	}*/
	return nil
}

func (b *MasterReplicatedBackend) regularActions() {
	for {
		time.Sleep(time.Minute)
		if err := b.updateLocalKeysFromCluster(); err != nil {
			log.Errorf(err.Error())
		}
	}
}

func (b *MasterReplicatedBackend) initFromEmptyBk() error {
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
			bk, err := newEncryptedBackend(b.baseBk, key)
			if err != nil {
				return trace.Wrap(err)
			}
			err = bk.Sign()
			if err != nil {
				return trace.Wrap(err)
			}
			b.ebk = append(b.ebk, bk)
			if len(key.PrivateValue) != 0 {
				if err := b.setSignKey(key); err != nil {
					return trace.Wrap(err)
				}
			}
			if err := b.addSignCheckingKey(key.Public()); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}
}

func (b *MasterReplicatedBackend) GetKeys(path []string) ([]string, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getKeys(path)
}

func (b *MasterReplicatedBackend) getKeys(path []string) ([]string, error) {
	var e error
	e = trace.Errorf("")
	log.Infof("len ", len(b.ebk))
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
	return nil, trace.Errorf("Backen can't be accessed because there are no valid decrypting keys. Last error message: %s", e.Error())
}

func (b *MasterReplicatedBackend) DeleteKey(path []string, key string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

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

func (b *MasterReplicatedBackend) DeleteBucket(path []string, bkt string) error {
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

func (b *MasterReplicatedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.upsertVal(path, key, val, ttl)
}

func (b *MasterReplicatedBackend) upsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	var resultErr error
	resultErr = nil

	for _, bk := range b.ebk {
		err := bk.UpsertVal(path, key, val, ttl)
		if err != nil {
			resultErr = err
		}
	}
	return resultErr
}

func (b *MasterReplicatedBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
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

func (b *MasterReplicatedBackend) GetVal(path []string, key string) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getVal(path, key)
}

func (b *MasterReplicatedBackend) getVal(path []string, key string) ([]byte, error) {
	err := trace.Errorf("Can't decrypt data or check signature: no valid keys")
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

func (b *MasterReplicatedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getValAndTTL(path, key)
}

func (b *MasterReplicatedBackend) getValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	err := trace.Errorf("Can't decrypt data or check signature: no valid keys")
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

func (b *MasterReplicatedBackend) AcquireLock(token string, ttl time.Duration) error {
	return b.baseBk.AcquireLock(token, ttl)
}

func (b *MasterReplicatedBackend) ReleaseLock(token string) error {
	return b.baseBk.ReleaseLock(token)
}

func (b *MasterReplicatedBackend) GenerateSealKey(name string) (encryptor.KeyDescription, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.generateSealKey(name, true)
}

func (b *MasterReplicatedBackend) generateSealKey(name string, copyData bool) (encryptor.KeyDescription, error) {
	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		return encryptor.KeyDescription{}, trace.Wrap(err)
	}
	for _, key := range localKeys {
		if key.Name == name {
			return encryptor.KeyDescription{}, trace.Errorf("Key with name '" + name + "' already exists")
		}
	}

	key, err := encryptor.GenerateGPGKey(name)
	if err != nil {
		return encryptor.KeyDescription{}, trace.Wrap(err)
	}

	keyDescription := encryptor.KeyDescription{
		ID:   key.ID,
		Name: key.Name,
	}

	if len(b.signKey.PrivateValue) == 0 {
		if err := b.setSignKey(key); err != nil {
			return encryptor.KeyDescription{}, trace.Wrap(err)
		}
	}

	if err := b.addSealKey(key, copyData); err != nil {
		return encryptor.KeyDescription{}, trace.Wrap(err)
	}

	if err := b.setSignKey(key); err != nil {
		return encryptor.KeyDescription{}, trace.Wrap(err)
	}

	return keyDescription, nil
}

func (b *MasterReplicatedBackend) GetLocalSealKey(id string) (encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.getLocalSealKey(id)
}

func (b *MasterReplicatedBackend) getLocalSealKey(id string) (encryptor.Key, error) {
	return b.keyStore.GetKey(id)
}

func (b *MasterReplicatedBackend) GetLocalSealKeys() ([]encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.getLocalSealKeys()
}

func (b *MasterReplicatedBackend) getLocalSealKeys() ([]encryptor.Key, error) {
	return b.keyStore.GetKeys()
	/*ids, err := b.keyStore.GetKeyIDs()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keys := make([]encryptor.Key, len(ids))
	for i, id := range ids {
		keys[i], err = b.keyStore.GetKey(id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return keys, nil*/
}

func (b *MasterReplicatedBackend) AddSealKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.addSealKey(key, true)
}

func (b *MasterReplicatedBackend) addSealKey(key encryptor.Key, copyData bool) error {
	log.Infof("Adding backend seal key '" + key.Name + "'")

	if len(key.Name) == 0 {
		return trace.Errorf("Key name is not provided")
	}
	keySha1 := sha256.Sum256(key.PublicValue)
	keyHash := hex.EncodeToString(keySha1[:])

	if !reflect.DeepEqual(key.ID, keyHash) {
		return trace.Errorf("Key is corrupted, key id mismatch key value")
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

	bk, err := newEncryptedBackend(b.baseBk, key)
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

	if copyData && len(b.ebk) > 0 && len(bk.GetSealKeyName()) == 0 {
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
			return trace.Errorf("Can't copy: no valid keys to decrypt data of verify signs")
		}
	}

	if len(bk.GetSealKeyName()) == 0 {
		bk.SetSealKeyName(key.Name)
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

	if err := bk.VerifySign(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *MasterReplicatedBackend) DeleteSealKey(id string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)

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
		return trace.Errorf("Key %s is the last valid key on this server, it can't be deleted", id)
	}

	if b.signKey.ID == id {
		if err := b.setSignKey(anotherKey); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := b.rewriteData(); err != nil {
		return trace.Wrap(err)
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
	b.baseBk.DeleteKey([]string{keysDir}, id)

	if err := b.deleteClusterPublicKey(id); err == nil {
		deletedGlobally = true
	}

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

	if !deletedGlobally && !deletedLocally {
		return trace.Errorf("Key " + id + " was not found in local and cluster keys")
	}

	return nil
}

func (b *MasterReplicatedBackend) GetClusterPublicSealKeys() ([]encryptor.Key, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.getClusterPublicSealKeys()
}

func (b *MasterReplicatedBackend) getClusterPublicSealKeys() ([]encryptor.Key, error) {
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
		}
	}

	return keys, nil
	/*ids, err := b.baseBk.GetKeys([]string{keysDir})

	keys := []encryptor.KeyDescription{}
	for _, id := range ids {
		name, err := b.baseBk.GetVal([]string{keysDir}, id)
		if err == nil && len(name) != 0 {
			key := encryptor.KeyDescription{
				ID:   id,
				Name: string(name),
			}
			keys = append(keys, key)
		}
	}

	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return keys, nil*/
}

func (b *MasterReplicatedBackend) SetSignKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.setSignKey(key)
}

func (b *MasterReplicatedBackend) setSignKey(key encryptor.Key) error {
	for _, ebk := range b.ebk {
		err := ebk.encryptor.SetSignKey(key)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	b.signKey = key
	return nil
}

func (b *MasterReplicatedBackend) GetSignKey() encryptor.Key {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.getSignKey()
}

func (b *MasterReplicatedBackend) getSignKey() encryptor.Key {
	return b.signKey
}

/*func (b *MasterReplicatedBackend) AddSignCheckingKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.addSignCheckingKey(key)
}*/

func (b *MasterReplicatedBackend) addSignCheckingKey(key encryptor.Key) error {
	for _, ebk := range b.ebk {
		err := ebk.encryptor.AddSignCheckingKey(key)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	b.signCheckingKeys = append(b.signCheckingKeys, key)
	return nil
}

func (b *MasterReplicatedBackend) RewriteData() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.rewriteData()
}

func (b *MasterReplicatedBackend) rewriteData() error {
	var srcBk *EncryptedBackend = nil
	for _, bk := range b.ebk {
		if bk.VerifySign() == nil {
			srcBk = bk
		}
	}

	if srcBk == nil {
		return trace.Errorf("No valid backend keys to decrypt data")
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
func (b *MasterReplicatedBackend) copy(src, dest *EncryptedBackend, path []string) error {
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

func (b *MasterReplicatedBackend) upsertKeyToPublicKeysList(key encryptor.Key) error {
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

func (b *MasterReplicatedBackend) deleteClusterPublicKey(keyID string) error {
	for _, bk := range b.ebk {
		ids, _ := bk.GetKeys([]string{publicKeysPath})

		for _, id := range ids {
			if id == keyID {
				keyJSON, err := bk.GetVal([]string{publicKeysPath}, id)
				if err == nil {
					var key encryptor.Key
					err = json.Unmarshal(keyJSON, &key)
					if err == nil && key.ID == keyID {

						bk.DeleteKey([]string{publicKeysPath}, id)
					}
				}
			}
		}
	}
	return nil
}

func (b *MasterReplicatedBackend) UpdateLocalKeysFromCluster() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.updateLocalKeysFromCluster()
}

func (b *MasterReplicatedBackend) updateLocalKeysFromCluster() error {
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
			bk, err := newEncryptedBackend(b.baseBk, key)
			if err != nil {
				return trace.Wrap(err)
			}
			b.ebk = append(b.ebk, bk)
		}

		if !b.keyStore.HasKey(key.ID) {
			if err := b.keyStore.AddKey(key); err != nil {
				return trace.Wrap(err)
			}
			if err := b.addSignCheckingKey(key); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// delete unused local keys from keystore
	for _, key := range localKeys {
		alreadyInitialized := false
		for _, bk := range b.ebk {
			if bk.KeyID == key.ID {
				alreadyInitialized = true
				break
			}
		}
		if !alreadyInitialized {
			b.keyStore.DeleteKey(key.ID)
		}
	}

	return nil

}

/*func (b *MasterReplicatedBackend) getPublicKeysList() ([]encryptor.Key, error) {
	ids, err := b.getKeys([]string{publicKeysPath})

	keys := []encryptor.Key{}

	for _, id := range ids {
		keyJSON, err := b.getVal([]string{publicKeysPath}, id)
		if err == nil {
			var key encryptor.Key
			err = json.Unmarshal(keyJSON, key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			keys := append(keys, key)
		}
	}

	return keys, nil
}*

// Deletes all untrusted public keys from backend.
// A key is trusted only if it was received directly from another node.
func (b *MasterReplicatedBackend) DeleteUntrustedData(keyID string) error {
	for bk := range b.ebk {
		ids, err := bk.GetKeys([]string{publicKeysPath})

		for _, id := range ids {
			keyJSON, err := bk.GetVal([]string{publicKeysPath}, id)
			if err == nil {
				var key encryptor.Key
				err = json.Unmarshal(keyJSON, key)
				if err == nil && key.ID == keyID {
					bk.DeleteKey([]string{publicKeysPath}, id)
				}
			}
		}
	}
}

/*func (b *MasterReplicatedBackend) checkKeysAreProvided() {
	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		for {
			log.Warningf("Please provide valid backend keys. Use tctl (or tscopectl) to add backend keys.")
			time.Sleep(2 * time.Second)
			if len(b.ebk) > 0 {
				log.Infof("Backend started in readonly mode")
				return
			}
		}
	}
}*/

const bkLock = "replicated"
const mySign = "mysign"
const publicKeysPath = "publickeys"

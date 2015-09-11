package encryptedbk

import (
	"sync"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
)

type ReplicatedBackend struct {
	baseBk     backend.Backend
	ebk        []*EncryptedBackend
	mutex      *sync.Mutex
	keyStorage backend.Backend
	readonly   bool
}

func New(backend backend.Backend, keysFile string) (*ReplicatedBackend, error) {
	var err error
	repBk := ReplicatedBackend{}
	repBk.mutex = &sync.Mutex{}
	repBk.baseBk = backend
	repBk.keyStorage, err = boltbk.New(keysFile)

	remoteKeys, _ := backend.GetKeys([]string{rootDir})
	if len(remoteKeys) != 0 {
		err = repBk.initFromExistingBk()
	} else {
		err = repBk.initFromEmptyBk()
	}
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return &repBk, nil
}

func (b *ReplicatedBackend) initFromExistingBk() error {
	log.Infof("Starting with an existing backend. Comparing local and remote keys.")

	localKeys, err := b.GetAllEncryptingKeys()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	if len(localKeys) == 0 {
		log.Warningf("No local backend encrypting keys were found. Backend will not work until you add encrypting keys")
		b.readonly = true
		return nil
	}

	remoteKeys, err := b.baseBk.GetKeys([]string{rootDir})
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	for _, key := range localKeys {
		bk, err := newEncryptedBackend(b.baseBk, key)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}

		if !bk.IsExisting() {
			log.Errorf("Backend encrypting key %s is not valid. It will not be used", key.ID)
		} else {
			b.ebk = append(b.ebk, bk)
		}
	}

	b.readonly = false
	for _, remoteKey := range remoteKeys {
		localKeyExists := false
		for _, bk := range b.ebk {
			if remoteKey == bk.KeyID {
				localKeyExists = true
			}
		}

		if !localKeyExists {
			log.Infof("Remote key %s is not provided in the local keys. Backend will work in readonly mode")
			b.readonly = true
		}
	}
	return nil
}

func (b *ReplicatedBackend) initFromEmptyBk() error {
	log.Infof("Starting with empty backend")

	b.readonly = false

	localKeys, err := b.GetAllEncryptingKeys()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	if len(localKeys) == 0 {
		log.Infof("No local backend encrypting keys were found, generating new key 'key0'")
		return b.NewEncryptingKey("key0", false)
	} else {

		for _, key := range localKeys {
			bk, err := newEncryptedBackend(b.baseBk, key)
			if err != nil {
				log.Errorf(err.Error())
				return err
			}
			err = bk.SetExistence()
			if err != nil {
				log.Errorf(err.Error())
				return err
			}
			b.ebk = append(b.ebk, bk)
		}
		return nil
	}
}

func (b *ReplicatedBackend) GetKeys(path []string) ([]string, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		return nil, trace.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
	}

	var err error

	for _, bk := range b.ebk {
		var keys []string
		keys, err = bk.GetKeys(path)
		if err == nil {
			return keys, nil
		}
		log.Warningf("Key %s is not valid", bk.KeyID)
	}
	return nil, err
}

func (b *ReplicatedBackend) DeleteKey(path []string, key string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		return trace.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
	}

	if b.readonly {
		log.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
		return trace.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
	}

	for _, bk := range b.ebk {
		err := bk.DeleteKey(path, key)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
	}
	return nil
}

func (b *ReplicatedBackend) DeleteBucket(path []string, bkt string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		return trace.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
	}

	if b.readonly {
		log.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
		return trace.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
	}

	for _, bk := range b.ebk {
		err := bk.DeleteBucket(path, bkt)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
	}
	return nil
}

func (b *ReplicatedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		return trace.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
	}

	if b.readonly {
		log.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
		return trace.Errorf("Can't modify backend data without all the remote encrypting keys. Backend work in readonly mode")
	}

	for _, bk := range b.ebk {
		err := bk.UpsertVal(path, key, val, ttl)
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
	}
	return nil
}

func (b *ReplicatedBackend) GetVal(path []string, key string) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.ebk) == 0 {
		log.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
		return nil, trace.Errorf("Backen can't be accessed because there are no valid local encrypting keys")
	}

	var err error
	for _, bk := range b.ebk {
		var val []byte
		val, err = bk.GetVal(path, key)
		if err == nil {
			return val, nil
		}
		log.Warningf("Key %s is not valid", bk.KeyID)
	}
	return nil, err
}

func (b *ReplicatedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for _, bk := range b.ebk {
		var val []byte
		var ttl time.Duration
		val, ttl, err = bk.GetValAndTTL(path, key)
		if err == nil {
			return val, ttl, nil
		}
		log.Warningf("Key %s is not valid", bk.KeyID)
	}
	return nil, 0, err
}

func (b *ReplicatedBackend) AcquireLock(token string, ttl time.Duration) error {
	return b.baseBk.AcquireLock(token, ttl)
}

func (b *ReplicatedBackend) ReleaseLock(token string) error {
	return b.baseBk.ReleaseLock(token)
}

func (b *ReplicatedBackend) NewEncryptingKey(id string, copyData bool) error {
	if b.readonly {
		log.Errorf("Can't generate new backend encrypting key without all the encrypting keys used in remote backend. Backend work in readonly mode")
		return trace.Errorf("Can't generate new backend encrypting key without all the encrypting keys used in remote backend. Backend work in readonly mode")
	}

	keyValue, err := secret.NewKey()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	key := Key{}
	key.ID = id
	key.Value = keyValue[:]
	return b.AddEncryptingKey(key, copyData)
}

func (b *ReplicatedBackend) GetEncryptingKey(id string) (Key, error) {
	value, err := b.keyStorage.GetVal([]string{"values"}, id)
	if err != nil {
		log.Errorf(err.Error())
		return Key{}, err
	}
	var key Key
	key.ID = id
	key.Value = value
	return key, nil
}

func (b *ReplicatedBackend) GetAllEncryptingKeys() ([]Key, error) {
	ids, err := b.keyStorage.GetKeys([]string{"values"})
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	keys := make([]Key, len(ids))
	for i, id := range ids {
		keys[i], err = b.GetEncryptingKey(id)
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
	}
	return keys, nil
}

func (b *ReplicatedBackend) AddEncryptingKey(key Key, copyData bool) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	_, err := b.GetEncryptingKey(key.ID)
	if err == nil {
		log.Errorf("Error: Key %s already exists", key)
		return trace.Errorf("Error: Key %s already exists", key)
	}
	if !teleport.IsNotFound(err) {
		log.Errorf(err.Error())
		return err
	}

	b.keyStorage.UpsertVal([]string{"values"}, key.ID, key.Value, 0)

	bk, err := newEncryptedBackend(b.baseBk, key)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	if copyData {
		err = b.copy(b.ebk[0], bk, []string{})
		if err != nil {
			log.Errorf(err.Error())
			return err
		}
	}
	bk.SetExistence()
	b.ebk = append(b.ebk, bk)
	return nil
}

func (b *ReplicatedBackend) DeleteEncryptingKey(id string) error {
	err := b.keyStorage.DeleteKey([]string{"values"}, id)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	for i, bk := range b.ebk {
		if bk.KeyID == id {
			err := bk.DeleteAll()
			if err != nil {
				log.Errorf(err.Error())
				return err
			}
			b.ebk = append(b.ebk[:i], b.ebk[i+1:]...)
			return nil
		}
	}
	log.Warningf("Deleting encrypting key: key %s doesn't exists in backend", id)
	return nil
}

func (b *ReplicatedBackend) ListRemoteEncryptingKeys() ([]string, error) {
	ids, err := b.baseBk.GetKeys([]string{rootDir})
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return ids, nil
}

// copy path and all the subpaths from one encrypted backend to another
func (b *ReplicatedBackend) copy(src, dest *EncryptedBackend, path []string) error {
	keys, err := src.GetKeys(path)
	if err == nil {
		for _, key := range keys {
			err = b.copy(src, dest, append(path, key))
			if err != nil {
				return err
			}
		}
	} else {
		val, ttl, err := src.GetValAndTTL(path[:len(path)-1], path[len(path)-1])
		if err != nil {
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

type Key struct {
	ID    string
	Value []byte
}

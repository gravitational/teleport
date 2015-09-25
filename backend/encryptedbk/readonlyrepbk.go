package encryptedbk

/*
import (
	"sync"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/encryptedbk/encryptor"
)

type ReadonlyReplicatedBackend struct {
	*MasterReplicatedBackend
}

func NewReadonlyReplicatedBackend(backend backend.Backend, keysFile string, additionalKeys []encryptor.Key) (*ReadonlyReplicatedBackend, error) {
	var err error
	backend.AcquireLock(bkLock, 0)
	defer backend.ReleaseLock(bkLock)

	repBk := ReadonlyReplicatedBackend{}
	repBk.mutex = &sync.Mutex{}
	repBk.baseBk = backend
	repBk.keyStore, err = NewKeyStore(keysFile)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	for _, key := range additionalKeys {
		repBk.keyStore.AddKey(key)
	}

	localKeys, err := repBk.getLocalSealKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(localKeys) == 0 {
		return nil, trace.Errorf("Can't initialize backend: no backend seal keys provided")
	}

	for _, key := range localKeys {
		bk, err := newEncryptedBackend(repBk.baseBk, key)
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}

		if len(bk.GetSealKeyName()) == 0 {
			log.Warningf("Backend encrypting key %s (%s) is not valid. It will not be used", key.ID, key.Name)
		} else {
			repBk.ebk = append(repBk.ebk, bk)
		}
	}

	if len(repBk.ebk) == 0 {
		return nil, trace.Errorf("Can't initialize backend: no valid backend seal keys were provided")
	}

	return &repBk, nil
}

func (b *ReadonlyReplicatedBackend) initFromEmptyBk() error {
	log.Infof("Starting with empty backend")

	localKeys, err := b.getLocalSealKeys()
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	if len(localKeys) == 0 {
		log.Infof("No local backend encrypting keys were found, generating new key 'key0'")
		_, err := b.generateSealKey("key0", false)
		return trace.Wrap(err)
	} else {

		for _, key := range localKeys {
			bk, err := newEncryptedBackend(b.baseBk, key)
			if err != nil {
				return trace.Wrap(err)
			}
			err = bk.SetSealKeyName(key.Name)
			if err != nil {
				return trace.Wrap(err)
			}
			b.ebk = append(b.ebk, bk)
		}
		return nil
	}
}

func (b *ReadonlyReplicatedBackend) DeleteKey(path []string, key string) error {
	return &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) DeleteBucket(path []string, bkt string) error {
	return &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	return &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	return nil, &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) GenerateSealKey(name string) (encryptor.KeyDescription, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.baseBk.AcquireLock(bkLock, 0)
	defer b.baseBk.ReleaseLock(bkLock)
	return b.MasterReplicatedBackend.generateSealKey(name, false)
}

func (b *ReadonlyReplicatedBackend) AddSealKey(key encryptor.Key) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.MasterReplicatedBackend.addSealKey(key, false)
}

func (b *ReadonlyReplicatedBackend) DeleteSealKey(id string) error {
	return &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) SetSignKey(key encryptor.Key) error {
	return &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) GetSignKey() (encryptor.Key, error) {
	return encryptor.Key{}, &teleport.ReadonlyError{}
}

func (b *ReadonlyReplicatedBackend) RewriteData() error {
	return &teleport.ReadonlyError{}
}
*/

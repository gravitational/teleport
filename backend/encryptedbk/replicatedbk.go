package encryptedbk

import (
	"sync"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"

	"github.com/gravitational/teleport/backend"
)

type ReplicatedBackend struct {
	backend backend.Backend
	ebks    []*EncryptedBackend
	mutex   *sync.Mutex
}

func New(backend backend.Backend, keysFilenames []string) (*ReplicatedBackend, error) {
	keys, _ := backend.GetKeys([]string{rootDir})
	if len(keys) != 0 {
		return newFromExistingBk(backend, keysFilenames)
	} else {
		return newFromEmptyBk(backend, keysFilenames)
	}
}

func newFromExistingBk(backend backend.Backend, keysFilenames []string) (*ReplicatedBackend, error) {
	repBk := ReplicatedBackend{}
	repBk.ebks = make([]*EncryptedBackend, len(keysFilenames))
	repBk.mutex = &sync.Mutex{}
	var err error

	for i, key := range keysFilenames {
		repBk.ebks[i], err = newEncryptedBackend(backend, key)
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}

		if !repBk.ebks[i].IsExisting() {
			log.Errorf("Key %s is not valid. It should be deleted", key)
			return nil, trace.Errorf("Key %s is not valit. It should be deleted.", key)
		}
	}
	return &repBk, nil
}

func newFromEmptyBk(backend backend.Backend, keysFilenames []string) (*ReplicatedBackend, error) {
	repBk := ReplicatedBackend{}
	repBk.ebks = make([]*EncryptedBackend, len(keysFilenames))
	repBk.mutex = &sync.Mutex{}
	var err error

	for i, key := range keysFilenames {
		repBk.ebks[i], err = newEncryptedBackend(backend, key)
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
		err = repBk.ebks[i].SetExistence()
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
	}
	return &repBk, nil
}

// copy path and all the subpaths
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

func (b *ReplicatedBackend) AddEncodingKey(keysFilename string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	bk, err := newEncryptedBackend(b.backend, keysFilename)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	err = b.copy(b.ebks[0], bk, []string{})
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	b.ebks = append(b.ebks, bk)
	return nil
}

func (b *ReplicatedBackend) DeleteEncodingKey(keysFilename string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for i, bk := range b.ebks {
		if bk.EncodingKey == keysFilename {
			b.ebks = append(b.ebks[:i], b.ebks[i+1:]...)
			return nil
		}
	}
	log.Errorf("Deleting error: key %s in not in used keys", keysFilename)
	return trace.Errorf("Deleting error: key %s in not in used keys", keysFilename)
}

func (b *ReplicatedBackend) GetKeys(path []string) ([]string, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.ebks[0].GetKeys(path)
}

func (b *ReplicatedBackend) DeleteKey(path []string, key string) error {
	for _, bk := range b.ebks {
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

	for _, bk := range b.ebks {
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

	for _, bk := range b.ebks {
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

	return b.ebks[0].GetVal(path, key)
}

func (b *ReplicatedBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.ebks[0].GetValAndTTL(path, key)
}

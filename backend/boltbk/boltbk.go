// package boltbk implements BoltDB backed backend for standalone instances
package boltbk

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/boltdb/bolt"
)

type BoltBackend struct {
	sync.Mutex

	db    *bolt.DB
	locks map[string]time.Time
}

func New(path string) (*BoltBackend, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &BoltBackend{
		db:    db,
		locks: make(map[string]time.Time),
	}, nil
}

func (b *BoltBackend) Close() error {
	return b.db.Close()
}

func (b *BoltBackend) GetKeys(path []string) ([]string, error) {
	keys, err := b.getKeys(path)
	if err != nil {
		if teleport.IsNotFound(err) {
			return []string{}, nil
		}
		return nil, err
	}
	// now do an iteration to expire keys
	for _, key := range keys {
		b.GetVal(path, key)
	}
	keys, err = b.getKeys(path)
	if err != nil {
		if teleport.IsNotFound(err) {
			return []string{}, nil
		}
		return nil, err
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *BoltBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	v := &kv{
		Created: time.Now(),
		Value:   val,
		TTL:     ttl,
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.upsertKey(path, key, bytes)
}

func (b *BoltBackend) GetVal(path []string, key string) ([]byte, error) {
	var val []byte
	if err := b.getKey(path, key, &val); err != nil {
		return nil, err
	}
	var k *kv
	if err := json.Unmarshal(val, &k); err != nil {
		return nil, err
	}
	if k.TTL != 0 && time.Now().Sub(k.Created) > k.TTL {
		if err := b.deleteKey(path, key); err != nil {
			return nil, err
		}
		return nil, &teleport.NotFoundError{
			Message: fmt.Sprintf("%v: %v not found", path, key)}
	}
	return k.Value, nil
}

func (b *BoltBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	var val []byte
	if err := b.getKey(path, key, &val); err != nil {
		return nil, 0, err
	}
	var k *kv
	if err := json.Unmarshal(val, &k); err != nil {
		return nil, 0, err
	}
	if k.TTL != 0 && time.Now().Sub(k.Created) > k.TTL {
		if err := b.deleteKey(path, key); err != nil {
			return nil, 0, err
		}
		return nil, 0, &teleport.NotFoundError{
			Message: fmt.Sprintf("%v: %v not found", path, key)}
	}
	var newTTL time.Duration
	newTTL = 0
	if k.TTL != 0 {
		newTTL = k.Created.Add(k.TTL).Sub(time.Now())
	}
	return k.Value, newTTL, nil
}

func (b *BoltBackend) DeleteKey(path []string, key string) error {
	return b.deleteKey(path, key)
}

func (b *BoltBackend) DeleteBucket(path []string, bucket string) error {
	return b.deleteBucket(path, bucket)
}

func (b *BoltBackend) deleteBucket(buckets []string, bucket string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return err
		}
		if bkt.Bucket([]byte(bucket)) == nil {
			return &teleport.NotFoundError{
				fmt.Sprintf("%v not found", bucket)}
		}
		return bkt.DeleteBucket([]byte(bucket))
	})
}

func (b *BoltBackend) deleteKey(buckets []string, key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return err
		}
		if bkt.Get([]byte(key)) == nil {
			return &teleport.NotFoundError{}
		}
		return bkt.Delete([]byte(key))
	})
}

func (b *BoltBackend) upsertKey(buckets []string, key string, bytes []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, buckets)
		if err != nil {
			return err
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) upsertJSONKey(buckets []string, key string, val interface{}) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, buckets)
		if err != nil {
			return err
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) getJSONKey(buckets []string, key string, val interface{}) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return &teleport.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			}
		}
		return json.Unmarshal(bytes, val)
	})
}

func (b *BoltBackend) getKey(buckets []string, key string, val *[]byte) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return &teleport.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			}
		}
		*val = make([]byte, len(bytes))
		copy(*val, bytes)
		return nil
	})
}

func (b *BoltBackend) getKeys(buckets []string) ([]string, error) {
	out := []string{}
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return err
		}
		c := bkt.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			out = append(out, string(k))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func UpsertBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt, err := b.CreateBucketIfNotExists([]byte(buckets[0]))
	if err != nil {
		return nil, err
	}
	for _, key := range buckets[1:] {
		bkt, err = bkt.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return nil, err
		}
	}
	return bkt, nil
}

func GetBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt := b.Bucket([]byte(buckets[0]))
	if bkt == nil {
		return nil, &teleport.NotFoundError{
			Message: fmt.Sprintf("bucket %v not found", buckets[0])}
	}
	for _, key := range buckets[1:] {
		bkt = bkt.Bucket([]byte(key))
		if bkt == nil {
			return nil, &teleport.NotFoundError{
				Message: fmt.Sprintf("bucket %v not found", key)}
		}
	}
	return bkt, nil
}

func (b *BoltBackend) AcquireLock(token string, ttl time.Duration) error {
	for {
		b.Lock()
		expires, ok := b.locks[token]
		if ok && (expires.IsZero() || expires.After(time.Now())) {
			b.Unlock()
			time.Sleep(100 * time.Millisecond)
		} else {
			if ttl == 0 {
				b.locks[token] = time.Time{}
			} else {
				b.locks[token] = time.Now().Add(ttl)
			}
			b.Unlock()
			return nil
		}
	}
}

func (b *BoltBackend) ReleaseLock(token string) error {
	b.Lock()
	defer b.Unlock()

	expires, ok := b.locks[token]
	if !ok || (!expires.IsZero() && expires.Before(time.Now())) {
		return &teleport.NotFoundError{
			Message: fmt.Sprintf(
				"lock %v is deleted or expired", token),
		}
	}
	delete(b.locks, token)
	return nil
}

type kv struct {
	Created time.Time     `json:"created"`
	TTL     time.Duration `json:"ttl"`
	Value   []byte        `json:"val"`
}

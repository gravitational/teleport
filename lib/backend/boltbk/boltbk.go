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
// package boltbk implements BoltDB backed backend for standalone instances
package boltbk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/teleport"

	"github.com/boltdb/bolt"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
)

// BoltBackend is a boltdb-based backend used in tests and standalone mode
type BoltBackend struct {
	sync.Mutex

	db    *bolt.DB
	clock timetools.TimeProvider
	locks map[string]time.Time
}

// Option sets functional options for the backend
type Option func(b *BoltBackend) error

// Clock sets clock for the backend, used in tests
func Clock(clock timetools.TimeProvider) Option {
	return func(b *BoltBackend) error {
		b.clock = clock
		return nil
	}
}

// New returns a new isntance of bolt backend
func New(path string, opts ...Option) (*BoltBackend, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert path")
	}
	dir := filepath.Dir(path)
	s, err := os.Stat(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !s.IsDir() {
		return nil, trace.Wrap(
			teleport.BadParameter(
				"path", fmt.Sprintf("path '%v' should be a valid directory", dir)))
	}
	b := &BoltBackend{
		locks: make(map[string]time.Time),
	}
	for _, option := range opts {
		if err := option(b); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if b.clock == nil {
		b.clock = &timetools.RealTime{}
	}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.db = db
	return b, nil
}

// Close closes the backend resources
func (b *BoltBackend) Close() error {
	return b.db.Close()
}

func (b *BoltBackend) GetKeys(path []string) ([]string, error) {
	keys, err := b.getKeys(path)
	if err != nil {
		if teleport.IsNotFound(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	// now do an iteration to expire keys
	for _, key := range keys {
		b.GetVal(path, key)
	}
	keys, err = b.getKeys(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *BoltBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	return b.upsertVal(path, key, val, ttl)
}

func (b *BoltBackend) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	v := &kv{
		Created: b.clock.UtcNow(),
		Value:   val,
		TTL:     ttl,
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.createKey(bucket, key, bytes)
	return trace.Wrap(err)
}

func (b *BoltBackend) TouchVal(bucket []string, key string, ttl time.Duration) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, bucket)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val == nil {
			return trace.Wrap(
				teleport.NotFound(fmt.Sprintf("'%v' already exists", key)))
		}
		var k *kv
		if err := json.Unmarshal(val, &k); err != nil {
			return trace.Wrap(err)
		}
		k.TTL = ttl
		k.Created = b.clock.UtcNow()
		bytes, err := json.Marshal(k)
		if err != nil {
			return trace.Wrap(err)
		}
		return bkt.Put([]byte(key), bytes)
	})
	return trace.Wrap(err)
}

func (b *BoltBackend) upsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	v := &kv{
		Created: b.clock.UtcNow(),
		Value:   val,
		TTL:     ttl,
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return trace.Wrap(err)
	}
	return b.upsertKey(path, key, bytes)
}

func (b *BoltBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	b.Lock()
	defer b.Unlock()

	storedVal, err := b.GetVal(path, key)
	if teleport.IsNotFound(err) {
		storedVal = []byte{}
		err = nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if string(prevVal) == string(storedVal) {
		err = b.upsertVal(path, key, val, ttl)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return storedVal, nil
	}
	return storedVal, trace.Wrap(&teleport.CompareFailedError{
		Message: "expected '" + string(prevVal) + "', obtained '" + string(storedVal) + "'",
	})
}

func (b *BoltBackend) GetVal(path []string, key string) ([]byte, error) {
	var val []byte
	if err := b.getKey(path, key, &val); err != nil {
		return nil, trace.Wrap(err)
	}
	var k *kv
	if err := json.Unmarshal(val, &k); err != nil {
		return nil, trace.Wrap(err)
	}
	if k.TTL != 0 && b.clock.UtcNow().Sub(k.Created) > k.TTL {
		if err := b.deleteKey(path, key); err != nil {
			return nil, err
		}
		return nil, trace.Wrap(&teleport.NotFoundError{
			Message: fmt.Sprintf("%v: %v not found", path, key)})
	}
	return k.Value, nil
}

func (b *BoltBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	var val []byte
	if err := b.getKey(path, key, &val); err != nil {
		return nil, 0, trace.Wrap(err)
	}
	var k *kv
	if err := json.Unmarshal(val, &k); err != nil {
		return nil, 0, trace.Wrap(err)
	}
	if k.TTL != 0 && b.clock.UtcNow().Sub(k.Created) > k.TTL {
		if err := b.deleteKey(path, key); err != nil {
			return nil, 0, trace.Wrap(err)
		}
		return nil, 0, trace.Wrap(&teleport.NotFoundError{
			Message: fmt.Sprintf("%v: %v not found", path, key)})
	}
	var newTTL time.Duration
	newTTL = 0
	if k.TTL != 0 {
		newTTL = k.Created.Add(k.TTL).Sub(b.clock.UtcNow())
	}
	return k.Value, newTTL, nil
}

func (b *BoltBackend) DeleteKey(path []string, key string) error {
	b.Lock()
	defer b.Unlock()
	return b.deleteKey(path, key)
}

func (b *BoltBackend) DeleteBucket(path []string, bucket string) error {
	b.Lock()
	defer b.Unlock()
	return b.deleteBucket(path, bucket)
}

func (b *BoltBackend) deleteBucket(buckets []string, bucket string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		if bkt.Bucket([]byte(bucket)) == nil {
			return trace.Wrap(&teleport.NotFoundError{
				Message: fmt.Sprintf("%v not found", bucket)})
		}
		return bkt.DeleteBucket([]byte(bucket))
	})
}

func (b *BoltBackend) deleteKey(buckets []string, key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		if bkt.Get([]byte(key)) == nil {
			return trace.Wrap(&teleport.NotFoundError{
				Message: fmt.Sprintf("%v is not found", key),
			})
		}
		return bkt.Delete([]byte(key))
	})
}

func (b *BoltBackend) upsertKey(buckets []string, key string, bytes []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) createKey(buckets []string, key string, bytes []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		val := bkt.Get([]byte(key))
		if val != nil {
			return trace.Wrap(
				teleport.AlreadyExists(fmt.Sprintf("'%v' already exists", key)))
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) upsertJSONKey(buckets []string, key string, val interface{}) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return trace.Wrap(err)
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := UpsertBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) getJSONKey(buckets []string, key string, val interface{}) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return trace.Wrap(&teleport.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			})
		}
		return json.Unmarshal(bytes, val)
	})
}

func (b *BoltBackend) getKey(buckets []string, key string, val *[]byte) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := GetBucket(tx, buckets)
		if err != nil {
			return trace.Wrap(err)
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return trace.Wrap(&teleport.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			})
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
			return trace.Wrap(err)
		}
		c := bkt.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			out = append(out, string(k))
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func UpsertBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt, err := b.CreateBucketIfNotExists([]byte(buckets[0]))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, key := range buckets[1:] {
		bkt, err = bkt.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return bkt, nil
}

func GetBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt := b.Bucket([]byte(buckets[0]))
	if bkt == nil {
		return nil, trace.Wrap(&teleport.NotFoundError{
			Message: fmt.Sprintf("bucket %v not found", buckets[0])})
	}
	for _, key := range buckets[1:] {
		bkt = bkt.Bucket([]byte(key))
		if bkt == nil {
			return nil, trace.Wrap(&teleport.NotFoundError{
				Message: fmt.Sprintf("bucket %v not found", key)})
		}
	}
	return bkt, nil
}

func (b *BoltBackend) AcquireLock(token string, ttl time.Duration) error {
	for {
		b.Lock()
		expires, ok := b.locks[token]
		if ok && (expires.IsZero() || expires.After(b.clock.UtcNow())) {
			b.Unlock()
			b.clock.Sleep(100 * time.Millisecond)
		} else {
			if ttl == 0 {
				b.locks[token] = time.Time{}
			} else {
				b.locks[token] = b.clock.UtcNow().Add(ttl)
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
	if !ok || (!expires.IsZero() && expires.Before(b.clock.UtcNow())) {
		return trace.Wrap(&teleport.NotFoundError{
			Message: fmt.Sprintf(
				"lock %v is deleted or expired", token),
		})
	}
	delete(b.locks, token)
	return nil
}

type kv struct {
	Created time.Time     `json:"created"`
	TTL     time.Duration `json:"ttl"`
	Value   []byte        `json:"val"`
}

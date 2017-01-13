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

package boltbk

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/boltdb/bolt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
)

const (
	// keysBoltFile is the BoltDB database file, usually stored in data_dir
	keysBoltFile = "keys.db"

	// openTimeout determines for how long BoltDB will wait before giving up
	// opening the locked DB file
	openTimeout = 5 * time.Second

	// openFileMode flag is passed to db.Open()
	openFileMode = 0600
)

// BoltBackend is a boltdb-based backend used in tests and standalone mode
type BoltBackend struct {
	sync.Mutex

	db    *bolt.DB
	clock timetools.TimeProvider
	locks map[string]time.Time
}

// GetName() is a part of the backend API and returns the name of this backend
// as shown in 'storage/type' section of Teleport YAML config
func GetName() string {
	return "bolt"
}

// New initializes and returns a fully created BoltDB backend. It's
// a properly implemented Backend.NewFunc, part of a backend API
func New(params backend.Params) (backend.Backend, error) {
	// look at 'path' parameter, if it's missing use 'data_dir' (default):
	path := params.GetString("path")
	if len(path) == 0 {
		path = params.GetString(teleport.DataDirParameterName)
	}
	// still nothing? return an error:
	if path == "" {
		return nil, trace.BadParameter("Bolt backend: 'path' is not set")
	}
	if !utils.IsDir(path) {
		return nil, trace.BadParameter("%v is not a valid directory", path)
	}
	path, err := filepath.Abs(filepath.Join(path, keysBoltFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db, err := bolt.Open(path, openFileMode, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, trace.Errorf("Local storage is locked. Another instance is running? (%v)", path)
		}
		return nil, trace.Wrap(err)
	}
	return &BoltBackend{
		locks: make(map[string]time.Time),
		clock: &timetools.RealTime{},
		db:    db,
	}, nil
}

// Close closes the backend resources
func (b *BoltBackend) Close() error {
	return b.db.Close()
}

func (b *BoltBackend) GetKeys(path []string) ([]string, error) {
	keys, err := b.getKeys(path)
	if err != nil {
		if trace.IsNotFound(err) {
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
		return nil, trace.NotFound("%v: %v not found", path, key)
	}
	return k.Value, nil
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
			return trace.NotFound("%v not found", bucket)
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
			return trace.NotFound("%v is not found", key)
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
			return trace.AlreadyExists("'%v' already exists", key)
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
			return trace.NotFound("%v %v not found", buckets, key)
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
			_, err := GetBucket(tx, append(buckets, key))
			if err == nil {
				return trace.BadParameter("key '%v 'is a bucket", key)
			}
			return trace.NotFound("%v %v not found", buckets, key)
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
		return nil, trace.NotFound("bucket %v not found", buckets[0])
	}
	for _, key := range buckets[1:] {
		bkt = bkt.Bucket([]byte(key))
		if bkt == nil {
			return nil, trace.NotFound("bucket %v not found", key)
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
		return trace.NotFound("lock %v is deleted or expired", token)
	}
	delete(b.locks, token)
	return nil
}

type kv struct {
	Created time.Time     `json:"created"`
	TTL     time.Duration `json:"ttl"`
	Value   []byte        `json:"val"`
}

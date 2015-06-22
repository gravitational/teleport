// package boltbk implements BoltDB backed backend for standalone instances
package boltbk

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/backend"

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

func (b *BoltBackend) AcquireLock(token string, ttl time.Duration) error {
	b.Lock()
	defer b.Unlock()

	expires, ok := b.locks[token]
	if ok && expires.After(time.Now()) {
		return &backend.AlreadyExistsError{
			Message: fmt.Sprintf("lock %v already locked", token)}
	}
	b.locks[token] = time.Now().Add(ttl)
	return nil
}

func (b *BoltBackend) ReleaseLock(token string) error {
	b.Lock()
	defer b.Unlock()

	expires, ok := b.locks[token]
	if !ok || expires.Before(time.Now()) {
		return &backend.NotFoundError{
			Message: fmt.Sprintf(
				"lock %v is deleted or expired", token),
		}
	}
	delete(b.locks, token)
	return nil
}

func (b *BoltBackend) Close() error {
	return nil
}

func (b *BoltBackend) UpsertRemoteCert(cert backend.RemoteCert, ttl time.Duration) error {
	return b.upsertKey([]string{"certs", cert.Type, "hosts", cert.FQDN}, cert.ID, cert.Value)
}

func (b *BoltBackend) GetRemoteCerts(ctype string, fqdn string) ([]backend.RemoteCert, error) {
	out := []backend.RemoteCert{}
	err := b.db.View(func(tx *bolt.Tx) error {
		hosts := []string{}
		if fqdn == "" {
			bkt, err := getBucket(tx, []string{"certs", ctype, "hosts"})
			if err != nil {
				if isNotFound(err) {
					return nil
				}
				return err
			}
			c := bkt.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				hosts = append(hosts, string(k))
			}
		} else {
			hosts = []string{fqdn}
		}
		for _, h := range hosts {
			bkt, err := getBucket(tx, []string{"certs", ctype, "hosts", h})
			if err != nil {
				return err
			}
			c := bkt.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				out = append(out, backend.RemoteCert{
					Type:  ctype,
					FQDN:  h,
					ID:    string(k),
					Value: v,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (b *BoltBackend) DeleteRemoteCert(ctype string, fqdn, id string) error {
	return b.deleteKey([]string{"certs", ctype, "hosts", fqdn}, id)
}

// GetUsers  returns a list of users registered in the backend
func (b *BoltBackend) GetUsers() ([]string, error) {
	out := []string{}
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		c := b.Cursor()
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

// DeleteUser deletes a user with all the keys from the backend
func (b *BoltBackend) DeleteUser(user string) error {
	return b.deleteBucket([]string{"users"}, user)
}

func (b *BoltBackend) UpsertUserCA(a backend.CA) error {
	return b.upsertJSONKey([]string{"userca"}, "val", a)
}

func (b *BoltBackend) GetUserCA() (*backend.CA, error) {
	var ca *backend.CA
	return ca, b.getJSONKey([]string{"userca"}, "val", &ca)
}

func (b *BoltBackend) GetUserCAPub() ([]byte, error) {
	ca, err := b.GetUserCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

func (b *BoltBackend) UpsertHostCA(a backend.CA) error {
	return b.upsertJSONKey([]string{"hostca"}, "val", a)
}

func (b *BoltBackend) GetHostCA() (*backend.CA, error) {
	var ca *backend.CA
	return ca, b.getJSONKey([]string{"hostca"}, "val", &ca)
}

func (b *BoltBackend) GetHostCAPub() ([]byte, error) {
	ca, err := b.GetHostCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

func (b *BoltBackend) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	if user == "" {
		return nil, &backend.MissingParameterError{Param: "user"}
	}
	values := []backend.AuthorizedKey{}
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, []string{"users", user, "keys"})
		if err != nil {
			if _, ok := err.(*backend.NotFoundError); ok {
				return nil
			}
			return err
		}
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var key *backend.AuthorizedKey
			if err := json.Unmarshal(v, &key); err != nil {
				return err
			}
			values = append(values, *key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

func (b *BoltBackend) UpsertToken(token, fqdn string, ttl time.Duration) error {
	return b.upsertKey([]string{"tokens"}, token, []byte(fqdn))
}

func (b *BoltBackend) GetToken(token string) (string, error) {
	var fqdn []byte
	if err := b.getKey([]string{"tokens"}, token, &fqdn); err != nil {
		return "", err
	}
	return string(fqdn), nil
}

func (b *BoltBackend) DeleteToken(token string) error {
	return b.deleteKey([]string{"tokens"}, token)
}

func (b *BoltBackend) UpsertUserKey(user string, key backend.AuthorizedKey, ttl time.Duration) error {
	if user == "" {
		return &backend.MissingParameterError{Param: "user"}
	}
	if key.ID == "" {
		return &backend.MissingParameterError{Param: "key.id"}
	}
	if len(key.Value) == 0 {
		return &backend.MissingParameterError{Param: "key.val"}
	}
	return b.upsertJSONKey([]string{"users", user, "keys"}, key.ID, key)
}

func (b *BoltBackend) DeleteUserKey(user, keyID string) error {
	if user == "" {
		return &backend.MissingParameterError{Param: "user"}
	}
	if keyID == "" {
		return &backend.MissingParameterError{Param: "key.id"}
	}
	return b.deleteKey([]string{"users", user, "keys"}, keyID)
}

func (b *BoltBackend) UpsertServer(s backend.Server, ttl time.Duration) error {
	return b.upsertJSONKey([]string{"servers"}, "val", s)
}

func (b *BoltBackend) GetServers() ([]backend.Server, error) {
	values := []backend.Server{}
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("servers"))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var key *backend.Server
			if err := json.Unmarshal(v, &key); err != nil {
				return err
			}
			values = append(values, *key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

func (b *BoltBackend) UpsertPasswordHash(user string, hash []byte) error {
	return b.upsertKey([]string{"users", user}, "password-hash", hash)
}

func (b *BoltBackend) GetPasswordHash(user string) ([]byte, error) {
	var hash []byte
	err := b.getKey([]string{"users", user}, "password-hash", &hash)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func (b *BoltBackend) UpsertWebSession(user, sid string, s backend.WebSession, ttl time.Duration) error {
	return b.upsertJSONKey([]string{"users", user, "web-sessions"}, sid, s)
}

func (b *BoltBackend) GetWebSession(user, sid string) (*backend.WebSession, error) {
	var ws *backend.WebSession
	return ws, b.getJSONKey([]string{"users", user, "web-sessions"}, sid, &ws)
}

func (b *BoltBackend) GetWebSessionsKeys(user string) ([]backend.AuthorizedKey, error) {
	values := []backend.AuthorizedKey{}
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, []string{"users", user, "web-sessions"})
		if err != nil {
			if _, ok := err.(*backend.NotFoundError); ok {
				return nil
			}
			return err
		}
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var ws *backend.WebSession
			if err := json.Unmarshal(v, &ws); err != nil {
				return err
			}
			values = append(values, backend.AuthorizedKey{
				ID:    string(k),
				Value: ws.Pub,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

func (b *BoltBackend) DeleteWebSession(user, sid string) error {
	return b.deleteKey([]string{"users", user, "web-sessions"}, sid)
}

func (b *BoltBackend) UpsertWebTun(t backend.WebTun, ttl time.Duration) error {
	if t.Prefix == "" {
		return &backend.MissingParameterError{Param: "Prefix"}
	}
	return b.upsertJSONKey([]string{"web-tuns"}, t.Prefix, t)
}

func (b *BoltBackend) GetWebTun(prefix string) (*backend.WebTun, error) {
	var wt *backend.WebTun
	return wt, b.getJSONKey([]string{"web-tuns"}, prefix, &wt)
}

func (b *BoltBackend) DeleteWebTun(prefix string) error {
	return b.deleteKey([]string{"web-tuns"}, prefix)
}

func (b *BoltBackend) GetWebTuns() ([]backend.WebTun, error) {
	out := []backend.WebTun{}
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, []string{"web-tuns"})
		if err != nil {
			return err
		}
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var wt *backend.WebTun
			if err := json.Unmarshal(v, &wt); err != nil {
				return err
			}
			out = append(out, *wt)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (b *BoltBackend) deleteBucket(buckets []string, bucket string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return err
		}
		if bkt.Bucket([]byte(bucket)) == nil {
			return &backend.NotFoundError{
				fmt.Sprintf("%v not found", bucket)}
		}
		return bkt.DeleteBucket([]byte(bucket))
	})
}

func (b *BoltBackend) deleteKey(buckets []string, key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return err
		}
		if bkt.Get([]byte(key)) == nil {
			return &backend.NotFoundError{}
		}
		return bkt.Delete([]byte(key))
	})
}

func (b *BoltBackend) upsertKey(buckets []string, key string, bytes []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := upsertBucket(tx, buckets)
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
		bkt, err := upsertBucket(tx, buckets)
		if err != nil {
			return err
		}
		return bkt.Put([]byte(key), bytes)
	})
}

func (b *BoltBackend) getJSONKey(buckets []string, key string, val interface{}) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return &backend.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			}
		}
		return json.Unmarshal(bytes, val)
	})
}

func (b *BoltBackend) getKey(buckets []string, key string, val *[]byte) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt, err := getBucket(tx, buckets)
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte(key))
		if bytes == nil {
			return &backend.NotFoundError{
				Message: fmt.Sprintf("%v %v not found", buckets, key),
			}
		}
		*val = bytes
		return nil
	})
}

func upsertBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
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

func getBucket(b *bolt.Tx, buckets []string) (*bolt.Bucket, error) {
	bkt := b.Bucket([]byte(buckets[0]))
	if bkt == nil {
		return nil, &backend.NotFoundError{
			Message: fmt.Sprintf("bucket %v not found", buckets[0])}
	}
	for _, key := range buckets[1:] {
		bkt = bkt.Bucket([]byte(key))
		if bkt == nil {
			return nil, &backend.NotFoundError{
				Message: fmt.Sprintf("bucket %v not found", key)}
		}
	}
	return bkt, nil
}

func isNotFound(err error) bool {
	_, ok := err.(*backend.NotFoundError)
	return ok
}

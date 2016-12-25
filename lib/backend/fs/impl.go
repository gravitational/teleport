/*
Copyright 2016 Gravitational, Inc.

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

package fs

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
	"github.com/mailgun/timetools"
)

const (
	defaultDirMode  os.FileMode = 0770
	defaultFileMode os.FileMode = 0600

	// subdirectory where locks are stored
	locksDir = ".locks"

	// selfLock is the lock used internally for compare-and-swap
	selfLock = ".backend"
)

// fs.Backend implements backend.Backend interface using a regular
// POSIX-style filesystem
type Backend struct {
	sync.Mutex

	// RootDir is the root (home) directory where the backend
	// stores all the data.
	RootDir string

	// LocksDir is where lock files are kept
	LocksDir string

	// Clock is a test-friendly source of current time
	Clock timetools.TimeProvider

	// locks keeps the map of active file locks
	locks map[string]string
}

// FromJSON creates a new filesystem-based storage backend using a JSON
// configuration string which must look like this:
//   { "path": "/var/lib/whatever" }
func FromJSON(jsonStr string) (bk *Backend, err error) {
	const key = "path"
	var m map[string]string
	if err = json.Unmarshal([]byte(jsonStr), &m); err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Errorf("Invalid file backend configuration: %v", err)
	}
	path, ok := m[key]
	if !ok {
		return nil, trace.Errorf("'%s' field is missing for the file backend", key)
	}
	return New(path)
}

// New creates a fully initialized filesystem backend
func New(rootDir string) (*Backend, error) {
	bk := &Backend{RootDir: rootDir, Clock: &timetools.RealTime{}}
	bk.LocksDir = path.Join(bk.RootDir, locksDir)
	bk.locks = make(map[string]string)
	if err := os.MkdirAll(bk.LocksDir, defaultDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return bk, nil
}

// GetKeys returns a list of keys for a given path
func (bk *Backend) GetKeys(bucket []string) ([]string, error) {
	files, err := ioutil.ReadDir(path.Join(bk.RootDir, path.Join(bucket...)))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, trace.ConvertSystemError(err)
	}
	// enumerate all directory entries and select only non-hidden files
	retval := make([]string, 0)
	for _, fi := range files {
		name := fi.Name()
		if !fi.IsDir() && name[0] != '.' {
			retval = append(retval, name)
		}
	}
	return retval, nil
}

// CreateVal creates value with a given TTL and key in the bucket
// if the value already exists, returns AlreadyExistsError
func (bk *Backend) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	log.Debugf("fs.CreateVal(%s/%s) '%v'", strings.Join(bucket, "/"), key, string(val))
	// do not allow keys that start with a dot
	if key[0] == '.' {
		return trace.BadParameter("Invalid key: '%s'. Key names cannot start with '.'", key)
	}
	// create the directory:
	dirPath := path.Join(bk.RootDir, path.Join(bucket...))
	err := os.MkdirAll(dirPath, defaultDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	// create the file (AKA "key"):
	filename := path.Join(dirPath, key)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, defaultFileMode)
	if err != nil {
		if os.IsExist(err) {
			return trace.AlreadyExists("%s/%s already exists", dirPath, key)
		}
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	n, err := f.Write(val)
	if err == nil && n < len(val) {
		return trace.Wrap(io.ErrShortWrite)
	}
	bk.applyTTL(dirPath, key, ttl)
	return nil
}

// UpsertVal updates or inserts value with a given TTL into a bucket
// ForeverTTL for no TTL
func (bk *Backend) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	log.Debugf("fs.UpsertVal(%s/%s) '%v'", strings.Join(bucket, "/"), key, string(val))
	// create the directory:
	dirPath := path.Join(bk.RootDir, path.Join(bucket...))
	err := os.MkdirAll(dirPath, defaultDirMode)
	if err != nil {
		return trace.Wrap(err)
	}
	// create the (or overwrite existing) file (AKA "key"):
	return ioutil.WriteFile(path.Join(dirPath, key), val, defaultFileMode)
}

// GetVal return a value for a given key in the bucket
func (bk *Backend) GetVal(bucket []string, key string) ([]byte, error) {
	log.Debugf("fs.GetVal(%s/%s)", strings.Join(bucket, "/"), key)
	dirPath := path.Join(path.Join(bk.RootDir, path.Join(bucket...)))
	expired, err := bk.checkTTL(dirPath, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if expired {
		bk.DeleteKey(bucket, key)
		return nil, trace.NotFound("key '%s' is not found", key)
	}
	return ioutil.ReadFile(path.Join(dirPath, key))
}

// DeleteKey deletes a key in a bucket
func (bk *Backend) DeleteKey(bucket []string, key string) error {
	dirPath := path.Join(bk.RootDir, path.Join(bucket...))
	if err := os.Remove(bk.ttlFile(dirPath, key)); err != nil {
		if !os.IsNotExist(err) {
			log.Warn(err)
		}
	}
	return trace.ConvertSystemError(os.Remove(
		path.Join(dirPath, key)))
}

// DeleteBucket deletes the bucket by a given path
func (bk *Backend) DeleteBucket(parent []string, bucket string) error {
	return trace.ConvertSystemError(os.RemoveAll(
		path.Join(path.Join(bk.RootDir, path.Join(parent...)), bucket)))
}

// AcquireLock grabs a lock that will be released automatically in TTL
func (bk *Backend) AcquireLock(token string, ttl time.Duration) (err error) {
	log.Debugf("fs.AcquireLock(%s)", token)
	lockPath := path.Join(bk.LocksDir, token)
	var f *os.File
	for {
		f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil { // success
			defer f.Close()
			break
		}
		if os.IsExist(err) { // locked? wait and repeat:
			bk.Clock.Sleep(time.Millisecond * 100)
			continue
		}
		return trace.ConvertSystemError(err)
	}

	// protect the locks map:
	bk.Lock()
	defer bk.Unlock()
	bk.locks[token] = f.Name()

	// start the goroutine which will release lock after the given TTL
	if ttl != backend.Forever {
		go func() {
			bk.Clock.Sleep(ttl)
			bk.ReleaseLock(token)
		}()
	}
	return nil
}

// ReleaseLock forces lock release before TTL
func (bk *Backend) ReleaseLock(token string) (err error) {
	log.Debugf("fs.ReleaseLock(%s)", token)
	// protect the locks map:
	bk.Lock()
	defer bk.Unlock()
	// find the lock:
	fn, found := bk.locks[token]
	if !found {
		return trace.NotFound("lock '%s' is not found", token)
	}
	// remove it:
	if err = os.Remove(fn); err != nil {
		if !os.IsNotExist(err) {
			log.Warn(err)
		}
	}
	return trace.ConvertSystemError(err)
}

// CompareAndSwap implements compare ans swap operation for a key
func (bk *Backend) CompareAndSwap(
	bucket []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	// lock the entire backend:
	bk.AcquireLock(selfLock, time.Second)
	defer bk.ReleaseLock(selfLock)

	storedVal, err := bk.GetVal(bucket, key)
	if err != nil {
		if trace.IsNotFound(err) && len(prevVal) != 0 {
			return nil, err
		}
	}
	if len(prevVal) == 0 && err == nil {
		return nil, trace.AlreadyExists("key '%v' already exists", key)
	}
	if string(prevVal) == string(storedVal) {
		if err = bk.UpsertVal(bucket, key, val, ttl); err != nil {
			return nil, trace.Wrap(err)
		}
		return storedVal, nil
	}
	return storedVal, trace.CompareFailed("expected: %v, got: %v", string(prevVal), string(storedVal))
}

// Close releases the resources taken up by this backend: locks
func (bk *Backend) Close() error {
	for lockName, _ := range bk.locks {
		bk.ReleaseLock(lockName)
	}
	return nil
}

// applyTTL assigns a given TTL to a file with sub-second granularity
func (bk *Backend) applyTTL(dirPath string, key string, ttl time.Duration) error {
	if ttl == backend.Forever {
		return nil
	}
	expiryTime := bk.Clock.UtcNow().Add(ttl)
	bytes, _ := expiryTime.MarshalText()
	return trace.ConvertSystemError(
		ioutil.WriteFile(bk.ttlFile(dirPath, key), bytes, defaultFileMode))
}

// checkTTL checks if a given file has TTL and returns 'true' if it's expired
func (bk *Backend) checkTTL(dirPath string, key string) (expired bool, err error) {
	bytes, err := ioutil.ReadFile(bk.ttlFile(dirPath, key))
	if err != nil {
		if os.IsNotExist(err) { // no TTL
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	var expiryTime time.Time
	if err = expiryTime.UnmarshalText(bytes); err != nil {
		return false, trace.Wrap(err)
	}
	return bk.Clock.UtcNow().After(expiryTime), nil
}

// ttlFile returns the full path of the "TTL file" where the TTL is
// stored for a given key, example: /root/bucket/.keyname.ttl
func (bk *Backend) ttlFile(dirPath, key string) string {
	return path.Join(dirPath, "."+key+".ttl")
}

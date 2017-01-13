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

package dir

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"

	"github.com/jonboulle/clockwork"
)

const (
	defaultDirMode  os.FileMode = 0770
	defaultFileMode os.FileMode = 0600

	// name of this backend type (as seen in 'storage/type' in YAML)
	backendName = "dir"

	// selfLock is the lock used internally for compare-and-swap
	selfLock = ".backend"

	// subdirectory where locks are stored
	locksBucket = ".locks"

	// reservedPrefix is a character which bucket/key names cannot begin with
	reservedPrefix = '.'
)

// fs.Backend implements backend.Backend interface using a regular
// POSIX-style filesystem
type Backend struct {
	// RootDir is the root (home) directory where the backend
	// stores all the data.
	RootDir string

	// Clock is a test-friendly source of current time
	Clock clockwork.Clock
}

// GetName
func GetName() string {
	return backendName
}

// New creates a new instance of Filesystem backend, it conforms to backend.NewFunc API
func New(params backend.Params) (backend.Backend, error) {
	rootDir := params.GetString("path")
	if rootDir == "" {
		return nil, trace.BadParameter("filesystem backend: 'path' is not set")
	}

	bk := &Backend{
		RootDir: rootDir,
		Clock:   clockwork.NewRealClock(),
	}

	// did tests pass the fake (test) clock?
	clockParam, ok := params["test_clock"]
	if ok {
		bk.Clock, _ = clockParam.(clockwork.Clock)
	}

	locksDir := path.Join(bk.RootDir, locksBucket)
	if err := os.MkdirAll(locksDir, defaultDirMode); err != nil {
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
		// legal keys cannot start with '.' (resrved prefix)
		if name[0] != reservedPrefix {
			retval = append(retval, name)
		}
	}
	return retval, nil
}

// CreateVal creates value with a given TTL and key in the bucket
// if the value already exists, returns AlreadyExistsError
func (bk *Backend) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	// do not allow keys that start with a dot
	if key[0] == reservedPrefix {
		return trace.BadParameter("invalid key: '%s'. Key names cannot start with '.'", key)
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
	dirPath := path.Join(path.Join(bk.RootDir, path.Join(bucket...)))
	expired, err := bk.checkTTL(dirPath, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if expired {
		bk.DeleteKey(bucket, key)
		return nil, trace.NotFound("key '%s' is not found", key)
	}
	fp := path.Join(dirPath, key)
	bytes, err := ioutil.ReadFile(fp)
	if err != nil {
		// GetVal() on a bucket must return 'BadParameter' error:
		if fi, _ := os.Stat(fp); fi != nil && fi.IsDir() {
			return nil, trace.BadParameter("%s is not a valid key", key)
		}
		return nil, trace.ConvertSystemError(err)
	}
	return bytes, nil
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

	if err = backend.ValidateLockTTL(ttl); err != nil {
		return trace.Wrap(err)
	}

	bucket := []string{locksBucket}
	for {
		// GetVal will clear TTL on a lock
		bk.GetVal(bucket, token)

		// CreateVal is atomic:
		err = bk.CreateVal(bucket, token, []byte{1}, ttl)
		if err == nil {
			break // success
		}
		if trace.IsAlreadyExists(err) { // locked? wait and repeat:
			bk.Clock.Sleep(time.Millisecond * 250)
			continue
		}
		return trace.ConvertSystemError(err)
	}
	return nil
}

// ReleaseLock forces lock release before TTL
func (bk *Backend) ReleaseLock(token string) (err error) {
	log.Debugf("fs.ReleaseLock(%s)", token)

	if err = bk.DeleteKey([]string{locksBucket}, token); err != nil {
		if !os.IsNotExist(err) {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// Close releases the resources taken up by a backend
func (bk *Backend) Close() error {
	return nil
}

// applyTTL assigns a given TTL to a file with sub-second granularity
func (bk *Backend) applyTTL(dirPath string, key string, ttl time.Duration) error {
	if ttl == backend.Forever {
		return nil
	}
	expiryTime := bk.Clock.Now().Add(ttl)
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
	return bk.Clock.Now().After(expiryTime), nil
}

// ttlFile returns the full path of the "TTL file" where the TTL is
// stored for a given key, example: /root/bucket/.keyname.ttl
func (bk *Backend) ttlFile(dirPath, key string) string {
	return path.Join(dirPath, "."+key+".ttl")
}

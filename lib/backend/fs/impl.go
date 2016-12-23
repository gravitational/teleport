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
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

const (
	defaultDirMode  os.FileMode = 0770
	defaultFileMode os.FileMode = 0600
)

type Backend struct {
	Path string
}

// FromJSON creates a new filesystem-based storage backend using a JSON
// configuration string which must look like this:
//   { "path": "/var/lib/whatever" }
func FromJSON(jsonStr string) (*Backend, error) {
	const key = "path"
	var m map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Errorf("Invalid file backend configuration: %v", err)
	}
	path, ok := m[key]
	if !ok {
		return nil, trace.Errorf("'%s' field is missing for the file backend", key)
	}
	if err := os.MkdirAll(path, defaultDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &Backend{Path: path}, nil
}

// GetKeys returns a list of keys for a given path
func (bk *Backend) GetKeys(bucket []string) ([]string, error) {
	files, err := ioutil.ReadDir(path.Join(bk.Path, path.Join(bucket...)))
	if err != nil {
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
	// create the directory:
	dirPath := path.Join(bk.Path, path.Join(bucket...))
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
	return nil
}

// UpsertVal updates or inserts value with a given TTL into a bucket
// ForeverTTL for no TTL
func (bk *Backend) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	// create the directory:
	dirPath := path.Join(bk.Path, path.Join(bucket...))
	err := os.MkdirAll(dirPath, defaultDirMode)
	if err != nil {
		return trace.Wrap(err)
	}
	// create the (or overwrite existing) file (AKA "key"):
	return ioutil.WriteFile(path.Join(dirPath, key), val, defaultFileMode)
}

// GetVal return a value for a given key in the bucket
func (bk *Backend) GetVal(bucket []string, key string) ([]byte, error) {
	return ioutil.ReadFile(
		path.Join(path.Join(bk.Path, path.Join(bucket...)), key))
}

// DeleteKey deletes a key in a bucket
func (bk *Backend) DeleteKey(bucket []string, key string) error {
	return trace.ConvertSystemError(os.Remove(
		path.Join(path.Join(bk.Path, path.Join(bucket...)), key)))
}

// DeleteBucket deletes the bucket by a given path
func (bk *Backend) DeleteBucket(parent []string, bucket string) error {
	return trace.ConvertSystemError(os.RemoveAll(
		path.Join(path.Join(bk.Path, path.Join(parent...)), bucket)))
}

// AcquireLock grabs a lock that will be released automatically in TTL
func (bk *Backend) AcquireLock(token string, ttl time.Duration) error {
	return nil
}

// ReleaseLock forces lock release before TTL
func (bk *Backend) ReleaseLock(token string) error {
	return nil
}

// CompareAndSwap implements compare ans swap operation for a key
func (bk *Backend) CompareAndSwap(
	bucket []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	return nil, nil
}

// Close releases the resources taken up by this backend
func (bk *Backend) Close() error {
	return nil
}

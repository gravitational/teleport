// Copyright 2018 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// storageFS is an implementation of the backend storage that stores data on disk
//
// The layout is the following:
//
// - rootDir
//   |- bucket1
//   \- bucket2
//     |- object1
//     \- object2
//
// Bucket and object names are url path escaped, so there's no special meaning of forward slashes.
type storageFS struct {
	rootDir string
	mtx     sync.RWMutex
}

// NewStorageFS creates an instance of the filesystem-backed storage backend.
func NewStorageFS(objects []Object, rootDir string) (Storage, error) {
	if !strings.HasSuffix(rootDir, "/") {
		rootDir += "/"
	}
	err := os.MkdirAll(rootDir, 0o700)
	if err != nil {
		return nil, err
	}
	s := &storageFS{rootDir: rootDir}
	for _, o := range objects {
		_, err := s.CreateObject(o)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// CreateBucket creates a bucket in the fs backend. A bucket is a folder in the
// root directory.
func (s *storageFS) CreateBucket(name string, versioningEnabled bool) error {
	if versioningEnabled {
		return errors.New("not implemented: fs storage type does not support versioning yet")
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.createBucket(name)
}

func (s *storageFS) createBucket(name string) error {
	return os.MkdirAll(filepath.Join(s.rootDir, url.PathEscape(name)), 0o700)
}

// ListBuckets returns a list of buckets from the list of directories in the
// root directory.
func (s *storageFS) ListBuckets() ([]Bucket, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	infos, err := ioutil.ReadDir(s.rootDir)
	if err != nil {
		return nil, err
	}
	buckets := []Bucket{}
	for _, info := range infos {
		if info.IsDir() {
			unescaped, err := url.PathUnescape(info.Name())
			if err != nil {
				return nil, fmt.Errorf("failed to unescape object name %s: %w", info.Name(), err)
			}
			buckets = append(buckets, Bucket{Name: unescaped})
		}
	}
	return buckets, nil
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}

// GetBucket returns information about the given bucket, or an error if it
// doesn't exist.
func (s *storageFS) GetBucket(name string) (Bucket, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	dirInfo, err := os.Stat(filepath.Join(s.rootDir, url.PathEscape(name)))
	if err != nil {
		return Bucket{}, err
	}
	return Bucket{Name: name, VersioningEnabled: false, TimeCreated: timespecToTime(createTimeFromFileInfo(dirInfo))}, err
}

// CreateObject stores an object as a regular file in the disk.
func (s *storageFS) CreateObject(obj Object) (Object, error) {
	if obj.Generation > 0 {
		return Object{}, errors.New("not implemented: fs storage type does not support objects generation yet")
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	err := s.createBucket(obj.BucketName)
	if err != nil {
		return Object{}, err
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		return Object{}, err
	}
	return obj, ioutil.WriteFile(filepath.Join(s.rootDir, url.PathEscape(obj.BucketName), url.PathEscape(obj.Name)), encoded, 0o600)
}

// ListObjects lists the objects in a given bucket with a given prefix and
// delimeter.
func (s *storageFS) ListObjects(bucketName string, versions bool) ([]Object, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	infos, err := ioutil.ReadDir(path.Join(s.rootDir, url.PathEscape(bucketName)))
	if err != nil {
		return nil, err
	}
	objects := []Object{}
	for _, info := range infos {
		unescaped, err := url.PathUnescape(info.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to unescape object name %s: %w", info.Name(), err)
		}
		object, err := s.getObject(bucketName, unescaped)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

// GetObject get an object by bucket and name.
func (s *storageFS) GetObject(bucketName, objectName string) (Object, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.getObject(bucketName, objectName)
}

// GetObjectWithGeneration retrieves an specific version of the object. Not
// implemented for this backend.
func (s *storageFS) GetObjectWithGeneration(bucketName, objectName string, generation int64) (Object, error) {
	return Object{}, errors.New("not implemented: fs storage type does not support versioning yet")
}

func (s *storageFS) getObject(bucketName, objectName string) (Object, error) {
	encoded, err := ioutil.ReadFile(filepath.Join(s.rootDir, url.PathEscape(bucketName), url.PathEscape(objectName)))
	if err != nil {
		return Object{}, err
	}
	var obj Object
	err = json.Unmarshal(encoded, &obj)
	if err != nil {
		return Object{}, err
	}
	obj.Name = filepath.ToSlash(objectName)
	obj.BucketName = bucketName
	return obj, nil
}

// DeleteObject deletes an object by bucket and name.
func (s *storageFS) DeleteObject(bucketName, objectName string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if objectName == "" {
		return errors.New("can't delete object with empty name")
	}
	return os.Remove(filepath.Join(s.rootDir, url.PathEscape(bucketName), url.PathEscape(objectName)))
}

// PatchObject patches the given object metadata.
func (s *storageFS) PatchObject(bucketName, objectName string, metadata map[string]string) (Object, error) {
	obj, err := s.GetObject(bucketName, objectName)
	if err != nil {
		return Object{}, err
	}
	if obj.Metadata == nil {
		obj.Metadata = map[string]string{}
	}
	for k, v := range metadata {
		obj.Metadata[k] = v
	}
	s.CreateObject(obj) // recreate object
	return obj, nil
}

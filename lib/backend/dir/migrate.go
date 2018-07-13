/*
Copyright 2018 Gravitational, Inc.

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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
)

// DELETE IN: 2.8.0
// migrate old directory backend to the new flat keyspace.
func migrate(rootDir string, b *Backend) error {
	// Check if the directory structure is the old bucket format.
	ok, err := isOld(rootDir)
	if err != nil {
		return trace.Wrap(err)
	}
	// Found the new flat keyspace directory backend, nothing to do.
	if !ok {
		b.log.Debugf("Found new flat keyspace, skipping migration.")
		return nil
	}

	// The old directory backend was found, make a backup in-case is needs to
	// be restored.
	backupDir := rootDir + ".backup-" + time.Now().Format(time.RFC3339)
	err = os.Rename(rootDir, backupDir)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.MkdirAll(rootDir, defaultDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	b.log.Infof("Migrating directory backend to new flat keyspace. Backup in %v.", backupDir)

	// Go over every file in the backend. If the key is not expired upsert
	// into the new backend.
	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		// Skip the locks directory completely.
		if info.IsDir() && info.Name() == ".locks" {
			return filepath.SkipDir
		}
		// Skip over directories themselves, but not their content.
		if info.IsDir() {
			return nil
		}
		// Skip over TTL files (they are handled by readTTL function).
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Construct key and flat bucket.
		key := info.Name()
		flatbucket := strings.TrimPrefix(path, backupDir)
		flatbucket = strings.TrimSuffix(flatbucket, string(filepath.Separator)+key)

		// Read in TTL for key. If the key is expired, skip over it.
		ttl, expired, err := readTTL(backupDir, flatbucket, key)
		if err != nil {
			return trace.Wrap(err)
		}
		if expired {
			b.log.Infof("Skipping migration of expired bucket %q and key %q.", flatbucket, info.Name())
			return nil
		}

		// Read in the value of the key.
		value, err := ioutil.ReadFile(path)
		if err != nil {
			return trace.Wrap(err)
		}

		// Upsert key and value (with TTL) into new flat keyspace backend.
		bucket := strings.Split(flatbucket, string(filepath.Separator))
		err = b.UpsertVal(bucket, key, value, ttl)
		if err != nil {
			return trace.Wrap(err)
		}
		b.log.Infof("Migrated bucket %q and key %q with TTL %v.", flatbucket, key, ttl)

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	b.log.Infof("Migration successful.")
	return nil
}

// DELETE IN: 2.8.0
// readTTL reads in TTL for the given key. If no TTL key is found,
// backend.Forever is returned.
func readTTL(rootDir string, bucket string, key string) (time.Duration, bool, error) {
	filename := filepath.Join(rootDir, bucket, "."+key+".ttl")

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return backend.Forever, false, nil
		}
		return backend.Forever, false, trace.Wrap(err)
	}
	if len(bytes) == 0 {
		return backend.Forever, false, nil
	}

	var expiryTime time.Time
	if err = expiryTime.UnmarshalText(bytes); err != nil {
		return backend.Forever, false, trace.Wrap(err)
	}

	ttl := expiryTime.Sub(time.Now())
	if ttl < 0 {
		return backend.Forever, true, nil
	}
	return ttl, false, nil
}

// DELETE IN: 2.8.0
// isOld checks if the directory backend is in the old format or not.
func isOld(rootDir string) (bool, error) {
	d, err := os.Open(rootDir)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	defer d.Close()

	files, err := d.Readdir(0)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}

	for _, fi := range files {
		if fi.IsDir() {
			return true, nil
		}
	}

	return false, nil
}

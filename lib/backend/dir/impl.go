/*
Copyright 2016-2017 Gravitational, Inc.

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
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

const (
	defaultDirMode  os.FileMode = 0770
	defaultFileMode os.FileMode = 0600

	// backendName of this backend type as seen in "storage/type" in YAML.
	backendName = "dir"

	// locksBucket is where backend locks are stored.
	locksBucket = ".locks"
)

// Backend implements backend.Backend interface using a regular
// POSIX-style filesystem
type Backend struct {
	// InternalClock is a test-friendly source of current time
	InternalClock clockwork.Clock

	// rootDir is the directory where the backend stores all the data.
	rootDir string

	// log is a structured component logger.
	log *logrus.Entry
}

// Clock returns the clock used by this backend.
func (b *Backend) Clock() clockwork.Clock {
	return b.InternalClock
}

// GetName returns the name of this backend.
func GetName() string {
	return backendName
}

// New creates a new instance of a directory based backend that implements
// backend.Backend.
func New(params backend.Params) (backend.Backend, error) {
	rootDir := params.GetString("path")
	if rootDir == "" {
		rootDir = params.GetString("data_dir")
	}
	if rootDir == "" {
		return nil, trace.BadParameter("filesystem backend: 'path' is not set")
	}

	// Ensure that the path to the root directory exists.
	err := os.MkdirAll(rootDir, defaultDirMode)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	bk := &Backend{
		InternalClock: clockwork.NewRealClock(),
		rootDir:       rootDir,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "backend:dir",
			trace.ComponentFields: logrus.Fields{
				"dir": rootDir,
			},
		}),
	}

	// DELETE IN: 2.8.0
	// Migrate data to new flat keyspace backend.
	err = migrate(rootDir, bk)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap the backend in a input sanitizer and return it.
	return backend.NewSanitizer(bk), nil
}

// Close releases the resources taken up the backend.
func (bk *Backend) Close() error {
	return nil
}

// GetKeys returns a list of keys for a given bucket.
func (bk *Backend) GetKeys(bucket []string) ([]string, error) {
	// Get all the key/value pairs for this bucket.
	items, err := bk.GetItems(bucket)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return only the keys, the keys are already sorted by GetItems.
	keys := make([]string, len(items))
	for i, e := range items {
		keys[i] = e.Key
	}

	return keys, nil
}

// GetItems returns all items (key/value pairs) in a given bucket.
func (bk *Backend) GetItems(bucket []string) ([]backend.Item, error) {
	var out []backend.Item

	// Get a list of all buckets in the backend.
	files, err := ioutil.ReadDir(path.Join(bk.rootDir))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Loop over all buckets in the backend.
	for _, fi := range files {
		pathToBucket := bk.pathToBucket(fi.Name())
		bucketPrefix := bk.flatten(bucket)

		// Skip over any buckets without a matching prefix.
		if !strings.HasPrefix(pathToBucket, bucketPrefix) {
			continue
		}

		// Open the bucket to work on the items.
		b, err := bk.openBucket(pathToBucket, os.O_RDWR)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer b.Close()

		// Loop over all keys, flatten them, and return key and value to caller.
		for k, v := range b.items {
			var key string

			// If bucket path on disk and the requested bucket were an exact match,
			// return the key as-is.
			//
			// However, if this was a partial match, for example pathToBucket is
			// "/roles/admin/params" but the bucketPrefix is "/roles" then extract
			// the first suffix (in this case "admin") and use this as the key. This
			// is consistent with our DynamoDB implementation.
			if pathToBucket == bucketPrefix {
				key = k
			} else {
				key, err = suffix(pathToBucket, bucketPrefix)
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}

			// If the bucket item is expired, update the bucket, and don't include
			// it in the output.
			if bk.isExpired(v) {
				b.deleteItem(k)
				continue
			}

			out = append(out, backend.Item{
				Key:   key,
				Value: v.Value,
			})
		}
	}

	// Sort and return results.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})

	return out, nil
}

// CreateVal creates a key/value pair with the given TTL in the bucket. If
// the key already exists in the bucket, trace.AlreadyExists is returned.
func (bk *Backend) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return trace.Wrap(err)
	}
	defer b.Close()

	// If the key exists and is not expired, return trace.AlreadyExists.
	item, ok := b.getItem(key)
	if ok && !bk.isExpired(item) {
		return trace.AlreadyExists("key already exists")
	}

	// Otherwise, update the item in the bucket.
	b.updateItem(key, val, ttl)

	return nil
}

// UpsertVal inserts (or updates if it already exists) the value for a key
// with the given TTL.
func (bk *Backend) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return trace.Wrap(err)
	}
	defer b.Close()

	// Update the item in the bucket.
	b.updateItem(key, val, ttl)

	return nil
}

// UpsertItems inserts (or updates if it already exists) all passed in
// backend.Items with the given TTL.
func (bk *Backend) UpsertItems(bucket []string, newItems []backend.Item) error {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_CREATE|os.O_RDWR)
	if err != nil {
		return trace.Wrap(err)
	}
	defer b.Close()

	// Update items in bucket.
	for _, e := range newItems {
		b.updateItem(e.Key, e.Value, e.TTL)
	}

	return nil
}

// GetVal return a value for a given key in the bucket
func (bk *Backend) GetVal(bucket []string, key string) ([]byte, error) {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_RDWR)
	if err != nil {
		// GetVal on a bucket needs to return trace.BadParameter. If opening the
		// bucket failed a partial match up to a bucket may still exist. To support
		// returning trace.BadParameter in this situation, loop over all keys in the
		// backend and see if any match the prefix. If any match the prefix return
		// trace.BadParameter, otherwise return the original error. This is
		// consistent with our DynamoDB implementation.
		files, er := ioutil.ReadDir(path.Join(bk.rootDir))
		if er != nil {
			return nil, trace.ConvertSystemError(er)
		}
		var matched int
		for _, fi := range files {
			pathToBucket := bk.pathToBucket(fi.Name())
			fullBucket := append(bucket, key)
			bucketPrefix := bk.flatten(fullBucket)

			// Prefix matched, for example if pathToBucket is "/foo/bar/baz" and
			// bucketPrefix is "/foo/bar".
			if strings.HasPrefix(pathToBucket, bucketPrefix) {
				matched = matched + 1
			}
		}
		if matched > 0 {
			return nil, trace.BadParameter("%v is not a valid key", key)
		}
		return nil, trace.ConvertSystemError(err)
	}
	defer b.Close()

	// If the key does not exist, return trace.NotFound right away.
	item, ok := b.getItem(key)
	if !ok {
		return nil, trace.NotFound("key %q is not found", key)
	}

	// If the key is expired, remove it from the bucket and write it out and exit.
	if bk.isExpired(item) {
		b.deleteItem(key)

		return nil, trace.NotFound("key %q is not found", key)
	}

	return item.Value, nil
}

// CompareAndSwapVal compares and swap values in atomic operation
func (bk *Backend) CompareAndSwapVal(bucket []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_CREATE|os.O_RDWR)
	if err != nil {
		er := trace.ConvertSystemError(err)
		if trace.IsNotFound(er) {
			return trace.CompareFailed("%v/%v did not match expected value", bucket, key)
		}
		return trace.Wrap(er)
	}
	defer b.Close()

	// Read in existing key. If it does not exist, is expired, or does not
	// match, return trace.CompareFailed.
	oldItem, ok := b.getItem(key)
	if !ok {
		return trace.CompareFailed("%v/%v did not match expected value", bucket, key)
	}
	if bk.isExpired(oldItem) {
		return trace.CompareFailed("%v/%v did not match expected value", bucket, key)
	}
	if bytes.Compare(oldItem.Value, prevVal) != 0 {
		return trace.CompareFailed("%v/%v did not match expected value", bucket, key)
	}

	// The compare was successful, update the item.
	b.updateItem(key, val, ttl)

	return nil
}

// DeleteKey deletes a key in a bucket.
func (bk *Backend) DeleteKey(bucket []string, key string) error {
	// Open the bucket to work on the items.
	b, err := bk.openBucket(bk.flatten(bucket), os.O_RDWR)
	if err != nil {
		return trace.Wrap(err)
	}
	defer b.Close()

	// If the key doesn't exist, return trace.NotFound.
	_, ok := b.getItem(key)
	if !ok {
		return trace.NotFound("key %v not found", key)
	}

	// Otherwise, delete key.
	b.deleteItem(key)

	return nil
}

// DeleteBucket deletes the bucket by a given path.
func (bk *Backend) DeleteBucket(parent []string, bucket string) error {
	fullBucket := append(parent, bucket)

	err := os.Remove(bk.flatten(fullBucket))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// AcquireLock grabs a lock that will be released automatically in TTL.
func (bk *Backend) AcquireLock(token string, ttl time.Duration) (err error) {
	bk.log.Debugf("AcquireLock(%s)", token)

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
			bk.Clock().Sleep(250 * time.Millisecond)
			continue
		}
		return trace.ConvertSystemError(err)
	}

	return nil
}

// ReleaseLock forces lock release before TTL.
func (bk *Backend) ReleaseLock(token string) (err error) {
	bk.log.Debugf("ReleaseLock(%s)", token)

	if err = bk.DeleteKey([]string{locksBucket}, token); err != nil {
		if !os.IsNotExist(err) {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// pathToBucket prepends the root directory to the bucket returning the full
// path to the bucket on the filesystem.
func (bk *Backend) pathToBucket(bucket string) string {
	return filepath.Join(bk.rootDir, bucket)
}

// flatten takes a bucket and flattens it (URL encodes) and prepends the root
// directory returning the full path to the bucket on the filesystem.
func (bk *Backend) flatten(bucket []string) string {
	// Convert ["foo", "bar"] to "foo/bar"
	raw := filepath.Join(bucket...)

	// URL encode bucket from "foo/bar" to "foo%2Fbar".
	flat := url.QueryEscape(raw)

	return filepath.Join(bk.rootDir, flat)
}

// isExpired checks if the bucket item is expired or not.
func (bk *Backend) isExpired(bv bucketItem) bool {
	if bv.ExpiryTime.IsZero() {
		return false
	}
	return bk.Clock().Now().After(bv.ExpiryTime)
}

// bucket contains a set of keys that map to values and a TTL.
type bucket struct {
	// backend is the underlying data store.
	backend *Backend

	// file is the underlying file that the bucket represents.
	file *os.File

	// items is a set of key/value pairs that this bucket holds.
	items map[string]bucketItem

	// itemsUpdated is used to control if the items have been updated and should
	// be written out to disk again.
	itemsUpdated bool
}

// bucketItem is the "Value" part of a key/value pair.
type bucketItem struct {
	// Value is content of the key.
	Value []byte `json:"value"`

	// ExpiryTime is when this value will expire.
	ExpiryTime time.Time `json:"expiry,omitempty"`
}

// openBucket will open a file, lock it, and then read in all the items in
// the bucket.
func (bk *Backend) openBucket(prefix string, openFlag int) (*bucket, error) {
	// Open bucket with requested flags.
	file, err := os.OpenFile(prefix, openFlag, defaultFileMode)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Lock the bucket so no one else can access it.
	if err := utils.FSWriteLock(file); err != nil {
		return nil, trace.Wrap(err)
	}

	// Read in all items from the bucket.
	items, err := readBucket(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &bucket{
		backend: bk,
		items:   items,
		file:    file,
	}, nil
}

func (b *bucket) getItem(key string) (bucketItem, bool) {
	item, ok := b.items[key]
	return item, ok
}

func (b *bucket) deleteItem(key string) {
	delete(b.items, key)
	b.itemsUpdated = true
}

func (b *bucket) updateItem(key string, value []byte, ttl time.Duration) {
	item := bucketItem{
		Value: value,
	}
	if ttl != backend.Forever {
		item.ExpiryTime = b.backend.Clock().Now().Add(ttl)
	}

	b.items[key] = item
	b.itemsUpdated = true
}

// Close will write out items (if requested), unlock file, and close it.
func (b *bucket) Close() error {
	var err error

	// If the items were updated, write them out to disk.
	if b.itemsUpdated {
		err = writeBucket(b.file, b.items)
		if err != nil {
			b.backend.log.Warnf("Unable to update keys in %v: %v.", b.file.Name(), err)
		}
	}

	err = utils.FSUnlock(b.file)
	if err != nil {
		b.backend.log.Warnf("Unable to unlock file: %v.", err)
	}

	err = b.file.Close()
	if err != nil {
		b.backend.log.Warnf("Unable to close file: %v.", err)
	}

	return nil
}

// readBucket will read in the bucket and return a map of keys. The second return
// value returns true to false to indicate if the file was empty or not.
func readBucket(f *os.File) (map[string]bucketItem, error) {
	// If the file is empty, return an empty bucket.
	ok, err := isEmpty(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ok {
		return map[string]bucketItem{}, nil
	}

	// The file is not empty, read it into a map.
	var items map[string]bucketItem
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	err = json.Unmarshal(bytes, &items)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return items, nil
}

// writeBucket will truncate the file and write out the items to the file f.
func writeBucket(f *os.File, items map[string]bucketItem) error {
	// Marshal items to disk format.
	bytes, err := json.Marshal(items)
	if err != nil {
		return trace.Wrap(err)
	}

	// Truncate the file.
	if _, err := f.Seek(0, 0); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := f.Truncate(0); err != nil {
		return trace.ConvertSystemError(err)
	}

	// Write out the contents to disk.
	n, err := f.Write(bytes)
	if err == nil && n < len(bytes) {
		return trace.Wrap(io.ErrShortWrite)
	}

	return nil
}

// isEmpty checks if the file is empty or not.
func isEmpty(f *os.File) (bool, error) {
	fi, err := f.Stat()
	if err != nil {
		return false, trace.Wrap(err)
	}

	if fi.Size() > 0 {
		return false, nil
	}

	return true, nil
}

// suffix returns the first bucket after where pathToBucket and bucketPrefix
// differ.  For example, if pathToBucket is "/roles/admin/params" and
// bucketPrefix is "/roles", then "admin" is returned.
func suffix(pathToBucket string, bucketPrefix string) (string, error) {
	full, err := url.QueryUnescape(pathToBucket)
	if err != nil {
		return "", trace.Wrap(err)
	}
	prefix, err := url.QueryUnescape(bucketPrefix)
	if err != nil {
		return "", trace.Wrap(err)
	}

	remain := full[len(prefix)+1:]
	if remain == "" {
		return "", trace.BadParameter("unable to split %v", remain)
	}

	vals := strings.Split(remain, string(filepath.Separator))
	if len(vals) == 0 {
		return "", trace.BadParameter("unable to split %v", remain)
	}

	return vals[0], nil
}

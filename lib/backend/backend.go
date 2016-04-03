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

// Package backend represents interface for accessing local or remote storage
package backend

import (
	"time"
)

// Forever means that object TTL will not expire unless deleted
const Forever time.Duration = 0

// Backend implements abstraction over local or remote storage backend
//
// Storage is modeled after BoltDB:
//  * bucket is a slice []string{"a", "b"}
//  * buckets contain key value pairs
//
type Backend interface {
	// GetKeys returns a list of keys for a given path
	GetKeys(bucket []string) ([]string, error)
	// CreateVal creates value with a given TTL and key in the bucket
	// if the value already exists, returns AlreadyExistsError
	CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error
	// TouchVal updates the TTL of the key without changing the value
	TouchVal(bucket []string, key string, ttl time.Duration) error
	// UpsertVal updates or inserts value with a given TTL into a bucket
	// ForeverTTL for no TTL
	UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error
	// GetVal return a value for a given key in the bucket
	GetVal(path []string, key string) ([]byte, error)
	// GetValAndTTL returns value and TTL for a key in bucket
	GetValAndTTL(bucket []string, key string) ([]byte, time.Duration, error)
	// DeleteKey deletes a key in a bucket
	DeleteKey(bucket []string, key string) error
	// DeleteBucket deletes the bucket by a given path
	DeleteBucket(path []string, bkt string) error
	// AcquireLock grabs a lock that will be released automatically in TTL
	AcquireLock(token string, ttl time.Duration) error
	// ReleaseLock forces lock release before TTL
	ReleaseLock(token string) error
	// CompareAndSwap implements compare ans swap operation for a key
	CompareAndSwap(bucket []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error)
	// Close releases the resources taken up by this backend
	Close() error
}

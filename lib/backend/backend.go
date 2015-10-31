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
// backend represents interface for accessing configuration backend for storing ACL lists and other settings
package backend

import (
	"time"
)

// TODO(klizhentas) this is bloated. Split it into little backend interfaces
// Backend represents configuration backend implementation for Teleport
type Backend interface {
	GetKeys(path []string) ([]string, error)
	UpsertVal(path []string, key string, val []byte, ttl time.Duration) error
	GetVal(path []string, key string) ([]byte, error)
	GetValAndTTL(path []string, key string) ([]byte, time.Duration, error)
	DeleteKey(path []string, key string) error
	DeleteBucket(path []string, bkt string) error
	// Grab a lock that will be released automatically in ttl time
	AcquireLock(token string, ttl time.Duration) error

	// Grab a lock that will be released automatically in ttl time
	ReleaseLock(token string) error

	CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error)
}

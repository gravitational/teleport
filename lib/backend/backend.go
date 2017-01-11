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

package backend

import (
	"time"

	"github.com/gravitational/trace"
)

// Forever means that object TTL will not expire unless deleted
const (
	Forever         time.Duration = 0
	MaxLockDuration time.Duration = time.Minute
)

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
	// if the value already exists, it must return trace.AlreadyExistsError
	CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error
	// UpsertVal updates or inserts value with a given TTL into a bucket
	// ForeverTTL for no TTL
	UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error
	// GetVal return a value for a given key in the bucket
	GetVal(path []string, key string) ([]byte, error)
	// DeleteKey deletes a key in a bucket
	DeleteKey(bucket []string, key string) error
	// DeleteBucket deletes the bucket by a given path
	DeleteBucket(path []string, bkt string) error
	// AcquireLock grabs a lock that will be released automatically in TTL
	AcquireLock(token string, ttl time.Duration) error
	// ReleaseLock forces lock release before TTL
	ReleaseLock(token string) error
	// Close releases the resources taken up by this backend
	Close() error
}

// backend.Params type defines a flexible unified back-end configuration API.
// It is just a map of key/value pairs which gets populated by `storage` section
// in Teleport YAML config.
type Params map[string]interface{}

// NewFunc type defines a function type which every backend must implement to
// instantiate itself
type NewFunc func(Params) (Backend, error)

type EtcdConfig struct {
	// Peers is a lsit of etcd peers,  valid only for etcd
	Peers []string `yaml:"peers,omitempty"`
	// Prefix is etcd key prefix, valid only for etcd
	Prefix string `yaml:"prefix,omitempty"`
	// TLSCertFile is a tls client cert file, used for etcd
	TLSCertFile string `yaml:"tls_cert_file,omitempty"`
	// TLSKeyFile is a file with TLS private key for client auth
	TLSKeyFile string `yaml:"tls_key_file,omitempty"`
	// TLSCAFile is a tls client trusted CA file, used for etcd
	TLSCAFile string `yaml:"tls_ca_file,omitempty"`
}

type DynamoConfig struct {
	// Region is where DynamoDB Table will be used to store k/v
	Region string `yaml:"region,omitempty"`
	// AWS AccessKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	AccessKey string `yaml:"access_key,omitempty"`
	// AWS SecretKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	SecretKey string `yaml:"secret_key,omitempty"`
	// Tablename where to store K/V in DynamoDB
	Tablename string `yaml:"table_name,omitempty"`
}

// Config is used for 'storage' config section. It's a combination of
// values for various backends: 'boltdb', 'etcd', 'filesystem' and 'dynamodb'
type Config struct {
	// Type can be "bolt" or "etcd" or "dynamodb"
	Type string `yaml:"type,omitempty"`

	EtcdConfig   `yaml:",inline"`
	DynamoConfig `yaml:",inline"`
}

// ValidateLockTTL helper allows all backends to validate lock TTL parameter
func ValidateLockTTL(ttl time.Duration) error {
	if ttl == Forever || ttl > MaxLockDuration {
		return trace.BadParameter("locks cannot exceed %v", MaxLockDuration)
	}
	return nil
}

// GetString returns a string value stored in Params map, or an empty string
// if nothing is found
func (p Params) GetString(key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

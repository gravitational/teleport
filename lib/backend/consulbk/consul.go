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
package consulbk

import (
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	consul "github.com/hashicorp/consul/api"
)

type bk struct {
	cfg    *Config
	client *consul.Client

	kv *consul.KV
	locks map[string]*consul.Lock
}

// Config represents JSON config for consul backend
type Config struct {
	// Passed into consul.Client
	Address    string `json:"address,omitempty"`
	Scheme     string `json:"scheme,omitempty"`
	Datacenter string `json:"datacenter,omitempty"`
	Token      string `json:"token,omitempty"`

	KeyPrefix string `json:"keyprefix,omitempty"`
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "consul"
}

func New(params backend.Params) (backend.Backend, error) {
	var err error
	if params == nil {
		return nil, trace.BadParameter("missing consul configuration")
	}

	// convert generic backend parameters structure to consul config:
	var cfg *Config
	if err = utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("invalid consul configuration", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	b := &bk{cfg: cfg}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

func (b *bk) Close() error {
	return nil
}

func (b *bk) reconnect() error {
	// TODO
	// Check config
	// create consul client

	b.kv = b.client.KV()
}

func (b *bk) key(keys ...string) string {
	return strings.Join(append([]string{b.cfg.KeyPrefix}, keys...), "/")
}

func (b *bk) GetKeys(bucket []string) ([]string, error) {
	keys, _, err := b.kv.Keys(b.key(bucket))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *bk) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	_, err 
}

func (b *bk) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
}

func (b *bk) DeleteKey(bucket []string, key string) error {
}

func (b *bk) DeleteBucket(path []string, bkt string) error {
}

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
	key := b.key("locks", token)
	lockOptions = &consul.LockOptions{
		Key:        key,
		SessionTTL: ttl,
	}

	lock = b.client.LockOpts(lockOptions)
	b.locks[key] = lock

	// TODO
	for {
		_, err := lock.Lock()
		err = convertErr(err)
		if err == nil {
			return nil
		}
		if err != nil {
			if !trace.IsCompareFailed(err) && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			time.Sleep(delayBetweenLockAttempts)
		}
	}
}

func (b *bk) ReleaseLock(token string) error {
	key := b.key("locks", token)
	return b.locks[key].Unlock()
}

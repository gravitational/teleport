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
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	consul "github.com/hashicorp/consul/api"
	"github.com/mailgun/timetools"
)

type bk struct {
	cfg    *Config
	client *consul.Client

	kv      *consul.KV
	session *consul.Session
	locks   map[string]*consul.Lock
	clock   timetools.TimeProvider
}

// Config represents JSON config for consul backend
type Config struct {
	Address    string `json:"address,omitempty"`
	Datacenter string `json:"datacenter,omitempty"`
	Key        string `json:"prefix,omitempty"`
	Scheme     string `json:"scheme,omitempty"`
	Token      string `json:"token,omitempty"`

	Insecure    bool   `json:"insecure,omitempty"`
	TLSCAFile   string `json:"tls_ca_file,omitempty"`
	TLSCertFile string `json:"tls_cert_file,omitempty"`
	TLSKeyFile  string `json:"tls_key_file,omitempty"`
}

// GetName returns the name of consul backend as it appears in 'storage/type' section
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

	b := &bk{
		cfg:   cfg,
		clock: &timetools.RealTime{},
	}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

func (cfg *Config) Validate() error {
	// if Key doesn't start with '/', add it.
	if strings.HasPrefix(cfg.Key, "/") {
		cfg.Key = "/" + cfg.Key
	}
	if cfg.Insecure == false {
		if cfg.TLSKeyFile == "" {
			return trace.BadParameter(`consul: missing "tls_key_file" setting`)
		}
		if cfg.TLSCertFile == "" {
			return trace.BadParameter(`consul: missing "tls_cert_file" setting`)
		}
		if cfg.TLSCAFile == "" {
			return trace.BadParameter(`consul: missing "tls_ca_file" setting`)
		}
	}
	return nil
}

func (b *bk) Close() error {
	return nil
}

func (b *bk) reconnect() error {
	config := consul.DefaultConfig()
	if config.Address != "" {
		config.Address = b.cfg.Address
	}
	if config.Datacenter != "" {
		config.Datacenter = b.cfg.Datacenter
	}
	if config.Scheme != "" {
		config.Scheme = b.cfg.Scheme
	}
	if config.Token != "" {
		config.Token = b.cfg.Token
	}

	config.HttpClient.Timeout = defaults.DefaultDialTimeout

	if b.cfg.Insecure == false {
		tlsConfig, err := consul.SetupTLSConfig(&consul.TLSConfig{
			Address:  b.cfg.Address,
			CAFile:   b.cfg.TLSCAFile,
			CertFile: b.cfg.TLSCertFile,
			KeyFile:  b.cfg.TLSKeyFile,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		config.HttpClient.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	var err error
	b.client, err = consul.NewClient(config)
	if err != nil {
		return trace.Wrap(err)
	}

	b.kv = b.client.KV()
	b.session = b.client.Session()

	return nil
}

func (b *bk) key(keys ...string) string {
	return strings.Join(append([]string{b.cfg.Key}, keys...), "/")
}

func (b *bk) GetKeys(bucket []string) ([]string, error) {
	keys, _, err := b.kv.Keys(b.key(bucket...), "", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *bk) CreateVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.GetVal(bucket, key)
	if !trace.IsNotFound(err) {
		return err
	}
	return b.UpsertVal(bucket, key, val, ttl)
}

func (b *bk) UpsertVal(bucket []string, key string, val []byte, ttl time.Duration) error {
	v := &value{Value: val}
	bytes, err := json.Marshal(v)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionStr, _, err := b.session.Create(&consul.SessionEntry{
		Behavior: "delete",
		TTL:      ttl.String(),
	}, nil)
	if err != nil {
		return trace.Wrap(convertErr(err))
	}

	kvpair := &consul.KVPair{
		Key:     b.key(append(bucket, key)...),
		Value:   bytes,
		Session: sessionStr,
	}

	_, err = b.kv.Put(kvpair, nil)
	if err != nil {
		return trace.Wrap(convertErr(err))
	}
	return nil
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	kvpair, _, err := b.kv.Get(b.key(append(path, key)...), nil)
	if err != nil {
		return nil, convertErr(err)
	}
	if kvpair == nil {
		return nil, trace.NotFound("%v: %v not found", path, key)
	}

	var v *value
	err = json.Unmarshal(kvpair.Value, &v)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return v.Value, nil
}

func (b *bk) DeleteKey(bucket []string, key string) error {
	_, err := b.kv.Delete(b.key(append(bucket, key)...), nil)
	if err != nil {
		return convertErr(err)
	}
	return nil
}

func (b *bk) DeleteBucket(path []string, bkt string) error {
	_, err := b.kv.DeleteTree(b.key(append(path, bkt)...), nil)
	if err != nil {
		return convertErr(err)
	}
	return nil
}

const delayBetweenLockAttempts = 100 * time.Millisecond

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
	key := b.key("locks", token)
	lockOptions := &consul.LockOptions{
		Key:        key,
		SessionTTL: ttl.String(),
	}

	lock, err := b.client.LockOpts(lockOptions)
	if err != nil {
		return convertErr(err)
	}
	b.locks[key] = lock

	stopCh := make(chan struct{}, 1)
	for {
		_, err := lock.Lock(stopCh)
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
	return convertErr(b.locks[key].Unlock())
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}

	if e == consul.ErrLockNotHeld {
		return trace.NotFound(e.Error())
	} else if e == consul.ErrLockConflict {
		return trace.AlreadyExists(e.Error())
	}

	return e
}

type value struct {
	Value []byte `json:"val"`
}

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

// Package etcdbk implements Etcd powered backend
package etcdbk

import (
	"encoding/base64"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/context"
)

type bk struct {
	nodes []string

	cfg     *Config
	etcdKey string
	client  client.Client
	api     client.KeysAPI
	cancelC chan bool
	stopC   chan bool
	clock   clockwork.Clock
}

// Config represents JSON config for etcd backend
type Config struct {
	Nodes       []string `json:"peers,omitempty"`
	Key         string   `json:"prefix,omitempty"`
	TLSKeyFile  string   `json:"tls_key_file,omitempty"`
	TLSCertFile string   `json:"tls_cert_file,omitempty"`
	TLSCAFile   string   `json:"tls_ca_file,omitempty"`
	Insecure    bool     `json:"insecure,omitempty"`
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "etcd"
}

// New returns new instance of Etcd-powered backend
func New(params backend.Params) (backend.Backend, error) {
	var err error
	if params == nil {
		return nil, trace.BadParameter("missing etcd configuration")
	}

	// convert generic backend parameters structure to etcd config:
	var cfg *Config
	if err = utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("invalid etcd configuration", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &bk{
		cfg:     cfg,
		nodes:   cfg.Nodes,
		etcdKey: cfg.Key,
		cancelC: make(chan bool, 1),
		stopC:   make(chan bool, 1),
		clock:   clockwork.NewRealClock(),
	}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap backend in a input sanitizer and return it.
	return backend.NewSanitizer(b), nil
}

// Validate checks if all the parameters are present/valid
func (cfg *Config) Validate() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`etcd: missing "prefix" setting`)
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`etcd: missing "peers" setting`)
	}
	if cfg.Insecure == false {
		if cfg.TLSKeyFile == "" {
			return trace.BadParameter(`etcd: missing "tls_key_file" setting`)
		}
		if cfg.TLSCertFile == "" {
			return trace.BadParameter(`etcd: missing "tls_cert_file" setting`)
		}
		if cfg.TLSCAFile == "" {
			return trace.BadParameter(`etcd: missing "tls_ca_file" setting`)
		}
	}
	return nil
}

func (b *bk) Clock() clockwork.Clock {
	return b.clock
}

func (b *bk) Close() error {
	return nil
}

func (b *bk) key(keys ...string) string {
	return strings.Join(append([]string{b.etcdKey}, keys...), "/")
}

func (b *bk) reconnect() error {
	tlsInfo := transport.TLSInfo{
		CAFile:   b.cfg.TLSCAFile,
		CertFile: b.cfg.TLSCertFile,
		KeyFile:  b.cfg.TLSKeyFile,
	}
	tr, err := transport.NewTransport(tlsInfo, defaults.DefaultDialTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := client.New(client.Config{
		Endpoints:               b.nodes,
		Transport:               tr,
		HeaderTimeoutPerRequest: defaults.ReadHeadersTimeout,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.client = clt
	b.api = client.NewKeysAPI(b.client)

	return nil
}

// GetItems fetches keys and values and returns them to the caller.
func (b *bk) GetItems(path []string) ([]backend.Item, error) {
	items, err := b.getItems(b.key(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return items, nil
}

// GetKeys fetches keys (and values) but only returns keys to the caller.
func (b *bk) GetKeys(path []string) ([]string, error) {
	items, err := b.getItems(b.key(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert from []backend.Item to []string and return keys.
	keys := make([]string, len(items))
	for i, e := range items {
		keys[i] = e.Key
	}

	return keys, nil
}

func (b *bk) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.api.Set(
		context.Background(),
		b.key(append(path, key)...), base64.StdEncoding.EncodeToString(val),
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
	return trace.Wrap(convertErr(err))
}

// CompareAndSwapVal compares and swap values in atomic operation,
// succeeds if prevVal matches the value stored in the databases,
// requires prevVal as a non-empty value. Returns trace.CompareFailed
// in case if value did not match
func (b *bk) CompareAndSwapVal(path []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	if len(prevVal) == 0 {
		return trace.BadParameter("missing prevVal parameter, to atomically create item, use CreateVal method")
	}
	encodedPrev := base64.StdEncoding.EncodeToString(prevVal)
	_, err := b.api.Set(
		context.Background(),
		b.key(append(path, key)...), base64.StdEncoding.EncodeToString(val),
		&client.SetOptions{PrevValue: encodedPrev, PrevExist: client.PrevExist, TTL: ttl})
	err = convertErr(err)
	if trace.IsNotFound(err) {
		return trace.CompareFailed(err.Error())
	}
	return trace.Wrap(err)
}

// maxOptimisticAttempts is the number of attempts optimistic locking
const maxOptimisticAttempts = 5

func (bk *bk) UpsertItems(bucket []string, items []backend.Item) error {
	return trace.BadParameter("not implemented")
}

func (b *bk) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.api.Set(
		context.Background(),
		b.key(append(path, key)...), base64.StdEncoding.EncodeToString(val), &client.SetOptions{TTL: ttl})
	return convertErr(err)
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	re, err := b.api.Get(context.Background(), b.key(append(path, key)...), nil)
	if err != nil {
		return nil, convertErr(err)
	}
	if re.Node.Dir {
		return nil, trace.BadParameter("'%v': trying to get value of bucket", key)
	}
	value, err := base64.StdEncoding.DecodeString(re.Node.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return value, nil
}

func (b *bk) DeleteKey(path []string, key string) error {
	_, err := b.api.Delete(context.Background(), b.key(append(path, key)...), nil)
	return convertErr(err)
}

func (b *bk) DeleteBucket(path []string, key string) error {
	_, err := b.api.Delete(context.Background(), b.key(append(path, key)...), &client.DeleteOptions{Dir: true, Recursive: true})
	return convertErr(err)
}

const delayBetweenLockAttempts = 100 * time.Millisecond

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
	for {
		_, err := b.api.Set(
			context.Background(),
			b.key("locks", token), "lock", &client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
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
	_, err := b.api.Delete(context.Background(), b.key("locks", token), nil)
	return convertErr(err)
}

// getItems fetches keys and values and returns them to the caller.
func (b *bk) getItems(path string) ([]backend.Item, error) {
	var vals []backend.Item

	re, err := b.api.Get(context.Background(), path, nil)
	if er := convertErr(err); er != nil {
		if trace.IsNotFound(er) {
			return vals, nil
		}
		return nil, trace.Wrap(er)
	}
	if !isDir(re.Node) {
		return nil, trace.BadParameter("'%v': expected directory", path)
	}

	// Convert etcd response of *client.Response to backend.Item.
	for _, n := range re.Node.Nodes {
		valueBytes, err := base64.StdEncoding.DecodeString(n.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		vals = append(vals, backend.Item{
			Key:   suffix(n.Key),
			Value: valueBytes,
		})
	}

	// Sort and return results.
	sort.Slice(vals, func(i, j int) bool {
		return vals[i].Key < vals[j].Key
	})

	return vals, nil
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case client.Error:
		switch err.Code {
		case client.ErrorCodeKeyNotFound:
			return trace.NotFound(err.Error())
		case client.ErrorCodeNotFile:
			return trace.BadParameter(err.Error())
		case client.ErrorCodeNodeExist:
			return trace.AlreadyExists(err.Error())
		case client.ErrorCodeTestFailed:
			return trace.CompareFailed(err.Error())
		}
	}
	return e
}

func isDir(n *client.Node) bool {
	return n != nil && n.Dir == true
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

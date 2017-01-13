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
}

// Config represents JSON config for etcd backend
type Config struct {
	Nodes       []string `json:"peers,omitempty"`
	Key         string   `json:"prefix,omitempty"`
	TLSKeyFile  string   `json:"tls_key_file,omitempty"`
	TLSCertFile string   `json:"tls_cert_file,omitempty"`
	TLSCAFile   string   `json:"tls_ca_file,omitempty"`
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "etcd"
}

// New returns new instance of Etcd-powered backend
func New(params backend.Params) (backend.Backend, error) {
	var err error

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
	}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// Validate checks if all the parameters are present/valid
func (cfg *Config) Validate() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`Key: supply a valid root key for Teleport data`)
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`Nodes: please supply a valid dictionary, e.g. {"nodes": ["http://localhost:4001]}`)
	}
	if cfg.TLSKeyFile == "" {
		return trace.BadParameter(`TLSKeyFile: please supply a path to TLS private key file`)
	}
	if cfg.TLSCertFile == "" {
		return trace.BadParameter(`TLSCertFile: please supply a path to TLS certificate file`)
	}
	return nil
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
		HeaderTimeoutPerRequest: defaults.DefaultReadHeadersTimeout,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.client = clt
	b.api = client.NewKeysAPI(b.client)

	return nil
}

func (b *bk) GetKeys(path []string) ([]string, error) {
	keys, err := b.getKeys(b.key(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *bk) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.api.Set(
		context.Background(),
		b.key(append(path, key)...), base64.StdEncoding.EncodeToString(val),
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
	return trace.Wrap(convertErr(err))
}

// maxOptimisticAttempts is the number of attempts optimistic locking
const maxOptimisticAttempts = 5

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

func (b *bk) getKeys(key string) ([]string, error) {
	var vals []string
	re, err := b.api.Get(context.Background(), key, nil)
	err = convertErr(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return vals, nil
		}
		return nil, trace.Wrap(err)
	}
	if !isDir(re.Node) {
		return nil, trace.BadParameter("'%v': expected directory", key)
	}
	for _, n := range re.Node.Nodes {
		vals = append(vals, suffix(n.Key))
	}
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

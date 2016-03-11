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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"golang.org/x/net/context"
)

// Option is a functional option parameter that can be passed to the backend
type Option func(b *bk) error

type bk struct {
	nodes []string

	etcdKey string
	client  client.Client
	api     client.KeysAPI
	cancelC chan bool
	stopC   chan bool
}

// New returns new instance of Etcd-powered backend
func New(nodes []string, key string, options ...Option) (backend.Backend, error) {
	if len(nodes) == 0 {
		return nil, trace.Wrap(
			teleport.BadParameter("nodes",
				"empty list of etcd nodes, supply at least one node"))
	}
	if len(key) == 0 {
		return nil, trace.Wrap(
			teleport.BadParameter("key",
				"provide key for this backend"))
	}
	b := &bk{
		nodes:   nodes,
		etcdKey: key,
		cancelC: make(chan bool, 1),
		stopC:   make(chan bool, 1),
	}
	for _, o := range options {
		if err := o(b); err != nil {
			return nil, err
		}
	}
	if err := b.reconnect(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *bk) Close() error {
	return nil
}

func (b *bk) key(keys ...string) string {
	return strings.Join(append([]string{b.etcdKey}, keys...), "/")
}

func (b *bk) reconnect() error {
	clt, err := client.New(client.Config{
		Endpoints:               b.nodes,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
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
		b.key(append(path, key)...), string(val),
		&client.SetOptions{PrevExist: client.PrevNoExist, TTL: ttl})
	return trace.Wrap(convertErr(err))
}

// maxOptimisticAttempts is the number of attempts optimistic locking
const maxOptimisticAttempts = 5

func (b *bk) TouchVal(path []string, key string, ttl time.Duration) error {
	var err error
	var re *client.Response
	for i := 0; i < maxOptimisticAttempts; i++ {
		re, err = b.api.Get(context.Background(), key, nil)
		if err != nil {
			return trace.Wrap(convertErr(err))
		}
		_, err = b.api.Set(
			context.Background(),
			b.key(append(path, key)...), string(re.Node.Value),
			&client.SetOptions{TTL: ttl, PrevValue: re.Node.Value, PrevExist: client.PrevExist})
		err = convertErr(err)
		if err == nil {
			return nil
		}
	}
	return trace.Wrap(err)
}

func (b *bk) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.api.Set(
		context.Background(),
		b.key(append(path, key)...), string(val), &client.SetOptions{TTL: ttl})
	return convertErr(err)
}

func (b *bk) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	var err error
	var re *client.Response
	if len(prevVal) != 0 {
		re, err = b.api.Set(
			context.Background(),
			b.key(append(path, key)...), string(val),
			&client.SetOptions{TTL: ttl, PrevValue: string(prevVal), PrevExist: client.PrevExist})
	} else {
		re, err = b.api.Set(
			context.Background(),
			b.key(append(path, key)...), string(val),
			&client.SetOptions{TTL: ttl, PrevExist: client.PrevNoExist})
	}
	err = convertErr(err)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if re.PrevNode != nil {
		return []byte(re.PrevNode.Value), nil
	}
	return nil, nil
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	re, err := b.api.Get(context.Background(), b.key(append(path, key)...), nil)
	if err != nil {
		return nil, convertErr(err)
	}
	if re.Node.Dir {
		return nil, trace.Wrap(teleport.BadParameter(key, "trying to get value of bucket"))
	}
	return []byte(re.Node.Value), nil
}

func (b *bk) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	re, err := b.api.Get(context.Background(), b.key(append(path, key)...), nil)
	if err != nil {
		return nil, 0, convertErr(err)
	}
	if re.Node.Dir {
		return nil, 0, trace.Wrap(
			teleport.BadParameter(key, "trying to get value of bucket"))
	}
	return []byte(re.Node.Value), time.Duration(re.Node.TTL) * time.Second, nil
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
			if !teleport.IsCompareFailed(err) && !teleport.IsAlreadyExists(err) {
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
		if teleport.IsNotFound(err) {
			return vals, nil
		}
		return nil, trace.Wrap(err)
	}
	if !isDir(re.Node) {
		return nil, trace.Wrap(teleport.BadParameter(key, "expected directory"))
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
			return &teleport.NotFoundError{Message: err.Error()}
		case client.ErrorCodeNotFile:
			return &teleport.BadParameterError{Err: err.Error()}
		case client.ErrorCodeNodeExist:
			return &teleport.AlreadyExistsError{Message: err.Error()}
		case client.ErrorCodeTestFailed:
			return &teleport.CompareFailedError{Message: err.Error()}
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

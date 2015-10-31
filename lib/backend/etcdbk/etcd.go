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
// etcdbk implements Etcd powered backend
package etcdbk

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/gravitational/teleport/lib/backend"
)

type BackendOption func(b *bk) error

func Consistency(v string) BackendOption {
	return func(b *bk) error {
		b.etcdConsistency = v
		return nil
	}
}

type bk struct {
	nodes []string

	etcdConsistency string
	etcdKey         string
	client          *etcd.Client
	cancelC         chan bool
	stopC           chan bool
}

func New(nodes []string, etcdKey string, options ...BackendOption) (backend.Backend, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty list of etcd nodes, supply at least one node")
	}

	if len(etcdKey) == 0 {
		return nil, fmt.Errorf("supply a valid etcd key")
	}

	b := &bk{
		nodes:   nodes,
		etcdKey: etcdKey,
		cancelC: make(chan bool, 1),
		stopC:   make(chan bool, 1),
	}
	b.etcdConsistency = etcd.WEAK_CONSISTENCY
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
	b.client = etcd.NewClient(b.nodes)
	return nil
}

func (b *bk) GetKeys(path []string) ([]string, error) {
	keys, err := b.getKeys(b.key(path...))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.StringSlice(keys))
	return keys, nil
}

func (b *bk) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	_, err := b.client.Set(
		b.key(append(path, key)...), string(val), uint64(ttl/time.Second))
	return convertErr(err)
}

func (b *bk) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
	var err error
	var resp *etcd.Response
	if len(prevVal) != 0 {
		resp, err = b.client.CompareAndSwap(
			b.key(append(path, key)...),
			string(val),
			uint64(ttl/time.Second),
			string(prevVal),
			0,
		)
	} else {
		resp, err = b.client.Create(
			b.key(append(path, key)...),
			string(val),
			uint64(ttl/time.Second),
		)
	}

	err = convertErr(err)

	if err != nil && !teleport.IsNotFound(err) {
		var e error
		resp, e = b.client.Get(
			b.key(append(path, key)...),
			false, false,
		)
		if e != nil {
			return nil, e
		}
	}

	prevValStr := ""
	if len(prevVal) > 0 {
		prevValStr = string(prevVal)
	}
	if err != nil {
		if teleport.IsCompareFailed(err) {
			e := &teleport.CompareFailedError{
				Message: "Expected '" + prevValStr + "', obtained '" + resp.Node.Value + "'",
			}
			return []byte(resp.Node.Value), e
		}
		if teleport.IsNotFound(err) {
			e := &teleport.CompareFailedError{
				Message: "Expected '" + prevValStr + "', obtained ''",
			}
			return []byte{}, e
		}
		if teleport.IsAlredyExists(err) {
			e := &teleport.CompareFailedError{
				Message: "Expected '', obtained '" + resp.Node.Value + "'",
			}
			return []byte(resp.Node.Value), e
		}
		return nil, convertErr(err)
	}
	if resp.PrevNode != nil {
		return []byte(resp.PrevNode.Value), nil
	} else {
		return nil, nil
	}
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	re, err := b.client.Get(b.key(append(path, key)...), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	if re.Node.Dir {
		return nil, &teleport.NotFoundError{Message: "Trying to get value of bucket"}
	}
	return []byte(re.Node.Value), nil
}

func (b *bk) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	re, err := b.client.Get(b.key(append(path, key)...), false, false)
	if err != nil {
		return nil, 0, convertErr(err)
	}
	if re.Node.Dir {
		return nil, 0, &teleport.NotFoundError{Message: "Trying to get value of bucket"}
	}
	return []byte(re.Node.Value), time.Duration(re.Node.TTL) * time.Second, nil
}

func (b *bk) DeleteKey(path []string, key string) error {
	_, err := b.client.Delete(b.key(append(path, key)...), false)
	return convertErr(err)
}

func (b *bk) DeleteBucket(path []string, key string) error {
	_, err := b.client.Delete(b.key(append(path, key)...), true)
	return convertErr(err)
}

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
	for {
		_, e := b.client.Create(
			b.key("locks", token), "lock", uint64(ttl/time.Second))
		if e == nil {
			return nil
		}
		switch err := e.(type) {
		case *etcd.EtcdError:
			switch err.ErrorCode {
			case 100:
				return &teleport.NotFoundError{Message: err.Error()}
			case 105:
				time.Sleep(100 * time.Millisecond)
			default:
				return e
			}
		default:
			return e
		}
	}
}

func (b *bk) ReleaseLock(token string) error {
	_, err := b.client.Delete(b.key("locks", token), false)
	return convertErr(err)
}

func (b *bk) getKeys(key string) ([]string, error) {
	vals := []string{}
	re, err := b.client.Get(key, true, false)
	if err != nil {
		if notFound(err) {
			return vals, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return nil, fmt.Errorf("expected directory")
	}
	for _, n := range re.Node.Nodes {
		vals = append(vals, suffix(n.Key))
	}
	return vals, nil
}

func notFound(e error) bool {
	err, ok := e.(*etcd.EtcdError)
	return ok && err.ErrorCode == 100
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case *etcd.EtcdError:
		switch err.ErrorCode {
		case 100:
			return &teleport.NotFoundError{Message: err.Error()}
		case 105:
			return &teleport.AlreadyExistsError{Message: err.Error()}
		case 101:
			return &teleport.CompareFailedError{Message: err.Error()}
		}
	}
	return e
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

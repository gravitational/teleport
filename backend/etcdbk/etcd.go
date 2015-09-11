// etcdbk implements Etcd powered backend
package etcdbk

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/gravitational/teleport/backend"
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

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	re, err := b.client.Get(b.key(append(path, key)...), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	return []byte(re.Node.Value), nil
}

func (b *bk) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	re, err := b.client.Get(b.key(append(path, key)...), false, false)
	if err != nil {
		return nil, 0, convertErr(err)
	}
	return []byte(re.Node.Value), time.Duration(re.Node.TTL) * time.Second, nil
}

func (b *bk) DeleteKey(path []string, key string) error {
	_, err := b.client.Delete(b.key(append(path, key)...), false)
	return convertErr(err)
}

func (b *bk) DeleteBucket(path []string, key string) error {
	_, err := b.client.Delete(b.key(append(path, key)...), false)
	return convertErr(err)
}

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
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
			return &teleport.AlreadyExistsError{Message: err.Error()}
		}
	}
	return e
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

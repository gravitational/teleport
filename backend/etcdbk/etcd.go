// etcdbk implements Etcd powered backend
package etcdbk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func (b *bk) UpsertUserCA(a backend.CA) error {
	return b.upsertCA(UserCA, a)
}

func (b *bk) GetUserCA() (*backend.CA, error) {
	return b.getCA(UserCA)
}

func (b *bk) GetUserCAPub() ([]byte, error) {
	ca, err := b.GetUserCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

func (b *bk) UpsertHostCA(a backend.CA) error {
	return b.upsertCA(HostCA, a)
}

func (b *bk) GetHostCA() (*backend.CA, error) {
	return b.getCA(HostCA)
}

func (b *bk) GetHostCAPub() ([]byte, error) {
	ca, err := b.GetHostCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

// GetUsers  returns a list of users registered in the backend
func (b *bk) GetUsers() ([]string, error) {
	values := []string{}
	re, err := b.client.Get(b.key("users"), true, false)
	if err != nil {
		if notFound(err) {
			return values, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return values, nil
	}
	for _, sn := range re.Node.Nodes {
		values = append(values, suffix(sn.Key))
	}
	return values, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (b *bk) DeleteUser(user string) error {
	_, err := b.client.Delete(b.key("users", user), true)
	return convertErr(err)
}

func (b *bk) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	values := []backend.AuthorizedKey{}
	re, err := b.client.Get(b.key("users", user, "keys"), true, true)
	if err != nil {
		if notFound(err) {
			return values, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return values, nil
	}
	for _, sn := range re.Node.Nodes {
		if !isDir(sn) {
			values = append(values, backend.AuthorizedKey{ID: suffix(sn.Key), Value: []byte(sn.Value)})
		}
	}
	return values, nil
}

func (b *bk) UpsertUserKey(user string, key backend.AuthorizedKey, ttl time.Duration) error {
	_, err := b.client.Set(b.key("users", user, "keys", key.ID), string(key.Value), uint64(ttl/time.Second))
	return convertErr(err)
}

func (b *bk) DeleteUserKey(user, keyID string) error {
	_, err := b.client.Delete(b.key("users", user, "keys", keyID), true)
	return convertErr(err)
}

func (b *bk) GetServers() ([]backend.Server, error) {
	values := []backend.Server{}
	re, err := b.client.Get(b.key("servers"), true, true)
	if err != nil {
		if notFound(err) {
			return values, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return values, nil
	}
	for _, sn := range re.Node.Nodes {
		if !isDir(sn) {
			values = append(values, backend.Server{ID: suffix(sn.Key), Addr: sn.Value})
		}
	}
	return values, nil
}

func (b *bk) UpsertServer(s backend.Server, ttl time.Duration) error {
	_, err := b.client.Set(b.key("servers", s.ID), string(s.Addr), uint64(ttl/time.Second))
	return convertErr(err)
}

func (b *bk) UpsertPasswordHash(user string, hash []byte) error {
	bytes, err := json.Marshal(hash)
	if err != nil {
		return err
	}
	_, err = b.client.Set(b.key("users", user, "web", "pwd"), string(bytes), 0)
	return convertErr(err)
}

func (b *bk) GetPasswordHash(user string) ([]byte, error) {
	re, err := b.client.Get(b.key("users", user, "web", "pwd"), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	var hash []byte
	if err := json.Unmarshal([]byte(re.Node.Value), &hash); err != nil {
		return nil, err
	}
	return hash, nil
}

func (b *bk) UpsertWebSession(user, sid string, s backend.WebSession, ttl time.Duration) error {
	bytes, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = b.client.Set(b.key("users", user, "web", "sessions", sid), string(bytes), uint64(ttl/time.Second))
	return convertErr(err)
}

func (b *bk) GetWebSession(user, sid string) (*backend.WebSession, error) {
	re, err := b.client.Get(b.key("users", user, "web", "sessions", sid), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	var sess *backend.WebSession
	if err := json.Unmarshal([]byte(re.Node.Value), &sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (b *bk) GetWebSessions(user string) ([]backend.WebSession, error) {
	values := []backend.WebSession{}
	re, err := b.client.Get(b.key("users", user, "web", "sessions"), true, true)
	if err != nil {
		if notFound(err) {
			return values, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return values, nil
	}
	for _, sn := range re.Node.Nodes {
		if isDir(sn) {
			continue
		}
		var sess *backend.WebSession
		if err := json.Unmarshal([]byte(sn.Value), &sess); err != nil {
			return nil, err
		}
		values = append(values, *sess)
	}
	return values, nil
}

func (b *bk) DeleteWebSession(user, sid string) error {
	_, err := b.client.Delete(b.key("users", user, "web", "sessions", sid), true)
	return convertErr(err)
}

func (b *bk) UpsertWebTun(t backend.WebTun, ttl time.Duration) error {
	if t.Prefix == "" {
		return &backend.MissingParameterError{Param: "Prefix"}
	}
	bytes, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = b.client.Set(b.key("tunnels", "web", t.Prefix), string(bytes), uint64(ttl/time.Second))
	return err
}

func (b *bk) DeleteWebTun(prefix string) error {
	_, err := b.client.Delete(b.key("tunnels", "web", prefix), true)
	return convertErr(err)
}

func (b *bk) GetWebTun(prefix string) (*backend.WebTun, error) {
	re, err := b.client.Get(b.key("tunnels", "web", prefix), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	var tun *backend.WebTun
	if err := json.Unmarshal([]byte(re.Node.Value), &tun); err != nil {
		return nil, err
	}
	return tun, nil
}

func (b *bk) GetWebTuns() ([]backend.WebTun, error) {
	values := []backend.WebTun{}
	re, err := b.client.Get(b.key("tunnels", "web"), true, true)
	if err != nil {
		if notFound(err) {
			return values, nil
		}
		return nil, convertErr(err)
	}
	if !isDir(re.Node) {
		return values, nil
	}
	for _, sn := range re.Node.Nodes {
		if isDir(sn) {
			continue
		}
		var tun *backend.WebTun
		if err := json.Unmarshal([]byte(sn.Value), &tun); err != nil {
			return nil, err
		}
		tun.Prefix = suffix(sn.Key)
		values = append(values, *tun)
	}
	return values, nil
}

func (b *bk) upsertCA(id string, a backend.CA) error {
	out, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = b.client.Set(b.key("auth", id, "key"), string(out), 0)
	return convertErr(err)
}

func (b *bk) getCA(id string) (*backend.CA, error) {
	re, err := b.client.Get(b.key("auth", id, "key"), false, false)
	if err != nil {
		return nil, convertErr(err)
	}
	var ca *backend.CA
	if err := json.Unmarshal([]byte(re.Node.Value), &ca); err != nil {
		return nil, err
	}
	return ca, nil
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
		if err.ErrorCode == 100 {
			return &backend.NotFoundError{Message: err.Error()}
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

const (
	HostCA = "host"
	UserCA = "user"
)

/*
Copyright 2015-2018 Gravitational, Inc.

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

package etcdbk

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
	"gopkg.in/check.v1"
)

const (
	examplePrefix = "/teleport.secrets/"
	customPrefix  = "/custom/"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestEtcd(t *testing.T) { check.TestingT(t) }

type EtcdSuite struct {
	bk     *EtcdBackend
	suite  test.BackendSuite
	config backend.Params
}

var _ = check.Suite(&EtcdSuite{})

func (s *EtcdSuite) SetUpSuite(c *check.C) {
	// This config must match examples/etcd/teleport.yaml
	s.config = backend.Params{
		"peers":         []string{"https://127.0.0.1:2379"},
		"prefix":        examplePrefix,
		"tls_key_file":  "../../../examples/etcd/certs/client-key.pem",
		"tls_cert_file": "../../../examples/etcd/certs/client-cert.pem",
		"tls_ca_file":   "../../../examples/etcd/certs/ca-cert.pem",
	}

	clock := clockwork.NewFakeClock()
	newBackend := func() (backend.Backend, error) {
		bk, err := New(context.Background(), s.config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		bk.clock = clock
		return bk, nil
	}
	s.suite.NewBackend = newBackend
	s.suite.Clock = fakeClock{FakeClock: clock}
}

func (s *EtcdSuite) SetUpTest(c *check.C) {
	if !etcdTestEnabled() {
		c.Skip("This test requires etcd, start it with examples/etcd/start-etcd.sh and set TELEPORT_ETCD_TEST=yes")
	}
	// Initiate a backend with a registry
	b, err := s.suite.NewBackend()
	c.Assert(err, check.IsNil)
	s.bk = b.(*EtcdBackend)
	s.suite.B = s.bk

	// Clean up any pre-stored records for all used prefixes
	ctx := context.Background()
	_, err = s.bk.client.Delete(ctx, strings.TrimSuffix(examplePrefix, "/"), clientv3.WithPrefix())
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Delete(ctx, strings.TrimSuffix(customPrefix, "/"), clientv3.WithPrefix())
	c.Assert(err, check.IsNil)
}

func (s *EtcdSuite) TearDownTest(c *check.C) {
	if s.bk == nil {
		return
	}
	err := s.bk.Close()
	c.Assert(err, check.IsNil)
}

func (s *EtcdSuite) TestCRUD(c *check.C) {
	s.suite.CRUD(c)
}

func (s *EtcdSuite) TestRange(c *check.C) {
	s.suite.Range(c)
}

func (s *EtcdSuite) TestDeleteRange(c *check.C) {
	s.suite.DeleteRange(c)
}

func (s *EtcdSuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *EtcdSuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *EtcdSuite) TestKeepAlive(c *check.C) {
	s.suite.KeepAlive(c)
}

func (s *EtcdSuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *EtcdSuite) TestWatchersClose(c *check.C) {
	s.suite.WatchersClose(c)
}

func (s *EtcdSuite) TestLocking(c *check.C) {
	s.suite.Locking(c, s.bk)
}

func (s *EtcdSuite) TestPrefix(c *check.C) {
	s.bk.cfg.Key = customPrefix
	c.Assert(s.bk.cfg.Validate(), check.IsNil)

	var (
		ctx  = context.Background()
		item = backend.Item{
			Key:   []byte("/foo"),
			Value: []byte("bar"),
		}
	)

	// Item key starts with '/'.
	_, err := s.bk.Put(ctx, item)
	c.Assert(err, check.IsNil)

	wantKey := s.bk.cfg.Key + string(item.Key)
	s.assertKV(ctx, c, wantKey, string(item.Value))
	got, err := s.bk.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	item.ID = got.ID
	c.Assert(*got, check.DeepEquals, item)

	// Item key does not start with '/'.
	item = backend.Item{
		Key:   []byte("foo"),
		Value: []byte("bar"),
	}
	_, err = s.bk.Put(ctx, item)
	c.Assert(err, check.IsNil)

	wantKey = s.bk.cfg.Key + string(item.Key)
	s.assertKV(ctx, c, wantKey, string(item.Value))
	got, err = s.bk.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	item.ID = got.ID
	c.Assert(*got, check.DeepEquals, item)
}

func (s *EtcdSuite) assertKV(ctx context.Context, c *check.C, key, val string) {
	c.Logf("assert that key %q contains value %q", key, val)
	resp, err := s.bk.client.Get(ctx, key)
	c.Assert(err, check.IsNil)
	c.Assert(len(resp.Kvs), check.Equals, 1)
	c.Assert(string(resp.Kvs[0].Key), check.Equals, key)
	// Note: EtcdBackend stores all values base64-encoded.
	gotValue, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
	c.Assert(err, check.IsNil)
	c.Assert(string(gotValue), check.Equals, val)
}

// TestCompareAndSwapOversizedValue ensures that the backend reacts with a proper
// error message if client sends a message exceeding the configured size maximum
// See https://github.com/gravitational/teleport/issues/4786
func TestCompareAndSwapOversizedValue(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, start it with examples/etcd/start-etcd.sh and set TELEPORT_ETCD_TEST=yes")
	}
	// setup
	const maxClientMsgSize = 128
	bk, err := New(context.Background(), backend.Params{
		"peers":                          []string{"https://127.0.0.1:2379"},
		"prefix":                         "/teleport",
		"tls_key_file":                   "../../../examples/etcd/certs/client-key.pem",
		"tls_cert_file":                  "../../../examples/etcd/certs/client-cert.pem",
		"tls_ca_file":                    "../../../examples/etcd/certs/ca-cert.pem",
		"dial_timeout":                   500 * time.Millisecond,
		"etcd_max_client_msg_size_bytes": maxClientMsgSize,
	})
	require.NoError(t, err)
	defer bk.Close()
	prefix := test.MakePrefix()
	// Explicitly exceed the message size
	value := make([]byte, maxClientMsgSize+1)

	// verify
	_, err = bk.CompareAndSwap(context.Background(),
		backend.Item{Key: prefix("one"), Value: []byte("1")},
		backend.Item{Key: prefix("one"), Value: value},
	)
	require.True(t, trace.IsLimitExceeded(err))
	require.Regexp(t, ".*ResourceExhausted.*", err)
}

func etcdTestEnabled() bool {
	return os.Getenv("TELEPORT_ETCD_TEST") != ""
}

func (r fakeClock) Advance(d time.Duration) {
	// We cannot rewind time for etcd since it will not have any effect on the server
	// so we actually sleep in this case
	time.Sleep(d)
}

type fakeClock struct {
	clockwork.FakeClock
}

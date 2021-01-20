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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/google/go-cmp/cmp"
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

	newBackend := func() (backend.Backend, error) {
		return New(context.Background(), s.config)
	}
	s.suite.NewBackend = newBackend
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

func TestKeepAlives(t *testing.T) {
	bk := newBackend(t)
	defer bk.Close()

	watcher := bk.newWatcher(context.TODO(), t)
	defer watcher.Close()

	expiresAt := addSeconds(bk.clock.Now(), 2)
	item, lease := bk.addItem(context.TODO(), t, "key", "val1", expiresAt)

	bk.clock.Advance(1 * time.Second)

	// Move the expiration further in the future to avoid processing
	// skew and ensure the item is available when we delete it.
	// It does not affect the running time of the test
	updatedAt := addSeconds(bk.clock.Now(), 60)
	err := bk.KeepAlive(context.TODO(), lease, updatedAt)
	require.NoError(t, err)

	// Since the backend translates absolute expiration timestamp to a TTL
	// and collecting events takes arbitrary time, the expiration timestamps
	// on the collected events might have a slight skew
	events := collectEvents(t, watcher, 3)
	verifyEvents(t, events, []backend.Event{
		{Type: backend.OpInit, Item: backend.Item{}},
		{Type: backend.OpPut, Item: backend.Item{Key: bk.prefix("key"), Value: []byte("val1"), Expires: expiresAt}},
		{Type: backend.OpPut, Item: backend.Item{Key: bk.prefix("key"), Value: []byte("val1"), Expires: updatedAt}},
	})

	err = bk.Delete(context.TODO(), item.Key)
	require.NoError(t, err)

	_, err = bk.Get(context.TODO(), item.Key)
	require.IsType(t, trace.NotFound(""), err)

	// keep alive on deleted or expired object should fail
	err = bk.KeepAlive(context.TODO(), lease, updatedAt.Add(1*time.Second))
	require.IsType(t, trace.NotFound(""), err)
}

func newBackend(t *testing.T) pack {
	bk, err := New(context.Background(), backend.Params{
		"peers":         []string{"https://127.0.0.1:2379"},
		"prefix":        "/teleport",
		"tls_key_file":  "../../../examples/etcd/certs/client-key.pem",
		"tls_cert_file": "../../../examples/etcd/certs/client-cert.pem",
		"tls_ca_file":   "../../../examples/etcd/certs/ca-cert.pem",
		"dial_timeout":  500 * time.Millisecond,
	})
	require.NoError(t, err)
	clock := clockwork.NewFakeClock()
	bk.clock = clock
	return pack{
		EtcdBackend: bk,
		prefix:      test.MakePrefix(),
		clock:       clock,
	}
}

func (r pack) addItem(ctx context.Context, t *testing.T, key, value string, expires time.Time) (backend.Item, backend.Lease) {
	item := backend.Item{
		Key:     r.prefix(key),
		Value:   []byte(value),
		Expires: expires,
	}
	lease, err := r.Put(ctx, item)
	require.NoError(t, err)
	return item, *lease
}

func (r pack) newWatcher(ctx context.Context, t *testing.T) backend.Watcher {
	watcher, err := r.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{r.prefix("")}})
	require.NoError(t, err)
	return watcher
}

type pack struct {
	*EtcdBackend
	prefix func(key string) []byte
	clock  clockwork.FakeClock
}

func collectEvents(t *testing.T, w backend.Watcher, count int) (events []backend.Event) {
	for i := 0; i < count; i++ {
		select {
		case e := <-w.Events():
			events = append(events, e)
		case <-w.Done():
			t.Fatal("Watcher has unexpectedly closed.")
		}
	}
	return events
}

func verifyEvents(t *testing.T, obtained, expected []backend.Event) {
	verifyIDsIncreasing(t, obtained)
	verifyIDsNoDuplicates(t, obtained)
	verifyExpireTimestampsIncreasing(t, obtained, expected)
}

func verifyIDsIncreasing(t *testing.T, obtained []backend.Event) {
	sorted := make([]backend.Event, len(obtained))
	copy(sorted, obtained)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Item.ID < sorted[j].Item.ID
	})
	require.Empty(t, cmp.Diff(obtained, sorted))
}

func verifyIDsNoDuplicates(t *testing.T, obtained []backend.Event) {
	dedup := make(map[int64]struct{})
	for _, event := range obtained {
		dedup[event.Item.ID] = struct{}{}
	}
	var expectedIDs, obtainedIDs []int64
	for id := range dedup {
		expectedIDs = append(expectedIDs, id)
	}
	for _, event := range obtained {
		obtainedIDs = append(obtainedIDs, event.Item.ID)
	}
	sort.Slice(expectedIDs, func(i, j int) bool {
		return expectedIDs[i] < expectedIDs[j]
	})
	require.Empty(t, cmp.Diff(obtainedIDs, expectedIDs))
}

func verifyExpireTimestampsIncreasing(t *testing.T, obtained, expected []backend.Event) {
	require.Len(t, obtained, len(expected))
	for i := range expected {
		if obtained[i].Item.Expires.After(expected[i].Item.Expires) {
			t.Errorf("Expected %v >= %v",
				obtained[i].Item.Expires,
				expected[i].Item.Expires,
			)
		}
	}
}

// addSeconds adds seconds with a seconds precission
// always rounding up to the next second,
// because TTL engines are usually 1 second precision
func addSeconds(now time.Time, seconds int64) time.Time {
	return time.Unix(now.Unix()+seconds+1, 0)
}

func etcdTestEnabled() bool {
	return os.Getenv("TELEPORT_ETCD_TEST") != ""
}

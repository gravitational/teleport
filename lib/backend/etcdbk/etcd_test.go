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
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/stretchr/testify/require"
)

const (
	examplePrefix = "/teleport.secrets/"
	customPrefix  = "/custom/"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// commonEtcdParams holds the common etcd configuration for all tests.
var commonEtcdParams = backend.Params{
	"peers":         []string{"https://127.0.0.1:2379"},
	"prefix":        examplePrefix,
	"tls_key_file":  "../../../examples/etcd/certs/client-key.pem",
	"tls_cert_file": "../../../examples/etcd/certs/client-cert.pem",
	"tls_ca_file":   "../../../examples/etcd/certs/ca-cert.pem",
}

func TestEtcd(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, start it with examples/etcd/start-etcd.sh and set TELEPORT_ETCD_TEST=yes")
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		opts, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if opts.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// No need to check target backend - all Etcd backends create by this test
		// point to the same datastore.

		bk, err := New(context.Background(), commonEtcdParams)
		if err != nil {
			return nil, nil, err
		}

		// we can't fiddle with clocks inside the etcd client, so instead of creating
		// and returning a fake clock, we wrap the real clock used by the etcd client
		// in a FakeClock interface that sleeps instead of instantly advancing.
		sleepingClock := blockingFakeClock{bk.clock}

		return bk, sleepingClock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func TestPrefix(t *testing.T) {
	ctx := context.Background()

	// Given an etcd backend with a minimal configuration...
	unprefixedUut, err := New(context.Background(), commonEtcdParams)
	require.NoError(t, err)
	defer unprefixedUut.Close()

	// ...and an etcd backend configured to use a custom prefix
	cfg := make(backend.Params)
	for k, v := range commonEtcdParams {
		cfg[k] = v
	}
	cfg["prefix"] = customPrefix

	prefixedUut, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer prefixedUut.Close()

	// When I push an item with a key starting with "/" into etcd via the
	// _prefixed_ client...
	item := backend.Item{
		Key:   []byte("/foo"),
		Value: []byte("bar"),
	}
	_, err = prefixedUut.Put(ctx, item)
	require.NoError(t, err)

	// Expect that I can retrieve it from the _un_prefixed client by
	// manually prepending a prefix to the key and asking for it.
	wantKey := prefixedUut.cfg.Key + string(item.Key)
	requireKV(ctx, t, unprefixedUut, wantKey, string(item.Value))
	got, err := prefixedUut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)

	// When I push an item with a key that does _not_ start with a separator
	// char (i.e. "/") into etcd via the _prefixed_ client...
	item = backend.Item{
		Key:   []byte("foo"),
		Value: []byte("bar"),
	}
	_, err = prefixedUut.Put(ctx, item)
	require.NoError(t, err)

	// Expect, again, that I can retrieve it from the _un_prefixed client
	// by manually prepending a prefix to the key and asking for it.
	wantKey = prefixedUut.cfg.Key + string(item.Key)
	requireKV(ctx, t, unprefixedUut, wantKey, string(item.Value))
	got, err = prefixedUut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
}

func requireKV(ctx context.Context, t *testing.T, bk *EtcdBackend, key, val string) {
	t.Logf("assert that key %q contains value %q", key, val)

	resp, err := bk.client.Get(ctx, key)
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	require.Equal(t, key, string(resp.Kvs[0].Key))

	// Note: EtcdBackend stores all values base64-encoded.
	gotValue, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
	require.NoError(t, err)
	require.Equal(t, val, string(gotValue))
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

func (r blockingFakeClock) Advance(d time.Duration) {
	if d < 0 {
		panic("Invalid argument, negative duration")
	}

	// We cannot rewind time for etcd since it will not have any effect on the server
	// so we actually sleep in this case
	time.Sleep(d)
}

func (r blockingFakeClock) BlockUntil(int) {
	panic("Not implemented")
}

type blockingFakeClock struct {
	clockwork.Clock
}

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package etcdbk

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
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
	"peers":         []string{etcdTestEndpoint()},
	"prefix":        examplePrefix,
	"tls_key_file":  "../../../fixtures/etcdcerts/client-key.pem",
	"tls_cert_file": "../../../fixtures/etcdcerts/client-cert.pem",
	"tls_ca_file":   "../../../fixtures/etcdcerts/ca-cert.pem",
}

var commonEtcdOptions = []Option{
	LeaseBucket(time.Second), // tests are more picky about expiry granularity
}

func TestEtcd(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		opts, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if opts.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// No need to check target backend - all Etcd backends create by this test
		// point to the same datastore.

		bk, err := New(context.Background(), commonEtcdParams, commonEtcdOptions...)
		if err != nil {
			return nil, nil, err
		}

		// we can't fiddle with clocks inside the etcd client, so instead of creating
		// and returning a fake clock, we wrap the real clock used by the etcd client
		// in a FakeClock interface that sleeps instead of instantly advancing.
		sleepingClock := test.BlockingFakeClock{Clock: bk.clock}

		return bk, sleepingClock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func TestPrefix(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}

	ctx := context.Background()

	// Given an etcd backend with a minimal configuration...
	unprefixedUut, err := New(context.Background(), commonEtcdParams, commonEtcdOptions...)
	require.NoError(t, err)
	defer unprefixedUut.Close()

	// ...and an etcd backend configured to use a custom prefix
	cfg := make(backend.Params)
	maps.Copy(cfg, commonEtcdParams)
	cfg["prefix"] = customPrefix

	prefixedUut, err := New(context.Background(), cfg, commonEtcdOptions...)
	require.NoError(t, err)
	defer prefixedUut.Close()

	// When I push an item with a key starting with "/" into etcd via the
	// _prefixed_ client...
	item := backend.Item{
		Key:   backend.NewKey("foo"),
		Value: []byte("bar"),
	}
	_, err = prefixedUut.Put(ctx, item)
	require.NoError(t, err)

	// Expect that I can retrieve it from the _un_prefixed client by
	// manually prepending a prefix to the key and asking for it.
	wantKey := prefixedUut.cfg.Key + item.Key.String()
	requireKV(ctx, t, unprefixedUut, wantKey, string(item.Value))
	got, err := prefixedUut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)

	// When I push an item with a key that does _not_ start with a separator
	// char (i.e. "/") into etcd via the _prefixed_ client...
	item = backend.Item{
		Key:   backend.NewKey("foo"),
		Value: []byte("bar"),
	}
	_, err = prefixedUut.Put(ctx, item)
	require.NoError(t, err)

	// Expect, again, that I can retrieve it from the _un_prefixed client
	// by manually prepending a prefix to the key and asking for it.
	wantKey = prefixedUut.cfg.Key + item.Key.String()
	requireKV(ctx, t, unprefixedUut, wantKey, string(item.Value))
	got, err = prefixedUut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
}

func requireKV(ctx context.Context, t *testing.T, bk *EtcdBackend, key, val string) {
	t.Logf("assert that key %q contains value %q", key, val)

	resp, err := bk.clients.Next().Get(ctx, key)
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
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}
	// setup
	const maxClientMsgSize = 128
	bk, err := New(context.Background(), backend.Params{
		"peers":                          []string{etcdTestEndpoint()},
		"prefix":                         "/teleport",
		"tls_key_file":                   "../../../fixtures/etcdcerts/client-key.pem",
		"tls_cert_file":                  "../../../fixtures/etcdcerts/client-cert.pem",
		"tls_ca_file":                    "../../../fixtures/etcdcerts/ca-cert.pem",
		"dial_timeout":                   500 * time.Millisecond,
		"etcd_max_client_msg_size_bytes": maxClientMsgSize,
	}, commonEtcdOptions...)
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

func TestLeaseBucketing(t *testing.T) {
	const pfx = "lease-bucket-test"
	const count = 40

	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}

	ctx := t.Context()

	var opts []Option
	opts = append(opts, commonEtcdOptions...)
	opts = append(opts, LeaseBucket(time.Second*2))

	bk, err := New(ctx, commonEtcdParams, opts...)
	require.NoError(t, err)
	defer bk.Close()

	buckets := make(map[int64]struct{})
	for i := range count {
		key := backend.NewKey(pfx, fmt.Sprintf("%d", i))
		_, err := bk.Put(ctx, backend.Item{
			Key:     key,
			Value:   fmt.Appendf(nil, "val-%d", i),
			Expires: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)

		item, err := bk.Get(ctx, key)
		require.NoError(t, err)

		buckets[item.Expires.Unix()] = struct{}{}
		time.Sleep(time.Millisecond * 200)
	}

	start := backend.NewKey(pfx, "")

	rslt, err := bk.GetRange(ctx, start, backend.RangeEnd(start), backend.NoLimit)
	require.NoError(t, err)
	require.Len(t, rslt.Items, count)

	// ensure that we averaged more than 1 item per lease, but
	// also spanned more than one bucket.
	require.NotEmpty(t, buckets)
	require.Less(t, len(buckets), count/2)
}

func etcdTestEnabled() bool {
	return os.Getenv("TELEPORT_ETCD_TEST") != ""
}

// Returns etcd host used in tests
func etcdTestEndpoint() string {
	host := os.Getenv("TELEPORT_ETCD_TEST_ENDPOINT")
	if host != "" {
		return host
	}
	return "https://127.0.0.1:2379"
}

func TestKeyPrefix(t *testing.T) {
	prefixes := []string{"teleport", "/teleport", "/teleport/"}

	for _, prefix := range prefixes {
		t.Run("prefix="+prefix, func(t *testing.T) {
			bk := EtcdBackend{cfg: &Config{Key: prefix}}

			t.Run("leading separator in key", func(t *testing.T) {
				prefixed := bk.prependPrefix(backend.NewKey("test", "llama"))
				assert.Equal(t, prefix+"/test/llama", prefixed)

				key := bk.trimPrefix([]byte(prefixed))
				assert.Equal(t, "/test/llama", key.String())
			})

			t.Run("no leading separator in key", func(t *testing.T) {
				prefixed := bk.prependPrefix(backend.KeyFromString(".locks/test/llama"))
				assert.Equal(t, prefix+".locks/test/llama", prefixed)

				key := bk.trimPrefix([]byte(prefixed))
				assert.Equal(t, ".locks/test/llama", key.String())
			})
		})
	}
}

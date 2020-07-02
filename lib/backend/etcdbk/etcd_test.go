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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"go.etcd.io/etcd/clientv3"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

const customPrefix = "/custom"

func TestEtcd(t *testing.T) { check.TestingT(t) }

type EtcdSuite struct {
	bk     *EtcdBackend
	suite  test.BackendSuite
	config backend.Params
	skip   bool
}

var _ = check.Suite(&EtcdSuite{})

func (s *EtcdSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())

	// this config must match examples/etcd/teleport.yaml
	s.config = backend.Params{
		"peers":         []string{"https://127.0.0.1:2379"},
		"prefix":        "/teleport",
		"tls_key_file":  "../../../examples/etcd/certs/client-key.pem",
		"tls_cert_file": "../../../examples/etcd/certs/client-cert.pem",
		"tls_ca_file":   "../../../examples/etcd/certs/ca-cert.pem",
		"dial_timeout":  500 * time.Millisecond,
	}

	newBackend := func() (backend.Backend, error) {
		return New(context.Background(), s.config)
	}
	s.suite.NewBackend = newBackend
	// Initiate a backend with a registry
	b, err := newBackend()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			fmt.Printf("WARNING: etcd cluster is not available: %v.\n", err)
			fmt.Printf("WARNING: Start examples/etcd/start-etcd.sh.\n")
			s.skip = true
			c.Skip(err.Error())
		}
		c.Assert(err, check.IsNil)
	}
	c.Assert(err, check.IsNil)
	s.bk = b.(*EtcdBackend)
	s.suite.B = s.bk

	// Check connectivity and disable the suite
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = s.bk.GetRange(ctx, []byte("/"), backend.RangeEnd([]byte("/")), backend.NoLimit)
	err = convertErr(err)
	if err != nil && !trace.IsNotFound(err) {
		if strings.Contains(err.Error(), "connection refused") || trace.IsConnectionProblem(err) {
			fmt.Println("WARNING: etcd cluster is not available. Start examples/etcd/start-etcd.sh")
			s.skip = true
			c.Skip(err.Error())
		}
		c.Assert(err, check.IsNil)
	}
}

func (s *EtcdSuite) SetUpTest(c *check.C) {
	if s.skip {
		fmt.Println("WARNING: etcd cluster is not available. Start examples/etcd/start-etcd.sh")
		c.Skip("Etcd is not available")
	}

	s.bk.cfg.Key = legacyDefaultPrefix
}

func (s *EtcdSuite) TearDownTest(c *check.C) {
	ctx := context.Background()
	_, err := s.bk.client.Delete(ctx, legacyDefaultPrefix, clientv3.WithPrefix())
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Delete(ctx, customPrefix, clientv3.WithPrefix())
	c.Assert(err, check.IsNil)
}

func (s *EtcdSuite) TearDownSuite(c *check.C) {
	if s.bk != nil {
		c.Assert(s.bk.Close(), check.IsNil)
	}
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
	s.suite.Locking(c)
}

func (s *EtcdSuite) TestPrefix(c *check.C) {
	var (
		ctx  = context.Background()
		item = backend.Item{
			Key:   []byte("/foo"),
			Value: []byte("bar"),
		}
	)

	s.bk.cfg.Key = customPrefix
	_, err := s.bk.Put(ctx, item)
	c.Assert(err, check.IsNil)

	wantKey := fmt.Sprintf("%s%s", s.bk.cfg.Key, item.Key)
	s.assertKV(ctx, c, wantKey, string(item.Value))
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

func (s *EtcdSuite) TestSyncLegacyPrefix(c *check.C) {
	ctx := context.Background()
	s.bk.cfg.Key = customPrefix

	reset := func() {
		_, err := s.bk.client.Delete(ctx, legacyDefaultPrefix, clientv3.WithPrefix())
		c.Assert(err, check.IsNil)
		_, err = s.bk.client.Delete(ctx, customPrefix, clientv3.WithPrefix())
		c.Assert(err, check.IsNil)
	}
	snapshot := func() map[string]string {
		kvs := make(map[string]string)

		resp, err := s.bk.client.Get(ctx, legacyDefaultPrefix, clientv3.WithPrefix())
		c.Assert(err, check.IsNil)
		for _, kv := range resp.Kvs {
			kvs[string(kv.Key)] = string(kv.Value)
		}

		resp, err = s.bk.client.Get(ctx, customPrefix, clientv3.WithPrefix())
		c.Assert(err, check.IsNil)
		for _, kv := range resp.Kvs {
			kvs[string(kv.Key)] = string(kv.Value)
		}

		return kvs
	}

	// Make sure the prefixes start off clean.
	reset()

	c.Log("both prefixes empty")
	err := s.bk.syncLegacyPrefix(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(snapshot(), check.DeepEquals, map[string]string{})
	reset()

	c.Log("data in custom prefix, no data in legacy prefix; custom prefix should be preserved")
	_, err = s.bk.client.Put(ctx, customPrefix+"/0", "c0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/1", "c1")
	c.Assert(err, check.IsNil)
	err = s.bk.syncLegacyPrefix(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(snapshot(), check.DeepEquals, map[string]string{
		customPrefix + "/0": "c0",
		customPrefix + "/1": "c1",
	})
	reset()

	c.Log("no data in custom prefix, data in legacy prefix copied over; custom prefix should be populated")
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/0", "l0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/1", "l1")
	c.Assert(err, check.IsNil)
	err = s.bk.syncLegacyPrefix(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(snapshot(), check.DeepEquals, map[string]string{
		legacyDefaultPrefix + "/0": "l0",
		legacyDefaultPrefix + "/1": "l1",
		customPrefix + "/0":        "l0",
		customPrefix + "/1":        "l1",
	})
	reset()

	c.Log("data in both prefixes, custom prefix is newer; custom prefix should be preserved")
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/0", "l0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/1", "l1")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/3", "l3")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/0", "c0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/1", "c1")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/2", "c2")
	c.Assert(err, check.IsNil)
	err = s.bk.syncLegacyPrefix(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(snapshot(), check.DeepEquals, map[string]string{
		legacyDefaultPrefix + "/0": "l0",
		legacyDefaultPrefix + "/1": "l1",
		legacyDefaultPrefix + "/3": "l3",
		customPrefix + "/0":        "c0",
		customPrefix + "/1":        "c1",
		customPrefix + "/2":        "c2",
	})
	reset()

	c.Log("data in both prefixes, legacy prefix is newer; custom prefix should be replaced")
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/0", "l0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/1", "l1")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/0", "c0")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/1", "c1")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, customPrefix+"/2", "c2")
	c.Assert(err, check.IsNil)
	_, err = s.bk.client.Put(ctx, legacyDefaultPrefix+"/3", "l3")
	c.Assert(err, check.IsNil)
	err = s.bk.syncLegacyPrefix(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(snapshot(), check.DeepEquals, map[string]string{
		legacyDefaultPrefix + "/0": "l0",
		legacyDefaultPrefix + "/1": "l1",
		legacyDefaultPrefix + "/3": "l3",
		customPrefix + "/0":        "l0",
		customPrefix + "/1":        "l1",
		customPrefix + "/3":        "l3",
	})
	reset()
}

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
package etcdbk

import (
	"os"
	"strings"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { TestingT(t) }

type EtcdSuite struct {
	bk         *bk
	suite      test.BackendSuite
	nodes      []string
	etcdPrefix string
	client     *etcd.Client
	changesC   chan interface{}
	key        string
	stopC      chan bool
}

var _ = Suite(&EtcdSuite{
	etcdPrefix: "/teleport_test",
})

func (s *EtcdSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
	nodes_string := os.Getenv("TELEPORT_TEST_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}
	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry
	b, err := New(s.nodes, s.etcdPrefix)
	c.Assert(err, IsNil)
	s.bk = b.(*bk)
	s.client = s.bk.client

	s.changesC = make(chan interface{})
	s.stopC = make(chan bool)

	// Delete all values under the given prefix
	_, err = s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		if !notFound(err) {
			c.Assert(err, IsNil)
		}
	} else {
		_, err = s.bk.client.Delete(s.etcdPrefix, true)
		if !notFound(err) {
			c.Assert(err, IsNil)
		}
	}

	// Set up suite
	s.suite.ChangesC = s.changesC
	s.suite.B = b
}

func (s *EtcdSuite) TearDownTest(c *C) {
	close(s.stopC)
	c.Assert(s.bk.Close(), IsNil)
}

func (s *EtcdSuite) TestFromObject(c *C) {
	b, err := FromObject(map[string]string{"nodes": s.nodes[0], "key": s.etcdPrefix})
	c.Assert(err, IsNil)
	c.Assert(b, NotNil)
}

func (s *EtcdSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *EtcdSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
}

func (s *EtcdSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *EtcdSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *EtcdSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTl(c)
}

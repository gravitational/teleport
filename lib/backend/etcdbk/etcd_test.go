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
	"testing"

	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"golang.org/x/net/context"
	. "gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { TestingT(t) }

type EtcdSuite struct {
	bk           *bk
	suite        test.BackendSuite
	configString string
	api          client.KeysAPI
	changesC     chan interface{}
	key          string
	stopC        chan bool
}

var _ = Suite(&EtcdSuite{})

func (s *EtcdSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
	configString := os.Getenv("TELEPORT_TEST_ETCD_CONFIG")
	if configString == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in TELEPORT_TEST_ETCD_CONFIG environment variable")
		return
	}
	s.configString = configString
}

func (s *EtcdSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry

	// TODO (ev) fix this:
	b, err := New(nil)
	c.Assert(err, IsNil)

	s.bk = b.(*bk)
	s.api = client.NewKeysAPI(s.bk.client)

	s.changesC = make(chan interface{})
	s.stopC = make(chan bool)

	// Delete all values under the given prefix
	_, err = s.api.Delete(context.Background(), s.bk.cfg.Key, &client.DeleteOptions{Recursive: true, Dir: true})
	err = convertErr(err)
	if err != nil && !trace.IsNotFound(err) {
		c.Assert(err, IsNil)
	}

	// Set up suite
	s.suite.ChangesC = s.changesC
	s.suite.B = b
}

func (s *EtcdSuite) TearDownTest(c *C) {
	close(s.stopC)
	c.Assert(s.bk.Close(), IsNil)
}

func (s *EtcdSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *EtcdSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *EtcdSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *EtcdSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}

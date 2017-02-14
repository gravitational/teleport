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

package consulbk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestConsul(t *testing.T) { TestingT(t) }

type ConsulSuite struct {
	bk     *bk
	suite  test.BackendSuite
	config backend.Params
	skip   bool
}

var _ = Suite(&ConsulSuite{})

func (s *ConsulSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()

	// this config must match examples/consul/teleport.yaml
	s.config = backend.Params{
		"prefix":        "teleport.secrets",
		"tls_key_file":  "../../../examples/consul/certs/client-key.pem",
		"tls_cert_file": "../../../examples/consul/certs/client-cert.pem",
		"tls_ca_file":   "../../../examples/consul/certs/ca-cert.pem",
	}
	// Initiate a backend with a registry
	b, err := New(s.config)
	c.Assert(err, IsNil)
	s.bk = b.(*bk)
	s.suite.B = b
}

func (s *ConsulSuite) SetUpTest(c *C) {
	if s.skip {
		c.Skip("consul is not available")
	}

	// Delete all values under the given prefix
	_, err := s.bk.kv.DeleteTree(s.bk.cfg.Key, nil)
	err = convertErr(err)
	if err != nil && !trace.IsNotFound(err) {
		if strings.Contains(err.Error(), "cluster is unavailable") {
			fmt.Println("WARNING: consul cluster is not available. Start examples/consul/start-consul.sh")
			s.skip = true
			c.Skip(err.Error())
		}
		c.Assert(err, IsNil)
	}
}

func (s *ConsulSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *ConsulSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *ConsulSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *ConsulSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *ConsulSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}

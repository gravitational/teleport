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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"go.etcd.io/etcd/clientv3"
	"gopkg.in/check.v1"
)

func TestEtcd(t *testing.T) { check.TestingT(t) }

type EtcdSuite struct {
	bk     *EtcdBackend
	suite  test.BackendSuite
	client *clientv3.Client
	config backend.Params
	skip   bool
}

var _ = check.Suite(&EtcdSuite{})

func (s *EtcdSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()

	// this config must match examples/etcd/teleport.yaml
	s.config = backend.Params{
		"peers":         []string{"https://127.0.0.1:2379"},
		"prefix":        "teleport.secrets",
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
		c.Skip("Etcd is not avialable")
	}
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

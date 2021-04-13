/*
Copyright 2019 Gravitational, Inc.

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

package memory

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestLite(t *testing.T) { check.TestingT(t) }

type MemorySuite struct {
	bk    *Memory
	suite test.BackendSuite
}

var _ = check.Suite(&MemorySuite{})

func (s *MemorySuite) SetUpSuite(c *check.C) {
	newBackend := func() (backend.Backend, error) {
		mem, err := New(Config{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return mem, nil
	}
	s.suite.NewBackend = newBackend
}

func (s *MemorySuite) SetUpTest(c *check.C) {
	bk, err := s.suite.NewBackend()
	c.Assert(err, check.IsNil)
	s.bk = bk.(*Memory)
	s.suite.B = s.bk
}

func (s *MemorySuite) TearDownTest(c *check.C) {
	if s.bk != nil {
		c.Assert(s.bk.Close(), check.IsNil)
	}
}

func (s *MemorySuite) TestCRUD(c *check.C) {
	s.suite.CRUD(c)
}

func (s *MemorySuite) TestRange(c *check.C) {
	s.suite.Range(c)
}

func (s *MemorySuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *MemorySuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *MemorySuite) TestKeepAlive(c *check.C) {
	s.suite.KeepAlive(c)
}

func (s *MemorySuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *MemorySuite) TestWatchersClose(c *check.C) {
	s.suite.WatchersClose(c)
}

func (s *MemorySuite) TestDeleteRange(c *check.C) {
	s.suite.DeleteRange(c)
}

func (s *MemorySuite) TestPutRange(c *check.C) {
	s.suite.PutRange(c)
}

func (s *MemorySuite) TestLocking(c *check.C) {
	s.suite.Locking(c, s.bk)
}

func (s *MemorySuite) TestConcurrentOperations(c *check.C) {
	bk, err := s.suite.NewBackend()
	c.Assert(err, check.IsNil)
	defer bk.Close()
	s.suite.B2 = bk
	s.suite.ConcurrentOperations(c)
}

func (s *MemorySuite) TestMirror(c *check.C) {
	mem, err := New(Config{
		Mirror: true,
	})
	c.Assert(err, check.IsNil)
	s.suite.Mirror(c, mem)
}

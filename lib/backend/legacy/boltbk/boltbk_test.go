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
package boltbk

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/legacy"
	"github.com/gravitational/teleport/lib/backend/legacy/test"
	btest "github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { check.TestingT(t) }

type BoltSuite struct {
	bk    legacy.Backend
	suite test.BackendSuite
}

var _ = check.Suite(&BoltSuite{})

func (s *BoltSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())
}

func (s *BoltSuite) SetUpTest(c *check.C) {
	var err error

	dir := c.MkDir()
	s.bk, err = New(legacy.Params{
		"path": dir,
	})
	c.Assert(err, check.IsNil)
	c.Assert(s.bk, check.NotNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *BoltSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *BoltSuite) TestBasicCRUD(c *check.C) {
	s.suite.BasicCRUD(c)
}

func (s *BoltSuite) TestBatchCRUD(c *check.C) {
	s.suite.BatchCRUD(c)
}

func (s *BoltSuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *BoltSuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *BoltSuite) TestLock(c *check.C) {
	s.suite.Locking(c)
}

func (s *BoltSuite) TestValueAndTTL(c *check.C) {
	s.suite.ValueAndTTL(c)
}

func (s *BoltSuite) TestExport(c *check.C) {
	dirName := c.MkDir()
	bk, err := New(legacy.Params{"path": dirName})
	c.Assert(err, check.IsNil)
	defer bk.Close()

	bk.CreateVal([]string{"root"}, "key", []byte("value"), backend.Forever)
	bk.CreateVal([]string{"root", "sub"}, "key2", []byte("value2"), time.Hour)
	bk.CreateVal([]string{"root", "sub", "sub3"}, "key3", []byte("value3"), time.Hour)

	items, err := bk.Export()
	c.Assert(err, check.IsNil)
	btest.ExpectItems(c, items,
		[]backend.Item{
			{Key: backend.Key("root", "key"), Value: []byte("value")},
			{Key: backend.Key("root", "sub", "key2"), Value: []byte("value2")},
			{Key: backend.Key("root", "sub", "sub3", "key3"), Value: []byte("value3")},
		},
	)

	// first item should not have expiry time, second one should
	c.Assert(items[0].Expires.IsZero(), check.Equals, true)
	c.Assert(items[1].Expires.IsZero(), check.Equals, false)
}

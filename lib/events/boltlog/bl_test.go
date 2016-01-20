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
package boltlog

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/events/test"

	. "gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltLogSuite struct {
	l     *BoltLog
	suite test.EventSuite
	dir   string
}

var _ = Suite(&BoltLogSuite{})

func (s *BoltLogSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.l, err = New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.suite.L = s.l
}

func (s *BoltLogSuite) TearDownTest(c *C) {
	c.Assert(s.l.Close(), IsNil)
}

func (s *BoltLogSuite) TestEventsCRUD(c *C) {
	s.suite.EventsCRUD(c)
}

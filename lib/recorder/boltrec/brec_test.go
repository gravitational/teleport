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
package boltrec

import (
	"testing"

	"github.com/gravitational/teleport/lib/recorder/test"

	. "gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltRecSuite struct {
	r     *boltRecorder
	suite test.RecorderSuite
	dir   string
}

var _ = Suite(&BoltRecSuite{})

func (s *BoltRecSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.r, err = New(s.dir)
	c.Assert(err, IsNil)

	s.suite.R = s.r
}

func (s *BoltRecSuite) TearDownTest(c *C) {
	//c.Assert(s.r.Close(), IsNil)
}

func (s *BoltRecSuite) TestRecorder(c *C) {
	s.suite.Recorder(c)
}

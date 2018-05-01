// +build dynamodb

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

package dynamo

import (
	"testing"

	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestDynamoDB(t *testing.T) { TestingT(t) }

type DynamoDBSuite struct {
	bk        *DynamoDBBackend
	suite     test.BackendSuite
	tableName string
}

var _ = Suite(&DynamoDBSuite{})

func (s *DynamoDBSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()

	var err error
	s.tableName = "teleport.dynamo.test"
	bk, err := New(map[string]interface{}{
		"table_name": s.tableName,
	})
	c.Assert(err, IsNil)
	s.bk = bk.(*DynamoDBBackend)
	c.Assert(err, IsNil)
	s.suite.B = s.bk
}

func (s *DynamoDBSuite) TearDownSuite(c *C) {
	if s.bk != nil && s.bk.svc != nil {
		s.bk.deleteTable(s.tableName, false)
	}
}

func (s *DynamoDBSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *DynamoDBSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *DynamoDBSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
}

func (s *DynamoDBSuite) TestBatchCRUD(c *C) {
	s.suite.BatchCRUD(c)
}

func (s *DynamoDBSuite) TestDirectories(c *C) {
	s.suite.Directories(c)
}

func (s *DynamoDBSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *DynamoDBSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *DynamoDBSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}

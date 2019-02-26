// +build dynamodb

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

package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestDynamoDB(t *testing.T) { check.TestingT(t) }

type DynamoDBSuite struct {
	bk        *DynamoDBBackend
	suite     test.BackendSuite
	tableName string
}

var _ = check.Suite(&DynamoDBSuite{})

func (s *DynamoDBSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	var err error

	s.tableName = "teleport.dynamo.test"
	newBackend := func() (backend.Backend, error) {
		return New(context.Background(), map[string]interface{}{
			"table_name":         s.tableName,
			"poll_stream_period": 300 * time.Millisecond,
		})
	}
	bk, err := newBackend()
	c.Assert(err, check.IsNil)
	s.bk = bk.(*DynamoDBBackend)
	s.suite.B = s.bk
	s.suite.NewBackend = newBackend
}

func (s *DynamoDBSuite) TearDownSuite(c *check.C) {
	if s.bk != nil && s.bk.svc != nil {
		//		s.bk.deleteTable(context.Background(), s.tableName, false)
		c.Assert(s.bk.Close(), check.IsNil)
	}
}

func (s *DynamoDBSuite) TestCRUD(c *check.C) {
	s.suite.CRUD(c)
}

func (s *DynamoDBSuite) TestRange(c *check.C) {
	s.suite.Range(c)
}

func (s *DynamoDBSuite) TestDeleteRange(c *check.C) {
	s.suite.DeleteRange(c)
}

func (s *DynamoDBSuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *DynamoDBSuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *DynamoDBSuite) TestKeepAlive(c *check.C) {
	s.suite.KeepAlive(c)
}

func (s *DynamoDBSuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *DynamoDBSuite) TestWatchersClose(c *check.C) {
	s.suite.WatchersClose(c)
}

func (s *DynamoDBSuite) TestLocking(c *check.C) {
	s.suite.Locking(c)
}

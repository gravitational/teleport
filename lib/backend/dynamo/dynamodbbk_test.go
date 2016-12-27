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

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestDynamoDB(t *testing.T) { TestingT(t) }

type DynamoDBSuite struct {
	bk    *DynamoDBBackend
	suite test.BackendSuite
	cfg   backend.Config
}

var _ = Suite(&DynamoDBSuite{})

func (s *DynamoDBSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()

	var err error
	s.cfg.Type = "dynamodb"
	s.cfg.Tablename = "teleport.dynamo.test"

	s.bk, err = New(&s.cfg)
	c.Assert(err, IsNil)
	s.suite.B = s.bk
}

func (s *DynamoDBSuite) TearDownSuite(c *C) {
	if s.bk != nil && s.bk.svc != nil {
		s.bk.deleteTable(s.cfg.Tablename, false)
	}
}

func (s *DynamoDBSuite) TestMigration(c *C) {
	s.cfg.Type = "dynamodb"
	s.cfg.Tablename = "teleport.dynamo.test"
	// migration uses its own instance of the backend:
	bk, err := New(&s.cfg)
	c.Assert(err, IsNil)

	var (
		legacytable      = "legacy.teleport.t"
		nonExistingTable = "nonexisting.teleport.t"
	)
	bk.deleteTable(legacytable, true)
	bk.deleteTable(legacytable+".bak", false)
	defer bk.deleteTable(legacytable, false)
	defer bk.deleteTable(legacytable+".bak", false)

	status, err := bk.getTableStatus(nonExistingTable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusMissing))

	err = bk.createTable(legacytable, oldPathAttr)
	c.Assert(err, IsNil)

	status, err = bk.getTableStatus(legacytable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusNeedsMigration))

	err = bk.migrate(legacytable)
	c.Assert(err, IsNil)

	status, err = bk.getTableStatus(legacytable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusOK))
}

func (s *DynamoDBSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *DynamoDBSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
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

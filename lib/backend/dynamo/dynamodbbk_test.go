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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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
	var err error

	utils.InitLoggerForTests()
	s.cfg.Type = "dynamodb"
	s.cfg.Tablename = "teleport.dynamo.test"

	s.bk, err = New(&s.cfg)
	c.Assert(err, IsNil)
	s.suite.B = s.bk
}

func (s *DynamoDBSuite) TearDownSuite(c *C) {
	if s.bk.svc != nil {
		s.bk.svc.DeleteTable(&dynamodb.DeleteTableInput{
			TableName: aws.String(s.cfg.Tablename),
		})
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

func (s *DynamoDBSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *DynamoDBSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *DynamoDBSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}

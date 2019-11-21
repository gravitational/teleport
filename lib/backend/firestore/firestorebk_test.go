// +build firestore

/*

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

package firestore

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestFirestoreDB(t *testing.T) { check.TestingT(t) }

type FirestoreSuite struct {
	bk             *FirestoreBackend
	suite          test.BackendSuite
	collectionName string
}

var _ = check.Suite(&FirestoreSuite{})

func (s *FirestoreSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	var err error

	newBackend := func() (backend.Backend, error) {
		return New(context.Background(), map[string]interface{}{
			"collection_name":                   "tp-cluster-data-test",
			"project_id":                        "tp-testproj",
			"endpoint":                          "localhost:8618",
			"purgeExpiredDocumentsPollInterval": time.Second,
		})
	}
	bk, err := newBackend()
	c.Assert(err, check.IsNil)
	s.bk = bk.(*FirestoreBackend)
	s.suite.B = s.bk
	s.suite.NewBackend = newBackend
}

func (s *FirestoreSuite) SetUpTest(c *check.C) {
	s.bk.deleteAllItems()
}

func (s *FirestoreSuite) TearDownSuite(c *check.C) {
	s.bk.deleteAllItems()
}

func (s *FirestoreSuite) TestCRUD(c *check.C) {
	s.suite.CRUD(c)
}

func (s *FirestoreSuite) TestRange(c *check.C) {
	s.suite.Range(c)
}

func (s *FirestoreSuite) TestDeleteRange(c *check.C) {
	s.suite.DeleteRange(c)
}

func (s *FirestoreSuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *FirestoreSuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *FirestoreSuite) TestKeepAlive(c *check.C) {
	s.suite.KeepAlive(c)
}

func (s *FirestoreSuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *FirestoreSuite) TestWatchersClose(c *check.C) {
	s.suite.WatchersClose(c)
}

func (s *FirestoreSuite) TestLocking(c *check.C) {
	s.suite.Locking(c)
}

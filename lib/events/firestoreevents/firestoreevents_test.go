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

package firestoreevents

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestFirestoreevents(t *testing.T) { check.TestingT(t) }

type FirestoreeventsSuite struct {
	log *Log
	test.EventsSuite
}

var _ = check.Suite(&FirestoreeventsSuite{})

func (s *FirestoreeventsSuite) SetUpSuite(c *check.C) {
	if !emulatorRunning() {
		c.Skip("Firestore emulator is not running, start it with: gcloud beta emulators firestore start --host-port=localhost:8618")
	}

	fakeClock := clockwork.NewFakeClock()

	config := EventsConfig{}
	config.SetFromParams(map[string]interface{}{
		"collection_name":                   "tp-events-test",
		"project_id":                        "tp-testproj",
		"endpoint":                          "localhost:8618",
		"purgeExpiredDocumentsPollInterval": time.Second,
	})

	config.Clock = fakeClock
	config.UIDGenerator = utils.NewFakeUID()

	log, err := New(config)

	c.Assert(err, check.IsNil)
	s.log = log
	s.EventsSuite.Log = log
	s.EventsSuite.Clock = fakeClock
	s.EventsSuite.QueryDelay = time.Second
}

func emulatorRunning() bool {
	con, err := net.Dial("tcp", "localhost:8618")
	if err != nil {
		return false
	}
	con.Close()
	return true
}

func (s *FirestoreeventsSuite) TearDownSuite(c *check.C) {
	if s.log != nil {
		s.log.Close()
	}
}

func (s *FirestoreeventsSuite) TearDownTest(c *check.C) {
	// Delete all documents.
	ctx := context.Background()
	docSnaps, err := s.log.svc.Collection(s.log.CollectionName).Documents(ctx).GetAll()
	c.Assert(err, check.IsNil)
	if len(docSnaps) == 0 {
		return
	}
	batch := s.log.svc.Batch()
	for _, docSnap := range docSnaps {
		batch.Delete(docSnap.Ref)
	}
	_, err = batch.Commit(ctx)
	c.Assert(err, check.IsNil)
}

func (s *FirestoreeventsSuite) TestSessionEventsCRUD(c *check.C) {
	s.SessionEventsCRUD(c)
}

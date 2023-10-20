/*
Copyright 2021 Gravitational, Inc

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

	"cloud.google.com/go/firestore"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type firestoreContext struct {
	log   *Log
	suite test.EventsSuite
}

func setupFirestoreContext(t *testing.T) *firestoreContext {
	if !emulatorRunning() {
		t.Skip("Firestore emulator is not running, start it with: gcloud beta emulators firestore start --host-port=localhost:8618")
	}

	require.NoError(t, os.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8618"))

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
	require.NoError(t, err)

	tt := &firestoreContext{
		log: log,
		suite: test.EventsSuite{
			Log:        log,
			Clock:      fakeClock,
			QueryDelay: time.Second * 5,
		},
	}

	t.Cleanup(func() {
		tt.Close(t)
	})

	return tt
}

func (tt *firestoreContext) setupTest(t *testing.T) {
	ctx := context.Background()

	// Delete all documents.
	docSnaps, err := tt.log.svc.Collection(tt.log.CollectionName).Documents(ctx).GetAll()
	require.NoError(t, err)
	if len(docSnaps) == 0 {
		return
	}
	batch := tt.log.svc.BulkWriter(tt.log.svcContext)
	jobs := make([]*firestore.BulkWriterJob, 0, len(docSnaps))
	for _, docSnap := range docSnaps {
		job, err := batch.Delete(docSnap.Ref)
		require.NoError(t, err)
		jobs = append(jobs, job)
	}

	batch.End()
	for _, job := range jobs {
		_, err := job.Results()
		require.NoError(t, err)
	}
}

func (tt *firestoreContext) Close(t *testing.T) {
	if tt.log != nil {
		err := tt.log.Close()
		require.NoError(t, err)
	}
}

func (tt *firestoreContext) testSessionEventsCRUD(t *testing.T) {
	tt.setupTest(t)
	tt.suite.SessionEventsCRUD(t)
}

func (tt *firestoreContext) testPagination(t *testing.T) {
	tt.setupTest(t)
	tt.suite.EventPagination(t)
}

func (tt *firestoreContext) testSearchSessionEvensBySessionID(t *testing.T) {
	tt.setupTest(t)
	tt.suite.SearchSessionEventsBySessionID(t)
}

func TestFirestoreEvents(t *testing.T) {
	tt := setupFirestoreContext(t)

	t.Run("TestSessionEventsCRUD", tt.testSessionEventsCRUD)
	t.Run("TestPagination", tt.testPagination)
	t.Run("TestSearchSessionEvensBySessionID", tt.testSearchSessionEvensBySessionID)
}

func emulatorRunning() bool {
	con, err := net.Dial("tcp", "localhost:8618")
	if err != nil {
		return false
	}
	con.Close()
	return true
}

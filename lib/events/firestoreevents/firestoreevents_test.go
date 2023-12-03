/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

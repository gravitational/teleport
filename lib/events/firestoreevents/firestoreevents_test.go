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
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
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
	config.SetFromParams(map[string]any{
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

func (tt *firestoreContext) testSearchEventsBySearchTerm(t *testing.T) {
	tt.setupTest(t)
	tt.suite.SearchEventsBySearchTerm(t)
}

func TestFirestoreEvents(t *testing.T) {
	tt := setupFirestoreContext(t)

	t.Run("TestSessionEventsCRUD", tt.testSessionEventsCRUD)
	t.Run("TestPagination", tt.testPagination)
	t.Run("TestSearchSessionEvensBySessionID", tt.testSearchSessionEvensBySessionID)
	t.Run("TestSearchEventsBySearchTerm", tt.testSearchEventsBySearchTerm)
}

func TestEmitAuditEvent_NoLossDeduplication(t *testing.T) {
	tt := setupFirestoreContext(t)
	tt.setupTest(t)
	ctx := t.Context()

	eventTime := tt.suite.Clock.Now().UTC()
	from := eventTime.Add(-time.Hour)
	to := eventTime.Add(time.Hour)

	t.Run("IdenticalReDeliveryIsDeduplicated", func(t *testing.T) {
		id := uuid.NewString()
		marker := "dedup-alpaca-" + uuid.NewString()
		ev := makeDedupTestEvent(id, eventTime, marker)

		before := testutil.ToFloat64(writeRequestsDeduped)
		require.NoError(t, tt.log.EmitAuditEvent(ctx, ev), "first delivery")
		require.NoError(t, tt.log.EmitAuditEvent(ctx, ev), "at-least-once re-delivery")

		requireStoredMarkerCount(ctx, t, tt.log, from, to, marker, 1)
		require.InDelta(t, before+1, testutil.ToFloat64(writeRequestsDeduped), 0.0001,
			"the duplicate delivery should increment the deduped counter exactly once")
	})

	t.Run("DistinctEventsSharingIDAreBothStored", func(t *testing.T) {
		id := uuid.NewString()
		prefix := "collide-" + uuid.NewString() + "-"
		alice := makeDedupTestEvent(id, eventTime, prefix+"alice")
		bob := makeDedupTestEvent(id, eventTime, prefix+"bob")

		before := testutil.ToFloat64(eventIDCollisions)
		require.NoError(t, tt.log.EmitAuditEvent(ctx, alice))
		require.NoError(t, tt.log.EmitAuditEvent(ctx, bob))

		requireStoredMarkerCount(ctx, t, tt.log, from, to, prefix+"alice", 1)
		requireStoredMarkerCount(ctx, t, tt.log, from, to, prefix+"bob", 1)
		require.InDelta(t, before+1, testutil.ToFloat64(eventIDCollisions), 0.0001,
			"the colliding distinct event should increment the collision counter exactly once")
	})

	t.Run("ConcurrentDistinctEventsSharingIDAreNotLost", func(t *testing.T) {
		id := uuid.NewString()
		prefix := "concurrent-" + uuid.NewString() + "-"
		const n = 20

		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Go(func() {
				ev := makeDedupTestEvent(id, eventTime, fmt.Sprintf("%suser-%02d", prefix, i))
				errs[i] = tt.log.EmitAuditEvent(ctx, ev)
			})
		}
		wg.Wait()

		require.NoError(t, errors.Join(errs...), "one or more audit goroutines errored")

		for i := range n {
			marker := fmt.Sprintf("%suser-%02d", prefix, i)
			requireStoredMarkerCount(ctx, t, tt.log, from, to, marker, 1)
		}
	})
}

func makeDedupTestEvent(id string, eventTime time.Time, marker string) *apievents.UserLogin {
	return &apievents.UserLogin{
		Method: events.LoginMethodSAML,
		Status: apievents.Status{Success: true},
		UserMetadata: apievents.UserMetadata{
			User:     marker,
			UserKind: apievents.UserKind_USER_KIND_HUMAN,
		},
		Metadata: apievents.Metadata{
			ID:          id,
			Type:        events.UserLoginEvent,
			ClusterName: "mycluster",
			Time:        eventTime,
		},
	}
}

func requireStoredMarkerCount(ctx context.Context, t *testing.T, log *Log, from, to time.Time, marker string, want int) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		got, err := searchAllEvents(ctx, log, from, to)
		if !assert.NoError(c, err) {
			return
		}
		assert.Equal(c, want, countMarker(got, marker), "stored count for marker %q", marker)
	}, 30*time.Second, time.Second)
}

func searchAllEvents(ctx context.Context, log *Log, from, to time.Time) ([]apievents.AuditEvent, error) {
	var out []apievents.AuditEvent
	var checkpoint string
	for {
		fetched, next, err := log.SearchEvents(ctx, events.SearchEventsRequest{
			From:     from,
			To:       to,
			Limit:    1000,
			Order:    types.EventOrderAscending,
			StartKey: checkpoint,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, fetched...)
		checkpoint = next
		if checkpoint == "" {
			return out, nil
		}
	}
}

func countMarker(evts []apievents.AuditEvent, marker string) int {
	var n int
	for _, e := range evts {
		data, err := utils.FastMarshal(e)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), marker) {
			n++
		}
	}
	return n
}

func emulatorRunning() bool {
	con, err := net.Dial("tcp", "localhost:8618")
	if err != nil {
		return false
	}
	con.Close()
	return true
}

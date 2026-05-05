/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package export

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
)

const day = time.Hour * 24

// TestExporterBasics tests the basic functionality of the exporter with and without random flake.
func TestExporterBasics(t *testing.T) {
	t.Parallel()

	now := normalizeDate(time.Now())
	startDate := now.Add(-7 * day)

	for _, randomFlake := range []bool{false, true} {

		// empty case verified export of a time range larger than backlog size with no events in it.
		t.Run(fmt.Sprintf("case=empty,randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()
			clt := newFakeClient()
			clt.setRandomFlake(randomFlake)

			testExportAll(t, exportTestCase{
				clt:       clt,
				startDate: startDate,
				expected:  []*auditlogpb.ExportEventUnstructured{},
			})
		})

		// sparse case verifies export of a time range with gaps larger than the backlog size.
		t.Run(fmt.Sprintf("case=sparse,randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()

			clt := newFakeClient()
			clt.setRandomFlake(randomFlake)

			var allEvents []*auditlogpb.ExportEventUnstructured
			allEvents = append(allEvents, addEvents(t, clt, startDate, 1, 1)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(4*day), 3, 2)...)

			testExportAll(t, exportTestCase{
				clt:       clt,
				startDate: startDate,
				expected:  allEvents,
			})
		})

		// dense case verifies export of a time range with many events in every date.
		t.Run(fmt.Sprintf("case=dense,randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()

			clt := newFakeClient()
			clt.setRandomFlake(randomFlake)

			var allEvents []*auditlogpb.ExportEventUnstructured
			allEvents = append(allEvents, addEvents(t, clt, startDate, 100, 1)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(day), 50, 2)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(2*day), 5, 20)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(3*day), 20, 5)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(4*day), 14, 7)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(5*day), 7, 14)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(6*day), 1, 100)...)

			testExportAll(t, exportTestCase{
				clt:       clt,
				startDate: startDate,
				expected:  allEvents,
			})
		})
	}
}

// addEvents is a helper for generating events in tests. It both inserts the specified event chunks/counts into the fake client
// and returns the generated events for comparison.
func addEvents(t *testing.T, clt *fakeClient, date time.Time, chunks, eventsPerChunk int) []*auditlogpb.ExportEventUnstructured {
	var allEvents []*auditlogpb.ExportEventUnstructured
	for range chunks {
		chunk, _ := makeEventChunk(t, date, eventsPerChunk)
		allEvents = append(allEvents, chunk...)
		clt.addChunk(date.Format(time.DateOnly), uuid.NewString(), chunk)
	}

	return allEvents
}

type exportTestCase struct {
	clt       Client
	startDate time.Time
	expected  []*auditlogpb.ExportEventUnstructured
}

// testExportAll verifies that the expected events are exported by the exporter given
// the supplied client state.
func testExportAll(t *testing.T, tc exportTestCase) {
	var exportedMu sync.Mutex
	var exported []*auditlogpb.ExportEventUnstructured

	exportFn := func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		exported = append(exported, event)
		return nil
	}

	getExported := func() []*auditlogpb.ExportEventUnstructured {
		exportedMu.Lock()
		defer exportedMu.Unlock()
		return append([]*auditlogpb.ExportEventUnstructured(nil), exported...)
	}

	var idleOnce sync.Once
	idleCh := make(chan struct{})

	exporter, err := NewExporter(ExporterConfig{
		Client:       tc.clt,
		StartDate:    tc.startDate,
		Export:       exportFn,
		OnIdle:       func(_ context.Context) { idleOnce.Do(func() { close(idleCh) }) },
		Concurrency:  2,
		BacklogSize:  2,
		MaxBackoff:   600 * time.Millisecond,
		PollInterval: 200 * time.Millisecond,
	})
	require.NoError(t, err)
	defer exporter.Close()

	timeout := time.After(30 * time.Second)
	select {
	case <-idleCh:
	case <-timeout:
		require.FailNow(t, "timeout waiting for exporter to become idle")
	}

	require.ElementsMatch(t, tc.expected, getExported())
}

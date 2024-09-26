package export

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/stretchr/testify/require"
)

const day = time.Hour * 24

func TestExporterBasics(t *testing.T) {
	t.Parallel()

	now := normalizeDate(time.Now())
	startDate := now.Add(-7 * day)

	for _, randomFlake := range []bool{false, true} {

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

		t.Run(fmt.Sprintf("case=sparse,randomFlake=%v", randomFlake), func(t *testing.T) {
			t.Parallel()

			clt := newFakeClient()
			clt.setRandomFlake(randomFlake)

			var allEvents []*auditlogpb.ExportEventUnstructured
			allEvents = append(allEvents, addEvents(t, clt, startDate, 1, 1)...)
			allEvents = append(allEvents, addEvents(t, clt, startDate.Add(2*day), 3, 2)...)

			testExportAll(t, exportTestCase{
				clt:       clt,
				startDate: startDate,
				expected:  allEvents,
			})
		})

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

func addEvents(t *testing.T, clt *fakeClient, date time.Time, chunks, eventsPerChunk int) []*auditlogpb.ExportEventUnstructured {
	var allEvents []*auditlogpb.ExportEventUnstructured
	for i := 0; i < chunks; i++ {
		chunk := makeEventChunk(t, date, eventsPerChunk)
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

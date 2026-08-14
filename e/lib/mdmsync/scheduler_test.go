package mdmsync_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/mdmsync"
)

func TestSyncScheduler_completeSchedule(t *testing.T) {
	entries := []*types.JamfInventoryEntry{
		{
			FilterRsql:        "full+partial", // Abuse RSQL strings for easier debugging.
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(6 * time.Hour),
			SyncPeriodFull:    types.DurationStringForJamfSpecV1(18 * time.Hour),
		},
		{
			FilterRsql:        "full only",
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(8 * time.Hour), // partial>=full means only full happens.
			SyncPeriodFull:    types.DurationStringForJamfSpecV1(8 * time.Hour),
		},
		{
			FilterRsql:        "partial only",
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(5 * time.Hour),
			SyncPeriodFull:    0, // disabled
		},
		{
			FilterRsql:        "none",
			SyncPeriodPartial: -1, // disabled, same as zero
			SyncPeriodFull:    -1, // disabled, same as zero
		},
	}
	fullPartial := entries[0]
	fullOnly := entries[1]
	partialOnly := entries[2]
	// "none" is never scheduled

	scheduler, err := mdmsync.New(entries, incDelay(), jamfInfo)
	if err != nil {
		t.Fatalf("NewSyncScheduler failed: %v", err)
	}

	assertSchedule(t, scheduler, []wantSchedule{
		// t=0, initial sync
		{
			offset: 0,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
		// t=1, initial sync
		{
			offset: 1,
			entry:  fullOnly,
			mode:   mdmsync.SyncModeFull,
		},
		// t=2, initial sync
		{
			offset: 1,
			entry:  partialOnly,
			mode:   mdmsync.SyncModePartial,
		},
		// t=5h
		{
			offset: 5 * time.Hour,
			entry:  partialOnly,
			mode:   mdmsync.SyncModePartial,
		},
		// t=6h
		{
			offset: 1 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=8h
		{
			offset: 2 * time.Hour,
			entry:  fullOnly,
			mode:   mdmsync.SyncModeFull,
		},
		// t=10h
		{
			offset: 2 * time.Hour,
			entry:  partialOnly,
			mode:   mdmsync.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=15h
		{
			offset: 3 * time.Hour,
			entry:  partialOnly,
			mode:   mdmsync.SyncModePartial,
		},
		// t=16h
		{
			offset: 1 * time.Hour,
			entry:  fullOnly,
			mode:   mdmsync.SyncModeFull,
		},
		// t=18h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
		// t=20h
		{
			offset: 2 * time.Hour,
			entry:  partialOnly,
			mode:   mdmsync.SyncModePartial,
		},
		// t=24h
		{
			offset: 4 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		{
			offset: 1,
			entry:  fullOnly,
			mode:   mdmsync.SyncModeFull,
		},
		// and so on...
	})
}

func TestSyncScheduler_partialNotDivisor(t *testing.T) {
	// SyncPeriodPartial is not a divisor of SyncPeriodFull, meaning the schedules
	// don't line up perfectly.
	entry := &types.JamfInventoryEntry{
		SyncPeriodPartial: types.DurationStringForJamfSpecV1(5 * time.Hour),
		SyncPeriodFull:    types.DurationStringForJamfSpecV1(12 * time.Hour),
	}

	scheduler, err := mdmsync.New(
		[]*types.JamfInventoryEntry{entry}, zeroDelay, jamfInfo)
	if err != nil {
		t.Fatalf("NewSyncScheduler failed: %v", err)
	}

	assertSchedule(t, scheduler, []wantSchedule{
		{
			entry: entry,
			mode:  mdmsync.SyncModeFull,
		},
		// t=5h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModePartial,
		},
		// t=10h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModeFull,
		},
		// t=17
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModePartial,
		},
		// t=22h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModePartial,
		},
		// t=24
		{
			offset: 2 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModeFull,
		},
		// t=29
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdmsync.SyncModePartial,
		},
		// and so on...
	})
}

func TestSyncScheduler_scheduleCantBeExhausted(t *testing.T) {
	entries := []*types.JamfInventoryEntry{
		{
			FilterRsql:        "full+partial", // Abuse RSQL strings for easier debugging.
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(1 * time.Minute),
			SyncPeriodFull:    types.DurationStringForJamfSpecV1(2 * time.Minute),
		},
		{
			FilterRsql:        "partial",
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(2 * time.Minute),
		},
		{
			FilterRsql:     "full",
			SyncPeriodFull: types.DurationStringForJamfSpecV1(2 * time.Minute),
		},
	}

	scheduler, err := mdmsync.New(entries, incDelay(), jamfInfo)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Skip initial entries, so we get to the "loop".
	scheduler.Next()
	scheduler.Next()
	scheduler.Next()

	want := []wantSchedule{
		// t=1
		{
			entry: entries[0],
			mode:  mdmsync.SyncModePartial,
		},
		// t=2
		{
			entry: entries[0],
			mode:  mdmsync.SyncModeFull,
		},
		{
			entry: entries[1],
			mode:  mdmsync.SyncModePartial,
		},
		{
			entry: entries[2],
			mode:  mdmsync.SyncModeFull,
		},
		// and then loops perfectly.
	}

	// Run an arbitrarily high number of iterations, showing that the schedule
	// loops perfectly and can't be exhausted.
	const iterations = 400
	for i := range iterations {
		w := want[i%len(want)]
		got := scheduler.Next()
		if diff := cmp.Diff(w.entry, got.Entry); diff != "" {
			t.Fatalf("Next(%v) mismatch (-want +got)\n%s", i, diff)
		}
		if got.Mode != w.mode {
			t.Errorf("Next(%v) mode=%v, want %v", i, got.Mode, w.mode)
		}
	}
}

func TestNewSyncScheduler_errors(t *testing.T) {
	tests := []struct {
		name    string
		entries []*types.JamfInventoryEntry
		wantErr error
	}{
		{
			name:    "empty entries",
			wantErr: mdmsync.ErrScheduleNoEntries,
		},
		{
			name: "empty schedule",
			entries: []*types.JamfInventoryEntry{
				{
					SyncPeriodPartial: 0, // both syncs disabled
					SyncPeriodFull:    0,
				},
				{
					SyncPeriodPartial: -1, // both syncs disabled
					SyncPeriodFull:    -1,
				},
			},
			wantErr: mdmsync.ErrScheduleEmpty,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := mdmsync.New(test.entries, zeroDelay, jamfInfo)
			if !errors.Is(err, test.wantErr) {
				t.Errorf("New returned err=%v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestSyncScheduler_resetSchedule(t *testing.T) {
	entries := []*types.JamfInventoryEntry{
		{
			FilterRsql:        "full+partial", // Abuse RSQL strings for easier debugging.
			SyncPeriodPartial: types.DurationStringForJamfSpecV1(6 * time.Hour),
			SyncPeriodFull:    types.DurationStringForJamfSpecV1(18 * time.Hour),
		},
	}
	fullPartial := entries[0]

	scheduler, err := mdmsync.New(entries, incDelay(), jamfInfo)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	assertSchedule(t, scheduler, []wantSchedule{
		// t=0, initial sync
		{
			offset: 0,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
		// t=6h
		{
			offset: 1 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=18h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
	})

	err = scheduler.Reset(incDelay())
	if err != nil {
		t.Fatalf("Scheduler reset failed: %v", err)
	}

	// Schedule starts from SyncModeFull again.
	assertSchedule(t, scheduler, []wantSchedule{
		// t=0, initial sync
		{
			offset: 0,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
		// t=6h
		{
			offset: 1 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModePartial,
		},
		// t=18h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdmsync.SyncModeFull,
		},
	})
}

// incDelay returns a function with a monotonically-increasing delay.
// Useful to space out starting entries evenly.
func incDelay() func() time.Duration {
	var delay time.Duration
	return func() time.Duration {
		delay++
		return delay
	}
}

func zeroDelay() time.Duration {
	return 0
}

func jamfInfo(e *types.JamfInventoryEntry) mdmsync.EntryInfo {
	return mdmsync.EntryInfo{
		SyncPeriodPartial: time.Duration(e.SyncPeriodPartial),
		SyncPeriodFull:    time.Duration(e.SyncPeriodFull),
	}
}

type wantSchedule struct {
	// offset is the offset of this entry in relation to the preceding entry.
	offset time.Duration
	entry  *types.JamfInventoryEntry
	mode   mdmsync.SyncMode
}

func assertSchedule(t *testing.T, scheduler *mdmsync.Scheduler[*types.JamfInventoryEntry], wantEntries []wantSchedule) {
	var elapsed time.Duration
	for i, want := range wantEntries {
		// Ignore nanos if the offset doesn't include any.
		// Makes testing simpler.
		var maxDiff time.Duration
		if want.offset%1000 == 0 {
			maxDiff = 2 // Up to 2ns of difference is OK.
		}

		// Assert NextOffset().
		gotOffset := scheduler.NextOffset()
		if got := scheduler.NextOffset(); want.offset-got > maxDiff {
			t.Errorf("NextOffset(%v) = %v, want %v", i, got, want.offset)
		}
		elapsed += gotOffset

		// Assert Next() schedule entry.
		t.Run(fmt.Sprintf("t=%v", elapsed), func(t *testing.T) {
			gotEntry := scheduler.Next()
			wantEntry := mdmsync.Entry[*types.JamfInventoryEntry]{
				Entry: want.entry,
				Mode:  want.mode,
			}
			if diff := cmp.Diff(
				wantEntry, gotEntry,
				cmpopts.IgnoreFields(gotEntry, "avoidReschedule", "offset"),
			); diff != "" {
				t.Errorf("Next(%v) mismatch (-want +got)\n%s", i, diff)
			}
		})
	}
}

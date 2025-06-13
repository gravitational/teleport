package mdm_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/mdm"
)

func TestSyncScheduler_completeSchedule(t *testing.T) {
	entries := []*types.JamfInventoryEntry{
		{
			FilterRsql:        "full+partial", // Abuse RSQL strings for easier debugging.
			SyncPeriodPartial: types.Duration(6 * time.Hour),
			SyncPeriodFull:    types.Duration(18 * time.Hour),
		},
		{
			FilterRsql:        "full only",
			SyncPeriodPartial: types.Duration(8 * time.Hour), // partial>=full means only full happens.
			SyncPeriodFull:    types.Duration(8 * time.Hour),
		},
		{
			FilterRsql:        "partial only",
			SyncPeriodPartial: types.Duration(5 * time.Hour),
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

	scheduler, err := mdm.NewSyncScheduler(entries, incDelay(), jamfInfo)
	if err != nil {
		t.Fatalf("NewSyncScheduler failed: %v", err)
	}

	assertSchedule(t, scheduler, []wantSchedule{
		// t=0, initial sync
		{
			offset: 0,
			entry:  fullPartial,
			mode:   mdm.SyncModeFull,
		},
		// t=1, initial sync
		{
			offset: 1,
			entry:  fullOnly,
			mode:   mdm.SyncModeFull,
		},
		// t=2, initial sync
		{
			offset: 1,
			entry:  partialOnly,
			mode:   mdm.SyncModePartial,
		},
		// t=5h
		{
			offset: 5 * time.Hour,
			entry:  partialOnly,
			mode:   mdm.SyncModePartial,
		},
		// t=6h
		{
			offset: 1 * time.Hour,
			entry:  fullPartial,
			mode:   mdm.SyncModePartial,
		},
		// t=8h
		{
			offset: 2 * time.Hour,
			entry:  fullOnly,
			mode:   mdm.SyncModeFull,
		},
		// t=10h
		{
			offset: 2 * time.Hour,
			entry:  partialOnly,
			mode:   mdm.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdm.SyncModePartial,
		},
		// t=15h
		{
			offset: 3 * time.Hour,
			entry:  partialOnly,
			mode:   mdm.SyncModePartial,
		},
		// t=16h
		{
			offset: 1 * time.Hour,
			entry:  fullOnly,
			mode:   mdm.SyncModeFull,
		},
		// t=18h
		{
			offset: 2 * time.Hour,
			entry:  fullPartial,
			mode:   mdm.SyncModeFull,
		},
		// t=20h
		{
			offset: 2 * time.Hour,
			entry:  partialOnly,
			mode:   mdm.SyncModePartial,
		},
		// t=24h
		{
			offset: 4 * time.Hour,
			entry:  fullPartial,
			mode:   mdm.SyncModePartial,
		},
		{
			offset: 1,
			entry:  fullOnly,
			mode:   mdm.SyncModeFull,
		},
		// and so on...
	})
}

func TestSyncScheduler_partialNotDivisor(t *testing.T) {
	// SyncPeriodPartial is not a divisor of SyncPeriodFull, meaning the schedules
	// don't line up perfectly.
	entry := &types.JamfInventoryEntry{
		SyncPeriodPartial: types.Duration(5 * time.Hour),
		SyncPeriodFull:    types.Duration(12 * time.Hour),
	}

	scheduler, err := mdm.NewSyncScheduler(
		[]*types.JamfInventoryEntry{entry}, zeroDelay, jamfInfo)
	if err != nil {
		t.Fatalf("NewSyncScheduler failed: %v", err)
	}

	assertSchedule(t, scheduler, []wantSchedule{
		{
			entry: entry,
			mode:  mdm.SyncModeFull,
		},
		// t=5h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModePartial,
		},
		// t=10h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModePartial,
		},
		// t=12h
		{
			offset: 2 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModeFull,
		},
		// t=17
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModePartial,
		},
		// t=22h
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModePartial,
		},
		// t=24
		{
			offset: 2 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModeFull,
		},
		// t=29
		{
			offset: 5 * time.Hour,
			entry:  entry,
			mode:   mdm.SyncModePartial,
		},
		// and so on...
	})
}

func TestSyncScheduler_scheduleCantBeExhausted(t *testing.T) {
	entries := []*types.JamfInventoryEntry{
		{
			FilterRsql:        "full+partial", // Abuse RSQL strings for easier debugging.
			SyncPeriodPartial: types.Duration(1 * time.Minute),
			SyncPeriodFull:    types.Duration(2 * time.Minute),
		},
		{
			FilterRsql:        "partial",
			SyncPeriodPartial: types.Duration(2 * time.Minute),
		},
		{
			FilterRsql:     "full",
			SyncPeriodFull: types.Duration(2 * time.Minute),
		},
	}

	scheduler, err := mdm.NewSyncScheduler(entries, incDelay(), jamfInfo)
	if err != nil {
		t.Fatalf("NewSyncScheduler failed: %v", err)
	}

	// Skip initial entries, so we get to the "loop".
	scheduler.Next()
	scheduler.Next()
	scheduler.Next()

	want := []wantSchedule{
		// t=1
		{
			entry: entries[0],
			mode:  mdm.SyncModePartial,
		},
		// t=2
		{
			entry: entries[0],
			mode:  mdm.SyncModeFull,
		},
		{
			entry: entries[1],
			mode:  mdm.SyncModePartial,
		},
		{
			entry: entries[2],
			mode:  mdm.SyncModeFull,
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
			wantErr: mdm.ErrScheduleNoEntries,
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
			wantErr: mdm.ErrScheduleEmpty,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := mdm.NewSyncScheduler(test.entries, zeroDelay, jamfInfo)
			if !errors.Is(err, test.wantErr) {
				t.Errorf("NewSyncScheduler returned err=%v, want %q", err, test.wantErr)
			}
		})
	}
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

func jamfInfo(e *types.JamfInventoryEntry) mdm.ScheduleEntryInfo {
	return mdm.ScheduleEntryInfo{
		SyncPeriodPartial: time.Duration(e.SyncPeriodPartial),
		SyncPeriodFull:    time.Duration(e.SyncPeriodFull),
	}
}

type wantSchedule struct {
	// offset is the offset of this entry in relation to the preceding entry.
	offset time.Duration
	entry  *types.JamfInventoryEntry
	mode   mdm.SyncMode
}

func assertSchedule(t *testing.T, scheduler *mdm.SyncScheduler[*types.JamfInventoryEntry], wantEntries []wantSchedule) {
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
			wantEntry := mdm.ScheduleEntry[*types.JamfInventoryEntry]{
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

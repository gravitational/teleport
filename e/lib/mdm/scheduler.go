package mdm

import (
	"errors"
	"slices"
	"time"

	"github.com/gravitational/trace"
)

// ErrScheduleNoEntries is returned when no entries are provided to the
// [SyncScheduler].
var ErrScheduleNoEntries = errors.New("no schedule entries provided")

// ErrScheduleEmpty is returned when all schedule entries are inactive.
var ErrScheduleEmpty = errors.New("schedule has no active entries")

// ScheduleEntry is an entry in the sync schedule.
type ScheduleEntry[E any] struct {
	// Entry is the underlying sync entry.
	Entry E
	// Mode is the sync mode to be used for the entry.
	Mode SyncMode

	avoidReschedule bool
	offset          time.Duration
}

// ScheduleEntryInfo is a subset of the information in sync inventory entries,
// like [types.JamfInventoryEntry].
type ScheduleEntryInfo struct {
	// SyncPeriodPartial is the PARTIAL sync period.
	// Zero or negavite disables PARTIAL syncs.
	SyncPeriodPartial time.Duration
	// SyncPeriodFull is the FULL sync period.
	// Zero or negavite disables FULL syncs.
	SyncPeriodFull time.Duration
}

// SyncScheduler calculates the sync schedule for MDM services.
//
// Call [NextOffset] to discover when the next sync entry is due. Call [Next] to
// "advance" time to the next entry and pop it from the schedule.
//
// Sync schedules are endless and will always return more entries.
type SyncScheduler[E any] struct {
	// schedule are the scheduled entries, sorted by ascending offset.
	// Guaranteed to always have at least one entry.
	schedule []ScheduleEntry[E]
	infoFn   func(E) ScheduleEntryInfo
}

// NewSyncScheduler creates a new MDM sync scheduler.
//
// `initialDelayFn` calculates the initial sync delay for entries. Guaranteed to
// be invoked once per entry in the `entries` slice, in input order.
//
// `infoFn` extracts a [ScheduleEntryInfo] from the underlying entry.
func NewSyncScheduler[E any](entries []E, initialDelayFn func() time.Duration, infoFn func(E) ScheduleEntryInfo) (*SyncScheduler[E], error) {
	switch {
	case len(entries) == 0:
		return nil, trace.Wrap(ErrScheduleNoEntries)
	case initialDelayFn == nil:
		return nil, trace.BadParameter("initialDelayFn required")
	case infoFn == nil:
		return nil, trace.BadParameter("infoFn required")
	}

	t := &SyncScheduler[E]{
		infoFn: infoFn,
	}
	t.initializeSchedule(entries, initialDelayFn)
	if len(t.schedule) == 0 {
		return nil, trace.Wrap(ErrScheduleEmpty)
	}

	return t, nil
}

// NextOffset returns when the next [ScheduleEntry] is due.
func (t *SyncScheduler[E]) NextOffset() time.Duration {
	return t.schedule[0].offset
}

// Next advances the schedule to the next [ScheduleEntry] and returns it.
func (t *SyncScheduler[E]) Next() ScheduleEntry[E] {
	// Pop next entry.
	entry := t.schedule[0]
	t.schedule = t.schedule[1:]

	// "Advance" schedule.
	for i := range t.schedule {
		t.schedule[i].offset -= entry.offset
	}

	// Reschedule future entries.
	if !entry.avoidReschedule {
		t.reschedule(entry.Entry, 0 /* initialDelay */, false /* initialSync */)
		t.sort()
	}

	return entry
}

func (t *SyncScheduler[E]) initializeSchedule(entries []E, initialDelayFn func() time.Duration) {
	for _, e := range entries {
		t.reschedule(e, initialDelayFn(), true /* initialSync */)
	}
	t.sort()
}

func (t *SyncScheduler[E]) reschedule(entry E, initialDelay time.Duration, initialSync bool) {
	// There are 2 "types" of schedules:
	//
	// 1) FULL + PARTIAL (PARTIAL < FULL)
	//    T0 ---- FULL ---- PARTIAL 1 ---- PARTIAL n (as many as can fit) ---- FULL ---- (Repeat)
	//
	//    Reschedule for PARTIALs and FULLs happens when time a FULL entry is
	//    popped, with exception of the initial FULL entry.
	//
	// 2) FULL only or PARTIAL only
	//    T0 ---- FULL or PARTIAL ---- (Repeat)
	//
	//    Reschedule happens whenever an entry is popped.

	info := t.infoFn(entry)
	hasFull := info.SyncPeriodFull > 0
	hasPartial := info.SyncPeriodPartial > 0

	switch {
	case hasFull && hasPartial && info.SyncPeriodPartial < info.SyncPeriodFull: // type "1"
		// Schedule initial FULL sync.
		if initialSync {
			t.schedule = append(t.schedule, ScheduleEntry[E]{
				Entry:           entry,
				Mode:            SyncModeFull,
				avoidReschedule: true,         // another FULL is in the schedule.
				offset:          initialDelay, // sync "immediately"
			})
		}

		// Schedule delayed FULL sync.
		fullOffset := initialDelay + info.SyncPeriodFull
		t.schedule = append(t.schedule, ScheduleEntry[E]{
			Entry:  entry,
			Mode:   SyncModeFull,
			offset: fullOffset,
		})

		// Schedule PARTIAL syncs in-between.
		partialOffset := initialDelay + info.SyncPeriodPartial
		for partialOffset < fullOffset {
			t.schedule = append(t.schedule, ScheduleEntry[E]{
				Entry:           entry,
				Mode:            SyncModePartial,
				avoidReschedule: true, // reschedule only on FULL.
				offset:          partialOffset,
			})
			partialOffset += info.SyncPeriodPartial
		}
	case hasFull || hasPartial: // type "2", FULL or PARTIAL
		var mode SyncMode
		var offset time.Duration

		// FULL takes precedence.
		if hasFull {
			offset = info.SyncPeriodFull
			mode = SyncModeFull
		} else {
			offset = info.SyncPeriodPartial
			mode = SyncModePartial
		}

		if initialSync {
			offset = initialDelay // sync "immediately"
		} else {
			offset += initialDelay
		}

		t.schedule = append(t.schedule, ScheduleEntry[E]{
			Entry:  entry,
			Mode:   mode,
			offset: offset,
		})
	}
}

func (t *SyncScheduler[E]) sort() {
	slices.SortFunc(t.schedule, func(a, b ScheduleEntry[E]) int {
		return int(a.offset - b.offset)
	})
}

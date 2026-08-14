package mdmsync

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/gravitational/trace"
)

// ErrScheduleNoEntries is returned when no entries are provided to the
// [Scheduler].
var ErrScheduleNoEntries = errors.New("no schedule entries provided")

// ErrScheduleEmpty is returned when all schedule entries are inactive.
var ErrScheduleEmpty = errors.New("schedule has no active entries")

// Entry is an entry in the sync schedule.
type Entry[E any] struct {
	// Entry is the underlying sync entry.
	Entry E
	// Mode is the sync mode to be used for the entry.
	Mode SyncMode

	avoidReschedule bool
	offset          time.Duration
}

// EntryInfo defines the partial and full schedule durations.
// It is typically a subset of the information in sync entries,
// like [types.JamfInventoryEntry].
type EntryInfo struct {
	// SyncPeriodPartial is the PARTIAL sync period.
	// Zero or negavite disables PARTIAL syncs.
	SyncPeriodPartial time.Duration
	// SyncPeriodFull is the FULL sync period.
	// Zero or negavite disables FULL syncs.
	SyncPeriodFull time.Duration
}

// Scheduler calculates the sync schedules.
//
// Call [NextOffset] to discover when the next sync entry is due. Call [Next] to
// "advance" time to the next entry and pop it from the schedule.
//
// Sync schedules are endless and will always return more entries.
type Scheduler[E any] struct {
	// schedule are the scheduled entries, sorted by ascending offset.
	// Guaranteed to always have at least one entry.
	schedule []Entry[E]
	infoFn   func(E) EntryInfo
	// entries are interval entries.
	entries []E
}

// New creates a new sync scheduler.
//
// `initialDelayFn` calculates the initial sync delay for entries. Guaranteed to
// be invoked once per entry in the `entries` slice, in input order.
//
// `infoFn` extracts a [EntryInfo] from the underlying entry.
func New[E any](entries []E, initialDelayFn func() time.Duration, infoFn func(E) EntryInfo) (*Scheduler[E], error) {
	switch {
	case len(entries) == 0:
		return nil, trace.Wrap(ErrScheduleNoEntries)
	case initialDelayFn == nil:
		return nil, trace.BadParameter("initialDelayFn required")
	case infoFn == nil:
		return nil, trace.BadParameter("infoFn required")
	}

	t := &Scheduler[E]{
		entries: slices.Clone(entries),
		infoFn:  infoFn,
	}
	t.initializeSchedule(initialDelayFn)
	if len(t.schedule) == 0 {
		return nil, trace.Wrap(ErrScheduleEmpty)
	}

	return t, nil
}

// NextOffset returns when the next [Entry] is due.
func (t *Scheduler[E]) NextOffset() time.Duration {
	return t.schedule[0].offset
}

// Next advances the schedule to the next [Entry] and returns it.
func (t *Scheduler[E]) Next() Entry[E] {
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

// Reset resets the schedule. Must only be called on a scheduler
// returned by [New].
//
// `delayFn` calculates the reset delay for each entries configured when
// creating a new schedule with [New], effectively delaying the first
// sync schedule after the reset.
func (t *Scheduler[E]) Reset(delayFn func() time.Duration) error {
	if delayFn == nil {
		return trace.BadParameter("delayFn is required")
	}
	// Reset existing schedules.
	t.schedule = t.schedule[:0]
	t.initializeSchedule(delayFn)
	if len(t.schedule) == 0 {
		return trace.Wrap(ErrScheduleEmpty)
	}
	return nil
}

func (t *Scheduler[E]) initializeSchedule(initialDelayFn func() time.Duration) {
	for _, e := range t.entries {
		t.reschedule(e, initialDelayFn(), true /* initialSync */)
	}
	t.sort()
}

func (t *Scheduler[E]) reschedule(entry E, initialDelay time.Duration, initialSync bool) {
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
			t.schedule = append(t.schedule, Entry[E]{
				Entry:           entry,
				Mode:            SyncModeFull,
				avoidReschedule: true,         // another FULL is in the schedule.
				offset:          initialDelay, // sync "immediately"
			})
		}

		// Schedule delayed FULL sync.
		fullOffset := initialDelay + info.SyncPeriodFull
		t.schedule = append(t.schedule, Entry[E]{
			Entry:  entry,
			Mode:   SyncModeFull,
			offset: fullOffset,
		})

		// Schedule PARTIAL syncs in-between.
		partialOffset := initialDelay + info.SyncPeriodPartial
		for partialOffset < fullOffset {
			t.schedule = append(t.schedule, Entry[E]{
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

		t.schedule = append(t.schedule, Entry[E]{
			Entry:  entry,
			Mode:   mode,
			offset: offset,
		})
	}
}

func (t *Scheduler[E]) sort() {
	slices.SortFunc(t.schedule, func(a, b Entry[E]) int {
		return cmp.Compare(a.offset, b.offset)
	})
}

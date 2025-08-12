package desktop

import (
	"context"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

// streamID uniquely identifies a stream
type streamID struct {
	path        string
	directoryID directoryID
	write       bool
}

// streamEvent is an abstraction of read/write events
// so that we need only one compactor implementation.
type streamEvent interface {
	Base() events.AuditEvent
	IsWriteEvent() bool
	GetDirectoryID() directoryID
	GetPath() string
	GetOffset() uint64
	GetLength() uint64
	SetLength(uint64)
}

// stream identifies a set of reads/writes
// to a particular file within some period of time.
type stream struct {
	expireTime time.Time
	events     []streamEvent
	timer      *time.Timer
	done       chan struct{}
}

// auditCompactor tracks streams within a particular desktop session
type auditCompactor struct {
	refreshInterval  time.Duration
	maxDelayInterval time.Duration
	emitFn           func(context.Context, events.AuditEvent)
	streams          map[streamID]*stream
	streamLock       sync.Mutex
}

func newAuditCompactor(refreshInterval, maxDelayInterval time.Duration, emitFn func(context.Context, events.AuditEvent)) auditCompactor {
	return auditCompactor{
		refreshInterval:  refreshInterval,
		maxDelayInterval: maxDelayInterval,
		emitFn:           emitFn,
		streams:          map[streamID]*stream{},
	}
}

func (s *stream) emitEvents(ctx context.Context, emitFn func(ctx context.Context, evnt events.AuditEvent)) {
	for _, evnt := range s.compactEvents() {
		emitFn(ctx, evnt.Base())
	}
}

func (s *stream) compactEvents() []streamEvent {
	offsetMapping := map[uint64][]streamEvent{}
	for _, evnt := range s.events {
		offsetMapping[evnt.GetOffset()] = append(offsetMapping[evnt.GetOffset()], evnt)
	}

	var finalEvents []streamEvent
	for len(offsetMapping) > 0 {
		// Find the read/write event with the lowest offset
		// so that we may greedily search for the longest
		// contiguous segment we can produce
		smallestKey := slices.Sorted(maps.Keys(offsetMapping))
		evnt := offsetMapping[smallestKey[0]][0]
		sequentialEvents, sequenceLength := s.compact(evnt, offsetMapping)
		base := sequentialEvents[0]

		// Remove each event in this sequence from the map
		for _, subsequent := range sequentialEvents {
			offset := subsequent.GetOffset()
			evnts := offsetMapping[offset]
			deleteIdx := slices.Index(evnts, subsequent)
			evnts = slices.Delete(evnts, deleteIdx, deleteIdx+1)
			if len(evnts) > 0 {
				offsetMapping[offset] = evnts
			} else {
				delete(offsetMapping, offset)
			}
		}
		base.SetLength(sequenceLength)
		finalEvents = append(finalEvents, base)
	}

	return finalEvents
}

// compact finds the longest contiguous set of reads/writes following the given 'evnt'
func (s *stream) compact(evnt streamEvent, eventsByOffset map[uint64][]streamEvent) ([]streamEvent, uint64) {
	// Determine the offset at which the next contiguous segment must start.
	offset := evnt.GetOffset() + evnt.GetLength()
	// Consule the map for any events with this offset.
	if len(eventsByOffset[offset]) > 0 {
		// There may be multiple candidate segments to follow.
		// Try each of them out and select the longest contiguous set of segments
		var winner []streamEvent
		var maxLength uint64
		for _, choice := range eventsByOffset[offset] {
			// TODO: We could probably speed this up with memoization/dynamic programmming,
			// but this code is fairly readable as-is and it's probably not likely that
			// we'll end up with too many possible paths in production.
			// Recursively evaluate each option.
			option, length := s.compact(choice, eventsByOffset)
			if length > maxLength {
				winner = option
				maxLength = length
			}
		}
		return append([]streamEvent{evnt}, winner...), maxLength + evnt.GetLength()
	}
	return []streamEvent{evnt}, evnt.GetLength()
}

func (s *stream) addEvent(evnt streamEvent) {
	s.events = append(s.events, evnt)
}

func (a *auditCompactor) handleEvent(ctx context.Context, evnt streamEvent) {
	// Does the event exist in the stream?
	key := streamID{
		write:       evnt.IsWriteEvent(),
		directoryID: evnt.GetDirectoryID(),
		path:        evnt.GetPath(),
	}

	newStream := true
	a.streamLock.Lock()
	defer a.streamLock.Unlock()

	if stream, exists := a.streams[key]; exists {
		// We're currently tracking this stream
		// Temporarily stop the timer (if possible)
		alreadyFired := !stream.timer.Stop()
		if !alreadyFired {
			// Update the current stream. It is a continuation of the current stream
			// and the timer has not yet fired for it.
			stream.addEvent(evnt)
			// Reset the timer either to the refresh interval, or until
			// the stream's expiration time
			stream.timer.Reset(time.Duration(math.Min(float64(a.refreshInterval), float64(time.Until(stream.expireTime)))))
			newStream = false
		} else {
			// The timer has already fired. Stop tracking this stream.
			// A new stream will be created below to handle this event.
			delete(a.streams, key)
		}
	}

	// We need to create a new stream due to one of the following:
	//   - We are not tracking any such stream yet.
	//   - We were tracking this stream but the timer has already fired.
	if newStream {
		s := &stream{
			done:       make(chan struct{}),
			expireTime: time.Now().Add(a.maxDelayInterval),
			events:     []streamEvent{evnt},
		}
		s.timer = time.AfterFunc(a.refreshInterval, func() {
			// Close done channel so that the 'flush' function can
			// verify that this goroutine has completed its work.
			defer close(s.done)
			a.streamLock.Lock()
			delete(a.streams, key)
			a.streamLock.Unlock()
			s.emitEvents(ctx, a.emitFn)

		})
		a.streams[key] = s
	}
}

// flush immediately compacts and emits audit events for all
// in-progress streams and blocks until completion.
func (a *auditCompactor) flush(ctx context.Context) {
	a.streamLock.Lock()
	wait := []chan struct{}{}
	for streamID, stream := range a.streams {
		if stream.timer.Stop() {
			// If we successfully stop the timer before it fires,
			// go ahead and emit the audit event.
			stream.emitEvents(ctx, a.emitFn)
			delete(a.streams, streamID)
		} else {
			// The timer was already firing, so wait until
			// the emitFn as been executed by the underlying goroutine.
			wait = append(wait, stream.done)
		}
	}
	// Unlock so that we may unblock timer functions.
	a.streamLock.Unlock()
	// Wait for pending timers to complete
	// We use our own "done" channel rather than the timer's
	// because we need to know that the timer's underlying goroutine.
	for _, doneChan := range wait {
		<-doneChan
	}
}

// Adapters for current read/write audit events.

type readEvent struct {
	*events.DesktopSharedDirectoryRead
}

func (r *readEvent) SetLength(len uint64) {
	r.Length = uint32(len)
}

func (r *readEvent) GetLength() uint64 {
	return uint64(r.Length)
}

func (r *readEvent) GetOffset() uint64 {
	return r.Offset
}

func (r *readEvent) GetPath() string {
	return r.Path
}

func (r *readEvent) IsWriteEvent() bool {
	return true
}

func (r *readEvent) GetDirectoryID() directoryID {
	return directoryID(r.DirectoryID)
}

func (r *readEvent) Base() events.AuditEvent {
	return r.DesktopSharedDirectoryRead
}

type writeEvent struct {
	*events.DesktopSharedDirectoryWrite
}

func (r *writeEvent) SetLength(len uint64) {
	r.Length = uint32(len)
}

func (r *writeEvent) GetLength() uint64 {
	return uint64(r.Length)
}

func (r *writeEvent) GetOffset() uint64 {
	return r.Offset
}

func (r *writeEvent) GetPath() string {
	return r.Path
}

func (r *writeEvent) IsWriteEvent() bool {
	return false
}

func (r *writeEvent) GetDirectoryID() directoryID {
	return directoryID(r.DirectoryID)
}

func (r *writeEvent) Base() events.AuditEvent {
	return r.DesktopSharedDirectoryWrite
}

func (a *auditCompactor) handleRead(ctx context.Context, evnt *events.DesktopSharedDirectoryRead) {
	a.handleEvent(ctx, &readEvent{DesktopSharedDirectoryRead: evnt})
}

func (a *auditCompactor) handleWrite(ctx context.Context, evnt *events.DesktopSharedDirectoryWrite) {
	a.handleEvent(ctx, &writeEvent{DesktopSharedDirectoryWrite: evnt})
}

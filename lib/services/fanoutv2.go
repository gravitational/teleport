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

package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	fb "github.com/gravitational/teleport/lib/utils/fanoutbuffer"
)

var errFanoutReset = errors.New("event fanout system reset")

var errFanoutClosed = errors.New("event fanout system closed")

var errWatcherClosed = errors.New("event watcher closed")

type FanoutV2Config struct {
	Capacity    uint64
	GracePeriod time.Duration
	Clock       clockwork.Clock
}

func (c *FanoutV2Config) SetDefaults() {
	if c.Capacity == 0 {
		c.Capacity = 1024
	}

	if c.GracePeriod == 0 {
		// the most frequent periodic writes happen once per minute. a grace period of 59s is a
		// reasonable default, since a cursor that can't catch up within 59s is likely to continue
		// to fall further behind.
		c.GracePeriod = 59 * time.Second
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
}

// FanoutV2 is a drop-in replacement for Fanout that offers a different set of performance characteristics. It
// supports variable-size buffers to better accommodate large spikes in event load, but it does so at the cost
// of higher levels of context-switching since all readers are notified of all events as well as higher baseline
// memory usage due to relying on a large shared buffer.
type FanoutV2 struct {
	cfg    FanoutV2Config
	rw     sync.RWMutex
	buf    *fb.Buffer[fanoutV2Entry]
	init   *fanoutV2Init
	closed bool
}

// NewFanoutV2 allocates a new fanout instance.
func NewFanoutV2(cfg FanoutV2Config) *FanoutV2 {
	cfg.SetDefaults()
	f := &FanoutV2{
		cfg: cfg,
	}
	f.setup()
	return f
}

// NewStream gets a new event stream. The provided context will form the basis of
// the stream's close context. Note that streams *must* be explicitly closed when
// completed in order to avoid performance issues.
func (f *FanoutV2) NewStream(ctx context.Context, watch types.Watch) stream.Stream[types.Event] {
	f.rw.RLock()
	defer f.rw.RUnlock()
	if f.closed {
		return stream.Fail[types.Event](errFanoutClosed)
	}
	return &fanoutV2Stream{
		closeContext: ctx,
		cursor:       f.buf.NewCursor(),
		init:         f.init,
		watch:        watch,
	}
}

func (f *FanoutV2) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, cancel := context.WithCancel(ctx)

	w := &streamWatcher{
		cancel: cancel,
		events: make(chan types.Event, 16),
		// note that we don't use ctx.Done() because we want to wait until
		// we've finished stream closure and extracted the resulting error
		// before signaling watcher closure.
		done: make(chan struct{}),
	}

	go w.run(ctx, f.NewStream(ctx, watch))

	return w, nil
}

type streamWatcher struct {
	cancel context.CancelFunc
	events chan types.Event
	done   chan struct{}
	emux   sync.Mutex
	err    error
}

func (w *streamWatcher) run(ctx context.Context, stream stream.Stream[types.Event]) {
	defer func() {
		if err := stream.Done(); err != nil {
			w.emux.Lock()
			w.err = err
			w.emux.Unlock()
		}
		close(w.done)
	}()

	for stream.Next() {
		select {
		case w.events <- stream.Item():
		case <-ctx.Done():
			return
		}
	}
}

func (w *streamWatcher) Events() <-chan types.Event {
	return w.events
}

func (w *streamWatcher) Done() <-chan struct{} {
	return w.done
}

func (w *streamWatcher) Close() error {
	w.cancel()
	return nil
}

func (w *streamWatcher) Error() error {
	w.emux.Lock()
	defer w.emux.Unlock()
	if w.err != nil {
		return w.err
	}

	select {
	case <-w.Done():
		return errWatcherClosed
	default:
		return nil
	}
}

func (f *FanoutV2) Emit(events ...types.Event) {
	f.rw.RLock()
	defer f.rw.RUnlock()
	if !f.init.isInit() {
		panic("Emit called on uninitialized fanout instance")
	}

	if f.closed {
		// emit racing with close is fairly common with how we
		// use this type, so its best to ignore it.
		return
	}

	// batch-process events to minimize the need to acquire the
	// fanout buffer's write lock (batching writes has a non-trivial
	// impact on fanout buffer benchmarks due to each cursor needing
	// to acquire the read lock individually).
	var ebuf [16]fanoutV2Entry
	for len(events) > 0 {
		n := min(len(events), len(ebuf))
		for i := 0; i < n; i++ {
			ebuf[i] = newFanoutV2Entry(events[i])
		}
		f.buf.Append(ebuf[:n]...)
		events = events[n:]
	}
}

func (f *FanoutV2) Reset() {
	f.rw.Lock()
	defer f.rw.Unlock()
	if f.closed {
		return
	}
	f.teardown(errFanoutReset)
	f.setup()
}

func (f *FanoutV2) Close() error {
	f.rw.Lock()
	defer f.rw.Unlock()
	if f.closed {
		return nil
	}
	f.teardown(errFanoutClosed)
	f.closed = true
	return nil
}

func (f *FanoutV2) setup() {
	f.init = newFanoutV2Init()
	f.buf = fb.NewBuffer[fanoutV2Entry](fb.Config{
		Capacity:    f.cfg.Capacity,
		GracePeriod: f.cfg.GracePeriod,
		Clock:       f.cfg.Clock,
	})
}

func (f *FanoutV2) teardown(err error) {
	f.init.setErr(err)
	f.buf.Close()
}

func (f *FanoutV2) SetInit(kinds []types.WatchKind) {
	f.rw.RLock()
	defer f.rw.RUnlock()

	km := make(map[resourceKind]types.WatchKind, len(kinds))
	for _, kind := range kinds {
		km[resourceKind{kind: kind.Kind, subKind: kind.SubKind}] = kind
	}
	f.init.setInit(km)
}

// fanoutV2Stream is a stream.Stream implementation that streams events from a FanoutV2 instance. It handles filtering
// out events that don't match the provided watch parameters, and construction of custom init events.
type fanoutV2Stream struct {
	closeContext context.Context
	cursor       *fb.Cursor[fanoutV2Entry]
	init         *fanoutV2Init
	watch        types.Watch
	rbuf         [16]fanoutV2Entry
	n, next      int
	event        types.Event
	err          error
}

func (s *fanoutV2Stream) Next() (ok bool) {
	if s.init != nil {
		s.event, s.err = s.waitInit(s.closeContext)
		s.init = nil
		return s.err == nil
	}
	for {
		// try finding the next matching event within read buffer
		var ok bool
		s.event, ok, s.err = s.advance()
		if ok {
			return true
		}

		// read a new batch of events into the read buffer
		s.next = 0
		s.n, s.err = s.cursor.Read(s.closeContext, s.rbuf[:])
		if s.err != nil {
			if errors.Is(s.err, fb.ErrBufferClosed) {
				s.err = errFanoutReset
			}
			return false
		}
	}
}

func (s *fanoutV2Stream) Item() types.Event {
	return s.event
}

func (s *fanoutV2Stream) Done() error {
	s.cursor.Close()
	return s.err
}

// waitInit waits for fanout initialization and builds an appropriate init event.
func (s *fanoutV2Stream) waitInit(ctx context.Context) (types.Event, error) {
	confirmedKinds, err := s.init.wait(ctx)
	if err != nil {
		return types.Event{}, trace.Wrap(err)
	}

	validKinds := make([]types.WatchKind, 0, len(s.watch.Kinds))
	for _, requested := range s.watch.Kinds {
		k := resourceKind{kind: requested.Kind, subKind: requested.SubKind}
		if configured, ok := confirmedKinds[k]; !ok || !configured.Contains(requested) {
			if s.watch.AllowPartialSuccess {
				continue
			}
			return types.Event{}, trace.BadParameter("resource type %q is not supported by this event stream", requested.Kind)
		}
		validKinds = append(validKinds, requested)
	}

	if len(validKinds) == 0 {
		return types.Event{}, trace.BadParameter("none of the requested resources are supported by this fanoutWatcher")
	}

	return types.Event{Type: types.OpInit, Resource: types.NewWatchStatus(validKinds)}, nil
}

// advance advances through the stream's internal read buffer looking for the
// next event that matches our specific watch parameters.
func (f *fanoutV2Stream) advance() (event types.Event, ok bool, err error) {
	for f.next < f.n {
		entry := f.rbuf[f.next]
		f.next++

		if entry.Event.Resource == nil {
			// events with no associated resources are special cases (e.g. OpUnreliable), and are
			// emitted to all watchers.
			return entry.Event, true, nil
		}

		for _, kind := range f.watch.Kinds {
			match, err := kind.Matches(entry.Event)
			if err != nil {
				return types.Event{}, false, trace.Wrap(err)
			}

			if !match {
				continue
			}

			if kind.LoadSecrets {
				return entry.EventWithSecrets, true, nil
			}
			return entry.Event, true, nil
		}
	}

	return types.Event{}, false, nil
}

// fanoutV2Entry is the underlying buffer entry that is fanned out to all
// cursors. Individual streams decide if they care about the version of the
// event with or without secrets based on their parameters.
type fanoutV2Entry struct {
	Event            types.Event
	EventWithSecrets types.Event
}

func newFanoutV2Entry(event types.Event) fanoutV2Entry {
	if e, err := client.EventToGRPC(event); err == nil {
		if b, err := proto.Marshal(e); err == nil {
			if f := protoreflect.RawFields(b); f.IsValid() {
				event.PreEncodedEventToGRPC = f
			}
		}
	}
	eventWithoutSecrets := filterEventSecrets(event)
	if len(eventWithoutSecrets.PreEncodedEventToGRPC) < 1 {
		if e, err := client.EventToGRPC(eventWithoutSecrets); err == nil {
			if b, err := proto.Marshal(e); err == nil {
				if f := protoreflect.RawFields(b); f.IsValid() {
					eventWithoutSecrets.PreEncodedEventToGRPC = f
				}
			}
		}
	}

	return fanoutV2Entry{
		Event:            eventWithoutSecrets,
		EventWithSecrets: event,
	}
}

func filterEventSecrets(event types.Event) types.Event {
	if r, ok := event.Resource.(types.ResourceWithSecrets); ok {
		event.Resource = r.WithoutSecrets()
		event.PreEncodedEventToGRPC = nil
	}

	// WebSessions do not implement the ResourceWithSecrets interface.
	if r, ok := event.Resource.(types.WebSession); ok {
		event.Resource = r.WithoutSecrets()
		event.PreEncodedEventToGRPC = nil
	}

	return event
}

type resourceKind struct {
	kind    string
	subKind string
}

// fanoutV2Init is a helper for blocking on and distributing the init event for a fanout
// instance. It uses a channel as both the init signal and a memory barrier to ensure
// good concurrent performance, and it is allocated behind a pointer so that it can be
// easily termianted and replaced during resets, ensuring that we don't need to handle
// edge-cases around old streams observing the wrong event/error.
type fanoutV2Init struct {
	once  sync.Once
	ch    chan struct{}
	kinds map[resourceKind]types.WatchKind
	err   error
}

func newFanoutV2Init() *fanoutV2Init {
	return &fanoutV2Init{
		ch: make(chan struct{}),
	}
}

func (i *fanoutV2Init) setInit(kinds map[resourceKind]types.WatchKind) {
	i.once.Do(func() {
		i.kinds = kinds
		close(i.ch)
	})
}

func (i *fanoutV2Init) setErr(err error) {
	i.once.Do(func() {
		i.err = err
		close(i.ch)
	})
}

func (i *fanoutV2Init) wait(ctx context.Context) (kinds map[resourceKind]types.WatchKind, err error) {
	select {
	case <-i.ch:
		return i.kinds, i.err
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}

func (i *fanoutV2Init) isInit() bool {
	select {
	case <-i.ch:
		return true
	default:
		return false
	}
}

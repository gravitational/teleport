/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// makeStream returns (evts, errs) channels pre-loaded with the given events,
// matching the StreamSessionEvents contract: evts is buffered and closed;
// errs is an open, empty channel (no error → never written to, never closed).
func makeStream(evs []apievents.AuditEvent) (<-chan apievents.AuditEvent, <-chan error) {
	ch := make(chan apievents.AuditEvent, len(evs)+1)
	for _, e := range evs {
		ch <- e
	}
	close(ch)
	return ch, make(chan error, 1) // open, never written — not closed
}

// makeErrorStream returns a stream that immediately sends an error.
// The events channel is closed (empty) and the error channel carries the error.
func makeErrorStream(err error) (<-chan apievents.AuditEvent, <-chan error) {
	ch := make(chan apievents.AuditEvent)
	close(ch)
	errs := make(chan error, 1)
	errs <- err
	return ch, errs
}

// appEvent builds an AppSessionRequest with the given timestamp, used as a
// stand-in event with a known time.
func appEvent(t *testing.T, ts time.Time) apievents.AuditEvent {
	t.Helper()
	return &apievents.AppSessionRequest{
		Metadata: apievents.Metadata{
			Time: ts,
			Type: AppSessionRequestEvent,
		},
	}
}

func timestamps(evs []apievents.AuditEvent) []time.Time {
	out := make([]time.Time, len(evs))
	for i, e := range evs {
		out[i] = e.GetTime()
	}
	return out
}

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func sec(n int) time.Time { return base.Add(time.Duration(n) * time.Second) }

// drainStream reads all events from evts until it is closed, then checks errs
// for a pending error. This mirrors the StreamSessionEvents contract: the events
// channel is always closed when the stream ends (success or failure); errors are
// sent on the error channel and can be checked non-blockingly after closure.
func drainStream(ctx context.Context, evts <-chan apievents.AuditEvent, errs <-chan error) ([]apievents.AuditEvent, error) {
	var out []apievents.AuditEvent
	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				select {
				case err := <-errs:
					return out, err
				default:
					return out, nil
				}
			}
			out = append(out, evt)
		case <-ctx.Done():
			return out, ctx.Err()
		}
	}
}

func TestMergeStreams_TwoStreams(t *testing.T) {
	t.Parallel()

	// stream A: t=0, t=2, t=4
	// stream B: t=1, t=3, t=5
	// expected merged: t=0,1,2,3,4,5
	streamA, errA := makeStream([]apievents.AuditEvent{appEvent(t, sec(0)), appEvent(t, sec(2)), appEvent(t, sec(4))})
	streamB, errB := makeStream([]apievents.AuditEvent{appEvent(t, sec(1)), appEvent(t, sec(3)), appEvent(t, sec(5))})

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{streamA, streamB},
		[]<-chan error{errA, errB},
	)

	got, err := drainStream(context.Background(), evts, errs)
	require.NoError(t, err)
	require.Len(t, got, 6)

	want := []time.Time{sec(0), sec(1), sec(2), sec(3), sec(4), sec(5)}
	require.Equal(t, want, timestamps(got))
}

func TestMergeStreams_EmptyStream(t *testing.T) {
	t.Parallel()

	// one populated, one empty
	streamA, errA := makeStream([]apievents.AuditEvent{appEvent(t, sec(0)), appEvent(t, sec(1))})
	streamB, errB := makeStream(nil)

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{streamA, streamB},
		[]<-chan error{errA, errB},
	)

	got, err := drainStream(context.Background(), evts, errs)
	require.NoError(t, err)
	require.Equal(t, []time.Time{sec(0), sec(1)}, timestamps(got))
}

func TestMergeStreams_AllEmpty(t *testing.T) {
	t.Parallel()

	s1, e1 := makeStream(nil)
	s2, e2 := makeStream(nil)

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{s1, s2},
		[]<-chan error{e1, e2},
	)

	got, err := drainStream(context.Background(), evts, errs)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestMergeStreams_ErrorPropagates(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("upstream error")
	streamA, errA := makeStream([]apievents.AuditEvent{appEvent(t, sec(0))})
	streamB, errB := makeErrorStream(sentinel)

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{streamA, streamB},
		[]<-chan error{errA, errB},
	)

	_, err := drainStream(context.Background(), evts, errs)
	require.ErrorIs(t, err, sentinel)
}

func TestMergeStreams_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Infinite blocking stream — never sends any event.
	blocked := make(chan apievents.AuditEvent)
	blockedErr := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	evts, errs := events.MergeStreams(ctx,
		[]<-chan apievents.AuditEvent{blocked},
		[]<-chan error{blockedErr},
	)

	_, err := drainStream(context.Background(), evts, errs)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMergeStreams_ThreeStreams(t *testing.T) {
	t.Parallel()

	// Verify heap ordering with 3 interleaved streams.
	s1, e1 := makeStream([]apievents.AuditEvent{appEvent(t, sec(0)), appEvent(t, sec(3)), appEvent(t, sec(6))})
	s2, e2 := makeStream([]apievents.AuditEvent{appEvent(t, sec(1)), appEvent(t, sec(4)), appEvent(t, sec(7))})
	s3, e3 := makeStream([]apievents.AuditEvent{appEvent(t, sec(2)), appEvent(t, sec(5)), appEvent(t, sec(8))})

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{s1, s2, s3},
		[]<-chan error{e1, e2, e3},
	)

	got, err := drainStream(context.Background(), evts, errs)
	require.NoError(t, err)
	require.Len(t, got, 9)

	want := []time.Time{sec(0), sec(1), sec(2), sec(3), sec(4), sec(5), sec(6), sec(7), sec(8)}
	require.Equal(t, want, timestamps(got))
}

func TestMergeStreams_SingleStream(t *testing.T) {
	t.Parallel()

	s, e := makeStream([]apievents.AuditEvent{appEvent(t, sec(2)), appEvent(t, sec(5))})

	evts, errs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{s},
		[]<-chan error{e},
	)

	got, err := drainStream(context.Background(), evts, errs)
	require.NoError(t, err)
	require.Equal(t, []time.Time{sec(2), sec(5)}, timestamps(got))
}

func TestMergeStreams_PanicOnMismatchedSlices(t *testing.T) {
	t.Parallel()

	s, _ := makeStream(nil)
	require.Panics(t, func() {
		events.MergeStreams(context.Background(),
			[]<-chan apievents.AuditEvent{s},
			[]<-chan error{},
		)
	})
}

// TestMergeStreams_IOEOFOnErrChannel verifies that a nil error sent on the
// error channel (which some implementations use to signal "done") is treated
// as EOF, not propagated as a real error.
func TestMergeStreams_NilErrTreatedAsEOF(t *testing.T) {
	t.Parallel()

	ch := make(chan apievents.AuditEvent, 1)
	ch <- appEvent(t, sec(0))
	close(ch)

	errs := make(chan error, 1)
	errs <- nil // nil error = stream done
	close(errs)

	evts, outErrs := events.MergeStreams(context.Background(),
		[]<-chan apievents.AuditEvent{ch},
		[]<-chan error{errs},
	)

	got, err := drainStream(context.Background(), evts, outErrs)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

// Ensure the test file compiles even though AppSessionRequestEvent is used.
const AppSessionRequestEvent = "app.session.request"



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

package player_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
)

func TestBasicStream(t *testing.T) {
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 3},
	})
	require.NoError(t, err)

	require.NoError(t, p.Play())

	count := 0
	for range p.C() {
		count++
	}

	require.Equal(t, 3, count)
	require.NoError(t, p.Err())
}

func TestPlayPause(t *testing.T) {
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 3},
	})
	require.NoError(t, err)

	// pausing an already paused player should be a no-op
	require.NoError(t, p.Pause())
	require.NoError(t, p.Pause())

	// toggling back and forth between play and pause
	// should not impact our ability to receive all
	// 3 events
	require.NoError(t, p.Play())
	require.NoError(t, p.Pause())
	require.NoError(t, p.Play())

	count := 0
	for range p.C() {
		count++
	}

	require.Equal(t, 3, count)
}

func TestAppliesTiming(t *testing.T) {
	for _, test := range []struct {
		desc    string
		speed   float64
		advance time.Duration
	}{
		{
			desc:    "half speed",
			speed:   0.5,
			advance: 2000 * time.Millisecond,
		},
		{
			desc:    "normal speed",
			speed:   1.0,
			advance: 1000 * time.Millisecond,
		},
		{
			desc:    "double speed",
			speed:   2.0,
			advance: 500 * time.Millisecond,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			clk := clockwork.NewFakeClock()
			p, err := player.New(&player.Config{
				Clock:     clk,
				SessionID: "test-session",
				Streamer:  &simpleStreamer{count: 3, delay: 1000},
			})
			require.NoError(t, err)

			require.NoError(t, p.SetSpeed(test.speed))
			require.NoError(t, p.Play())

			clk.BlockUntil(1) // player is now waiting to emit event 0

			// advance to next event (player will have emitted event 0
			// and will be waiting to emit event 1)
			clk.Advance(test.advance)
			clk.BlockUntil(1)
			evt := <-p.C()
			require.Equal(t, int64(0), evt.GetIndex())

			// repeat the process (emit event 1, wait for event 2)
			clk.Advance(test.advance)
			clk.BlockUntil(1)
			evt = <-p.C()
			require.Equal(t, int64(1), evt.GetIndex())

			// advance the player to allow event 2 to be emitted
			clk.Advance(test.advance)
			evt = <-p.C()
			require.Equal(t, int64(2), evt.GetIndex())

			// channel should be closed
			_, ok := <-p.C()
			require.False(t, ok, "player should be closed")
		})
	}
}

func TestClose(t *testing.T) {
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 2, delay: 1000},
	})
	require.NoError(t, err)

	require.NoError(t, p.Play())

	clk.BlockUntil(1) // player is now waiting to emit event 0

	// advance to next event (player will have emitted event 0
	// and will be waiting to emit event 1)
	clk.Advance(1001 * time.Millisecond)
	clk.BlockUntil(1)
	evt := <-p.C()
	require.Equal(t, int64(0), evt.GetIndex())

	require.NoError(t, p.Close())

	// channel should have been closed
	_, ok := <-p.C()
	require.False(t, ok, "player channel should have been closed")
	require.NoError(t, p.Err())
	require.Equal(t, int64(1000), p.LastPlayed())
}

func TestSeekForward(t *testing.T) {
	clk := clockwork.NewFakeClock()

	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 10, delay: 1000},
	})
	require.NoError(t, err)
	require.NoError(t, p.Play())

	clk.BlockUntil(1) // player is now waiting to emit event 0

	// advance playback until right before the last event
	p.SetPos(9001 * time.Millisecond)

	// advance the clock to unblock the player
	// (it should now spit out all but the last event in rapid succession)
	clk.Advance(1001 * time.Millisecond)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		for evt := range p.C() {
			t.Logf("got event %v (delay=%v)", evt.GetID(), evt.GetCode())
		}
	}()

	clk.BlockUntil(1)
	require.Equal(t, int64(9000), p.LastPlayed())

	clk.Advance(999 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		require.FailNow(t, "player hasn't closed in time")
	}
}

func TestSeekForwardTwice(t *testing.T) {
	clk := clockwork.NewRealClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 1, delay: 6000},
	})
	require.NoError(t, err)
	t.Cleanup(func() { p.Close() })
	require.NoError(t, p.Play())

	time.Sleep(100 * time.Millisecond)
	p.SetPos(500 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	p.SetPos(5900 * time.Millisecond)

	select {
	case <-p.C():
	case <-time.After(5 * time.Second):
		require.FailNow(t, "event not emitted on time")
	}
}

// TestInterruptsDelay tests that the player responds to playback
// controls even when it is waiting to emit an event.
func TestInterruptsDelay(t *testing.T) {
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 3, delay: 5000},
	})
	require.NoError(t, err)
	require.NoError(t, p.Play())

	t.Cleanup(func() { p.Close() })

	clk.BlockUntil(1) // player is now waiting to emit event 0

	// emulate the user seeking forward while the player is waiting..
	p.SetPos(10_001 * time.Millisecond)

	// expect event 0 and event 1 to be emitted right away
	// even without advancing the clock
	evt0 := <-p.C()
	evt1 := <-p.C()

	require.Equal(t, int64(0), evt0.GetIndex())
	require.Equal(t, int64(1), evt1.GetIndex())
}

func TestRewind(t *testing.T) {
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:     clk,
		SessionID: "test-session",
		Streamer:  &simpleStreamer{count: 10, delay: 1000},
	})
	require.NoError(t, err)
	require.NoError(t, p.Play())

	// play through 7 events at regular speed
	for i := 0; i < 7; i++ {
		clk.BlockUntil(1)                    // player is now waiting to emit event
		clk.Advance(1000 * time.Millisecond) // unblock event
		<-p.C()                              // read event
	}

	// now "rewind" to the point just prior to event index 3 (4000 ms into session)
	clk.BlockUntil(1)
	p.SetPos(3900 * time.Millisecond)

	// when we advance the clock, we expect the following behavior:
	// - event index 7 (which we were blocked on) comes out right away
	// - playback restarts, events 0 through 2 are emitted immediately
	// - event index 3 is emitted after another 100ms
	clk.Advance(1000 * time.Millisecond)
	require.Equal(t, int64(7), (<-p.C()).GetIndex())
	require.Equal(t, int64(0), (<-p.C()).GetIndex(), "expected playback to retart for rewind")
	require.Equal(t, int64(1), (<-p.C()).GetIndex(), "expected rapid streaming up to rewind point")
	require.Equal(t, int64(2), (<-p.C()).GetIndex())
	clk.BlockUntil(1)
	clk.Advance(100 * time.Millisecond)
	require.Equal(t, int64(3), (<-p.C()).GetIndex())

	p.Close()
}

// simpleStreamer streams a fake session that contains
// count events, emitted at a particular interval
type simpleStreamer struct {
	count int64
	delay int64 // milliseconds
}

func (s *simpleStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	evts := make(chan apievents.AuditEvent)

	go func() {
		defer close(evts)

		for i := int64(0); i < s.count; i++ {
			select {
			case <-ctx.Done():
				return
			case evts <- &apievents.SessionPrint{
				Metadata: apievents.Metadata{
					Type:  events.SessionPrintEvent,
					Index: i,
					ID:    strconv.Itoa(int(i)),
					Code:  strconv.FormatInt((i+1)*s.delay, 10),
				},
				Data:              []byte(fmt.Sprintf("event %d\n", i)),
				ChunkIndex:        i, // TODO(zmb3) deprecate this
				DelayMilliseconds: (i + 1) * s.delay,
			}:
			}
		}
	}()

	return evts, errors
}

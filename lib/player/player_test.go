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
	"github.com/stretchr/testify/assert"
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
	require.Equal(t, time.Second, p.LastPlayed())
}

func TestSeekForward(t *testing.T) {
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

func TestUseDatabaseTranslator(t *testing.T) {
	t.Run("SupportedProtocols", func(t *testing.T) {
		for _, protocol := range player.SupportedDatabaseProtocols {
			queryEventCount := 3
			t.Run(protocol, func(t *testing.T) {
				clk := clockwork.NewFakeClock()
				p, err := player.New(&player.Config{
					Clock:     clk,
					SessionID: "test-session",
					Streamer:  &databaseStreamer{protocol: protocol, count: int64(queryEventCount)},
				})
				require.NoError(t, err)
				require.NoError(t, p.Play())

				count := 0
				for evt := range p.C() {
					// When using translator, the only returned events are
					// prints. Everything else indicates a translator was not
					// used.
					switch evt.(type) {
					case *apievents.SessionPrint:
					default:
						require.Fail(t, "expected only session start/end and session print events but got %T", evt)
					}
					count++
				}

				// Queries + start/end
				require.Equal(t, queryEventCount+2, count)
				require.NoError(t, p.Err())
			})
		}
	})

	t.Run("UnsupportedProtocol", func(t *testing.T) {
		queryEventCount := 3
		clk := clockwork.NewFakeClock()
		p, err := player.New(&player.Config{
			Clock:     clk,
			SessionID: "test-session",
			Streamer:  &databaseStreamer{protocol: "random-protocol", count: int64(queryEventCount)},
		})
		require.NoError(t, err)
		require.NoError(t, p.Play())

		count := 0
		for evt := range p.C() {
			switch evt.(type) {
			case *apievents.DatabaseSessionStart, *apievents.DatabaseSessionEnd, *apievents.DatabaseSessionQuery:
			default:
				require.Fail(t, "expected only database events but got %T", evt)
			}
			count++
		}

		// Queries + start/end
		require.Equal(t, queryEventCount+2, count)
		require.NoError(t, p.Err())
	})
}

func TestSkipIdlePeriods(t *testing.T) {
	eventCount := 3
	delayMilliseconds := 60000
	clk := clockwork.NewFakeClock()
	p, err := player.New(&player.Config{
		Clock:        clk,
		SessionID:    "test-session",
		SkipIdleTime: true,
		Streamer:     &simpleStreamer{count: int64(eventCount), delay: int64(delayMilliseconds)},
	})
	require.NoError(t, err)
	require.NoError(t, p.Play())

	for i := range eventCount {
		// Consume events in an eventually loop to avoid firing the clock
		// events before the timer is set.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			clk.Advance(player.MaxIdleTime)
			select {
			case evt := <-p.C():
				assert.Equal(t, int64(i), evt.GetIndex())
			default:
				assert.Fail(t, "expected to receive event after short period, but got nothing")
			}
		}, 3*time.Second, 100*time.Millisecond)
	}
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

					// stuff the event delay in the code field so that it's easy
					// to access without a type assertion
					Code: strconv.FormatInt((i+1)*s.delay, 10),
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

type databaseStreamer struct {
	protocol string
	count    int64
	idx      int64
}

func (d *databaseStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	evts := make(chan apievents.AuditEvent)

	go func() {
		defer close(evts)

		d.sendEvent(ctx, evts, &apievents.DatabaseSessionStart{
			Metadata: apievents.Metadata{
				Type:  events.DatabaseSessionStartEvent,
				Index: d.idx,
			},
			DatabaseMetadata: apievents.DatabaseMetadata{
				DatabaseProtocol: d.protocol,
			},
		})

		for i := int64(0); i < d.count; i++ {
			if ctx.Err() != nil {
				return
			}

			d.sendEvent(ctx, evts, &apievents.DatabaseSessionQuery{
				Metadata: apievents.Metadata{
					Type:  events.DatabaseSessionQueryEvent,
					Index: d.idx + i,
				},
			})
		}

		d.sendEvent(ctx, evts, &apievents.DatabaseSessionEnd{
			Metadata: apievents.Metadata{
				Type:  events.DatabaseSessionEndEvent,
				Index: d.idx,
			},
		})
	}()

	return evts, errors
}

func (d *databaseStreamer) sendEvent(ctx context.Context, evts chan apievents.AuditEvent, evt apievents.AuditEvent) {
	select {
	case <-ctx.Done():
	case evts <- evt:
		d.idx++
	}
}

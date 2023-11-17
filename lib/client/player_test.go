/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bytes"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/events"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestEmptyPlay verifies that a playback of 0 events
// immediately transitions to a stopped state.
func TestEmptyPlay(t *testing.T) {
	c := clockwork.NewFakeClock()
	p := newSessionPlayer(nil, nil, testTerm(t))
	p.clock = c

	p.Play()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for player to complete")
	case <-p.stopC:
	}

	require.True(t, p.Stopped(), "p.Stopped() returned an unexpected value")
}

// TestStop verifies that we can stop playback.
func TestStop(t *testing.T) {
	c := clockwork.NewFakeClock()
	events := printEvents(100, 200)
	p := newSessionPlayer(events, nil, testTerm(t))
	p.clock = c

	p.Play()

	// wait for player to see the first event and apply the delay
	c.BlockUntil(1)

	p.EndPlayback()

	// advance the clock:
	// at this point, the player will write the first event and then
	// see that we requested a stop
	c.Advance(100 * time.Millisecond)

	require.Eventually(t, p.Stopped, 2*time.Second, 200*time.Millisecond)
}

// TestPlayPause verifies the play/pause functionality.
func TestPlayPause(t *testing.T) {
	c := clockwork.NewFakeClock()

	// in this test, we let the player play 2 of the 3 events,
	// then pause it and verify the pause state before resuming
	// playback for the final event.
	events := printEvents(100, 200, 300)
	var stream []byte // intentionally empty, we dont care about stream contents here
	p := newSessionPlayer(events, stream, testTerm(t))
	p.clock = c

	p.Play()

	// wait for player to see the first event and apply the delay
	c.BlockUntil(1)

	// advance the clock:
	// at this point, the player will write the first event
	c.Advance(100 * time.Millisecond)

	// wait for the player to sleep on the 2nd event
	c.BlockUntil(1)

	// pause playback
	// note: we don't use p.TogglePause here, as it waits for the state transition,
	// and the state won't transition until we advance the clock
	p.Lock()
	p.setState(stateStopping)
	p.Unlock()

	// advance the clock again:
	// the player will write the second event and
	// then realize that it's been asked to pause
	c.Advance(100 * time.Millisecond)

	p.Lock()
	p.waitUntil(stateStopped)
	p.Unlock()

	ch := make(chan struct{})
	go func() {
		// resume playback
		p.TogglePause()
		ch <- struct{}{}
	}()

	// playback should resume for the 3rd and final event:
	// in this case, the first two events are written immediately without delay,
	// and we block here until the player is sleeping prior to the 3rd event
	c.BlockUntil(1)

	// make sure that we've resumed
	<-ch
	require.False(t, p.Stopped(), "p.Stopped() returned true when it should have returned false")

	// advance the clock a final time, forcing the player to write the last event
	// note: on the resume, we play the successful events immediately, and then sleep
	// up to the resume point, which is why we advance by 300ms here
	c.Advance(300 * time.Millisecond)

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for player to complete")
	case <-p.stopC:
	}
	require.True(t, p.Stopped(), "p.Stopped() returned an unexpected value")
}

func TestEndPlaybackWhilePlaying(t *testing.T) {
	c := clockwork.NewFakeClock()

	// in this test, we let the player play 1 of the 2 events,
	// then end the playback and confirm
	// that the stopC channel was written to.
	events := printEvents(100, 200)
	var stream []byte // intentionally empty, we dont care about stream contents here
	p := newSessionPlayer(events, stream, testTerm(t))
	p.clock = c

	p.Play()

	// wait for player to see the first event and apply the delay
	c.BlockUntil(1)

	// end playback
	p.EndPlayback()

	// advance the clock:
	// the player will write the first event and
	// then realize that it's been asked to end playback
	c.Advance(100 * time.Millisecond)

	// check that stopC was written to
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for player to complete")
	case <-p.stopC:
		require.True(t, p.Stopped(), "p.Stopped() returned an unexpected value")
	}
}

// TestEndPlaybackWhilePaused tests that playback can be ended
// by calling EndPlayback while playback is paused.
func TestEndPlaybackWhilePaused(t *testing.T) {
	c := clockwork.NewFakeClock()

	// in this test, we let the player play 1 of the 2 events,
	// then pause it and verify the pause state before ending playback.
	events := printEvents(100, 200)
	var stream []byte // intentionally empty, we dont care about stream contents here
	p := newSessionPlayer(events, stream, testTerm(t))
	p.clock = c

	p.Play()

	// wait for player to see the first event and apply the delay
	c.BlockUntil(1)

	// advance the clock:
	// at this point, the player will write the first event
	c.Advance(100 * time.Millisecond)

	// wait for the player to sleep on the 2nd event
	c.BlockUntil(1)

	// pause playback
	// note: we don't use p.TogglePause here, as it waits for the state transition,
	// and the state won't transition until we advance the clock
	p.Lock()
	p.setState(stateStopping)
	p.Unlock()

	// advance the clock again:
	// the player will write the second event and
	// then realize that it's been asked to pause
	c.Advance(100 * time.Millisecond)

	// wait until the pause is in effect
	p.Lock()
	p.waitUntil(stateStopped)
	p.Unlock()

	// end playback
	p.EndPlayback()

	// check that stopC was written to
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for player to complete")
	case <-p.stopC:
		require.True(t, p.Stopped(), "p.Stopped() returned an unexpected value")
	}
}

func testTerm(t *testing.T) *terminal.Terminal {
	t.Helper()
	term, err := terminal.New(bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, term.Close())
	})
	return term
}

func printEvents(delays ...int) []events.EventFields {
	result := make([]events.EventFields, len(delays))
	for i := range result {
		result[i] = events.EventFields{
			events.EventType: events.SessionPrintEvent,
			"ms":             delays[i],
		}
	}
	return result
}

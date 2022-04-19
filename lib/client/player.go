/*
Copyright 2015 Gravitational, Inc.

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
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/events"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

type tshPlayerState int

const (
	// The player has stopped, either for an action to take place,
	// because the playback has reached the end of the recording,
	// or because a hard stop was requested.
	stateStopped tshPlayerState = iota
	// A stop has been requested so that an action (forward, rewind, etc) can take place.
	stateStopping
	// An end to the playback has been requested.
	stateEnding
	// The player is playing.
	statePlaying
)

// sessionPlayer implements replaying terminal sessions. It runs a playback goroutine
// and allows to control it
type sessionPlayer struct {
	sync.Mutex
	cond *sync.Cond

	state    tshPlayerState
	position int // position is the index of the last event successfully played back

	clock         clockwork.Clock
	stream        []byte
	sessionEvents []events.EventFields
	term          *terminal.Terminal

	// stopC is closed when playback ends (either because the end of the stream has
	// been reached, or a hard stop was requested via EndPlayback().
	stopC    chan struct{}
	stopOnce sync.Once

	log *logrus.Logger
}

func newSessionPlayer(sessionEvents []events.EventFields, stream []byte, term *terminal.Terminal) *sessionPlayer {
	p := &sessionPlayer{
		clock:         clockwork.NewRealClock(),
		position:      -1, // position is the last successfully written event
		stream:        stream,
		sessionEvents: sessionEvents,
		term:          term,
		stopC:         make(chan struct{}),
		log:           logrus.New(),
	}
	p.cond = sync.NewCond(p)
	return p
}

func (p *sessionPlayer) Play() {
	p.playRange(0, 0)
}

func (p *sessionPlayer) Stopped() bool {
	p.Lock()
	defer p.Unlock()
	return p.state == stateStopped
}

func (p *sessionPlayer) Rewind() {
	p.Lock()
	defer p.Unlock()
	if p.state != stateStopped {
		p.setState(stateStopping)
		p.waitUntil(stateStopped)
	}
	if p.position > 0 {
		p.playRange(p.position-1, p.position)
	}
}

func (p *sessionPlayer) stopOrEndRequested() bool {
	p.Lock()
	defer p.Unlock()
	return p.state == stateStopping || p.state == stateEnding
}

func (p *sessionPlayer) Forward() {
	p.Lock()
	defer p.Unlock()
	if p.state != stateStopped {
		p.setState(stateStopping)
		p.waitUntil(stateStopped)
	}
	if p.position < len(p.sessionEvents) {
		p.playRange(p.position+2, p.position+2)
	}
}

func (p *sessionPlayer) TogglePause() {
	p.Lock()
	defer p.Unlock()
	if p.state == statePlaying {
		p.setState(stateStopping)
		p.waitUntil(stateStopped)
	} else {
		p.playRange(p.position+1, 0)
		p.waitUntil(statePlaying)
	}
}

// EndPlayback makes an asynchronous request for the player to end the playback.
// Playback might not stop before this method returns.
func (p *sessionPlayer) EndPlayback() {
	p.Lock()
	defer p.Unlock()

	switch p.state {
	case stateEnding:
		// We're already ending, no need to do anything.
	case stateStopped:
		// The playRange goroutine has already returned, so we can
		// signal the end of playback by closing the stopC channel right here.
		p.close()
	case stateStopping, statePlaying:
		// The playRange goroutine is still running, and may be sleeping
		// while waiting for the right time to print the next characters.
		// setState to stateEnding so that the playRange goroutine
		// knows to return on the next loop. The stopC channel will
		// be closed by the playback routine upon completion.
		p.setState(stateEnding)
	default:
		// Cases should be exhaustive, this should never happen.
		p.log.Error("unexpected playback error")
	}
}

// waitUntil waits for the specified state to be reached.
// Callers must hold the lock on p.Mutex before calling.
func (p *sessionPlayer) waitUntil(state tshPlayerState) {
	for state != p.state {
		p.cond.Wait()
	}
}

// setState sets the current player state and notifies any
// goroutines waiting in waitUntil(). Callers must hold the
// lock on p.Mutex before calling.
func (p *sessionPlayer) setState(state tshPlayerState) {
	p.state = state
	p.cond.Broadcast()
}

// timestampFrame prints 'event timestamp' in the top right corner of the
// terminal after playing every 'print' event
func timestampFrame(term *terminal.Terminal, message string) {
	const (
		saveCursor    = "7"
		restoreCursor = "8"
	)
	width, _, err := term.Size()
	if err != nil {
		return
	}
	esc := func(s string) {
		os.Stdout.Write([]byte("\x1b" + s))
	}
	esc(saveCursor)
	defer esc(restoreCursor)

	// move cursor to -10:0
	// TODO(timothyb89): message length does not account for unicode characters
	// or ANSI sequences.
	esc(fmt.Sprintf("[%d;%df", 0, int(width)-len(message)))
	os.Stdout.WriteString(message)
}

func (p *sessionPlayer) close() {
	p.stopOnce.Do(func() { close(p.stopC) })
}

// playRange plays events from a given from:to range. In order for the replay
// to render correctly, playRange always plays from the beginning, but starts
// applying timing info (delays) only after 'from' event, creating an impression
// that playback starts from there.
func (p *sessionPlayer) playRange(from, to int) {
	if to > len(p.sessionEvents) || from < 0 {
		p.Lock()
		p.setState(stateStopped)
		p.Unlock()
		return
	}
	if to == 0 {
		to = len(p.sessionEvents)
	}
	// clear screen between runs:
	os.Stdout.Write([]byte("\x1bc"))

	// playback goroutine:
	go func() {
		var i int

		defer func() {

			p.Lock()
			endRequested := p.state == stateEnding
			p.setState(stateStopped)
			p.Unlock()

			// An end was manually requested, or we played the last event?
			if endRequested || i == len(p.sessionEvents) {
				p.close()
			}
		}()

		p.Lock()
		p.setState(statePlaying)
		p.Unlock()

		prev := time.Duration(0)
		offset, bytes := 0, 0
		for i = 0; i < to; i++ {
			if p.stopOrEndRequested() {
				return
			}

			e := p.sessionEvents[i]

			switch e.GetString(events.EventType) {
			// 'print' event (output)
			case events.SessionPrintEvent:
				// delay is only necessary once we've caught up to the "from" event
				if i >= from {
					prev = p.applyDelay(prev, e)
				}
				offset = e.GetInt("offset")
				bytes = e.GetInt("bytes")
				os.Stdout.Write(p.stream[offset : offset+bytes])
			// resize terminal event (also on session start)
			case events.ResizeEvent, events.SessionStartEvent:
				parts := strings.Split(e.GetString("size"), ":")
				if len(parts) != 2 {
					continue
				}
				width, height := parts[0], parts[1]
				// resize terminal window by sending control sequence:
				os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%s;%st", height, width)))
			default:
				continue
			}
			p.Lock()
			p.position = i
			p.Unlock()
		}
	}()
}

// applyDelay waits until it is time to play back the current event.
// It returns the duration from the start of the session up until the current event.
func (p *sessionPlayer) applyDelay(previousTimestamp time.Duration, e events.EventFields) time.Duration {
	eventTime := time.Duration(e.GetInt("ms") * int(time.Millisecond))
	delay := eventTime - previousTimestamp

	// make playback smoother:
	switch {
	case delay < 10*time.Millisecond:
		delay = 0
	case delay > 250*time.Millisecond && delay < 500*time.Millisecond:
		delay = 250 * time.Millisecond
	case delay > 500*time.Millisecond && delay < 1*time.Second:
		delay = 500 * time.Millisecond
	case delay > time.Second:
		delay = time.Second
	}

	timestampFrame(p.term, e.GetString("time"))
	p.clock.Sleep(delay)
	return eventTime
}

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
)

type tshPlayerState int

const (
	stateStopped tshPlayerState = iota
	stateStopping
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

	// stopC is used to tell the caller that player has finished playing
	stopC    chan int
	stopOnce sync.Once
}

func newSessionPlayer(sessionEvents []events.EventFields, stream []byte, term *terminal.Terminal) *sessionPlayer {
	p := &sessionPlayer{
		clock:         clockwork.NewRealClock(),
		position:      -1, // position is the last successfully written event
		stream:        stream,
		sessionEvents: sessionEvents,
		term:          term,
		stopC:         make(chan int),
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

func (p *sessionPlayer) stopRequested() bool {
	p.Lock()
	defer p.Unlock()
	return p.state == stateStopping
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

// RequestStop makes an asynchronous request for the player to stop playing.
// Playback may not stop before this method returns.
func (p *sessionPlayer) RequestStop() {
	p.Lock()
	defer p.Unlock()

	switch p.state {
	case stateStopped, stateStopping:
		// do nothing if stop already in progress
	default:
		p.setState(stateStopping)
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
			p.setState(stateStopped)
			p.Unlock()

			// played last event?
			if i == len(p.sessionEvents) {
				p.stopOnce.Do(func() { close(p.stopC) })
			}
		}()

		p.Lock()
		p.setState(statePlaying)
		p.Unlock()

		prev := time.Duration(0)
		offset, bytes := 0, 0
		for i = 0; i < to; i++ {
			if p.stopRequested() {
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

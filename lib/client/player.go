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

	"github.com/moby/term"

	"github.com/gravitational/teleport/lib/events"
)

const (
	stateStopped = iota
	stateStopping
	statePlaying
)

// sessionPlayer implements replaying terminal sessions. It runs a playback goroutine
// and allows to control it
type sessionPlayer struct {
	sync.Mutex
	stream        []byte
	sessionEvents []events.EventFields

	state    int
	position int

	// stopC is used to tell the caller that player has finished playing
	stopC chan int
}

func newSessionPlayer(sessionEvents []events.EventFields, stream []byte) *sessionPlayer {
	return &sessionPlayer{
		stream:        stream,
		sessionEvents: sessionEvents,
		stopC:         make(chan int),
	}
}

func (p *sessionPlayer) Play() {
	p.playRange(0, 0)
}

func (p *sessionPlayer) Stop() {
	p.Lock()
	defer p.Unlock()
	if p.stopC != nil {
		close(p.stopC)
		p.stopC = nil
	}
}

func (p *sessionPlayer) Stopped() bool {
	return p.state == stateStopped
}

func (p *sessionPlayer) Rewind() {
	p.Lock()
	defer p.Unlock()
	if p.state != stateStopped {
		p.state = stateStopping
		p.waitUntil(stateStopped)
	}
	if p.position > 0 {
		p.playRange(p.position-1, p.position)
	}
}

func (p *sessionPlayer) Forward() {
	p.Lock()
	defer p.Unlock()
	if p.state != stateStopped {
		p.state = stateStopping
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
		p.state = stateStopping
		p.waitUntil(stateStopped)
	} else {
		p.playRange(p.position, 0)
		p.waitUntil(statePlaying)
	}
}

func (p *sessionPlayer) waitUntil(state int) {
	for state != p.state {
		time.Sleep(time.Millisecond)
	}
}

// timestampFrame prints 'event timestamp' in the top right corner of the
// terminal after playing every 'print' event
func timestampFrame(message string) {
	const (
		saveCursor    = "7"
		restoreCursor = "8"
	)
	sz, err := term.GetWinsize(0)
	if err != nil {
		return
	}
	esc := func(s string) {
		os.Stdout.Write([]byte("\x1b" + s))
	}
	esc(saveCursor)
	defer esc(restoreCursor)

	// move cursor to -10:0
	esc(fmt.Sprintf("[%d;%df", 0, int(sz.Width)-len(message)))
	os.Stdout.WriteString(message)
}

// playRange plays events from a given from:to range. In order for the replay
// to render correctly, playRange always plays from the beginning, but starts
// applying timing info (delays) only after 'from' event, creating an impression
// that playback starts from there.
func (p *sessionPlayer) playRange(from, to int) {
	if to > len(p.sessionEvents) || from < 0 {
		p.state = stateStopped
		return
	}
	if to == 0 {
		to = len(p.sessionEvents)
	}
	// clear screen between runs:
	os.Stdout.Write([]byte("\x1bc"))
	// wait: waits between events during playback
	prev := time.Duration(0)
	wait := func(i int, e events.EventFields) {
		ms := time.Duration(e.GetInt("ms"))
		// before "from"? play that instantly:
		if i >= from {
			delay := ms - prev
			// make playback smoother:
			if delay < 10 {
				delay = 0
			}
			if delay > 250 && delay < 500 {
				delay = 250
			}
			if delay > 500 && delay < 1000 {
				delay = 500
			}
			if delay > 1000 {
				delay = 1000
			}
			timestampFrame(e.GetString("time"))
			time.Sleep(time.Millisecond * delay)
		}
		prev = ms
	}
	// playback goroutine:
	go func() {
		defer func() {
			p.state = stateStopped
		}()
		p.state = statePlaying
		i, offset, bytes := 0, 0, 0
		for i = 0; i < to; i++ {
			if p.state == stateStopping {
				return
			}
			e := p.sessionEvents[i]

			switch e.GetString(events.EventType) {
			// 'print' event (output)
			case events.SessionPrintEvent:
				wait(i, e)
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
			p.position = i
		}
		// played last event?
		if i == len(p.sessionEvents) {
			p.Stop()
		}
	}()
}

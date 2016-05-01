package client

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
		stopC:         make(chan int, 0),
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
	os.Stdout.Write([]byte("\033[H\033[2J"))
	// wait: waits between events during playback
	prev := 0
	wait := func(i int, e events.EventFields) {
		// before "from"? play that instantly:
		if i <= from {
			return
		}
		ms := e.GetInt("ms")
		// do not stop for longer than 1 second:
		delay := ms - prev
		if delay > 1000 {
			delay = 1000
		}
		// normalize delays for a nicer experience
		if delay > 100 && delay < 500 {
			delay = 100
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
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
				offset = e.GetInt("offset")
				bytes = e.GetInt("bytes")
				os.Stdout.Write(p.stream[offset : offset+bytes])
			// resize terminal event:
			case events.TerminalSize:
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
			wait(i, e)
		}
		// played last event?
		if i == len(p.sessionEvents) {
			p.Stop()
		}
	}()
}

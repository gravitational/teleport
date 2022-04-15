// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package srv

import (
	"io"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// maxHistoryBytes is the maximum bytes that are retained as history and broadcasted to new clients.
const maxHistoryBytes = 1000

// maxPausedHistoryBytes is maximum bytes that are buffered when a session is paused.
const maxPausedHistoryBytes = 10000

// TermManager handles the streams of terminal-like sessions.
// It performs a number of tasks including:
// - multiplexing
// - history scrollback for new clients
// - stream breaking
type TermManager struct {
	// These two fields need to be first in the struct so that they are 64-bit aligned which is a requirement
	// for atomic operations on certain architectures.
	countWritten uint64
	countRead    uint64

	writers      map[string]io.Writer
	readers      map[string]chan struct{}
	OnWriteError func(idString string, err error)
	// buffer is used to buffer writes when turned off
	buffer []byte
	on     bool
	// history is used to store the scrollback history sent to new clients
	history []byte
	// incoming is a stream of incoming stdin data
	incoming chan []byte
	// remaining is a partially read chunk of stdin data
	// we only support one concurrent reader so this isn't mutex protected
	remaining []byte

	terminateNotifiers []chan struct{}
	addNotifier        chan chan struct{}
	addWriter          chan writer
	delWriter          chan string
	addReader          chan reader
	delReader          chan string
	write              chan []byte
	read               chan read
	broadcast          chan string
	done               chan struct{}
	onCh               chan bool
	termination        chan struct{}
	historyCh          chan chan []byte
}

type writer struct {
	io.Writer
	name string
}

type reader struct {
	done chan struct{}
	name string
}

type read struct {
	data chan []byte
	len  int
}

// NewTermManager creates a new TermManager.
func NewTermManager() *TermManager {
	g := &TermManager{
		writers:     make(map[string]io.Writer),
		readers:     make(map[string]chan struct{}),
		incoming:    make(chan []byte),
		addNotifier: make(chan chan struct{}),
		addWriter:   make(chan writer),
		delWriter:   make(chan string),
		addReader:   make(chan reader),
		delReader:   make(chan string),
		write:       make(chan []byte),
		read:        make(chan read),
		broadcast:   make(chan string),
		done:        make(chan struct{}),
		onCh:        make(chan bool),
		termination: make(chan struct{}),
		historyCh:   make(chan chan []byte),
	}

	go func() {
		var lastWasBroadcast bool
		var pendingRead read

		for {
			select {
			case <-g.done:
				for _, ch := range g.terminateNotifiers {
					close(ch)
				}
				return
			case ch := <-g.addNotifier:
				g.terminateNotifiers = append(g.terminateNotifiers, ch)
			case w := <-g.addWriter:
				g.writers[w.name] = w
			case w := <-g.delWriter:
				delete(g.writers, w)
			case r := <-g.addReader:
				g.readers[r.name] = r.done
			case r := <-g.delReader:
				if done, ok := g.readers[r]; ok {
					close(done)
					delete(g.readers, r)
				}
			case b := <-g.write:
				if g.on {
					g.writeToClients(b)
				} else {
					// Only keep the last maxPausedHistoryBytes of stdout/stderr while the session is paused.
					// The alternative is flushing to disk but this should be a pretty rare occurrence and shouldn't
					//be an issue in practice.
					g.buffer = appendAndTruncateFront(g.buffer, b, maxPausedHistoryBytes)
				}
				lastWasBroadcast = false
			case m := <-g.broadcast:
				data := []byte("\r\nTeleport > " + m + "\r\n")
				if !lastWasBroadcast {
					data = data[2:]
				}
				g.writeToClients(data)
				lastWasBroadcast = true
			case on := <-g.onCh:
				g.on = on
			case <-g.termination:
				if !g.on {
					for _, ch := range g.terminateNotifiers {
						ch <- struct{}{}
					}
				}
			case r := <-g.read:
				if len(g.remaining) == 0 {
					pendingRead = r
					continue
				}
				b := make([]byte, r.len)
				n := copy(b, g.remaining)
				g.remaining = g.remaining[n:]
				r.data <- b
			case incoming := <-g.incoming:
				if pendingRead.data == nil {
					g.remaining = append(g.remaining, incoming...)
					continue
				}
				b := make([]byte, pendingRead.len)
				n := copy(b, incoming)
				g.remaining = append(g.remaining, incoming[n:]...)
				pendingRead.data <- b
				pendingRead.data = nil
			case ch := <-g.historyCh:
				ch <- g.history
			}
		}
	}()

	return g
}

func (g *TermManager) writeToClients(p []byte) {
	g.history = appendAndTruncateFront(g.history, p, maxHistoryBytes)

	atomic.AddUint64(&g.countWritten, uint64(len(p)))
	for key, w := range g.writers {
		_, err := w.Write(p)
		if err == nil {
			continue
		}

		if err != io.EOF {
			log.Warnf("Failed to write to remote terminal: %v", err)
		}

		if g.OnWriteError != nil {
			g.OnWriteError(key, err)
		}

		delete(g.writers, key)
	}
}

func (g *TermManager) TerminateNotifier() <-chan struct{} {
	ch := make(chan struct{})
	select {
	case <-g.done:
		close(ch)
	case g.addNotifier <- ch:
	}
	return ch
}

func (g *TermManager) Write(p []byte) (int, error) {
	select {
	case <-g.done:
		return 0, io.EOF
	case g.write <- p:
		return len(p), nil
	}
}

func (g *TermManager) Read(p []byte) (int, error) {
	data := make(chan []byte)
	select {
	case <-g.done:
		return 0, io.EOF
	case g.read <- read{data, len(p)}:
	}
	select {
	case <-g.done:
		return 0, io.EOF
	case b := <-data:
		n := copy(p, b)
		atomic.AddUint64(&g.countRead, uint64(n))
		return n, nil
	}
}

// BroadcastMessage injects a message into the stream.
func (g *TermManager) BroadcastMessage(message string) error {
	select {
	case <-g.done:
	case g.broadcast <- message:
	}
	return nil
}

// On allows data to flow through the manager.
func (g *TermManager) On() {
	select {
	case <-g.done:
	case g.onCh <- true:
	}
}

// Off buffers incoming writes and reads until turned on again.
func (g *TermManager) Off() {
	select {
	case <-g.done:
	case g.onCh <- false:
	}
}

func (g *TermManager) AddWriter(name string, w io.Writer) {
	select {
	case <-g.done:
	case g.addWriter <- writer{w, name}:
	}
}

func (g *TermManager) DeleteWriter(name string) {
	select {
	case <-g.done:
	case g.delWriter <- name:
	}
}

func (g *TermManager) AddReader(name string, r io.Reader) {
	done := make(chan struct{})
	select {
	case <-g.done:
		return
	case g.addReader <- reader{done, name}:
	}

	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			if err != nil {
				log.Warnf("Failed to read from remote terminal: %v", err)
				g.DeleteReader(name)
				return
			}

			for _, b := range buf[:n] {
				// This is the ASCII control code for CTRL+C.
				if b == 0x03 {
					select {
					case <-g.done:
					case g.termination <- struct{}{}:
					}
					break
				}
			}

			select {
			case <-g.done:
				return
			case <-done:
				return
			case g.incoming <- buf[:n]:
			}
		}
	}()
}

func (g *TermManager) DeleteReader(name string) {
	select {
	case <-g.done:
	case g.delReader <- name:
	}
}

func (g *TermManager) Close() {
	select {
	case <-g.done:
	default:
		close(g.done)
	}
}

func (g *TermManager) CountWritten() uint64 {
	return atomic.LoadUint64(&g.countWritten)
}

func (g *TermManager) CountRead() uint64 {
	return atomic.LoadUint64(&g.countRead)
}

func (g *TermManager) GetRecentHistory() []byte {
	data := make(chan []byte)
	select {
	case <-g.done:
	case g.historyCh <- data:
	}
	select {
	case <-g.done:
		return nil
	case b := <-data:
		return b
	}
}

func appendAndTruncateFront(s1, s2 []byte, max int) []byte {
	slice := append(s1, s2...)
	if len(slice) > max {
		return slice[len(slice)-max:]
	}

	return slice
}

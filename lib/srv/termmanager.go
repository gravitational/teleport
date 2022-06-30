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
	"sync"
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

	mu           sync.Mutex
	writers      map[string]io.Writer
	readerState  map[string]bool
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
	remaining         []byte
	readStateUpdate   *sync.Cond
	closed            chan struct{}
	lastWasBroadcast  bool
	terminateNotifier chan struct{}

	// called when data is discarded due to multiplexing being disabled, used in tests
	onDiscard func()
}

// NewTermManager creates a new TermManager.
func NewTermManager() *TermManager {
	return &TermManager{
		writers:           make(map[string]io.Writer),
		readerState:       make(map[string]bool),
		closed:            make(chan struct{}),
		readStateUpdate:   sync.NewCond(&sync.Mutex{}),
		incoming:          make(chan []byte, 100),
		terminateNotifier: make(chan struct{}, 1),
	}
}

// writeToClients writes to underlying clients
func (g *TermManager) writeToClients(p []byte) {
	g.lastWasBroadcast = false
	g.history = truncateFront(append(g.history, p...), maxHistoryBytes)

	atomic.AddUint64(&g.countWritten, uint64(len(p)))
	for key, w := range g.writers {
		_, err := w.Write(p)
		if err != nil {
			if err != io.EOF {
				log.Warnf("Failed to write to remote terminal: %v", err)
			}

			// Let term manager decide how to handle broken party writers
			if g.OnWriteError != nil {
				g.OnWriteError(key, err)
			}

			delete(g.writers, key)
		}
	}

}

func (g *TermManager) TerminateNotifier() <-chan struct{} {
	return g.terminateNotifier
}

func (g *TermManager) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.isClosed() {
		return 0, io.EOF
	}

	if g.on {
		g.writeToClients(p)
	} else {
		// Only keep the last maxPausedHistoryBytes of stdout/stderr while the session is paused.
		// The alternative is flushing to disk but this should be a pretty rare occurrence and shouldn't be an issue in practice.
		g.buffer = truncateFront(append(g.buffer, p...), maxPausedHistoryBytes)
	}

	return len(p), nil
}

func (g *TermManager) Read(p []byte) (int, error) {
	if len(g.remaining) > 0 {
		n := copy(p, g.remaining)
		g.remaining = g.remaining[n:]
		return n, nil
	}

	q := make(chan struct{})
	defer close(q)
	c := make(chan bool)
	go func() {
		g.readStateUpdate.L.Lock()
		defer g.readStateUpdate.L.Unlock()

		for {
			g.mu.Lock()
			on := g.on
			g.mu.Unlock()

			select {
			case c <- on:
			case <-q:
				close(c)
				return
			}

			g.readStateUpdate.Wait()
		}
	}()

	on := <-c
	for {
		if !on {
			select {
			case <-g.closed:
				return 0, io.EOF
			case on = <-c:
				continue
			}
		}

		select {
		case <-g.closed:
			return 0, io.EOF
		case on = <-c:
			continue
		case g.remaining = <-g.incoming:
			n := copy(p, g.remaining)
			g.remaining = g.remaining[n:]
			return n, nil
		}
	}
}

// BroadcastMessage injects a message into the stream.
func (g *TermManager) BroadcastMessage(message string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	data := []byte("Teleport > " + message + "\r\n")
	if g.lastWasBroadcast {
		data = append([]byte("\r\n"), data...)
	} else {
		g.lastWasBroadcast = true
	}

	g.writeToClients(data)
}

// On allows data to flow through the manager.
func (g *TermManager) On() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.on = true
	g.readStateUpdate.Broadcast()
	g.writeToClients(g.buffer)
}

// Off buffers incoming writes and reads until turned on again.
func (g *TermManager) Off() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.on = false
	g.readStateUpdate.Broadcast()
}

func (g *TermManager) AddWriter(name string, w io.Writer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.writers[name] = w
}

func (g *TermManager) DeleteWriter(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.writers, name)
}

func (g *TermManager) AddReader(name string, r io.Reader) {
	g.readerState[name] = false

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
					g.mu.Lock()
					if !g.on && !g.isClosed() {
						select {
						case g.terminateNotifier <- struct{}{}:
						default:
						}
					}
					g.mu.Unlock()
					break
				}
			}

			g.mu.Lock()

			if g.on {
				g.mu.Unlock()
				g.incoming <- buf[:n]
				g.mu.Lock()
			} else {
				if g.onDiscard != nil {
					g.onDiscard()
				}
			}

			if g.isClosed() || g.readerState[name] {
				g.mu.Unlock()
				return
			}
			g.mu.Unlock()
		}
	}()
}

func (g *TermManager) DeleteReader(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.readerState[name] = true
}

func (g *TermManager) CountWritten() uint64 {
	return atomic.LoadUint64(&g.countWritten)
}

func (g *TermManager) CountRead() uint64 {
	return atomic.LoadUint64(&g.countRead)
}

func (g *TermManager) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.isClosed() {
		close(g.closed)
		close(g.terminateNotifier)
	}
}

func (g *TermManager) isClosed() bool {
	select {
	case <-g.closed:
		return true
	default:
		return false
	}
}

func (g *TermManager) GetRecentHistory() []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	data := make([]byte, 0, len(g.history))
	data = append(data, g.history...)
	return data
}

func truncateFront(slice []byte, max int) []byte {
	if len(slice) > max {
		return slice[len(slice)-max:]
	}

	return slice
}

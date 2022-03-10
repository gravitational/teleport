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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const maxHistory = 1000

// TermManager handles the streams of terminal-like sessions.
// It performs a number of tasks including:
// - multiplexing
// - history scrollback for new clients
// - stream breaking
type TermManager struct {
	mu           sync.Mutex
	writers      map[string]io.Writer
	readerState  map[string]*int32
	OnWriteError func(idString string, err error)
	countWritten uint64
	countRead    uint64
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
	closed            *int32
	lastWasBroadcast  bool
	terminateNotifier chan bool
}

// NewTermManager creates a new TermManager.
func NewTermManager() *TermManager {
	return &TermManager{
		writers:           make(map[string]io.Writer),
		readerState:       make(map[string]*int32),
		closed:            new(int32),
		readStateUpdate:   sync.NewCond(&sync.Mutex{}),
		incoming:          make(chan []byte, 100),
		terminateNotifier: make(chan bool),
	}
}

func (g *TermManager) writeToClients(p []byte) int {
	g.lastWasBroadcast = false
	truncateFront := func(slice []byte, max int) []byte {
		if len(slice) > max {
			return slice[len(slice)-max:]
		}

		return slice
	}

	g.history = append(g.history, truncateFront(p, maxHistory)...)
	if len(g.history) > maxHistory {
		g.history = g.history[:maxHistory]
	}

	atomic.AddUint64(&g.countWritten, uint64(len(p)))
	for key, w := range g.writers {
		_, err := w.Write(p)
		if err != nil {
			if err != io.EOF {
				log.Warnf("Failed to write to remote terminal: %v", err)
			}

			if g.OnWriteError != nil {
				g.OnWriteError(key, err)
			}

			delete(g.writers, key)
		}
	}

	return len(p)
}

func (g *TermManager) TerminateNotifier() <-chan bool {
	return g.terminateNotifier
}

func (g *TermManager) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.on {
		g.writeToClients(p)
	} else {
		g.buffer = append(g.buffer, p...)
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
			on = <-c
			continue
		}

		select {
		case on = <-c:
			continue
		case g.remaining = <-g.incoming:
			close(q)
			n := copy(p, g.remaining)
			g.remaining = g.remaining[n:]
			return n, nil
		}
	}
}

// writeUnconditional allows unconditional writes to the underlying writers.
func (g *TermManager) writeUnconditional(p []byte) (int, error) {
	return g.writeToClients(p), nil
}

// BroadcastMessage injects a message into the stream.
func (g *TermManager) BroadcastMessage(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	data := []byte("Teleport > " + message + "\r\n")
	if g.lastWasBroadcast {
		data = append([]byte("\r\n"), data...)
	} else {
		g.lastWasBroadcast = true
	}
	_, err := g.writeUnconditional(data)
	return trace.Wrap(err)
}

// On allows data to flow through the manager.
func (g *TermManager) On() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.on = true
	g.readStateUpdate.Broadcast()
	_, err := g.writeUnconditional(g.buffer)
	return trace.Wrap(err)
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
	readerState := new(int32)
	g.readerState[name] = readerState

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
					if !g.on {
						select {
						case g.terminateNotifier <- true:
						default:
						}
					}
					g.mu.Unlock()
					return
				}
			}

			g.incoming <- buf[:n]
			if atomic.LoadInt32(g.closed) == 1 || atomic.LoadInt32(readerState) == 1 {
				return
			}
		}
	}()
}

func (g *TermManager) DeleteReader(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.readerState[name] != nil {
		atomic.StoreInt32(g.readerState[name], 1)
	}
}

func (g *TermManager) CountWritten() uint64 {
	return atomic.LoadUint64(&g.countWritten)
}

func (g *TermManager) CountRead() uint64 {
	return atomic.LoadUint64(&g.countRead)
}

func (g *TermManager) Close() {
	close(g.terminateNotifier)
	atomic.StoreInt32(g.closed, 1)
}

func (g *TermManager) GetRecentHistory() []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	data := make([]byte, len(g.history))
	data = append(data, g.history...)
	return data
}

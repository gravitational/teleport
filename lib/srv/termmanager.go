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

package srv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
)

// maxHistoryBytes is the maximum bytes that are retained as history and broadcasted to new clients.
const maxHistoryBytes = 1000

// maxPausedHistoryBytes is maximum bytes that are buffered when a session is paused.
const maxPausedHistoryBytes = 10000

// termState indicates the current state of the terminal
type termState int

const (
	// dataFlowOff indicates that data shouldn't be transmitted to clients
	dataFlowOff termState = iota
	// dataFlowOn indicates that data is allowed to be transmitted to clients
	dataFlowOn
)

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
	OnReadError  func(idString string, err error)
	OnWriteError func(idString string, err error)
	// buffer is used to buffer writes when turned off
	buffer []byte
	// state dictates whether data flow is permitted
	state termState
	// history is used to store the terminal history sent to new clients
	history []byte
	// incoming is a stream of incoming stdin data
	incoming chan []byte
	// remaining is a partially read chunk of stdin data
	// we only support one concurrent reader so this isn't mutex protected
	remaining []byte
	// stateUpdate signals that a state transition has occurred as a result
	// of calls to On/Off
	stateUpdate       chan struct{}
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
		stateUpdate:       make(chan struct{}, 1),
		incoming:          make(chan []byte, 100),
		terminateNotifier: make(chan struct{}, 1),
	}
}

// writeToClients writes to underlying clients
func (g *TermManager) writeToClients(p []byte) {
	g.lastWasBroadcast = false
	g.history = truncateFront(append(g.history, p...), maxHistoryBytes)

	atomic.AddUint64(&g.countWritten, uint64(len(p)))
	var toDelete []struct {
		key string
		err error
	}
	for key, w := range g.writers {
		_, err := w.Write(p)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				slog.WarnContext(context.Background(), "Failed to write to remote terminal", "error", err)
			}
			toDelete = append(
				toDelete, struct {
					key string
					err error
				}{key, err})

			delete(g.writers, key)
		}
	}

	// Let term manager decide how to handle broken party writers
	if g.OnWriteError != nil {
		// writeToClients is called with the lock held, so we need to release it
		// before calling OnWriteError to avoid a deadlock if OnWriteError
		// calls DeleteWriter/DeleteReader.
		g.mu.Unlock()
		for _, deleteWriter := range toDelete {
			g.OnWriteError(deleteWriter.key, deleteWriter.err)
		}
		g.mu.Lock()
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

	if g.state == dataFlowOn {
		g.writeToClients(p)
	} else {
		// Only keep the last maxPausedHistoryBytes of stdout/stderr while the session is paused.
		// The alternative is flushing to disk but this should be a pretty rare occurrence and shouldn't be an issue in practice.
		g.buffer = truncateFront(append(g.buffer, p...), maxPausedHistoryBytes)
	}

	return len(p), nil
}

func (g *TermManager) Read(p []byte) (int, error) {
	// wait until data flow is enabled and there is data to be read, or the session is terminated.
	for {
		g.mu.Lock()
		state := g.state
		closed := g.isClosed()
		switch {
		case closed:
			g.mu.Unlock()
			return 0, io.EOF
		case !closed && state == dataFlowOn && len(g.remaining) > 0:
			n := copy(p, g.remaining)
			g.remaining = g.remaining[n:]
			g.mu.Unlock()
			return n, nil
		}
		g.mu.Unlock()

		select {
		case <-g.closed:
			return 0, io.EOF
		case <-g.stateUpdate:
		case data := <-g.incoming:
			g.remaining = append(g.remaining, data...)
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
	g.state = dataFlowOn
	select {
	case g.stateUpdate <- struct{}{}:
	default:
	}
	g.writeToClients(g.buffer)
}

// Off buffers incoming writes and reads until turned on again.
func (g *TermManager) Off() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state = dataFlowOff
	select {
	case g.stateUpdate <- struct{}{}:
	default:
	}
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
	// AddReader is called by goroutines so we need to hold the lock.
	g.mu.Lock()
	g.readerState[name] = false
	g.mu.Unlock()

	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.WarnContext(context.Background(), "Failed to read from remote terminal", "error", err)
				}
				// Let term manager decide how to handle broken party readers.
				if g.OnReadError != nil {
					g.OnReadError(name, err)
				}
				g.DeleteReader(name)
				return
			}

			if slices.Contains(buf[:n], 0x03) {
				g.mu.Lock()
				if g.state == dataFlowOff && !g.isClosed() {
					select {
					case g.terminateNotifier <- struct{}{}:
					default:
					}
				}
				g.mu.Unlock()
			}

			g.mu.Lock()

			if g.state == dataFlowOn {
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

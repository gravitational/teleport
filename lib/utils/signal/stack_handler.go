/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package signal

import (
	"container/list"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Handler implements stack for context cancellation.
type Handler struct {
	mu   sync.Mutex
	list *list.List
}

var handler = &Handler{
	list: list.New(),
}

// GetSignalHandler returns global singleton instance of signal
func GetSignalHandler() *Handler {
	return handler
}

// NotifyContext creates context which going to be canceled after SIGINT, SIGTERM
// in order of adding them to the stack. When very first context is canceled
// we stop watching the OS signals.
func (s *Handler) NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.list.Len() == 0 {
		s.listenSignals()
	}

	ctx, cancel := context.WithCancel(parent)
	element := s.list.PushBack(cancel)

	return ctx, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.list.Remove(element)
		cancel()
	}
}

// listenSignals sets up the signal listener for SIGINT, SIGTERM.
func (s *Handler) listenSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for {
			if sig := <-sigChan; sig == nil {
				return
			}
			if !s.cancelNext() {
				signal.Stop(sigChan)
				return
			}
		}
	}()
}

// cancelNext calls the most recent cancel func in the stack.
func (s *Handler) cancelNext() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.list.Len() > 0 {
		cancel := s.list.Remove(s.list.Back())
		if cancel != nil {
			cancel.(context.CancelFunc)()
		}
	}

	return s.list.Len() != 0
}

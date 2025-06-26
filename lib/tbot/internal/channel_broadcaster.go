/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package internal

import "sync"

// NewChannelBroadcaster creates a ChannelBroadcaster.
func NewChannelBroadcaster() *ChannelBroadcaster {
	return &ChannelBroadcaster{
		chanSet: map[chan struct{}]struct{}{},
	}
}

// ChannelBroadcaster implements a simple pub/sub system for notifying services
// of an event (e.g. a CA rotation).
type ChannelBroadcaster struct {
	mu      sync.Mutex
	chanSet map[chan struct{}]struct{}
}

// Subscribe returns a channel you can receive events from. You must call the
// unsubscribe function to free resources when you no longer want to receive
// notifications.
func (cb *ChannelBroadcaster) Subscribe() (<-chan struct{}, func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	ch := make(chan struct{}, 1)
	cb.chanSet[ch] = struct{}{}

	// Returns a function that should be called to unsubscribe the channel
	return ch, func() {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		_, ok := cb.chanSet[ch]
		if ok {
			delete(cb.chanSet, ch)
			close(ch)
		}
	}
}

// Broadcast a notification to all subscribers.
func (cb *ChannelBroadcaster) Broadcast() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	for ch := range cb.chanSet {
		select {
		case ch <- struct{}{}:
			// Successfully sent notification
		default:
			// Channel already has valued queued
		}
	}
}

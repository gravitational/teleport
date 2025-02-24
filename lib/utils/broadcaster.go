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

package utils

import (
	"sync"
)

// NewCloseBroadcaster returns new instance of close broadcaster
func NewCloseBroadcaster() *CloseBroadcaster {
	return &CloseBroadcaster{
		C: make(chan struct{}),
	}
}

// CloseBroadcaster is a helper struct
// that implements io.Closer and uses channel
// to broadcast its closed state once called
type CloseBroadcaster struct {
	sync.Once
	C chan struct{}
}

// Close closes channel (once) to start broadcasting its closed state
func (b *CloseBroadcaster) Close() error {
	b.Do(func() {
		close(b.C)
	})
	return nil
}
